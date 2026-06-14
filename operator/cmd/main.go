package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/controller"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(workloadv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr, watchNamespace string
	var devMode bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "metrics endpoint")
	flag.StringVar(&watchNamespace, "namespace", "", "namespace to watch (empty = all)")
	flag.BoolVar(&devMode, "dev", false, "enable development-mode logging (human-friendly, not for production)")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(devMode)))
	setupLog := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), buildManagerOptions(scheme, metricsAddr, watchNamespace))
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err = (&controller.WorkloadReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Workload")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "watchNamespace", watchNamespace)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager exited with error")
		os.Exit(1)
	}
}

// buildManagerOptions is extracted so the namespace-scoping is unit-testable WITHOUT starting a
// manager. When watchNamespace != "" the cache is restricted to that single namespace
// (Cache.DefaultNamespaces), which is what makes the controller namespace-scoped.
func buildManagerOptions(scheme *runtime.Scheme, metricsAddr, watchNamespace string) ctrl.Options {
	options := ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: metricsAddr},
	}
	if watchNamespace != "" {
		options.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{watchNamespace: {}},
		}
	}
	return options
}
