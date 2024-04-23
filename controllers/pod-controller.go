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

// Reconcile reconciles the Pod object.
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

	// Perform reconciliation logic for the Vault Pod.
	return r.reconcileVaultPod(ctx, l, pod)
}

// reconcileVaultPod reconciles a Vault Pod.
func (r *PodReconciler) reconcileVaultPod(ctx context.Context, l logr.Logger, pod *corev1.Pod) (ctrl.Result, error) {
	// Get the address of the Vault server.
	addr := getVaultAddress(ctx, pod)
	cl, err := r.newClient(addr)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Check the seal status of the Vault server.
	st, err := cl.System.SealStatus(ctx)
	if err != nil {
		l.Error(err, "Error checking seal status")
		return reconcile.Result{}, err
	}

	// If the Vault server is not initialized, requeue after 10 seconds.
	if !st.Data.Initialized {
		l.Info("vault is not initialized")
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}

	// Get the VaultInfo for the StatefulSet associated with the Pod.
	vi := r.Cache.VaultInfoFor(getStatefulSetFor(pod))
	if vi == nil {
		return reconcile.Result{}, nil
	}

	vaultLog := ctrl.Log.WithName("vault").WithValues(
		"namespace", pod.GetNamespace(),
		"pod", pod.GetName(),
		"stateful-set", vi.StatefulSet,
	)

	// If the Vault server is sealed, unseal it.
	if st.Data.Sealed {
		if len(vi.UnsealKeys) == 0 {
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		}
		if err := r.unseal(ctx, cl, vi); err != nil {
			return reconcile.Result{}, err
		}
		vaultLog.Info("successfully unsealed vault")

		// If the Vault server is unsealed and there are no unseal keys, authenticate.
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

		r.Cache.SetVaultInfoFor(vi.StatefulSet, vi)
		vaultLog.WithValues("keys", len(vi.UnsealKeys), "method", method).
			Info("successfully read unseal keys from vault")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager, secrets []corev1.Secret) error {
	// Populate the cache with VaultInfo from the provided Secrets.
	for _, s := range secrets {
		statefulSet := s.GetLabels()[constants.LabelStatefulSetName]
		if r.Cache.VaultInfoFor(statefulSet) == nil {
			v := &types.VaultInfo{
				Username:    string(s.Data[constants.KeyUsername]),
				Password:    string(s.Data[constants.KeyPassword]),
				Role:        string(s.Data[constants.KeyRole]),
				SecretPath:  string(s.Data[constants.KeySecretPath]),
				StatefulSet: statefulSet,
			}

			for key, val := range s.Data {
				if strings.HasPrefix(key, constants.KeyPrefixUnsealKey) {
					v.UnsealKeys = append(v.UnsealKeys, string(val))
				}
			}

			r.Cache.SetVaultInfoFor(statefulSet, v)
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(r).
		Complete(r)
}
