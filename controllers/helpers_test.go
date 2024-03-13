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

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	cpName      = "my-managed-cp"
	clusterName = "my-cluster"
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

	requests := mapper(context.TODO(), &infrav1.AzureCluster{
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
	_ = corev1.AddToScheme(scheme)

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
			expectedControlPlaneConfig: vmssCloudConfig,
			expectedWorkerNodeConfig:   vmssCloudConfig,
		},
	}

	os.Setenv("AZURE_CLIENT_ID", "fooClient")
	os.Setenv("AZURE_CLIENT_SECRET", "fooSecret")
	os.Setenv("AZURE_TENANT_ID", "fooTenant")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fakeIdentity := &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-identity",
					Namespace: "default",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ServicePrincipal,
					ClientID:     "fooClient",
					TenantID:     "fooTenant",
					ClientSecret: corev1.SecretReference{Name: "fooSecret", Namespace: "default"},
				},
			}
			fakeSecret := getASOSecret(tc.cluster, func(s *corev1.Secret) {
				s.ObjectMeta.Name = "fooSecret"
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AZURE_CLIENT_SECRET":   []byte("fooSecret"),
					"clientSecret":          []byte("fooSecret"),
				}
			})

			initObjects := []runtime.Object{tc.cluster, tc.azureCluster, fakeIdentity, fakeSecret}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()
			resultSecret := &corev1.Secret{}
			key := client.ObjectKey{Name: fakeSecret.Name, Namespace: fakeSecret.Namespace}
			g.Expect(fakeClient.Get(context.Background(), key, resultSecret)).To(Succeed())

			clusterScope, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
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
			kind:       infrav1.AzureMachinePoolKind,
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

	fakeIdentity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:     infrav1.ServicePrincipal,
			TenantID: "fake-tenantid",
		},
	}
	fakeSecret := &corev1.Secret{Data: map[string][]byte{"clientSecret": []byte("fooSecret")}}
	initObjects := []runtime.Object{fakeIdentity, fakeSecret}

	scheme := setupScheme(g)
	kubeclient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	clusterScope, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
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
				clusterv1.ClusterNameLabel: clusterName,
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
				IdentityRef: &corev1.ObjectReference{
					Kind:      "AzureClusterIdentity",
					Name:      "fake-identity",
					Namespace: "default",
				},
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
				IdentityRef: &corev1.ObjectReference{
					Kind:      "AzureClusterIdentity",
					Name:      "fake-identity",
					Namespace: "default",
				},
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true,
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": true,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true,
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true,
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true,
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
    "loadBalancerName": "",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true,
    "enableVmssFlexNodes": true,
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
    "loadBalancerName": "",
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

func TestAzureManagedClusterToAzureManagedMachinePoolsMapper(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())
	initObjects := []runtime.Object{
		newCluster(clusterName),
		// Create two Machines with an infrastructure ref and one without.
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-2"),
		newMachinePool(clusterName, "my-machine-2"),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().Enabled(4).Return(true)
	sink.EXPECT().WithValues("AzureManagedCluster", "my-cluster", "Namespace", "default").Return(sink)
	sink.EXPECT().Info(4, "gk does not match", "gk", gomock.Any(), "infraGK", gomock.Any())
	mapper, err := AzureManagedClusterToAzureManagedMachinePoolsMapper(context.Background(), fakeClient, scheme, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper(context.TODO(), &infrav1.AzureManagedCluster{
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
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-0",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-1",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-2",
				Namespace: "default",
			},
		},
	}))
}

func TestAzureManagedControlPlaneToAzureManagedMachinePoolsMapper(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())
	cluster := newCluster("my-cluster")
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       infrav1.AzureManagedControlPlaneKind,
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}
	initObjects := []runtime.Object{
		cluster,
		newAzureManagedControlPlane(cpName),
		// Create two Machines with an infrastructure ref and one without.
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-2"),
		newMachinePool(clusterName, "my-machine-2"),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().Enabled(4).Return(true)
	sink.EXPECT().WithValues("AzureManagedControlPlane", cpName, "Namespace", cluster.Namespace).Return(sink)
	sink.EXPECT().Info(4, "gk does not match", "gk", gomock.Any(), "infraGK", gomock.Any())
	mapper, err := AzureManagedControlPlaneToAzureManagedMachinePoolsMapper(context.Background(), fakeClient, scheme, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper(context.TODO(), &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpName,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	})
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-0",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-1",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-2",
				Namespace: "default",
			},
		},
	}))
}

