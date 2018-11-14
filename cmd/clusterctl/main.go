package main

import (
	"github.com/golang/glog"
	"github.com/platform9/azure-provider/pkg/cloud/azure/actuators/machine"

	"sigs.k8s.io/cluster-api/cmd/clusterctl/cmd"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
)

func main() {
	var err error
	machine.Actuator, err = machine.NewMachineActuator(machine.MachineActuatorParams{})
	if err != nil {
		glog.Fatalf("Error creating cluster provisioner for azure : %v", err)
	}
	common.RegisterClusterProvisioner(machine.ProviderName, machine.Actuator)
	cmd.Execute()
}
