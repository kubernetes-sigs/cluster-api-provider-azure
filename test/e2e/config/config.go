/*
Copyright 2019 The Kubernetes Authors.

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

package config

import (
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds global test configuration
type Config struct { // ClusterName allows you to set the name of a cluster already created
	Location          string   `envconfig:"LOCATION"`                                                       // Location where you want to create the cluster
	ClusterConfigPath string   `envconfig:"CLUSTERCONFIG_PATH" default:"config/base/v1alpha1_cluster.yaml"` // path to the YAML for the cluster we're creating
	MachineConfigPath string   `envconfig:"MACHINECONFIG_PATH" default:"config/base/v1alpha1_machine.yaml"` // path to the YAML describing the machines we're creating
	ClientID          string   `envconfig:"AZURE_CLIENT_ID" required:"true"`
	ClientSecret      string   `envconfig:"AZURE_CLIENT_SECRET" required:"true"`
	PublicSSHKey      string   `envconfig:"PUBLIC_SSH_KEY"`
	SubscriptionID    string   `envconfig:"AZURE_SUBSCRIPTION_ID" required:"true"`
	TenantID          string   `envconfig:"TENANT_ID" required:"true"`
	KubernetesVersion string   `envconfig:"KUBERNETES_VERSION" required:"true"`
	Regions           []string `envconfig:"REGIONS"` // A whitelist of available regions
}

// ParseConfig will parse needed environment variables for running the tests
func ParseConfig() (*Config, error) {
	c := new(Config)
	if err := envconfig.Process("config", c); err != nil {
		return nil, err
	}
	if c.Location == "" {
		c.SetRandomRegion()
	}
	return c, nil
}

// SetRandomRegion sets Location to a random region
func (c *Config) SetRandomRegion() {
	var regions []string
	if c.Regions == nil || len(c.Regions) == 0 {
		regions = []string{"eastus", "southcentralus", "southeastasia", "westus2", "westeurope"}
	} else {
		regions = c.Regions
	}
	log.Printf("Picking Random Region from list %s\n", regions)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	c.Location = regions[r.Intn(len(regions))]
	os.Setenv("LOCATION", c.Location)
	log.Printf("Picked Random Region:%s\n", c.Location)
}
