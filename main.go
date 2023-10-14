package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"strings"

	"github.com/bakito/vault-unsealer/controllers"
	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	crtlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var enableLeaderElection bool
	var enableSharedCache bool
	deploymentSelector := selector{}
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableSharedCache, "shared-cache", false, "Enable shared cache between the operator instances.")
	flag.Var(&deploymentSelector, "deployment-selector", "Label selector to evaluate other pods of the same deployment")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	logging.PrepareLogger(true)
	watchNamespace := os.Getenv(constants.EnvWatchNamespace)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Cache: crtlcache.Options{
			DefaultNamespaces: map[string]crtlcache.Config{
				watchNamespace: {},
			},
		},
		Metrics: server.Options{
			BindAddress: ":8080",
		},
		WebhookServer:           webhook.NewServer(webhook.Options{Port: 9443}),
		HealthProbeBindAddress:  ":8081",
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        constants.OperatorID,
		LeaderElectionNamespace: watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := context.TODO()

	if enableSharedCache {
		if len(deploymentSelector) == 0 {
			setupLog.Error(nil, "deployment selector is needed for shared cache")
			os.Exit(1)
		}

		_, members, err := cache.FindMemberPodIPs(ctx, mgr, watchNamespace, deploymentSelector)
		if err != nil {
			setupLog.Error(err, "unable to find operator pods")
			os.Exit(1)
		}

		c, err := cache.NewK8s(mgr.GetAPIReader(), members)
		if err != nil {
			setupLog.Error(err, "unable to setup cache")
			os.Exit(1)
		}
		go run(ctx, mgr, watchNamespace, c)

		if err = c.Start(ctx); err != nil {
			setupLog.Error(err, "unable to start cache")
			os.Exit(1)
		}
	} else {
		run(ctx, mgr, watchNamespace, cache.NewSimple())
	}
}

func run(ctx context.Context, mgr manager.Manager, watchNamespace string, cache cache.Cache) {
	secrets := &corev1.SecretList{}
	if err := mgr.GetAPIReader().List(
		ctx,
		secrets,
		client.HasLabels{constants.LabelStatefulSetName},
		client.InNamespace(watchNamespace),
	); err != nil {
		setupLog.Error(err, "unable to find secrets")
		os.Exit(1)
	}
	setupLog.WithValues("secrets", len(secrets.Items)).Info("found unseal secrets")

	if err := (&controllers.PodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Cache:  cache,
	}).SetupWithManager(mgr, secrets.Items); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pod")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

type selector map[string]string

func (i selector) String() string {
	return "selector labels"
}

func (i selector) Set(value string) error {
	parts := strings.Split(value, ":")
	if len(parts) > 2 {
		return fmt.Errorf("invalid selector %q", value)
	}
	i[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	return nil
}
