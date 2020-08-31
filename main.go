/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint
	"os"
	"time"

	// +kubebuilder:scaffold:imports

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cgrecord "k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/klog/klogr"

	capifeature "sigs.k8s.io/cluster-api/feature"

	infrav1alpha2 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1alpha3exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	infrav1controllersexp "sigs.k8s.io/cluster-api-provider-azure/exp/controllers"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	version "sigs.k8s.io/cluster-api-provider-azure/version"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"sigs.k8s.io/cluster-api-provider-azure/feature"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	klog.InitFlags(nil)

	_ = clientgoscheme.AddToScheme(scheme)
	_ = infrav1alpha2.AddToScheme(scheme)
	_ = infrav1alpha3.AddToScheme(scheme)
	_ = infrav1alpha3exp.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = clusterv1exp.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

var (
	metricsAddr                 string
	enableLeaderElection        bool
	leaderElectionNamespace     string
	watchNamespace              string
	profilerAddress             string
	azureClusterConcurrency     int
	azureMachineConcurrency     int
	azureMachinePoolConcurrency int
	syncPeriod                  time.Duration
	healthAddr                  string
	webhookPort                 int
	reconcileTimeout            time.Duration
)

func InitFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&metricsAddr,
		"metrics-addr",
		":8080",
		"The address the metric endpoint binds to.",
	)

	fs.BoolVar(
		&enableLeaderElection,
		"enable-leader-election",
		false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)

	flag.StringVar(
		&leaderElectionNamespace,
		"leader-election-namespace",
		"",
		"Namespace that the controller performs leader election in. If unspecified, the controller will discover which namespace it is running in.",
	)

	fs.StringVar(
		&watchNamespace,
		"namespace",
		"",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.",
	)

	fs.StringVar(
		&profilerAddress,
		"profiler-address",
		"",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)",
	)

	fs.IntVar(&azureClusterConcurrency,
		"azurecluster-concurrency",
		10,
		"Number of AzureClusters to process simultaneously",
	)

	fs.IntVar(&azureMachineConcurrency,
		"azuremachine-concurrency",
		10,
		"Number of AzureMachines to process simultaneously",
	)

	fs.IntVar(&azureMachinePoolConcurrency,
		"azuremachinepool-concurrency",
		10,
		"Number of AzureMachinePools to process simultaneously")

	fs.DurationVar(&syncPeriod,
		"sync-period",
		10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)",
	)

	fs.StringVar(&healthAddr,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)

	fs.IntVar(&webhookPort,
		"webhook-port",
		0,
		"Webhook Server port, disabled by default. When enabled, the manager will only work as webhook server, no reconcilers are installed.",
	)

	fs.DurationVar(&reconcileTimeout,
		"reconcile-timeout",
		reconciler.DefaultLoopTimeout,
		"The maximum duration a reconcile loop can run (e.g. 90m)",
	)

	feature.MutableGates.AddFlag(fs)
}