func TestMachinePoolToAzureManagedControlPlaneMapFuncSuccess(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())
	cluster := newCluster(clusterName)
	controlPlane := newAzureManagedControlPlane(cpName)
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       infrav1.AzureManagedControlPlaneKind,
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}

	managedMachinePool0 := newManagedMachinePoolInfraReference(clusterName, "my-mmp-0")
	azureManagedMachinePool0 := newAzureManagedMachinePool(clusterName, "azuremy-mmp-0", "System")
	managedMachinePool0.Spec.ClusterName = clusterName

	managedMachinePool1 := newManagedMachinePoolInfraReference(clusterName, "my-mmp-1")
	azureManagedMachinePool1 := newAzureManagedMachinePool(clusterName, "azuremy-mmp-1", "User")
	managedMachinePool1.Spec.ClusterName = clusterName

	initObjects := []runtime.Object{
		cluster,
		controlPlane,
		managedMachinePool0,
		azureManagedMachinePool0,
		// Create two Machines with an infrastructure ref and one without.
		managedMachinePool1,
		azureManagedMachinePool1,
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	mapper := MachinePoolToAzureManagedControlPlaneMapFunc(context.Background(), fakeClient, infrav1.GroupVersion.WithKind(infrav1.AzureManagedControlPlaneKind), logr.New(sink))

	// system pool should trigger
	requests := mapper(context.TODO(), newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"))
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "my-managed-cp",
				Namespace: "default",
			},
		},
	}))

	// any other pool should not trigger
	requests = mapper(context.TODO(), newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"))
	g.Expect(requests).To(BeNil())
}

func TestMachinePoolToAzureManagedControlPlaneMapFuncFailure(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())
	cluster := newCluster(clusterName)
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       infrav1.AzureManagedControlPlaneKind,
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}
	managedMachinePool := newManagedMachinePoolInfraReference(clusterName, "my-mmp-0")
	managedMachinePool.Spec.ClusterName = clusterName
	initObjects := []runtime.Object{
		cluster,
		managedMachinePool,
		// Create two Machines with an infrastructure ref and one without.
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().Error(gomock.Any(), "failed to fetch default pool reference")
	sink.EXPECT().Error(gomock.Any(), "failed to fetch default pool reference") // twice because we are testing two calls

	mapper := MachinePoolToAzureManagedControlPlaneMapFunc(context.Background(), fakeClient, infrav1.GroupVersion.WithKind(infrav1.AzureManagedControlPlaneKind), logr.New(sink))

	// default pool should trigger if owned cluster could not be fetched
	requests := mapper(context.TODO(), newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"))
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "my-managed-cp",
				Namespace: "default",
			},
		},
	}))

	// any other pool should also trigger if owned cluster could not be fetched
	requests = mapper(context.TODO(), newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"))
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "my-managed-cp",
				Namespace: "default",
			},
		},
	}))
}

func TestAzureManagedClusterToAzureManagedControlPlaneMapper(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())
	cluster := newCluster("my-cluster")
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       infrav1.AzureManagedControlPlaneKind,
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}

	initObjects := []runtime.Object{
		cluster,
		newAzureManagedControlPlane(cpName),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().WithValues("AzureManagedCluster", "az-"+cluster.Name, "Namespace", "default")

	mapper, err := AzureManagedClusterToAzureManagedControlPlaneMapper(context.Background(), fakeClient, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())
	requests := mapper(context.TODO(), &infrav1.AzureManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "az-" + cluster.Name,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	})
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests).To(Equal([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      cpName,
				Namespace: cluster.Namespace,
			},
		},
	}))
}

