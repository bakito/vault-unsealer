package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/bakito/vault-unsealer/controllers"
	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/hierarchy"
	"github.com/bakito/vault-unsealer/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
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

	addrEnvVarName     string
	vaultContainerName string
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
	flag.StringVar(
		&vaultContainerName,
		"container-name",
		"",
		fmt.Sprintf(
			"Override the vault container name. Defaults to (%s | %s).",
			constants.ContainerNameVault,
			constants.ContainerNameOpenbao,
		),
	)
	flag.StringVar(
		&addrEnvVarName,
		"address-env-var-name",
		"",
		fmt.Sprintf(
			"Override the vault|openbao address env variable. Defaults to (%s for vault | %s for openbao).",
			constants.EnvVaultAddr,
			constants.EnvBaoAddr,
		),
	)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	logging.SetupLogger(true)
	podNamespace := os.Getenv(constants.EnvNamespace)

	cfg := ctrl.GetConfigOrDie()

	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(cfg)
	versionInfo, err := discoveryClient.ServerVersion()
	if err != nil {
		setupLog.Error(err, "unable to get kubernetes version")
		os.Exit(1)
	}

	minor, err := strconv.Atoi(versionInfo.Minor)
	if err != nil {
		setupLog.Error(err, "unable to parse kubernetes version")
		os.Exit(1)
	}

	past132 := minor >= 33

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Cache: crtlcache.Options{
			DefaultNamespaces: map[string]crtlcache.Config{
				podNamespace: {},
			},
		},
		Metrics: server.Options{
			BindAddress: ":8080",
		},
		WebhookServer:                 webhook.NewServer(webhook.Options{Port: 9443}),
		HealthProbeBindAddress:        ":8081",
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              constants.OperatorID,
		LeaderElectionNamespace:       podNamespace,
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := context.TODO()
	var c cache.Cache
	if enableSharedCache {
		k8sCache, err := cache.NewK8s(mgr.GetAPIReader(), past132)
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
		c = cache.NewSimple(past132)
	}
	run(ctx, mgr, podNamespace, c)
}

func run(ctx context.Context, mgr manager.Manager, podNamespace string, c cache.Cache) {
	secretsStatefulSet := &corev1.SecretList{}
	if err := mgr.GetAPIReader().List(
		ctx,
		secretsStatefulSet,
		client.HasLabels{constants.LabelStatefulSetName},
		client.InNamespace(podNamespace),
	); err != nil {
		setupLog.Error(err, "unable to find secrets statefulset")
		os.Exit(1)
	}
	setupLog.WithValues("secrets", len(secretsStatefulSet.Items)).Info("found unseal secrets statefulset")

	secretsExternal := &corev1.SecretList{}
	if err := mgr.GetAPIReader().List(
		ctx,
		secretsExternal,
		client.HasLabels{constants.LabelExternal},
		client.InNamespace(podNamespace),
	); err != nil {
		setupLog.Error(err, "unable to find secrets external")
		os.Exit(1)
	}
	setupLog.WithValues("secrets", len(secretsExternal.Items)).Info("found unseal secrets external")

	sel, err := hierarchy.GetDeploymentSelector(ctx, mgr.GetAPIReader())
	if err != nil {
		setupLog.Error(err, "unable to find deployment of unsealer")
		os.Exit(1)
	}

	if err := (&controllers.EndpointsReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		Cache:            c,
		UnsealerSelector: sel,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Endpoint")
		os.Exit(1)
	}
	if err := (&controllers.PodReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Cache:              c,
		VaultContainerName: vaultContainerName,
		AddrEnvVarName:     addrEnvVarName,
	}).SetupWithManager(mgr, secretsStatefulSet.Items); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pod")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := (&controllers.ExternalHandler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Cache:  c,
	}).SetupWithManager(mgr, secretsExternal.Items); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "External")
		os.Exit(1)
	}

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
