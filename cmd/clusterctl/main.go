package main

import (
	"github.com/golang/glog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators/machine"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/cmd"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
)

func main() {
	var err error
	machineActuator, err := machine.NewMachineActuator(machine.MachineActuatorParams{})
	if err != nil {
		glog.Fatalf("Error creating cluster provisioner for azure : %v", err)
	}
	common.RegisterClusterProvisioner(machine.ProviderName, machineActuator)
	cmd.Execute()
}
