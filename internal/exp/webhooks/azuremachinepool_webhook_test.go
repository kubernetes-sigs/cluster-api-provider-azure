/*
Copyright The Kubernetes Authors.

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

package webhooks

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	guuid "github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	apiinternal "sigs.k8s.io/cluster-api-provider-azure/internal/api/v1beta1"
	apifixtures "sigs.k8s.io/cluster-api-provider-azure/internal/test/apifixtures"
)

var (
	validSSHPublicKey = apifixtures.GenerateSSHPublicKey(true)
	zero              = intstr.FromInt(0)
	one               = intstr.FromInt(1)
)

type mockClient struct {
	client.Client
	Version     string
	ReturnError bool
}

func (m mockClient) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	obj.(*clusterv1.MachinePool).Spec.Template.Spec.Version = m.Version
	return nil
}

func (m mockClient) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	if m.ReturnError {
		return errors.New("MachinePool.cluster.x-k8s.io \"mock-machinepool-mp-0\" not found")
	}
	mp := &clusterv1.MachinePool{}
	mp.Spec.Template.Spec.Version = m.Version
	list.(*clusterv1.MachinePoolList).Items = []clusterv1.MachinePool{*mp}

	return nil
}

func TestAzureMachinePool_ValidateCreate(t *testing.T) {
	tests := []struct {
		name          string
		amp           *infrav1exp.AzureMachinePool
		version       string
		ownerNotFound bool
		wantErr       bool
	}{
		{
			name:    "valid",
			amp:     getKnownValidAzureMachinePool(),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - full",
			amp:     createMachinePoolWithMarketPlaceImage("PUB1234", "OFFER1234", "SKU1234", "1.0.0", ptr.To(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing publisher",
			amp:     createMachinePoolWithMarketPlaceImage("", "OFFER1234", "SKU1234", "1.0.0", ptr.To(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with shared gallery image - full",
			amp:     createMachinePoolWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", ptr.To(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing subscription",
			amp:     createMachinePoolWithSharedImage("", "RG123", "NAME123", "GALLERY1", "1.0.0", ptr.To(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with image by - with id",
			amp:     createMachinePoolWithImageByID("ID123", ptr.To(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with image by - without id",
			amp:     createMachinePoolWithImageByID("", ptr.To(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with valid SSHPublicKey",
			amp:     createMachinePoolWithSSHPublicKey(validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with invalid SSHPublicKey",
			amp:     createMachinePoolWithSSHPublicKey("invalid ssh key"),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with wrong terminate notification",
			amp:     createMachinePoolWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", ptr.To(35)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with system assigned identity",
			amp:     createMachinePoolWithSystemAssignedIdentity(string(uuid.NewUUID())),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with system assigned identity, but invalid role",
			amp:     createMachinePoolWithSystemAssignedIdentity("not_a_uuid"),
			wantErr: true,
		},
		{
			name: "azuremachinepool with user assigned identity",
			amp: createMachinePoolWithUserAssignedIdentity([]string{
				"azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-20202-control-plane-7w265",
				"azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-20202-control-plane-a6b7d",
			}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with user assigned identity, but without any provider ids",
			amp:     createMachinePoolWithUserAssignedIdentity([]string{}),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with managed diagnostics profile",
			amp:     createMachinePoolWithDiagnostics(infrav1.ManagedDiagnosticsStorage, nil),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with disabled diagnostics profile",
			amp:     createMachinePoolWithDiagnostics(infrav1.ManagedDiagnosticsStorage, nil),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with user managed diagnostics profile and defined user managed storage account",
			amp:     createMachinePoolWithDiagnostics(infrav1.UserManagedDiagnosticsStorage, &infrav1.UserManagedBootDiagnostics{StorageAccountURI: "https://fakeurl"}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with empty diagnostics profile",
			amp:     createMachinePoolWithDiagnostics("", nil),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with user managed diagnostics profile, but empty user managed storage account",
			amp:     createMachinePoolWithDiagnostics(infrav1.UserManagedDiagnosticsStorage, nil),
			wantErr: true,
		},
		{
			name: "azuremachinepool with invalid MaxSurge and MaxUnavailable rolling upgrade configuration",
			amp: createMachinePoolWithStrategy(infrav1exp.AzureMachinePoolDeploymentStrategy{
				Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &zero,
				},
			}),
			wantErr: true,
		},
		{
			name: "azuremachinepool with valid MaxSurge and MaxUnavailable rolling upgrade configuration",
			amp: createMachinePoolWithStrategy(infrav1exp.AzureMachinePoolDeploymentStrategy{
				Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &one,
				},
			}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with valid legacy network configuration",
			amp:     createMachinePoolWithNetworkConfig("testSubnet", []infrav1.NetworkInterface{}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with invalid legacy network configuration",
			amp:     createMachinePoolWithNetworkConfig("testSubnet", []infrav1.NetworkInterface{{SubnetName: "testSubnet"}}),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with valid networkinterface configuration",
			amp:     createMachinePoolWithNetworkConfig("", []infrav1.NetworkInterface{{SubnetName: "testSubnet"}}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with Flexible orchestration mode",
			amp:     createMachinePoolWithOrchestrationMode(armcompute.OrchestrationModeFlexible),
			version: "v1.26.0",
			wantErr: false,
		},
		{
			name:    "azuremachinepool with Flexible orchestration mode and invalid Kubernetes version",
			amp:     createMachinePoolWithOrchestrationMode(armcompute.OrchestrationModeFlexible),
			version: "v1.25.6",
			wantErr: true,
		},
		{
			name:          "azuremachinepool with Flexible orchestration mode and invalid Kubernetes version, no owner",
			amp:           createMachinePoolWithOrchestrationMode(armcompute.OrchestrationModeFlexible),
			version:       "v1.25.6",
			ownerNotFound: true,
			wantErr:       true,
		},
		{
			name: "azuremachinepool with invalid DiffDiskSettings",
			amp: createMachinePoolWithDiffDiskSettings(infrav1.DiffDiskSettings{
				Placement: ptr.To(infrav1.DiffDiskPlacementResourceDisk),
			}),
			wantErr: true,
		},
		{
			name: "azuremachinepool with valid DiffDiskSettings",
			amp: createMachinePoolWithDiffDiskSettings(infrav1.DiffDiskSettings{
				Option:    string(armcompute.DiffDiskOptionsLocal),
				Placement: ptr.To(infrav1.DiffDiskPlacementResourceDisk),
			}),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		c := mockClient{Version: tc.version, ReturnError: tc.ownerNotFound}
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ampw := &AzureMachinePoolWebhook{
				Client: c,
			}
			_, err := ampw.ValidateCreate(t.Context(), tc.amp)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

type mockDefaultClient struct {
	client.Client
	Name           string
	ClusterName    string
	SubscriptionID string
	Version        string
	ReturnError    bool
}

func (m mockDefaultClient) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	switch obj := obj.(type) {
	case *infrav1.AzureCluster:
		obj.Spec.SubscriptionID = m.SubscriptionID
	case *clusterv1.Cluster:
		obj.Spec.InfrastructureRef = clusterv1.ContractVersionedObjectReference{
			Kind: infrav1.AzureClusterKind,
			Name: "test-cluster",
		}
	default:
		return errors.New("invalid object type")
	}
	return nil
}

func (m mockDefaultClient) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	list.(*clusterv1.MachinePoolList).Items = []clusterv1.MachinePool{
		{
			Spec: clusterv1.MachinePoolSpec{
				Template: clusterv1.MachineTemplateSpec{
					Spec: clusterv1.MachineSpec{
						InfrastructureRef: clusterv1.ContractVersionedObjectReference{
							Name: m.Name,
						},
					},
				},
				ClusterName: m.ClusterName,
			},
		},
	}

	return nil
}

func TestAzureMachinePool_ValidateUpdate(t *testing.T) {
	var (
		zero = intstr.FromInt(0)
		one  = intstr.FromInt(1)
	)

	tests := []struct {
		name    string
		oldAMP  *infrav1exp.AzureMachinePool
		amp     *infrav1exp.AzureMachinePool
		wantErr bool
	}{
		{
			name:    "azuremachinepool with valid SSHPublicKey",
			oldAMP:  createMachinePoolWithSSHPublicKey(""),
			amp:     createMachinePoolWithSSHPublicKey(validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with invalid SSHPublicKey",
			oldAMP:  createMachinePoolWithSSHPublicKey(""),
			amp:     createMachinePoolWithSSHPublicKey("invalid ssh key"),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with system-assigned identity, and role unchanged",
			oldAMP:  createMachinePoolWithSystemAssignedIdentity("30a757d8-fcf0-4c8b-acf0-9253a7e093ea"),
			amp:     createMachinePoolWithSystemAssignedIdentity("30a757d8-fcf0-4c8b-acf0-9253a7e093ea"),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with system-assigned identity, and role changed",
			oldAMP:  createMachinePoolWithSystemAssignedIdentity(string(uuid.NewUUID())),
			amp:     createMachinePoolWithSystemAssignedIdentity(string(uuid.NewUUID())),
			wantErr: true,
		},
		{
			name:   "azuremachinepool with invalid MaxSurge and MaxUnavailable rolling upgrade configuration",
			oldAMP: createMachinePoolWithStrategy(infrav1exp.AzureMachinePoolDeploymentStrategy{}),
			amp: createMachinePoolWithStrategy(infrav1exp.AzureMachinePoolDeploymentStrategy{
				Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &zero,
				},
			}),
			wantErr: true,
		},
		{
			name:   "azuremachinepool with valid MaxSurge and MaxUnavailable rolling upgrade configuration",
			oldAMP: createMachinePoolWithStrategy(infrav1exp.AzureMachinePoolDeploymentStrategy{}),
			amp: createMachinePoolWithStrategy(infrav1exp.AzureMachinePoolDeploymentStrategy{
				Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &one,
				},
			}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with valid network interface config",
			oldAMP:  createMachinePoolWithNetworkConfig("", []infrav1.NetworkInterface{{SubnetName: "testSubnet"}}),
			amp:     createMachinePoolWithNetworkConfig("", []infrav1.NetworkInterface{{SubnetName: "testSubnet2"}}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with valid network interface config",
			oldAMP:  createMachinePoolWithNetworkConfig("", []infrav1.NetworkInterface{{SubnetName: "testSubnet"}}),
			amp:     createMachinePoolWithNetworkConfig("subnet", []infrav1.NetworkInterface{{SubnetName: "testSubnet2"}}),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with valid network interface config",
			oldAMP:  createMachinePoolWithNetworkConfig("subnet", []infrav1.NetworkInterface{}),
			amp:     createMachinePoolWithNetworkConfig("subnet", []infrav1.NetworkInterface{{SubnetName: "testSubnet2"}}),
			wantErr: true,
		},
		{
			name:    "azuremachinepool disableVMBootstrapExtension transition from nil to true is allowed",
			oldAMP:  createMachinePoolWithDisableVMBootstrapExtension(nil),
			amp:     createMachinePoolWithDisableVMBootstrapExtension(ptr.To(true)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool disableVMBootstrapExtension transition from nil to false is allowed",
			oldAMP:  createMachinePoolWithDisableVMBootstrapExtension(nil),
			amp:     createMachinePoolWithDisableVMBootstrapExtension(ptr.To(false)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool disableVMBootstrapExtension is immutable once explicitly set",
			oldAMP:  createMachinePoolWithDisableVMBootstrapExtension(ptr.To(true)),
			amp:     createMachinePoolWithDisableVMBootstrapExtension(ptr.To(false)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool disableVMBootstrapExtension same value passes immutability",
			oldAMP:  createMachinePoolWithDisableVMBootstrapExtension(ptr.To(true)),
			amp:     createMachinePoolWithDisableVMBootstrapExtension(ptr.To(true)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool disableVMBootstrapExtension cannot be unset once explicitly set",
			oldAMP:  createMachinePoolWithDisableVMBootstrapExtension(ptr.To(true)),
			amp:     createMachinePoolWithDisableVMBootstrapExtension(nil),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ampw := &AzureMachinePoolWebhook{}
			_, err := ampw.ValidateUpdate(t.Context(), tc.oldAMP, tc.amp)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachinePool_Default(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		amp *infrav1exp.AzureMachinePool
	}

	existingPublicKey := validSSHPublicKey
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey("")}

	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"

	fakeSubscriptionID := guuid.New().String()
	fakeClusterName := "testcluster"
	fakeMachinePoolName := "testmachinepool"
	c := mockDefaultClient{Name: fakeMachinePoolName, ClusterName: fakeClusterName, SubscriptionID: fakeSubscriptionID}

	roleAssignmentExistTest := test{amp: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Identity: "SystemAssigned",
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         existingRoleAssignmentName,
				Scope:        "scope",
				DefinitionID: "definitionID",
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeMachinePoolName,
		},
	}}

	emptyTest := test{amp: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Identity:                   "SystemAssigned",
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeMachinePoolName,
		},
	}}

	systemAssignedIdentityRoleExistTest := test{amp: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Identity: "SystemAssigned",
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				DefinitionID: "testroledefinitionid",
				Scope:        "testscope",
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeMachinePoolName,
		},
	}}

	ampw := &AzureMachinePoolWebhook{
		Client: c,
	}

	err := ampw.Default(t.Context(), roleAssignmentExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(roleAssignmentExistTest.amp.Spec.SystemAssignedIdentityRole.Name).To(Equal(existingRoleAssignmentName))

	err = ampw.Default(t.Context(), publicKeyExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	err = ampw.Default(t.Context(), publicKeyNotExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyNotExistTest.amp.Spec.Template.SSHPublicKey).NotTo(BeEmpty())

	err = ampw.Default(t.Context(), systemAssignedIdentityRoleExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(systemAssignedIdentityRoleExistTest.amp.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal("testroledefinitionid"))
	g.Expect(systemAssignedIdentityRoleExistTest.amp.Spec.SystemAssignedIdentityRole.Scope).To(Equal("testscope"))

	err = ampw.Default(t.Context(), emptyTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole.Name).To(Not(BeEmpty()))
	g.Expect(guuid.Validate(emptyTest.amp.Spec.SystemAssignedIdentityRole.Name)).To(Succeed())
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole).To(Not(BeNil()))
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole.Scope).To(Equal(fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID)))
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal(fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, apiinternal.ContributorRoleID)))
}

func createMachinePoolWithMarketPlaceImage(publisher, offer, sku, version string, terminateNotificationTimeout *int) *infrav1exp.AzureMachinePool {
	image := infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			ImagePlan: infrav1.ImagePlan{
				Publisher: publisher,
				Offer:     offer,
				SKU:       sku,
			},
			Version: version,
		},
	}

	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithSharedImage(subscriptionID, resourceGroup, name, gallery, version string, terminateNotificationTimeout *int) *infrav1exp.AzureMachinePool {
	image := infrav1.Image{
		SharedGallery: &infrav1.AzureSharedGalleryImage{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Name:           name,
			Gallery:        gallery,
			Version:        version,
		},
	}

	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithNetworkConfig(subnetName string, interfaces []infrav1.NetworkInterface) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				SubnetName:        subnetName,
				NetworkInterfaces: interfaces,
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithDisableVMBootstrapExtension(disable *bool) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				SSHPublicKey:                validSSHPublicKey,
				DisableVMBootstrapExtension: disable,
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithImageByID(imageID string, terminateNotificationTimeout *int) *infrav1exp.AzureMachinePool {
	image := infrav1.Image{
		ID: &imageID,
	}

	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithSSHPublicKey(sshPublicKey string) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				SSHPublicKey: sshPublicKey,
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "testmachinepool",
		},
	}
}

func createMachinePoolWithSystemAssignedIdentity(role string) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Identity: infrav1.VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         role,
				Scope:        "scope",
				DefinitionID: "definitionID",
			},
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithDiagnostics(diagnosticsType infrav1.BootDiagnosticsStorageAccountType, userManaged *infrav1.UserManagedBootDiagnostics) *infrav1exp.AzureMachinePool {
	var diagnostics *infrav1.Diagnostics

	if diagnosticsType != "" {
		diagnostics = &infrav1.Diagnostics{
			Boot: &infrav1.BootDiagnostics{
				StorageAccountType: diagnosticsType,
			},
		}
	}

	if userManaged != nil {
		diagnostics.Boot.UserManaged = userManaged
	}

	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Diagnostics: diagnostics,
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithUserAssignedIdentity(providerIDs []string) *infrav1exp.AzureMachinePool {
	userAssignedIdentities := make([]infrav1.UserAssignedIdentity, len(providerIDs))

	for _, providerID := range providerIDs {
		userAssignedIdentities = append(userAssignedIdentities, infrav1.UserAssignedIdentity{
			ProviderID: providerID,
		})
	}

	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Identity:               infrav1.VMIdentityUserAssigned,
			UserAssignedIdentities: userAssignedIdentities,
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithStrategy(strategy infrav1exp.AzureMachinePoolDeploymentStrategy) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Strategy: strategy,
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithOrchestrationMode(mode armcompute.OrchestrationMode) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			OrchestrationMode: infrav1.OrchestrationModeType(mode),
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
		},
	}
}

func createMachinePoolWithDiffDiskSettings(settings infrav1.DiffDiskSettings) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				OSDisk: infrav1.OSDisk{
					DiffDiskSettings: &settings,
				},
			},
		},
	}
}

func TestAzureMachinePool_ValidateCreateFailure(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name               string
		amp                *infrav1exp.AzureMachinePool
		featureGateEnabled *bool
		expectError        bool
	}{
		{
			name:               "feature gate implicitly enabled",
			amp:                getKnownValidAzureMachinePool(),
			featureGateEnabled: nil,
			expectError:        false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.featureGateEnabled != nil {
				utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, *tc.featureGateEnabled)
			}
			ampw := &AzureMachinePoolWebhook{}
			_, err := ampw.ValidateCreate(t.Context(), tc.amp)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getKnownValidAzureMachinePool() *infrav1exp.AzureMachinePool {
	image := infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			ImagePlan: infrav1.ImagePlan{
				Publisher: "PUB1234",
				Offer:     "OFFER1234",
				SKU:       "SKU1234",
			},
			Version: "1.0.0",
		},
	}
	return &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: ptr.To(10),
				OSDisk: infrav1.OSDisk{
					CachingType: "None",
					OSType:      "Linux",
				},
			},
			Identity: infrav1.VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         string(uuid.NewUUID()),
				Scope:        "scope",
				DefinitionID: "definitionID",
			},
			Strategy: infrav1exp.AzureMachinePoolDeploymentStrategy{
				Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &one,
				},
			},
		},
	}
}
