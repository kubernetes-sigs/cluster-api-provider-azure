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

	"k8s.io/apimachinery/pkg/api/resource"

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
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
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
	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	log := mock_log.NewMockLogger(mockCtrl)
	log.EXPECT().WithValues("AzureCluster", "my-cluster", "Namespace", "default")
	mapper, err := AzureClusterToAzureMachinesMapper(context.Background(), client, &infrav1.AzureMachine{}, scheme, log)
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper(&infrav1.AzureCluster{
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
	})
	g.Expect(requests).To(HaveLen(2))
}

func TestGetCloudProviderConfig(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cluster := newCluster("foo")
	cluster.Default()
	azureCluster := newAzureCluster("foo", "bar")
	azureCluster.Default()
	azureClusterCustomVnet := newAzureClusterWithCustomVnet("foo", "bar")
	azureClusterCustomVnet.Default()

	cases := map[string]struct {
		cluster                    *clusterv1.Cluster
		azureCluster               *infrav1.AzureCluster
		identityType               infrav1.VMIdentity
		identityID                 string
		expectedControlPlaneConfig string
		expectedWorkerNodeConfig   string
	}{
		"serviceprincipal": {
			cluster:                    cluster,
			azureCluster:               azureCluster,
			identityType:               infrav1.VMIdentityNone,
			expectedControlPlaneConfig: spControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   spWorkerNodeCloudConfig,
		},
		"system-assigned-identity": {
			cluster:                    cluster,
			azureCluster:               azureCluster,
			identityType:               infrav1.VMIdentitySystemAssigned,
			expectedControlPlaneConfig: systemAssignedControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   systemAssignedWorkerNodeCloudConfig,
		},
		"user-assigned-identity": {
			cluster:                    cluster,
			azureCluster:               azureCluster,
			identityType:               infrav1.VMIdentityUserAssigned,
			identityID:                 "foobar",
			expectedControlPlaneConfig: userAssignedControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   userAssignedWorkerNodeCloudConfig,
		},
		"serviceprincipal with custom vnet": {
			cluster:                    cluster,
			azureCluster:               azureClusterCustomVnet,
			identityType:               infrav1.VMIdentityNone,
			expectedControlPlaneConfig: spCustomVnetControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   spCustomVnetWorkerNodeCloudConfig,
		},
		"with rate limits": {
			cluster:                    cluster,
			azureCluster:               withRateLimits(*azureCluster),
			identityType:               infrav1.VMIdentityNone,
			expectedControlPlaneConfig: rateLimitsControlPlaneCloudConfig,
			expectedWorkerNodeConfig:   rateLimitsWorkerNodeCloudConfig,
		},
		"with back-off config": {
			cluster:                    cluster,
			azureCluster:               withbackOffConfig(*azureCluster),
			identityType:               infrav1.VMIdentityNone,
			expectedControlPlaneConfig: backOffCloudConfig,
			expectedWorkerNodeConfig:   backOffCloudConfig,
		},
	}

	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			initObjects := []runtime.Object{tc.cluster, tc.azureCluster}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			clusterScope, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Cluster:      tc.cluster,
				AzureCluster: tc.azureCluster,
				Client:       fakeClient,
			})
			g.Expect(err).NotTo(HaveOccurred())

			cloudConfig, err := GetCloudProviderSecret(clusterScope, "default", "foo", metav1.OwnerReference{}, tc.identityType, tc.identityID)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cloudConfig.Data).NotTo(BeNil())

			if diff := cmp.Diff(tc.expectedControlPlaneConfig, string(cloudConfig.Data["control-plane-azure.json"])); diff != "" {
				t.Errorf(diff)
			}
			if diff := cmp.Diff(tc.expectedWorkerNodeConfig, string(cloudConfig.Data["worker-node-azure.json"])); diff != "" {
				t.Errorf(diff)
			}
			if diff := cmp.Diff(tc.expectedControlPlaneConfig, string(cloudConfig.Data["azure.json"])); diff != "" {
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
			apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
			ownerName:  "azureMachineName",
		},
		"azuremachinepool should reconcile secret successfully": {
			kind:       "AzureMachinePool",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
			ownerName:  "azureMachinePoolName",
		},
		"azuremachinetemplate should reconcile secret successfully": {
			kind:       "AzureMachineTemplate",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
			ownerName:  "azureMachineTemplateName",
		},
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	testLog := ctrl.Log.WithName("reconcileAzureSecret")

	cluster := newCluster("foo")
	azureCluster := newAzureCluster("foo", "bar")

	cluster.Default()
	azureCluster.Default()

	scheme := setupScheme(g)
	kubeclient := fake.NewClientBuilder().WithScheme(scheme).Build()

	clusterScope, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Cluster:      cluster,
		AzureCluster: azureCluster,
		Client:       kubeclient,
	})
	g.Expect(err).NotTo(HaveOccurred())

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

