package main

import (
	"github.com/golang/glog"
	machineoptions "github.com/platform9/azure-provider/cloud/azure/controllers/machine/options"
	"github.com/platform9/azure-provider/cloud/azure/controllers/machine"
	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/util/logs"
	"sigs.k8s.io/cluster-api/pkg/controller/config"
)

func main() {
	fs := pflag.CommandLine
	var machineSetupConfigsPath string
	fs.StringVar(&machineSetupConfigsPath, "machinesetup", machineSetupConfigsPath, "path to machine setup configs file")
	config.ControllerConfig.AddFlags(pflag.CommandLine)
	pflag.Parse()

	logs.InitLogs()
	defer logs.FlushLogs()
	machineServer := machineoptions.NewMachineControllerServer(machineSetupConfigsPath)
	if err := machine_controller.RunMachineController(machineServer); err != nil {
		glog.Errorf("Failed to start machine controller. Err: %v", err)
	}
}