func main() {
	InitFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	if watchNamespace != "" {
		setupLog.Info("Watching cluster-api objects only in namespace for reconciliation", "namespace", watchNamespace)
	}

	if profilerAddress != "" {
		setupLog.Info("Profiler listening for requests", "profiler-address", profilerAddress)
		go func() {
			setupLog.Error(http.ListenAndServe(profilerAddress, nil), "listen and serve error")
		}()
	}

	ctrl.SetLogger(klogr.New())

	// Machine and cluster operations can create enough events to trigger the event recorder spam filter
	// Setting the burst size higher ensures all events will be recorded and submitted to the API
	broadcaster := cgrecord.NewBroadcasterWithCorrelatorOptions(cgrecord.CorrelatorOptions{
		BurstSize: 100,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "controller-leader-election-capz",
		LeaderElectionNamespace: leaderElectionNamespace,
		SyncPeriod:              &syncPeriod,
		Namespace:               watchNamespace,
		HealthProbeBindAddress:  healthAddr,
		Port:                    webhookPort,
		EventBroadcaster:        broadcaster,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("azure-controller"))

	if webhookPort == 0 {
		if err = (&controllers.AzureMachineReconciler{
			Client:           mgr.GetClient(),
			Log:              ctrl.Log.WithName("controllers").WithName("AzureMachine"),
			Recorder:         mgr.GetEventRecorderFor("azuremachine-reconciler"),
			ReconcileTimeout: reconcileTimeout,
		}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureMachineConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "AzureMachine")
			os.Exit(1)
		}
		if err = (&controllers.AzureClusterReconciler{
			Client:           mgr.GetClient(),
			Log:              ctrl.Log.WithName("controllers").WithName("AzureCluster"),
			Recorder:         mgr.GetEventRecorderFor("azurecluster-reconciler"),
			ReconcileTimeout: reconcileTimeout,
		}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureClusterConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "AzureCluster")
			os.Exit(1)
		}
		if err = (&controllers.AzureJSONTemplateReconciler{
			Client:           mgr.GetClient(),
			Log:              ctrl.Log.WithName("controllers").WithName("AzureJSONTemplate"),
			Recorder:         mgr.GetEventRecorderFor("azurejsontemplate-reconciler"),
			ReconcileTimeout: reconcileTimeout,
		}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureMachineConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "AzureJSONTemplate")
			os.Exit(1)
		}
		if err = (&controllers.AzureJSONMachineReconciler{
			Client:           mgr.GetClient(),
			Log:              ctrl.Log.WithName("controllers").WithName("AzureJSONMachine"),
			Recorder:         mgr.GetEventRecorderFor("azurejsonmachine-reconciler"),
			ReconcileTimeout: reconcileTimeout,
		}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureMachineConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "AzureJSONMachine")
			os.Exit(1)
		}
		// just use CAPI MachinePool feature flag rather than create a new one
		setupLog.V(1).Info(fmt.Sprintf("%+v\n", feature.Gates))
		if feature.Gates.Enabled(capifeature.MachinePool) {
			if err = (&infrav1controllersexp.AzureMachinePoolReconciler{
				Client:           mgr.GetClient(),
				Log:              ctrl.Log.WithName("controllers").WithName("AzureMachinePool"),
				Recorder:         mgr.GetEventRecorderFor("azurecluster-reconciler"),
				ReconcileTimeout: reconcileTimeout,
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureMachinePoolConcurrency}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "AzureMachinePool")
				os.Exit(1)
			}
			if err = (&controllers.AzureJSONMachinePoolReconciler{
				Client:           mgr.GetClient(),
				Log:              ctrl.Log.WithName("controllers").WithName("AzureJSONMachinePool"),
				Recorder:         mgr.GetEventRecorderFor("azurejsonmachinepool-reconciler"),
				ReconcileTimeout: reconcileTimeout,
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureMachinePoolConcurrency}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "AzureJSONMachinePool")
				os.Exit(1)
			}
			if feature.Gates.Enabled(feature.AKS) {
				if err = (&infrav1controllersexp.AzureManagedMachinePoolReconciler{
					Client:           mgr.GetClient(),
					Log:              ctrl.Log.WithName("controllers").WithName("AzureManagedMachinePool"),
					Recorder:         mgr.GetEventRecorderFor("azuremachine-reconciler"),
					ReconcileTimeout: reconcileTimeout,
				}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureMachineConcurrency}); err != nil {
					setupLog.Error(err, "unable to create controller", "controller", "AzureManagedMachinePool")
					os.Exit(1)
				}
				if err = (&infrav1controllersexp.AzureManagedClusterReconciler{
					Client:           mgr.GetClient(),
					Log:              ctrl.Log.WithName("controllers").WithName("AzureManagedCluster"),
					Recorder:         mgr.GetEventRecorderFor("azuremanagedcluster-reconciler"),
					ReconcileTimeout: reconcileTimeout,
				}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureClusterConcurrency}); err != nil {
					setupLog.Error(err, "unable to create controller", "controller", "AzureManagedCluster")
					os.Exit(1)
				}
				if err = (&infrav1controllersexp.AzureManagedControlPlaneReconciler{
					Client:           mgr.GetClient(),
					Log:              ctrl.Log.WithName("controllers").WithName("AzureManagedControlPlane"),
					Recorder:         mgr.GetEventRecorderFor("azuremanagedcontrolplane-reconciler"),
					ReconcileTimeout: reconcileTimeout,
				}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: azureClusterConcurrency}); err != nil {
					setupLog.Error(err, "unable to create controller", "controller", "AzureManagedControlPlane")
					os.Exit(1)
				}
			}
		}
	} else {
		if err = (&infrav1alpha3.AzureCluster{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "AzureCluster")
			os.Exit(1)
		}
		if err = (&infrav1alpha3.AzureMachine{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "AzureMachine")
			os.Exit(1)
		}
		if err = (&infrav1alpha3.AzureMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "AzureMachineTemplate")
			os.Exit(1)
		}
		// just use CAPI MachinePool feature flag rather than create a new one
		if feature.Gates.Enabled(capifeature.MachinePool) {
			if err = (&infrav1alpha3exp.AzureMachinePool{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "AzureMachinePool")
				os.Exit(1)
			}
		}
		if feature.Gates.Enabled(feature.AKS) {
			if err = (&infrav1alpha3exp.AzureManagedControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "AzureManagedControlPlane")
				os.Exit(1)
			}
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "version", version.Get().String())
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