func withRateLimits(ac infrav1.AzureCluster) *infrav1.AzureCluster {
	cloudProviderRateLimitQPS := resource.MustParse("1.2")
	rateLimits := []infrav1.RateLimitSpec{
		{
			Name: "defaultRateLimit",
			Config: infrav1.RateLimitConfig{
				CloudProviderRateLimit:    true,
				CloudProviderRateLimitQPS: &cloudProviderRateLimitQPS,
			},
		},
		{
			Name: "loadBalancerRateLimit",
			Config: infrav1.RateLimitConfig{
				CloudProviderRateLimitBucket: 10,
			},
		},
	}
	ac.Spec.CloudProviderConfigOverrides = &infrav1.CloudProviderConfigOverrides{RateLimits: rateLimits}
	return &ac
}

func withbackOffConfig(ac infrav1.AzureCluster) *infrav1.AzureCluster {
	cloudProviderBackOffExponent := resource.MustParse("1.2")
	backOff := infrav1.BackOffConfig{
		CloudProviderBackoff:         true,
		CloudProviderBackoffRetries:  1,
		CloudProviderBackoffExponent: &cloudProviderBackOffExponent,
		CloudProviderBackoffDuration: 60,
		CloudProviderBackoffJitter:   &cloudProviderBackOffExponent,
	}
	ac.Spec.CloudProviderConfigOverrides = &infrav1.CloudProviderConfigOverrides{BackOffs: backOff}
	return &ac
}

func newAzureClusterWithCustomVnet(name, location string) *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterSpec{
			Location: location,
			NetworkSpec: infrav1.NetworkSpec{
				Vnet: infrav1.VnetSpec{
					Name:          "custom-vnet",
					ResourceGroup: "custom-vnet-resource-group",
				},
				Subnets: infrav1.Subnets{
					infrav1.SubnetSpec{
						Name: "foo-controlplane-subnet",
						Role: infrav1.SubnetControlPlane,
					},
					infrav1.SubnetSpec{
						Name: "foo-node-subnet",
						Role: infrav1.SubnetNode,
					},
				},
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
    "useManagedIdentityExtension": true,
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
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "userAssignedIdentityId": "foobar"
}`
	spCustomVnetControlPlaneCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "aadClientId": "fooClient",
    "aadClientSecret": "fooSecret",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "custom-vnet-resource-group",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "custom-vnet",
    "vnetResourceGroup": "custom-vnet-resource-group",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`
	spCustomVnetWorkerNodeCloudConfig = `{
    "cloud": "AzurePublicCloud",
    "tenantId": "fooTenant",
    "subscriptionId": "baz",
    "aadClientId": "fooClient",
    "aadClientSecret": "fooSecret",
    "resourceGroup": "bar",
    "securityGroupName": "foo-node-nsg",
    "securityGroupResourceGroup": "custom-vnet-resource-group",
    "location": "bar",
    "vmType": "vmss",
    "vnetName": "custom-vnet",
    "vnetResourceGroup": "custom-vnet-resource-group",
    "subnetName": "foo-node-subnet",
    "routeTableName": "foo-node-routetable",
    "loadBalancerSku": "Standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`
	rateLimitsControlPlaneCloudConfig = `{
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
    "useInstanceMetadata": true,
    "cloudProviderRateLimit": true,
    "cloudProviderRateLimitQPS": 1.2,
    "loadBalancerRateLimit": {
        "cloudProviderRateLimitBucket": 10
    }
}`
	rateLimitsWorkerNodeCloudConfig = `{
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
    "useInstanceMetadata": true,
    "cloudProviderRateLimit": true,
    "cloudProviderRateLimitQPS": 1.2,
    "loadBalancerRateLimit": {
        "cloudProviderRateLimitBucket": 10
    }
}`
	backOffCloudConfig = `{
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
    "useInstanceMetadata": true,
    "cloudProviderBackoff": true,
    "cloudProviderBackoffRetries": 1,
    "cloudProviderBackoffExponent": 1.2000000000000002,
    "cloudProviderBackoffDuration": 60,
    "cloudProviderBackoffJitter": 1.2000000000000002
}`
)
