package cluster_controller

import (
	"os"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/apiserver-builder/pkg/controller"
	azure "github.com/platform9/azure-provider/cloud/azure/actuators/cluster"
	"github.com/platform9/azure-provider/cloud/azure/controllers"
	"github.com/platform9/azure-provider/cloud/azure/controllers/cluster/options"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	clusterapiclientsetscheme "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/scheme"
	"sigs.k8s.io/cluster-api/pkg/controller/cluster"
	"sigs.k8s.io/cluster-api/pkg/controller/config"
	"sigs.k8s.io/cluster-api/pkg/controller/sharedinformers"
)

const (
	azureClusterControllerName = "azure-controller"
)

func StartClusterController(server *options.ClusterControllerServer, shutdown <-chan struct{}) {
	config, err := controller.GetConfig(server.CommonConfig.Kubeconfig)
	if err != nil {
		glog.Fatalf("Could not create Config for talking to the apiserver: %v", err)
	}

	client, err := clientset.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Could not create client for talking to the apiserver: %v", err)
	}

	// Load the environment variables needed for azure clients
	if err := controllers.PrepareEnvironment(); err != nil {
		glog.Fatalf("Could not prepare environment for azure cluster actuator: %v", err)
	}

	params := azure.ClusterActuatorParams{
		ClusterClient: client.ClusterV1alpha1().Clusters(corev1.NamespaceDefault),
	}
	actuator, err := azure.NewClusterActuator(params)
	if err != nil {
		glog.Fatalf("Could not create azure cluster actuator: %v", err)
	}

	si := sharedinformers.NewSharedInformers(config, shutdown)
	// If this doesn't compile, the code generator probably
	// overwrote the customized NewClusterController function.
	c := cluster.NewClusterController(config, si, actuator)
	c.RunAsync(shutdown)

	select {}
}

func RunClusterController(server *options.ClusterControllerServer) error {
	kubeConfig, err := controller.GetConfig(server.CommonConfig.Kubeconfig)
	if err != nil {
		glog.Errorf("Could not create Config for talking to the apiserver: %v", err)
		return err
	}

	kubeClientControl, err := kubernetes.NewForConfig(
		rest.AddUserAgent(kubeConfig, "cluster-controller-manager"),
	)
	if err != nil {
		glog.Errorf("Invalid API configuration for kubeconfig-control: %v", err)
		return err
	}

	recorder, err := createRecorder(kubeClientControl)
	if err != nil {
		glog.Errorf("Could not create event recorder : %v", err)
		return err
	}

	// run function will block and never return.
	run := func(stop <-chan struct{}) {
		StartClusterController(server, stop)
	}

	leaderElectConfig := config.GetLeaderElectionConfig()
	if !leaderElectConfig.LeaderElect {
		run(make(<-chan (struct{})))
	}

	// Identity used to distinguish between multiple cluster controller instances.
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	leaderElectionClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(kubeConfig, "cluster-leader-election"))

	id = id + "-" + string(uuid.NewUUID())
	// Lock required for leader election
	rl, err := resourcelock.New(
		leaderElectConfig.ResourceLock,
		metav1.NamespaceSystem,
		azureClusterControllerName,
		leaderElectionClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id + "-" + azureClusterControllerName,
			EventRecorder: recorder,
		})
	if err != nil {
		return err
	}

	// Try and become the leader and start cluster controller loops
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: leaderElectConfig.LeaseDuration.Duration,
		RenewDeadline: leaderElectConfig.RenewDeadline.Duration,
		RetryPeriod:   leaderElectConfig.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	})
	panic("unreachable")
}

func createRecorder(kubeClient *kubernetes.Clientset) (record.EventRecorder, error) {

	eventsScheme := runtime.NewScheme()
	if err := corev1.AddToScheme(eventsScheme); err != nil {
		return nil, err
	}
	// We also emit events for our own types
	clusterapiclientsetscheme.AddToScheme(eventsScheme)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(eventsScheme, corev1.EventSource{Component: azureClusterControllerName}), nil
}
