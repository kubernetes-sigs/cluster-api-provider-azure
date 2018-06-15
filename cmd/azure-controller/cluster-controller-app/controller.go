package cluster_controller_app

import (
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/apiserver-builder/pkg/controller"
	azureactuator "github.com/platform9/azure-provider"
	"github.com/platform9/azure-provider/cmd/azure-controller/cluster-controller-app/options"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/cluster-api/pkg/controller/cluster"
	"sigs.k8s.io/cluster-api/pkg/controller/sharedinformers"
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

	params := azureactuator.ClusterActuatorParams{
		ClusterClient: client.ClusterV1alpha1().Clusters(corev1.NamespaceDefault),
	}
	actuator, err := azureactuator.NewClusterActuator(params)
	if err != nil {
		glog.Fatalf("Could not create Google cluster actuator: %v", err)
	}

	si := sharedinformers.NewSharedInformers(config, shutdown)
	// If this doesn't compile, the code generator probably
	// overwrote the customized NewClusterController function.
	c := cluster.NewClusterController(config, si, actuator)
	c.RunAsync(shutdown)

	select {}
}

func RunClusterController(server *options.ClusterControllerServer) error {
	return nil
}
