package main

import (
	"context"
	"flag"
	"os"

	"github.com/bakito/vault-unsealer/controllers"
	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/hierarchy"
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
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
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
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableSharedCache, "shared-cache", false, "Enable shared cache between the operator instances.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	logging.SetupLogger(true)
	podNamespace := os.Getenv(constants.EnvNamespace)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Cache: crtlcache.Options{
			DefaultNamespaces: map[string]crtlcache.Config{
				podNamespace: {},
			},
		},
		Metrics: server.Options{
			BindAddress: ":8080",
		},
		WebhookServer:           webhook.NewServer(webhook.Options{Port: 9443}),
		HealthProbeBindAddress:  ":8081",
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        constants.OperatorID,
		LeaderElectionNamespace: podNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := context.TODO()
	var c cache.Cache
	if enableSharedCache {
		k8sCache, err := cache.NewK8s(mgr.GetAPIReader())
		if err != nil {
			setupLog.Error(err, "unable to create cache")
			os.Exit(1)
		}

		if err := k8sCache.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup cache")
			os.Exit(1)
		}
		c = k8sCache
	} else {
		c = cache.NewSimple()
	}
	run(ctx, mgr, podNamespace, c)
}

func run(ctx context.Context, mgr manager.Manager, podNamespace string, cache cache.Cache) {
	secrets := &corev1.SecretList{}
	if err := mgr.GetAPIReader().List(
		ctx,
		secrets,
		client.HasLabels{constants.LabelStatefulSetName},
		client.InNamespace(podNamespace),
	); err != nil {
		setupLog.Error(err, "unable to find secrets")
		os.Exit(1)
	}
	setupLog.WithValues("secrets", len(secrets.Items)).Info("found unseal secrets")

	sel, err := hierarchy.GetDeploymentSelector(ctx, mgr.GetAPIReader())
	if err != nil {
		setupLog.Error(err, "unable to find deployment of unsealer")
		os.Exit(1)
	}

	if err := (&controllers.EndpintsReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		Cache:            cache,
		UnsealerSelector: sel,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Endpoint")
		os.Exit(1)
	}
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
