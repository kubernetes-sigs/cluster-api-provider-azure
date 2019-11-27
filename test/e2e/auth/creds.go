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
	"log"
	"os"

	"github.com/pelletier/go-toml"
)

type Config struct {
	Creds
}

type Creds struct {
	ClientID           string
	ClientSecret       string
	TenantID           string
	SubscriptionID     string
	StorageAccountName string
	StorageAccountKey  string
}

func LoadCredentialsFromFile(credsFile string) (Creds, error) {
	log.Printf("Loading credentials from file %v", credsFile)
	config := Config{}

	content, err := ioutil.ReadFile(credsFile)
	if err != nil {
		return Creds{}, fmt.Errorf("error reading credentials file %v %v", credsFile, err)
	}
	if err = toml.Unmarshal(content, &config); err != nil {
		return Creds{}, fmt.Errorf("error parsing credentials file %v %v", credsFile, err)
	}
	return config.Creds, nil
}

func LoadCredentialsFromEnvironment() (Creds, error) {
	log.Print("Loading credentials from environment")
	creds := Creds{}

	if tenantID, found := os.LookupEnv("AZURE_TENANT_ID"); found {
		creds.TenantID = tenantID
	} else {
		return Creds{}, fmt.Errorf("required variable AZURE_TENANT_ID is not set")
	}

	if subscriptionID, found := os.LookupEnv("AZURE_SUBSCRIPTION_ID"); found {
		creds.SubscriptionID = subscriptionID
	} else {
		return Creds{}, fmt.Errorf("required variable AZURE_SUBSCRIPTION_ID is not set")
	}

	if clientID, found := os.LookupEnv("AZURE_CLIENT_ID"); found {
		creds.ClientID = clientID
	} else {
		return Creds{}, fmt.Errorf("required variable AZURE_CLIENT_ID is not set")
	}

	if clientSecret, found := os.LookupEnv("AZURE_CLIENT_SECRET"); found {
		creds.ClientSecret = clientSecret
	} else {
		return Creds{}, fmt.Errorf("required variable AZURE_CLIENT_SECRET is not set")
	}

	return creds, nil
}
