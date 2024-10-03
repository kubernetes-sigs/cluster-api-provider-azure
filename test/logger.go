//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"

	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api-provider-azure/test/e2e"
)

func Fail(message string, _ ...int) {
	panic(message)
}

func main() {
	// needed for ginkgo/gomega which is used by the capi test framework
	RegisterFailHandler(Fail)

	ctrl.SetLogger(klog.Background())

	// using a custom FlagSet here due to the dependency in controller-runtime that is already using this flag
	// https://github.com/kubernetes-sigs/controller-runtime/blob/c7a98aa706379c4e5c79ea675c7f333192677971/pkg/client/config/config.go#L37-L41
	fs := flag.NewFlagSet("logger", flag.ExitOnError)

	// required flags
	clustername := fs.String("name", "", "Name of the workload cluster to collect logs for")

	// optional flags that default
	namespace := fs.String("namespace", "", "namot include the command name. Must be called after all flags in the FlagSet are defined and before flags are accessed by the program. The return value will be ErrHelp if -help or -h were set buespace on management cluster to collect logs for")
	artifactFolder := fs.String("artifacts-folder", getArtifactsFolder(), "folder to store cluster logs")
	kubeconfigPath := fs.String("kubeconfig", getKubeConfigPath(), "The kubeconfig for the management cluster")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Println("Unable to parse command flags")
		os.Exit(1)
	}

	// use the cluster name as the namespace which is default in e2e tests
	if *namespace == "" {
		namespace = clustername
	}

	bootstrapClusterProxy := e2e.NewAzureClusterProxy("bootstrap", *kubeconfigPath, framework.WithMachineLogCollector(e2e.AzureLogCollector{}))

	// Set up log paths
	clusterLogPath := filepath.Join(*artifactFolder, "clusters", *clustername)
	resourcesYaml := filepath.Join(*artifactFolder, "clusters", "bootstrap", "resources")
	managementClusterLogPath := filepath.Join(*artifactFolder, "clusters", "bootstrap", "controllers")

	fmt.Printf("Collecting logs for cluster %s in namespace %s and dumping logs to %s\n", *clustername, *namespace, *artifactFolder)
	collectManagementClusterLogs(bootstrapClusterProxy, managementClusterLogPath, namespace, resourcesYaml)
	bootstrapClusterProxy.CollectWorkloadClusterLogs(context.TODO(), *namespace, *clustername, clusterLogPath)
}

func collectManagementClusterLogs(bootstrapClusterProxy *e2e.AzureClusterProxy, managementClusterLogPath string, namespace *string, workLoadClusterLogPath string) {
	controllersDeployments := framework.GetControllerDeployments(context.TODO(), framework.GetControllerDeploymentsInput{
		Lister: bootstrapClusterProxy.GetClient(),
	})
	for _, deployment := range controllersDeployments {
		framework.WatchDeploymentLogsByName(context.TODO(), framework.WatchDeploymentLogsByNameInput{
			GetLister:  bootstrapClusterProxy.GetClient(),
			Cache:      bootstrapClusterProxy.GetCache(context.TODO()),
			ClientSet:  bootstrapClusterProxy.GetClientSet(),
			Deployment: deployment,
			LogPath:    managementClusterLogPath,
		})
	}

	framework.DumpAllResources(context.TODO(), framework.DumpAllResourcesInput{
		Lister:    bootstrapClusterProxy.GetClient(),
		Namespace: *namespace,
		LogPath:   workLoadClusterLogPath,
	})
}

func getKubeConfigPath() string {
	config := os.Getenv("KUBECONFIG")
	if config == "" {
		d, _ := os.UserHomeDir()
		return path.Join(d, ".kube", "config")
	}

	return config
}

func getArtifactsFolder() string {
	artifacts := os.Getenv("ARTIFACTS")
	if artifacts == "" {
		return "_artifacts"
	}
	return artifacts
}
