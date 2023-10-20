package controllers

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/go-logr/logr"
	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Cache            cache.Cache
	UnsealerSelector labels.Selector
	myPodName        string
}

//+kubebuilder:rbac:groups=.com,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=,resources=pods/finalizers,verbs=update

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	pod := &corev1.Pod{}
	err := r.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the req.
		l.Error(err, "Error reading namespace")
		return reconcile.Result{}, err
	}

	if r.isUnsealer(pod) {
		return r.reconcileUnsealerPod(ctx, l, pod)
	}

	return r.reconcileVaultPod(ctx, l, pod)
}

func (r *PodReconciler) reconcileUnsealerPod(_ context.Context, _ logr.Logger, pod *corev1.Pod) (ctrl.Result, error) {
	if pod.GetName() != r.myPodName {
		if pod.Status.Phase == corev1.PodRunning {
			r.Cache.AddMember(pod.Status.PodIP)
		} else if pod.DeletionTimestamp == nil {
			r.Cache.RemoveMember(pod.Status.PodIP)
		}

		r.Cache.Sync()
	}
	return ctrl.Result{}, nil
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

	vault := r.Cache.VaultInfoFor(getOwner(pod))
	if vault == nil {
		return reconcile.Result{}, nil
	}

	if st.Data.Sealed {
		if len(vault.UnsealKeys) == 0 {
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		}

		if err := r.unseal(ctx, cl, vault); err != nil {
			return reconcile.Result{}, err
		}
		l.Info("successfully unsealed vault")

	} else if len(vault.UnsealKeys) == 0 {
		t, err := userpassLogin(ctx, cl, vault.Username, vault.Password)
		if err != nil {
			return reconcile.Result{}, err
		}
		err = cl.SetToken(t)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err := readSecret(ctx, cl, vault); err != nil {
			return reconcile.Result{}, err
		}

		r.Cache.SetVaultInfoFor(vault.Owner, vault)
		l.WithValues("keys", len(vault.UnsealKeys)).Info("successfully read unseal keys from vault")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager, secrets []corev1.Secret, depl *appsv1.Deployment) error {
	for _, s := range secrets {
		owner := s.GetLabels()[constants.LabelStatefulSetName]
		if r.Cache.VaultInfoFor(owner) == nil {
			v := &types.VaultInfo{
				Username: string(s.Data[constants.KeyUsername]),
				Password: string(s.Data[constants.KeyPassword]),
				Owner:    owner,
			}
			if p, ok := s.Data[constants.KeySecretPath]; ok {
				v.SecretPath = string(p)
			} else {
				v.SecretPath = constants.DefaultSecretPath
			}

			for key, val := range s.Data {
				if strings.HasPrefix(key, constants.KeyPrefixUnsealKey) {
					v.UnsealKeys = append(v.UnsealKeys, string(val))
				}
			}

			r.Cache.SetVaultInfoFor(owner, v)
		}
	}

	sel, err := metav1.LabelSelectorAsSelector(depl.Spec.Selector)
	if err != nil {
		return err
	}
	r.UnsealerSelector = sel
	r.myPodName = os.Getenv("HOSTNAME")

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(r).
		Complete(r)
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
