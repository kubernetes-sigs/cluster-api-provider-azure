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

package actuators

import (
	"testing"

	"github.com/ghodss/yaml"
	clusterv1 "github.com/openshift/cluster-api/pkg/apis/cluster/v1alpha1"
	machinev1 "github.com/openshift/cluster-api/pkg/apis/machine/v1beta1"
	"github.com/openshift/cluster-api/pkg/client/clientset_generated/clientset/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	providerv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	controllerfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func providerSpecFromMachine(in *providerv1.AzureMachineProviderSpec) (*machinev1.ProviderSpec, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &machinev1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func newMachine(t *testing.T) *machinev1.Machine {
	machineConfig := providerv1.AzureMachineProviderSpec{}
	providerSpec, err := providerSpecFromMachine(&machineConfig)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}
	return &machinev1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "machine-test",
		},
		Spec: machinev1.MachineSpec{
			ProviderSpec: *providerSpec,
		},
	}
}

func TestNilClusterScope(t *testing.T) {
	m := newMachine(t)
	params := MachineScopeParams{
		AzureClients: AzureClients{},
		Cluster:      nil,
		CoreClient:   nil,
		Machine:      m,
		Client:       fake.NewSimpleClientset(m).MachineV1beta1(),
	}
	_, err := NewMachineScope(params)
	if err != nil {
		t.Errorf("Expected New machine scope to succeed with nil cluster: %v", err)
	}
}

func TestCredentialsSecretSuccess(t *testing.T) {
	credentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCredentials",
			Namespace: "dummyNamespace",
		},
		Data: map[string][]byte{
			"azure_subscription_id": []byte("dummySubID"),
			"azure_client_id":       []byte("dummyClientID"),
			"azure_client_secret":   []byte("dummyClientSecret"),
			"azure_tenant_id":       []byte("dummyTenantID"),
			"azure_resourcegroup":   []byte("dummyResourceGroup"),
			"azure_region":          []byte("dummyRegion"),
			"azure_resource_prefix": []byte("dummyClusterName"),
		},
	}
	scope := &Scope{Cluster: &clusterv1.Cluster{}, ClusterConfig: &providerv1.AzureClusterProviderSpec{}}
	err := updateScope(
		controllerfake.NewFakeClient(credentialsSecret),
		&corev1.SecretReference{Name: "testCredentials", Namespace: "dummyNamespace"},
		scope)
	if err != nil {
		t.Errorf("Expected New credentials secrets to succeed: %v", err)
	}

	if scope.SubscriptionID != "dummySubID" {
		t.Errorf("Expected subscriptionID to be dummySubID but found %s", scope.SubscriptionID)
	}

	if scope.Location() != "dummyRegion" {
		t.Errorf("Expected location to be dummyRegion but found %s", scope.Location())
	}

	if scope.Cluster.Name != "dummyClusterName" {
		t.Errorf("Expected cluster name to be dummyClusterName but found %s", scope.Cluster.Name)
	}

	if scope.ClusterConfig.ResourceGroup != "dummyResourceGroup" {
		t.Errorf("Expected resourcegroup to be dummyResourceGroup but found %s", scope.ClusterConfig.ResourceGroup)
	}
}

func testCredentialFields(credentialsSecret *corev1.Secret) error {
	scope := &Scope{Cluster: &clusterv1.Cluster{}, ClusterConfig: &providerv1.AzureClusterProviderSpec{}}
	return updateScope(
		controllerfake.NewFakeClient(credentialsSecret),
		&corev1.SecretReference{Name: "testCredentials", Namespace: "dummyNamespace"},
		scope)
}

func TestCredentialsSecretFailures(t *testing.T) {
	credentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCredentials",
			Namespace: "dummyNamespace",
		},
		Data: map[string][]byte{},
	}

	if err := testCredentialFields(credentialsSecret); err == nil {
		t.Errorf("Expected New credentials secrets to fail")
	}

	credentialsSecret.Data["azure_subscription_id"] = []byte("dummyValue")
	if err := testCredentialFields(credentialsSecret); err == nil {
		t.Errorf("Expected New credentials secrets to fail")
	}

	credentialsSecret.Data["azure_client_id"] = []byte("dummyValue")
	if err := testCredentialFields(credentialsSecret); err == nil {
		t.Errorf("Expected New credentials secrets to fail")
	}

	credentialsSecret.Data["azure_client_secret"] = []byte("dummyValue")
	if err := testCredentialFields(credentialsSecret); err == nil {
		t.Errorf("Expected New credentials secrets to fail")
	}

	credentialsSecret.Data["azure_tenant_id"] = []byte("dummyValue")
	if err := testCredentialFields(credentialsSecret); err == nil {
		t.Errorf("Expected New credentials secrets to fail")
	}

	credentialsSecret.Data["azure_resourcegroup"] = []byte("dummyValue")
	if err := testCredentialFields(credentialsSecret); err == nil {
		t.Errorf("Expected New credentials secrets to fail")
	}

	credentialsSecret.Data["azure_region"] = []byte("dummyValue")
	if err := testCredentialFields(credentialsSecret); err == nil {
		t.Errorf("Expected New credentials secrets to fail")
	}

	credentialsSecret.Data["azure_resource_prefix"] = []byte("dummyValue")
	if err := testCredentialFields(credentialsSecret); err != nil {
		t.Errorf("Expected New credentials secrets to succeed but found : %v", err)
	}
}
