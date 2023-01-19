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
	"strings"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	utilfeature "k8s.io/component-base/featuregate/testing"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	sink := mock_log.NewMockLogSink(mockCtrl)
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().WithValues("AzureCluster", "my-cluster", "Namespace", "default")
	mapper, err := AzureClusterToAzureMachinesMapper(context.Background(), client, &infrav1.AzureMachine{}, scheme, logr.New(sink))
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
	azureCluster := newAzureCluster("bar")
	azureCluster.Default()
	azureClusterCustomVnet := newAzureClusterWithCustomVnet("bar")
	azureClusterCustomVnet.Default()

	cases := map[string]struct {
		cluster                    *clusterv1.Cluster
		azureCluster               *infrav1.AzureCluster
		identityType               infrav1.VMIdentity
		identityID                 string
		machinePoolFeature         bool
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
		"with machinepools": {
			cluster:                    cluster,
			azureCluster:               azureCluster,
			identityType:               infrav1.VMIdentityNone,
			machinePoolFeature:         true,
			expectedControlPlaneConfig: vmssCloudConfig,
			expectedWorkerNodeConfig:   vmssCloudConfig,
		},
	}

	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.machinePoolFeature {
				defer utilfeature.SetFeatureGateDuringTest(t, capifeature.Gates, capifeature.MachinePool, true)()
			}
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
		kind             string
		apiVersion       string
		ownerName        string
		existingSecret   *corev1.Secret
		expectedNoChange bool
	}{
		"azuremachine should reconcile secret successfully": {
			kind:       "AzureMachine",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			ownerName:  "azureMachineName",
		},
		"azuremachinepool should reconcile secret successfully": {
			kind:       "AzureMachinePool",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			ownerName:  "azureMachinePoolName",
		},
		"azuremachinetemplate should reconcile secret successfully": {
			kind:       "AzureMachineTemplate",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			ownerName:  "azureMachineTemplateName",
		},
		"should not replace the content of the pre-existing unowned secret": {
			kind:       "AzureMachine",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			ownerName:  "azureMachineName",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "azureMachineName-azure-json",
					Namespace: "default",
					Labels:    map[string]string{"testCluster": "foo"},
				},
				Data: map[string][]byte{
					"azure.json": []byte("foobar"),
				},
			},
			expectedNoChange: true,
		},
		"should not replace the content of the pre-existing unowned secret without the label": {
			kind:       "AzureMachine",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			ownerName:  "azureMachineName",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "azureMachineName-azure-json",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"azure.json": []byte("foobar"),
				},
			},
			expectedNoChange: true,
		},
		"should replace the content of the pre-existing owned secret": {
			kind:       "AzureMachine",
			apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			ownerName:  "azureMachineName",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "azureMachineName-azure-json",
					Namespace: "default",
					Labels:    map[string]string{"testCluster": string(infrav1.ResourceLifecycleOwned)},
				},
				Data: map[string][]byte{
					"azure.json": []byte("foobar"),
				},
			},
		},
	}

	cluster := newCluster("foo")
	azureCluster := newAzureCluster("bar")

	azureCluster.Default()
	cluster.Name = "testCluster"

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
			if tc.existingSecret != nil {
				_ = kubeclient.Delete(context.Background(), tc.existingSecret)
				_ = kubeclient.Create(context.Background(), tc.existingSecret)
				defer func() {
					_ = kubeclient.Delete(context.Background(), tc.existingSecret)
				}()
			}

			owner := metav1.OwnerReference{
				APIVersion: tc.apiVersion,
				Kind:       tc.kind,
				Name:       tc.ownerName,
			}
			cloudConfig, err := GetCloudProviderSecret(clusterScope, "default", tc.ownerName, owner, infrav1.VMIdentitySystemAssigned, "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cloudConfig.Data).NotTo(BeNil())

			if err := reconcileAzureSecret(context.Background(), kubeclient, owner, cloudConfig, cluster.Name); err != nil {
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

			if tc.expectedNoChange {
				g.Expect(cloudConfig.Data).NotTo(Equal(found.Data))
			} else {
				g.Expect(cloudConfig.Data).To(Equal(found.Data))
				g.Expect(found.OwnerReferences).To(Equal(cloudConfig.OwnerReferences))
			}
		})
	}
}

func setupScheme(g *WithT) *runtime.Scheme {
	scheme := runtime.NewScheme()
	g.Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
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

func newAzureCluster(location string) *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				Location:       location,
				SubscriptionID: "baz",
			},
			NetworkSpec: infrav1.NetworkSpec{
				Vnet: infrav1.VnetSpec{},
			},
			ResourceGroup: "bar",
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

