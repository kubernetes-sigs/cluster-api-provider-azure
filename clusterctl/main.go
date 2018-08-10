package main

import (
	_ "github.com/platform9/azure-provider/cloud/azure"
	"sigs.k8s.io/cluster-api/clusterctl/cmd"
)

func main() {
	cmd.Execute()
}
