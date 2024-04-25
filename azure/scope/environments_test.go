/*
Copyright 2024 The Kubernetes Authors.

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

package scope

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	batchResourceID      = "--batch-resource-id--"
	datalakeResourceID   = "--datalake-resource-id--"
	graphResourceID      = "--graph-resource-id--"
	keyvaultResourceID   = "--keyvault-resource-id--"
	opInsightsResourceID = "--operational-insights-resource-id--"
	ossRDBMSResourceID   = "--oss-rdbms-resource-id--"
	cosmosDBResourceID   = "--cosmosdb-resource-id--"
	managedHSMResourceID = "--managed-hsm-resource-id--"
)

// This correlates with the expected contents of ./testdata/test_environment_1.json
var testEnvironment1 = Environment{
	Name:                         "--unit-test--",
	ManagementPortalURL:          "--management-portal-url",
	PublishSettingsURL:           "--publish-settings-url--",
	ServiceManagementEndpoint:    "--service-management-endpoint--",
	ResourceManagerEndpoint:      "--resource-management-endpoint--",
	ActiveDirectoryEndpoint:      "--active-directory-endpoint--",
	GalleryEndpoint:              "--gallery-endpoint--",
	KeyVaultEndpoint:             "--key-vault--endpoint--",
	ManagedHSMEndpoint:           "--managed-hsm-endpoint--",
	GraphEndpoint:                "--graph-endpoint--",
	StorageEndpointSuffix:        "--storage-endpoint-suffix--",
	CosmosDBDNSSuffix:            "--cosmos-db-dns-suffix--",
	MariaDBDNSSuffix:             "--maria-db-dns-suffix--",
	MySQLDatabaseDNSSuffix:       "--mysql-database-dns-suffix--",
	PostgresqlDatabaseDNSSuffix:  "--postgresql-database-dns-suffix--",
	SQLDatabaseDNSSuffix:         "--sql-database-dns-suffix--",
	TrafficManagerDNSSuffix:      "--traffic-manager-dns-suffix--",
	KeyVaultDNSSuffix:            "--key-vault-dns-suffix--",
	ManagedHSMDNSSuffix:          "--managed-hsm-dns-suffix--",
	ServiceBusEndpointSuffix:     "--service-bus-endpoint-suffix--",
	ServiceManagementVMDNSSuffix: "--asm-vm-dns-suffix--",
	ResourceManagerVMDNSSuffix:   "--arm-vm-dns-suffix--",
	ContainerRegistryDNSSuffix:   "--container-registry-dns-suffix--",
	TokenAudience:                "--token-audience",
	ResourceIdentifiers: ResourceIdentifier{
		Batch:               batchResourceID,
		Datalake:            datalakeResourceID,
		Graph:               graphResourceID,
		KeyVault:            keyvaultResourceID,
		OperationalInsights: opInsightsResourceID,
		OSSRDBMS:            ossRDBMSResourceID,
		CosmosDB:            cosmosDBResourceID,
		ManagedHSM:          managedHSMResourceID,
	},
}

func TestEnvironment_EnvironmentFromFile(t *testing.T) {
	got, err := EnvironmentFromFile(filepath.Join("testdata", "test_environment_1.json"))
	if err != nil {
		t.Error(err)
	}

	if got != testEnvironment1 {
		t.Logf("got: %v want: %v", got, testEnvironment1)
		t.Fail()
	}
}

func TestEnvironment_EnvironmentFromName_Stack(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	prevEnvFilepathValue := os.Getenv(EnvironmentFilepathName)
	os.Setenv(EnvironmentFilepathName, filepath.Join(path.Dir(currentFile), "testdata", "test_environment_1.json"))
	defer os.Setenv(EnvironmentFilepathName, prevEnvFilepathValue)

	got, err := EnvironmentFromName("AZURESTACKCLOUD")
	if err != nil {
		t.Error(err)
	}

	if got != testEnvironment1 {
		t.Logf("got: %v want: %v", got, testEnvironment1)
		t.Fail()
	}
}

func TestEnvironmentFromName(t *testing.T) {
	tests := map[string]*Environment{
		"azurechinacloud":        &ChinaCloud,
		"AzureChinaCloud":        &ChinaCloud,
		"azuregermancloud":       &GermanCloud,
		"AzureGermanCloud":       &GermanCloud,
		"AzureCloud":             &PublicCloud,
		"azurepubliccloud":       &PublicCloud,
		"AzurePublicCloud":       &PublicCloud,
		"azureusgovernmentcloud": &USGovernmentCloud,
		"AzureUSGovernmentCloud": &USGovernmentCloud,
		"azureusgovernment":      &USGovernmentCloud,
		"AzureUSGovernment":      &USGovernmentCloud,
		"thisisnotarealcloudenv": nil,
	}
	for name, v := range tests {
		t.Run(name, func(t *testing.T) {
			env, err := EnvironmentFromName(name)
			if v != nil && env != *v {
				t.Errorf("Expected %v, but got %v", *v, env)
			}
			if v == nil && err == nil {
				t.Errorf("Expected an error for %q, but got none", name)
			}
			if v != nil && err != nil {
				t.Errorf("Expected no error for %q, but got %v", name, err)
			}
		})
	}
}

func TestDeserializeEnvironment(t *testing.T) {
	env := `{
		"name": "--name--",
		"ActiveDirectoryEndpoint": "--active-directory-endpoint--",
		"galleryEndpoint": "--gallery-endpoint--",
		"graphEndpoint": "--graph-endpoint--",
		"serviceBusEndpoint": "--service-bus-endpoint--",
		"keyVaultDNSSuffix": "--key-vault-dns-suffix--",
		"keyVaultEndpoint": "--key-vault-endpoint--",
		"managedHSMDNSSuffix": "--managed-hsm-dns-suffix--",
		"managedHSMEndpoint": "--managed-hsm-endpoint--",
		"managementPortalURL": "--management-portal-url--",
		"publishSettingsURL": "--publish-settings-url--",
		"resourceManagerEndpoint": "--resource-manager-endpoint--",
		"serviceBusEndpointSuffix": "--service-bus-endpoint-suffix--",
		"serviceManagementEndpoint": "--service-management-endpoint--",
		"cosmosDBDNSSuffix": "--cosmos-db-dns-suffix--",
		"mariaDBDNSSuffix": "--maria-db-dns-suffix--",
		"mySqlDatabaseDNSSuffix": "--mysql-database-dns-suffix--",
		"postgresqlDatabaseDNSSuffix": "--postgresql-database-dns-suffix--",
		"sqlDatabaseDNSSuffix": "--sql-database-dns-suffix--",
		"storageEndpointSuffix": "--storage-endpoint-suffix--",
		"trafficManagerDNSSuffix": "--traffic-manager-dns-suffix--",
		"serviceManagementVMDNSSuffix": "--asm-vm-dns-suffix--",
		"resourceManagerVMDNSSuffix": "--arm-vm-dns-suffix--",
		"containerRegistryDNSSuffix": "--container-registry-dns-suffix--",
		"resourceIdentifiers": {
			"batch": "` + batchResourceID + `",
			"datalake": "` + datalakeResourceID + `",
			"graph": "` + graphResourceID + `",
			"keyVault": "` + keyvaultResourceID + `",
			"operationalInsights": "` + opInsightsResourceID + `",
			"ossRDBMS": "` + ossRDBMSResourceID + `",
			"cosmosDB": "` + cosmosDBResourceID + `",
			"managedHSM": "` + managedHSMResourceID + `"
		}
	}`

	testSubject := Environment{}
	err := json.Unmarshal([]byte(env), &testSubject)
	if err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	checks := map[string]string{
		"--name--":                           testSubject.Name,
		"--management-portal-url--":          testSubject.ManagementPortalURL,
		"--publish-settings-url--":           testSubject.PublishSettingsURL,
		"--service-management-endpoint--":    testSubject.ServiceManagementEndpoint,
		"--resource-manager-endpoint--":      testSubject.ResourceManagerEndpoint,
		"--active-directory-endpoint--":      testSubject.ActiveDirectoryEndpoint,
		"--gallery-endpoint--":               testSubject.GalleryEndpoint,
		"--key-vault-endpoint--":             testSubject.KeyVaultEndpoint,
		"--managed-hsm-endpoint--":           testSubject.ManagedHSMEndpoint,
		"--graph-endpoint--":                 testSubject.GraphEndpoint,
		"--service-bus-endpoint--":           testSubject.ServiceBusEndpoint,
		"--storage-endpoint-suffix--":        testSubject.StorageEndpointSuffix,
		"--cosmos-db-dns-suffix--":           testSubject.CosmosDBDNSSuffix,
		"--maria-db-dns-suffix--":            testSubject.MariaDBDNSSuffix,
		"--mysql-database-dns-suffix--":      testSubject.MySQLDatabaseDNSSuffix,
		"--postgresql-database-dns-suffix--": testSubject.PostgresqlDatabaseDNSSuffix,
		"--sql-database-dns-suffix--":        testSubject.SQLDatabaseDNSSuffix,
		"--key-vault-dns-suffix--":           testSubject.KeyVaultDNSSuffix,
		"--managed-hsm-dns-suffix--":         testSubject.ManagedHSMDNSSuffix,
		"--service-bus-endpoint-suffix--":    testSubject.ServiceBusEndpointSuffix,
		"--asm-vm-dns-suffix--":              testSubject.ServiceManagementVMDNSSuffix,
		"--arm-vm-dns-suffix--":              testSubject.ResourceManagerVMDNSSuffix,
		"--container-registry-dns-suffix--":  testSubject.ContainerRegistryDNSSuffix,
		batchResourceID:                      testSubject.ResourceIdentifiers.Batch,
		datalakeResourceID:                   testSubject.ResourceIdentifiers.Datalake,
		graphResourceID:                      testSubject.ResourceIdentifiers.Graph,
		keyvaultResourceID:                   testSubject.ResourceIdentifiers.KeyVault,
		opInsightsResourceID:                 testSubject.ResourceIdentifiers.OperationalInsights,
		ossRDBMSResourceID:                   testSubject.ResourceIdentifiers.OSSRDBMS,
		cosmosDBResourceID:                   testSubject.ResourceIdentifiers.CosmosDB,
		managedHSMResourceID:                 testSubject.ResourceIdentifiers.ManagedHSM,
	}

	for k, v := range checks {
		if k != v {
			t.Errorf("Expected %q, but got %q", k, v)
		}
	}
}

func TestRoundTripSerialization(t *testing.T) {
	env := Environment{
		Name:                         "--unit-test--",
		ManagementPortalURL:          "--management-portal-url",
		PublishSettingsURL:           "--publish-settings-url--",
		ServiceManagementEndpoint:    "--service-management-endpoint--",
		ResourceManagerEndpoint:      "--resource-management-endpoint--",
		ActiveDirectoryEndpoint:      "--active-directory-endpoint--",
		GalleryEndpoint:              "--gallery-endpoint--",
		KeyVaultEndpoint:             "--key-vault--endpoint--",
		GraphEndpoint:                "--graph-endpoint--",
		ServiceBusEndpoint:           "--service-bus-endpoint--",
		StorageEndpointSuffix:        "--storage-endpoint-suffix--",
		CosmosDBDNSSuffix:            "--cosmos-db-dns-suffix--",
		MariaDBDNSSuffix:             "--maria-db-dns-suffix--",
		MySQLDatabaseDNSSuffix:       "--mysql-database-dns-suffix--",
		PostgresqlDatabaseDNSSuffix:  "--postgresql-database-dns-suffix--",
		SQLDatabaseDNSSuffix:         "--sql-database-dns-suffix--",
		TrafficManagerDNSSuffix:      "--traffic-manager-dns-suffix--",
		KeyVaultDNSSuffix:            "--key-vault-dns-suffix--",
		ServiceBusEndpointSuffix:     "--service-bus-endpoint-suffix--",
		ServiceManagementVMDNSSuffix: "--asm-vm-dns-suffix--",
		ResourceManagerVMDNSSuffix:   "--arm-vm-dns-suffix--",
		ContainerRegistryDNSSuffix:   "--container-registry-dns-suffix--",
		ResourceIdentifiers: ResourceIdentifier{
			Batch:               batchResourceID,
			Datalake:            datalakeResourceID,
			Graph:               graphResourceID,
			KeyVault:            keyvaultResourceID,
			OperationalInsights: opInsightsResourceID,
			OSSRDBMS:            ossRDBMSResourceID,
			CosmosDB:            cosmosDBResourceID,
		},
	}

	bytes, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("failed to marshal: %s", err)
	}

	testSubject := Environment{}
	err = json.Unmarshal(bytes, &testSubject)
	if err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	checks := map[string]string{
		env.Name:                                    testSubject.Name,
		env.ManagementPortalURL:                     testSubject.ManagementPortalURL,
		env.PublishSettingsURL:                      testSubject.PublishSettingsURL,
		env.ServiceManagementEndpoint:               testSubject.ServiceManagementEndpoint,
		env.ResourceManagerEndpoint:                 testSubject.ResourceManagerEndpoint,
		env.ActiveDirectoryEndpoint:                 testSubject.ActiveDirectoryEndpoint,
		env.GalleryEndpoint:                         testSubject.GalleryEndpoint,
		env.ServiceBusEndpoint:                      testSubject.ServiceBusEndpoint,
		env.KeyVaultEndpoint:                        testSubject.KeyVaultEndpoint,
		env.GraphEndpoint:                           testSubject.GraphEndpoint,
		env.StorageEndpointSuffix:                   testSubject.StorageEndpointSuffix,
		env.CosmosDBDNSSuffix:                       testSubject.CosmosDBDNSSuffix,
		env.MariaDBDNSSuffix:                        testSubject.MariaDBDNSSuffix,
		env.MySQLDatabaseDNSSuffix:                  testSubject.MySQLDatabaseDNSSuffix,
		env.PostgresqlDatabaseDNSSuffix:             testSubject.PostgresqlDatabaseDNSSuffix,
		env.SQLDatabaseDNSSuffix:                    testSubject.SQLDatabaseDNSSuffix,
		env.TrafficManagerDNSSuffix:                 testSubject.TrafficManagerDNSSuffix,
		env.KeyVaultDNSSuffix:                       testSubject.KeyVaultDNSSuffix,
		env.ServiceBusEndpointSuffix:                testSubject.ServiceBusEndpointSuffix,
		env.ServiceManagementVMDNSSuffix:            testSubject.ServiceManagementVMDNSSuffix,
		env.ResourceManagerVMDNSSuffix:              testSubject.ResourceManagerVMDNSSuffix,
		env.ContainerRegistryDNSSuffix:              testSubject.ContainerRegistryDNSSuffix,
		env.ResourceIdentifiers.Batch:               testSubject.ResourceIdentifiers.Batch,
		env.ResourceIdentifiers.Datalake:            testSubject.ResourceIdentifiers.Datalake,
		env.ResourceIdentifiers.Graph:               testSubject.ResourceIdentifiers.Graph,
		env.ResourceIdentifiers.KeyVault:            testSubject.ResourceIdentifiers.KeyVault,
		env.ResourceIdentifiers.OperationalInsights: testSubject.ResourceIdentifiers.OperationalInsights,
		env.ResourceIdentifiers.OSSRDBMS:            testSubject.ResourceIdentifiers.OSSRDBMS,
		env.ResourceIdentifiers.CosmosDB:            testSubject.ResourceIdentifiers.CosmosDB,
	}

	for k, v := range checks {
		if k != v {
			t.Errorf("Expected %q, but got %q", k, v)
		}
	}
}
