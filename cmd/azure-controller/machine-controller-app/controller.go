package machine_controller_app

import (
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/apiserver-builder/pkg/controller"
	microsoft "github.com/platform9/azure-provider"
	"github.com/platform9/azure-provider/cmd/azure-controller/machine-controller-app/options"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
	"sigs.k8s.io/cluster-api/pkg/controller/sharedinformers"
	//"github.com/platform9/azure-provider/cmd/azure-controller/machine-controller/options"
	//"github.com/platform9/azure-provider/machinesetup"
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

	params := microsoft.MachineActuatorParams{
		V1Alpha1Client: client.ClusterV1alpha1(),
		KubeadmToken:   "dummy",
	}
	actuator, err := microsoft.NewMachineActuator(params)

	if err != nil {
		glog.Fatalf("Could not create Microsoft machine actuator: %v", err)
	}

	si := sharedinformers.NewSharedInformers(config, shutdown)
	// If this doesn't compile, the code generator probably
	// overwrote the customized NewMachineController function.
	c := machine.NewMachineController(config, si, actuator)
	c.RunAsync(shutdown)

	select {}
}

func RunMachineController(server *options.MachineControllerServer) error {
	return nil
}
