package main

import (
	"github.com/golang/glog"
	"github.com/platform9/azure-provider/cloud/azure/controllers/cluster"
	clusteroptions "github.com/platform9/azure-provider/cloud/azure/controllers/cluster/options"
	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/util/logs"
	"sigs.k8s.io/cluster-api/pkg/controller/config"
)

func main() {
	config.ControllerConfig.AddFlags(pflag.CommandLine)
	pflag.Parse()

	logs.InitLogs()
	defer logs.FlushLogs()
	machineServer := clusteroptions.NewClusterControllerServer()
	if err := cluster_controller.RunClusterController(machineServer); err != nil {
		glog.Errorf("Failed to start machine controller. Err: %v", err)
	}
}
