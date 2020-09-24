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

package controllers

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

func TestAzureClusterToAzureMachinesMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := setupScheme(g)
	clusterName := "my-cluster"
	initObjects := []runtime.Object{
		newCluster(clusterName),
		// Create two Machines with an infrastructure ref and one without.
		newMachineWithInfrastructureRef(clusterName, "my-machine-0"),
		newMachineWithInfrastructureRef(clusterName, "my-machine-1"),
		newMachine(clusterName, "my-machine-2"),
	}
	client := fake.NewFakeClientWithScheme(scheme, initObjects...)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	log := mock_log.NewMockLogger(mockCtrl)
	log.EXPECT().WithValues("AzureCluster", "my-cluster", "Namespace", "default")
	mapper, err := AzureClusterToAzureMachinesMapper(client, scheme, log)
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper.Map(handler.MapObject{
		Object: &infrav1.AzureCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:       clusterName,
						Kind:       "Cluster",
						APIVersion: clusterv1.GroupVersion.String(),
					},
				},
			},
		},
	})
	g.Expect(requests).To(HaveLen(2))
}

func TestGetCloudProviderConfig(t *testing.T) {
	g := NewWithT(t)

	cases := map[string]struct {
		identityType               infrav1.VMIdentity
		identityID                 string
		expectedControlPlaneConfig string
		expectedWorkerNodeConfig   string
	}{
		"serviceprincipal": {
			identityType:               infrav1.VMIdentityNone,
			expectedControlPlaneConfig: spControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   spWorkerNodeCloudConfig,
		},
		"system-assigned-identity": {
			identityType:               infrav1.VMIdentitySystemAssigned,
			expectedControlPlaneConfig: systemAssignedControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   systemAssignedWorkerNodeCloudConfig,
		},
		"user-assigned-identity": {
			identityType:               infrav1.VMIdentityUserAssigned,
			identityID:                 "foobar",
			expectedControlPlaneConfig: userAssignedControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   userAssignedWorkerNodeCloudConfig,
		},
	}

	cluster := newCluster("foo")
	cluster.Default()
	azureCluster := newAzureCluster("foo", "bar")
	azureCluster.Default()

	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	g.Expect(err).NotTo(HaveOccurred())

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cloudConfig, err := GetCloudProviderSecret(clusterScope, "default", "foo", metav1.OwnerReference{}, tc.identityType, tc.identityID)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cloudConfig.Data).NotTo(BeNil())

			if diff := cmp.Diff(tc.expectedControlPlaneConfig, string(cloudConfig.Data["control-plane-azure.json"])); diff != "" {
				t.Errorf(diff)
			}
			if diff := cmp.Diff(tc.expectedWorkerNodeConfig, string(cloudConfig.Data["worker-node-azure.json"])); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func TestReconcileAzureSecret(t *testing.T) {
	g := NewWithT(t)

	cases := map[string]struct {
		kind       string
		apiVersion string
		ownerName  string
	}{
		"azuremachine should reconcile secret successfully": {
			kind:       "AzureMachine",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
			ownerName:  "azureMachineName",
		},
		"azuremachinepool should reconcile secret successfully": {
			kind:       "AzureMachinePool",
			apiVersion: "exp.infrastructure.cluster.x-k8s.io/v1alpha3",
			ownerName:  "azureMachinePoolName",
		},
		"azuremachinetemplate should reconcile secret successfully": {
			kind:       "AzureMachineTemplate",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
			ownerName:  "azureMachineTemplateName",
		},
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	testLog := ctrl.Log.WithName("reconcileAzureSecret")

	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")

	cluster := newCluster("foo")
	azureCluster := newAzureCluster("foo", "bar")

	cluster.Default()
	azureCluster.Default()

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	g.Expect(err).NotTo(HaveOccurred())

	scheme := setupScheme(g)

	kubeclient := fake.NewFakeClientWithScheme(scheme)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			owner := metav1.OwnerReference{
				APIVersion: tc.apiVersion,
				Kind:       tc.kind,
				Name:       tc.ownerName,
			}
			cloudConfig, err := GetCloudProviderSecret(clusterScope, "default", tc.ownerName, owner, infrav1.VMIdentitySystemAssigned, "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cloudConfig.Data).NotTo(BeNil())

			if err := reconcileAzureSecret(context.Background(), testLog, kubeclient, owner, cloudConfig, azureCluster.ClusterName); err != nil {
				t.Error(err)
			}

			key := types.NamespacedName{
				Namespace: "default",
				Name:      fmt.Sprintf("%s-azure-json", tc.ownerName),
			}
			found := &corev1.Secret{}
			if err := kubeclient.Get(context.Background(), key, found); err != nil {
				t.Error(err)
			}
			g.Expect(cloudConfig.Data).To(Equal(found.Data))
			g.Expect(found.OwnerReferences).To(Equal(cloudConfig.OwnerReferences))
		})
	}
}

func setupScheme(g *WithT) *runtime.Scheme {
	scheme := runtime.NewScheme()
	g.Expect(clientgoscheme.AddToScheme(scheme)).ToNot(HaveOccurred())
	g.Expect(infrav1.AddToScheme(scheme)).ToNot(HaveOccurred())
	g.Expect(clusterv1.AddToScheme(scheme)).ToNot(HaveOccurred())
	return scheme
}

func newMachine(clusterName, machineName string) *clusterv1.Machine {
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
	}
}

func newMachineWithInfrastructureRef(clusterName, machineName string) *clusterv1.Machine {
	m := newMachine(clusterName, machineName)
	m.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "AzureMachine",
		Namespace:  "default",
		Name:       "azure" + machineName,
		APIVersion: infrav1.GroupVersion.String(),
	}
	return m
}

func newCluster(name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}

func newAzureCluster(name, location string) *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterSpec{
			Location: location,
			NetworkSpec: infrav1.NetworkSpec{
				Vnet: infrav1.VnetSpec{},
			},
			ResourceGroup:  "bar",
			SubscriptionID: "baz",
		},
	}
}

const (
	spControlPlaneCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "aadClientId": "fooClient",
    "aadClientSecret": "fooSecret",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "bar",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "foo-vnet",
    "vnetResourceGroup": "bar",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`
	spWorkerNodeCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "bar",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "foo-vnet",
    "vnetResourceGroup": "bar",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`

	systemAssignedControlPlaneCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "bar",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "foo-vnet",
    "vnetResourceGroup": "bar",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true
}`
	systemAssignedWorkerNodeCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "bar",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "foo-vnet",
    "vnetResourceGroup": "bar",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`

	userAssignedControlPlaneCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "bar",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "foo-vnet",
    "vnetResourceGroup": "bar",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "userAssignedIdentityId": "foobar"
}`
	userAssignedWorkerNodeCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "bar",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "foo-vnet",
    "vnetResourceGroup": "bar",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`
)