func TestAzureManagedControlPlaneToAzureManagedClusterMapper(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())
	cluster := newCluster("my-cluster")
	azManagedCluster := &infrav1.AzureManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "az-" + cluster.Name,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	}

	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       infrav1.AzureManagedControlPlaneKind,
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}
	cluster.Spec.InfrastructureRef = &corev1.ObjectReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       infrav1.AzureManagedClusterKind,
		Name:       azManagedCluster.Name,
		Namespace:  azManagedCluster.Namespace,
	}

	initObjects := []runtime.Object{
		cluster,
		newAzureManagedControlPlane(cpName),
		azManagedCluster,
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().WithValues("AzureManagedControlPlane", cpName, "Namespace", cluster.Namespace)

	mapper, err := AzureManagedControlPlaneToAzureManagedClusterMapper(context.Background(), fakeClient, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())
	requests := mapper(context.TODO(), &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpName,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	})
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests).To(Equal([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      azManagedCluster.Name,
				Namespace: azManagedCluster.Namespace,
			},
		},
	}))
}

func newAzureManagedControlPlane(cpName string) *infrav1.AzureManagedControlPlane {
	return &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpName,
			Namespace: "default",
		},
	}
}

func newManagedMachinePoolInfraReference(clusterName, poolName string) *expv1.MachinePool {
	m := newMachinePool(clusterName, poolName)
	m.Spec.ClusterName = clusterName
	m.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "AzureManagedMachinePool",
		Namespace:  m.Namespace,
		Name:       "azure" + poolName,
		APIVersion: infrav1.GroupVersion.String(),
	}
	return m
}

func newAzureManagedMachinePool(clusterName, poolName, mode string) *infrav1.AzureManagedMachinePool {
	var cpuManagerPolicyStatic = infrav1.CPUManagerPolicyStatic
	var topologyManagerPolicy = infrav1.TopologyManagerPolicyBestEffort
	var transparentHugePageDefragMAdvise = infrav1.TransparentHugePageOptionMadvise
	var transparentHugePageEnabledAlways = infrav1.TransparentHugePageOptionAlways
	return &infrav1.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
		Spec: infrav1.AzureManagedMachinePoolSpec{
			AzureManagedMachinePoolClassSpec: infrav1.AzureManagedMachinePoolClassSpec{
				Mode:         mode,
				SKU:          "Standard_B2s",
				OSDiskSizeGB: ptr.To(512),
				KubeletConfig: &infrav1.KubeletConfig{
					CPUManagerPolicy:      &cpuManagerPolicyStatic,
					TopologyManagerPolicy: &topologyManagerPolicy,
				},
				LinuxOSConfig: &infrav1.LinuxOSConfig{
					TransparentHugePageDefrag:  &transparentHugePageDefragMAdvise,
					TransparentHugePageEnabled: &transparentHugePageEnabledAlways,
				},
			},
		},
	}
}

func newMachinePool(clusterName, poolName string) *expv1.MachinePool {
	return &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
		Spec: expv1.MachinePoolSpec{
			Replicas: ptr.To[int32](2),
		},
	}
}

func newManagedMachinePoolWithInfrastructureRef(clusterName, poolName string) *expv1.MachinePool {
	m := newMachinePool(clusterName, poolName)
	m.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "AzureManagedMachinePool",
		Namespace:  m.Namespace,
		Name:       "azure" + poolName,
		APIVersion: infrav1.GroupVersion.String(),
	}
	return m
}

