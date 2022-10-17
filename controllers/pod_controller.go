package controllers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/hashicorp/vault/api"
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
	vaults map[string]*vaultInfo
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

	addr := getVaultAddress(ctx, pod)
	cl, err := r.newClient(addr)
	if err != nil {
		return reconcile.Result{}, err
	}
	st, err := cl.Sys().SealStatusWithContext(ctx)
	if err != nil {
		l.Error(err, "Error checking seal status")
		return reconcile.Result{}, err
	}

	if !st.Initialized {
		l.Info("vault is not initialized")
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}

	vault := r.vaults[getOwner(pod)]
	if vault == nil {
		return reconcile.Result{}, nil
	}

	if st.Sealed {
		if len(vault.unsealKeys) == 0 {
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		}

		if err := r.unseal(ctx, cl, vault); err != nil {
			return reconcile.Result{}, err
		}
		l.Info("successfully unsealed vault")

	} else if len(vault.unsealKeys) == 0 {
		t, err := userpassLogin(cl, vault.username, vault.password)
		if err != nil {
			return reconcile.Result{}, err
		}
		cl.SetToken(t)

		if err := readSecret(cl, vault); err != nil {
			return reconcile.Result{}, err
		}
		l.WithValues("count", len(vault.unsealKeys)).Info("successfully read unseal keys from vault")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager, secrets []corev1.Secret) error {
	r.vaults = make(map[string]*vaultInfo)
	for _, s := range secrets {
		v := &vaultInfo{
			username: string(s.Data[constants.KeyUsername]),
			password: string(s.Data[constants.KeyPassword]),
			owner:    s.GetLabels()[constants.LabelStatefulSetName],
		}
		if p, ok := s.Data[constants.KeySecretPath]; ok {
			v.secretPath = string(p)
		} else {
			v.secretPath = constants.DefaultSecretPath
		}

		for key, val := range s.Data {
			if strings.HasPrefix(key, constants.KeyPrefixUnsealKey) {
				v.unsealKeys = append(v.unsealKeys, string(val))
			}
		}

		r.vaults[v.owner] = v
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(r).
		Complete(r)
}

func (r *PodReconciler) unseal(ctx context.Context, cl *api.Client, vault *vaultInfo) error {
	for _, key := range vault.unsealKeys {
		resp, err := cl.Sys().UnsealWithContext(ctx, key)
		if err != nil {
			return err
		}
		if !resp.Sealed {
			return nil
		}
	}
	return errors.New("could not unseal vault")
}

type vaultInfo struct {
	owner      string
	username   string
	password   string
	unsealKeys []string
	secretPath string
}