func newAzureClusterWithCustomVnet(location string) *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				Location:       location,
				SubscriptionID: "baz",
			},
			NetworkSpec: infrav1.NetworkSpec{
				Vnet: infrav1.VnetSpec{
					Name:          "custom-vnet",
					ResourceGroup: "custom-vnet-resource-group",
				},
				Subnets: infrav1.Subnets{
					infrav1.SubnetSpec{
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Name: "foo-controlplane-subnet",
							Role: infrav1.SubnetControlPlane,
						},
					},
					infrav1.SubnetSpec{
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Name: "foo-node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				},
			},
			ResourceGroup: "bar",
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
    "loadBalancerName": "foo",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`
	//nolint:gosec // Ignore "G101: Potential hardcoded credentials" check.
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
    "loadBalancerName": "foo",
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
    "loadBalancerName": "foo",
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
    "loadBalancerName": "foo",
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
    "loadBalancerName": "foo",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "userAssignedIdentityID": "foobar"
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
    "loadBalancerName": "foo",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "userAssignedIdentityID": "foobar"
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
    "loadBalancerName": "foo",
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
    "loadBalancerName": "foo",
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
    "loadBalancerName": "foo",
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
    "loadBalancerName": "foo",
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
    "loadBalancerName": "foo",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "cloudProviderBackoff": true,
    "cloudProviderBackoffRetries": 1,
    "cloudProviderBackoffExponent": 1.2000000000000002,
    "cloudProviderBackoffDuration": 60,
    "cloudProviderBackoffJitter": 1.2000000000000002
}`
	vmssCloudConfig = `{
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
    "loadBalancerName": "foo",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true
}`
)

func Test_clusterIdentityFinalizer(t *testing.T) {
	type args struct {
		prefix           string
		clusterNamespace string
		clusterName      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "cluster identity finalizer should be deterministic",
			args: args{
				prefix:           infrav1.ClusterFinalizer,
				clusterNamespace: "foo",
				clusterName:      "bar",
			},
			want: "azurecluster.infrastructure.cluster.x-k8s.io/48998dbcd8fb929369c78981cbfb6f26145ea0412e6e05a1423941a6",
		},
		{
			name: "long cluster name and namespace",
			args: args{
				prefix:           infrav1.ClusterFinalizer,
				clusterNamespace: "this-is-a-very-very-very-very-very-very-very-very-very-long-namespace-name",
				clusterName:      "this-is-a-very-very-very-very-very-very-very-very-very-long-cluster-name",
			},
			want: "azurecluster.infrastructure.cluster.x-k8s.io/557d064144d2b495db694dedc53c9a1e9bd8575bdf06b5b151972614",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clusterIdentityFinalizer(tt.args.prefix, tt.args.clusterNamespace, tt.args.clusterName)
			if got != tt.want {
				t.Errorf("clusterIdentityFinalizer() = %v, want %v", got, tt.want)
			}
			key := strings.Split(got, "/")[1]
			if len(key) > 63 {
				t.Errorf("clusterIdentityFinalizer() name %v length = %v should be less than 63 characters", key, len(key))
			}
		})
	}
}

func Test_deprecatedClusterIdentityFinalizer(t *testing.T) {
	type args struct {
		prefix           string
		clusterNamespace string
		clusterName      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "cluster identity finalizer should be deterministic",
			args: args{
				prefix:           infrav1.ClusterFinalizer,
				clusterNamespace: "foo",
				clusterName:      "bar",
			},
			want: "azurecluster.infrastructure.cluster.x-k8s.io/foo-bar",
		},
		{
			name: "long cluster name and namespace",
			args: args{
				prefix:           infrav1.ClusterFinalizer,
				clusterNamespace: "this-is-a-very-very-very-very-very-very-very-very-very-long-namespace-name",
				clusterName:      "this-is-a-very-very-very-very-very-very-very-very-very-long-cluster-name",
			},
			want: "azurecluster.infrastructure.cluster.x-k8s.io/this-is-a-very-very-very-very-very-very-very-very-very-long-namespace-name-this-is-a-very-very-very-very-very-very-very-very-very-long-cluster-name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deprecatedClusterIdentityFinalizer(tt.args.prefix, tt.args.clusterNamespace, tt.args.clusterName); got != tt.want {
				t.Errorf("deprecatedClusterIdentityFinalizer() = %v, want %v", got, tt.want)
			}
		})
	}
}
