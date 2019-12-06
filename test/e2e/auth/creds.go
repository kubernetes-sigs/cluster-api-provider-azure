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

package auth

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pelletier/go-toml"
)

// Config stores the auth info
type Config struct {
	Creds
}

// Creds stores the auth info
type Creds struct {
	ClientID           string
	ClientSecret       string
	TenantID           string
	SubscriptionID     string
	StorageAccountName string
	StorageAccountKey  string
}

// LoadFromFile loads auth info from the input file path
// This was mostly copied from the test-infra repo
func LoadFromFile(credsFile string) (Creds, error) {
	fmt.Printf("Loading credentials from file %v\n", credsFile)
	content, err := ioutil.ReadFile(credsFile)
	if err != nil {
		return Creds{}, fmt.Errorf("error reading credentials file %v %v", credsFile, err)
	}
	config := Config{}
	if err = toml.Unmarshal(content, &config); err != nil {
		return Creds{}, fmt.Errorf("error parsing credentials file %v %v", credsFile, err)
	}
	return config.Creds, nil
}

// LoadFromEnvironment loads auth info from the environment
func LoadFromEnvironment() (Creds, error) {
	fmt.Print("Loading credentials from environment\n")
	return Creds{
		TenantID:       os.Getenv("AZURE_TENANT_ID"),
		SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"),
		ClientID:       os.Getenv("AZURE_CLIENT_ID"),
		ClientSecret:   os.Getenv("AZURE_CLIENT_SECRET"),
	}, nil
}
