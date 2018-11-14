package main

import (
	"sigs.k8s.io/cluster-api/cmd/clusterctl/cmd"

	_ "github.com/platform9/azure-provider/cloud/azure"
)

func main() {
	cmd.Execute()
}