func Test_ManagedMachinePoolToInfrastructureMapFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(logMock *mock_log.MockLogSink)
		MapObjectFactory func(*GomegaWithT) client.Object
		Expect           func(*GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "MachinePoolToAzureManagedMachinePool",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newManagedMachinePoolWithInfrastructureRef("azureManagedCluster", "ManagedMachinePool")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(1))
				g.Expect(reqs[0]).To(Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "azureManagedMachinePool",
						Namespace: "default",
					},
				}))
			},
		},
		{
			Name: "MachinePoolWithoutMatchingInfraRef",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newMachinePool("azureManagedCluster", "machinePool")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				ampGK := infrav1.GroupVersion.WithKind("AzureManagedMachinePool").GroupKind()
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				logMock.EXPECT().Enabled(4).Return(true)
				logMock.EXPECT().Info(4, "gk does not match", "gk", ampGK, "infraGK", gomock.Any())
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
		{
			Name: "NotAMachinePool",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newCluster("azureManagedCluster")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				logMock.EXPECT().Enabled(4).Return(true)
				logMock.EXPECT().Info(4, "attempt to map incorrect type", "type", "*v1beta1.Cluster")
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			sink := mock_log.NewMockLogSink(mockCtrl)
			if c.Setup != nil {
				c.Setup(sink)
			}
			f := MachinePoolToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureManagedMachinePool"), logr.New(sink))
			reqs := f(context.TODO(), c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

func TestClusterPauseChangeAndInfrastructureReady(t *testing.T) {
	tests := []struct {
		name   string
		event  any // an event.(Create|Update)Event
		expect bool
	}{
		{
			name: "create cluster infra not ready, not paused",
			event: event.CreateEvent{
				Object: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: false,
					},
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: false,
					},
				},
			},
			expect: false,
		},
		{
			name: "create cluster infra ready, not paused",
			event: event.CreateEvent{
				Object: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: false,
					},
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
			},
			expect: true,
		},
		{
			name: "create cluster infra not ready, paused",
			event: event.CreateEvent{
				Object: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: true,
					},
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: false,
					},
				},
			},
			expect: false,
		},
		{
			name: "create cluster infra ready, paused",
			event: event.CreateEvent{
				Object: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: true,
					},
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
			},
			expect: true,
		},
		{
			name: "update cluster infra ready true->true",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
			},
			expect: false,
		},
		{
			name: "update cluster infra ready false->true",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: false,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
			},
			expect: true,
		},
		{
			name: "update cluster infra ready true->false",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: false,
					},
				},
			},
			expect: false,
		},
		{
			name: "update cluster infra ready false->false",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: false,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: false,
					},
				},
			},
			expect: false,
		},
		{
			name: "update cluster paused false->false",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: false,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: false,
					},
				},
			},
			expect: false,
		},
		{
			name: "update cluster paused false->true",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: false,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: true,
					},
				},
			},
			expect: true,
		},
		{
			name: "update cluster paused true->false",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: true,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: false,
					},
				},
			},
			expect: true,
		},
		{
			name: "update cluster paused true->true",
			event: event.UpdateEvent{
				ObjectOld: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: true,
					},
				},
				ObjectNew: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Paused: true,
					},
				},
			},
			expect: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			p := ClusterPauseChangeAndInfrastructureReady(logr.New(nil))
			var actual bool
			switch e := test.event.(type) {
			case event.CreateEvent:
				actual = p.Create(e)
			case event.UpdateEvent:
				actual = p.Update(e)
			default:
				panic("unimplemented event type")
			}
			NewGomegaWithT(t).Expect(actual).To(Equal(test.expect))
		})
	}
}

func TestAddBlockMoveAnnotation(t *testing.T) {
	tests := []struct {
		name                string
		annotations         map[string]string
		expectedAnnotations map[string]string
		expected            bool
	}{
		{
			name:                "annotation does not exist",
			annotations:         nil,
			expectedAnnotations: map[string]string{clusterctlv1.BlockMoveAnnotation: "true"},
			expected:            true,
		},
		{
			name:                "annotation already exists",
			annotations:         map[string]string{clusterctlv1.BlockMoveAnnotation: "this value might be different but it doesn't matter"},
			expectedAnnotations: map[string]string{clusterctlv1.BlockMoveAnnotation: "this value might be different but it doesn't matter"},
			expected:            false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obj := &metav1.ObjectMeta{
				Annotations: test.annotations,
			}
			actual := AddBlockMoveAnnotation(obj)
			if test.expected != actual {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
			if !maps.Equal(test.expectedAnnotations, obj.GetAnnotations()) {
				t.Errorf("expected %v, got %v", test.expectedAnnotations, obj.GetAnnotations())
			}
		})
	}
}

func TestRemoveBlockMoveAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    map[string]string
	}{
		{
			name:        "nil",
			annotations: nil,
			expected:    nil,
		},
		{
			name:        "annotation not present",
			annotations: map[string]string{"another": "annotation"},
			expected:    map[string]string{"another": "annotation"},
		},
		{
			name: "annotation present",
			annotations: map[string]string{
				clusterctlv1.BlockMoveAnnotation: "any value",
				"another":                        "annotation",
			},
			expected: map[string]string{
				"another": "annotation",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obj := &metav1.ObjectMeta{
				Annotations: maps.Clone(test.annotations),
			}
			RemoveBlockMoveAnnotation(obj)
			actual := obj.GetAnnotations()
			if !maps.Equal(test.expected, actual) {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}
