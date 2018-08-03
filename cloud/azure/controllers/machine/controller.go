package machine_controller

import (
	"os"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/apiserver-builder/pkg/controller"
	azure "github.com/platform9/azure-provider/cloud/azure/actuators/machine"
	"github.com/platform9/azure-provider/cloud/azure/controllers/machine/options"
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
	"sigs.k8s.io/cluster-api/pkg/controller/config"
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
	"sigs.k8s.io/cluster-api/pkg/controller/sharedinformers"
)

const (
	azureMachineControllerName = "azure-controller"
)

func StartMachineController(server *options.MachineControllerServer, shutdown <-chan struct{}) {
	config, err := controller.GetConfig(server.CommonConfig.Kubeconfig)
	if err != nil {
		glog.Fatalf("Could not create Config for talking to the apiserver: %v", err)
	}

	client, err := clientset.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Could not create client for talking to the apiserver: %v", err)
	}

	params := azure.MachineActuatorParams{
		V1Alpha1Client: client.ClusterV1alpha1(),
		KubeadmToken:   "dummy",
	}
	actuator, err := azure.NewMachineActuator(params)

	if err != nil {
		glog.Fatalf("Could not create azure machine actuator: %v", err)
	}

	si := sharedinformers.NewSharedInformers(config, shutdown)
	// If this doesn't compile, the code generator probably
	// overwrote the customized NewMachineController function.
	c := machine.NewMachineController(config, si, actuator)
	c.RunAsync(shutdown)

	select {}
}

func RunMachineController(server *options.MachineControllerServer) error {
	kubeConfig, err := controller.GetConfig(server.CommonConfig.Kubeconfig)
	if err != nil {
		glog.Errorf("Could not create Config for talking to the apiserver: %v", err)
		return err
	}

	kubeClientControl, err := kubernetes.NewForConfig(
		rest.AddUserAgent(kubeConfig, "machine-controller-manager"),
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
		StartMachineController(server, stop)
	}

	leaderElectConfig := config.GetLeaderElectionConfig()
	if !leaderElectConfig.LeaderElect {
		run(make(<-chan (struct{})))
	}

	// Identity used to distinguish between multiple machine controller instances.
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	leaderElectionClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(kubeConfig, "machine-leader-election"))

	id = id + "-" + string(uuid.NewUUID())
	// Lock required for leader election
	rl, err := resourcelock.New(
		leaderElectConfig.ResourceLock,
		metav1.NamespaceSystem,
		azureMachineControllerName,
		leaderElectionClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id + "-" + azureMachineControllerName,
			EventRecorder: recorder,
		})
	if err != nil {
		return err
	}

	// Try and become the leader and start machine controller loops
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
	return eventBroadcaster.NewRecorder(eventsScheme, corev1.EventSource{Component: azureMachineControllerName}), nil
}
