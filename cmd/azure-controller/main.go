package main

import (
	"github.com/golang/glog"
	"github.com/platform9/azure-provider/cmd/azure-controller/cluster-controller-app"
	clusteroptions "github.com/platform9/azure-provider/cmd/azure-controller/cluster-controller-app/options"
	"github.com/platform9/azure-provider/cmd/azure-controller/machine-controller-app"
	machineoptions "github.com/platform9/azure-provider/cmd/azure-controller/machine-controller-app/options"
	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/util/logs"
	"sigs.k8s.io/cluster-api/pkg/controller/config"
)

func main() {
	fs := pflag.CommandLine
	var controllerType, machineSetupConfigsPath string
	fs.StringVar(&controllerType, "controller", controllerType, "specify whether this should run the machine or cluster controller")
	fs.StringVar(&machineSetupConfigsPath, "machinesetup", machineSetupConfigsPath, "path to machine setup configs file")
	config.ControllerConfig.AddFlags(pflag.CommandLine)
	pflag.Parse()

	logs.InitLogs()
	defer logs.FlushLogs()

	if controllerType == "machine" {
		machineServer := machineoptions.NewMachineControllerServer(machineSetupConfigsPath)
		if err := machine_controller_app.RunMachineController(machineServer); err != nil {
			glog.Errorf("Failed to start machine controller. Err: %v", err)
		}
	} else if controllerType == "cluster" {
		clusterServer := clusteroptions.NewClusterControllerServer()
		if err := cluster_controller_app.RunClusterController(clusterServer); err != nil {
			glog.Errorf("Failed to start cluster controller. Err: %v", err)
		}
	} else {
		glog.Errorf("Failed to start controller, `controller` flag must be either `machine` or `cluster` but was %v.", controllerType)
	}
}
