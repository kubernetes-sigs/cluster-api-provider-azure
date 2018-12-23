/*

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
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators/cluster"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators/machine"
	capis "sigs.k8s.io/cluster-api/pkg/apis"
	apicluster "sigs.k8s.io/cluster-api/pkg/controller/cluster"
	apimachine "sigs.k8s.io/cluster-api/pkg/controller/machine"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	flag.Parse()

	logf.SetLogger(logf.ZapLogger(false))
	log := logf.Log.WithName("entrypoint")

	// Get a config to talk to the apiserver
	log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	if err := prepareEnvironment(); err != nil {
		log.Error(err, "unable to prepare environment for actuators")
		os.Exit(1)
	}

	clusterActuator, err := cluster.NewClusterActuator(cluster.ClusterActuatorParams{Client: mgr.GetClient()})
	if err != nil {
		log.Error(err, "error creating cluster actuator")
		os.Exit(1)
	}
	machineActuator, err := machine.NewMachineActuator(machine.MachineActuatorParams{Client: mgr.GetClient(), Scheme: mgr.GetScheme()})
	if err != nil {
		log.Error(err, "error creating machine actuator")
		os.Exit(1)
	}

	log.Info("Registering Components.")
	common.RegisterClusterProvisioner(machine.ProviderName, machineActuator)

	// Setup Scheme for all resources
	log.Info("setting up scheme")

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	if err := capis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	apimachine.AddWithActuator(mgr, machineActuator)
	apicluster.AddWithActuator(mgr, clusterActuator)

	// Start the Cmd
	log.Info("Starting the Cmd.")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "unable to run the manager")
		os.Exit(1)
	}
}

func prepareEnvironment() error {
	//Parse in environment variables if necessary
	if os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
		err := godotenv.Load()
		if err == nil && os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
			return fmt.Errorf("couldn't find environment variable for the Azure subscription: %v", err)
		}
		if err != nil {
			return fmt.Errorf("failed to load environment variables: %v", err)
		}
	}
	return nil
}
