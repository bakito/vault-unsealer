package controllers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/go-logr/logr"
	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  cache.Cache
}

//+kubebuilder:rbac:groups=,resources=pods;secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=,resources=pods/status,verbs=get

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	pod := &corev1.Pod{}
	err := r.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the req.
		l.Error(err, "Error reading pod")
		return reconcile.Result{}, err
	}

	return r.reconcileVaultPod(ctx, l, pod)
}

func (r *PodReconciler) reconcileVaultPod(ctx context.Context, l logr.Logger, pod *corev1.Pod) (ctrl.Result, error) {
	addr := getVaultAddress(ctx, pod)
	cl, err := r.newClient(addr)
	if err != nil {
		return reconcile.Result{}, err
	}
	st, err := cl.System.SealStatus(ctx)
	if err != nil {
		l.Error(err, "Error checking seal status")
		return reconcile.Result{}, err
	}

	if !st.Data.Initialized {
		l.Info("vault is not initialized")
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}

	vi := r.Cache.VaultInfoFor(getOwner(pod))
	if vi == nil {
		return reconcile.Result{}, nil
	}

	if st.Data.Sealed {
		if len(vi.UnsealKeys) == 0 {
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		}

		if err := r.unseal(ctx, cl, vi); err != nil {
			return reconcile.Result{}, err
		}
		l.Info("successfully unsealed vault")

	} else if len(vi.UnsealKeys) == 0 {
		var token string
		var method string
		if len(vi.Username) != 0 && len(vi.Password) != 0 {
			method = "userpass"
			token, err = userPassLogin(ctx, cl, vi.Username, vi.Password)
		} else if len(strings.TrimSpace(vi.Role)) != 0 {
			method = "kubernetes"
			token, err = kubernetesLogin(ctx, cl, vi.Role)
		}
		if err != nil {
			return reconcile.Result{}, err
		}
		if token == "" {
			return reconcile.Result{Requeue: false}, errors.New("no supported auth method is used")
		}
		err = cl.SetToken(token)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err := readUnsealKeys(ctx, cl, vi); err != nil {
			return reconcile.Result{}, err
		}

		r.Cache.SetVaultInfoFor(vi.Owner, vi)
		l.WithValues("keys", len(vi.UnsealKeys), "method", method).
			Info("successfully read unseal keys from vault")
	}

	return ctrl.Result{}, nil
}

func (r *PodReconciler) unseal(ctx context.Context, cl *vault.Client, vault *types.VaultInfo) error {
	for _, key := range vault.UnsealKeys {
		resp, err := cl.System.Unseal(ctx, schema.UnsealRequest{Key: key})
		if err != nil {
			return err
		}
		if !resp.Data.Sealed {
			return nil
		}
	}
	return errors.New("could not unseal vault")
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager, secrets []corev1.Secret) error {
	for _, s := range secrets {
		owner := s.GetLabels()[constants.LabelStatefulSetName]
		if r.Cache.VaultInfoFor(owner) == nil {
			v := &types.VaultInfo{
				Username:   string(s.Data[constants.KeyUsername]),
				Password:   string(s.Data[constants.KeyPassword]),
				Role:       string(s.Data[constants.KeyRole]),
				SecretPath: string(s.Data[constants.KeySecretPath]),
				Owner:      owner,
			}

			for key, val := range s.Data {
				if strings.HasPrefix(key, constants.KeyPrefixUnsealKey) {
					v.UnsealKeys = append(v.UnsealKeys, string(val))
				}
			}

			r.Cache.SetVaultInfoFor(owner, v)
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(r).
		Complete(r)
}
