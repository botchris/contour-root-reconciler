package main

import (
	"flag"
	"os"

	"github.com/botchris/contour-root-reconciler/cmd/controller/internal"
	projcontour "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("http-proxy-reconciler")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme)) // Core Kubernetes types
	utilruntime.Must(projcontour.AddToScheme(scheme))    // Contour HTTPProxy types
}

func main() {
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Metrics endpoint bind address.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Health probe bind address.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for high availability.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "child-http-proxy-controller",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconciler := internal.NewChildReconciler(mgr.GetClient())
	if eErr := reconciler.SetupWithManager(mgr); eErr != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ChildReconciler")
		os.Exit(1)
	}

	if aErr := mgr.AddHealthzCheck("healthz", healthz.Ping); aErr != nil {
		setupLog.Error(aErr, "unable to set up health check")
		os.Exit(1)
	}

	if aErr := mgr.AddReadyzCheck("readyz", healthz.Ping); aErr != nil {
		setupLog.Error(aErr, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")

	if aErr := mgr.Start(ctrl.SetupSignalHandler()); aErr != nil {
		setupLog.Error(aErr, "problem running manager")
		os.Exit(1)
	}
}
