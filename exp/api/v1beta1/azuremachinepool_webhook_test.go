/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	guuid "github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	validSSHPublicKey = generateSSHPublicKey(true)
	zero              = intstr.FromInt(0)
	one               = intstr.FromInt(1)
)

type mockClient struct {
	client.Client
	Version     string
	ReturnError bool
}

func (m mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	obj.(*expv1.MachinePool).Spec.Template.Spec.Version = &m.Version
	return nil
}

func (m mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.ReturnError {
		return errors.New("MachinePool.cluster.x-k8s.io \"mock-machinepool-mp-0\" not found")
	}
	mp := &expv1.MachinePool{}
	mp.Spec.Template.Spec.Version = &m.Version
	list.(*expv1.MachinePoolList).Items = []expv1.MachinePool{*mp}

	return nil
}

func TestAzureMachinePool_ValidateCreate(t *testing.T) {
	tests := []struct {
		name          string
		amp           *AzureMachinePool
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
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &zero,
				},
			}),
			wantErr: true,
		},
		{
			name: "azuremachinepool with valid MaxSurge and MaxUnavailable rolling upgrade configuration",
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
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
	}

	for _, tc := range tests {
		client := mockClient{Version: tc.version, ReturnError: tc.ownerNotFound}
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ampw := &azureMachinePoolWebhook{
				Client: client,
			}
			_, err := ampw.ValidateCreate(context.Background(), tc.amp)
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

func (m mockDefaultClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	switch obj := obj.(type) {
	case *infrav1.AzureCluster:
		obj.Spec.SubscriptionID = m.SubscriptionID
	case *clusterv1.Cluster:
		obj.Spec.InfrastructureRef = &corev1.ObjectReference{
			Kind: infrav1.AzureClusterKind,
			Name: "test-cluster",
		}
	default:
		return errors.New("invalid object type")
	}
	return nil
}

func (m mockDefaultClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	list.(*expv1.MachinePoolList).Items = []expv1.MachinePool{
		{
			Spec: expv1.MachinePoolSpec{
				Template: clusterv1.MachineTemplateSpec{
					Spec: clusterv1.MachineSpec{
						InfrastructureRef: corev1.ObjectReference{
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
		oldAMP  *AzureMachinePool
		amp     *AzureMachinePool
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
			oldAMP: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{}),
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &zero,
				},
			}),
			wantErr: true,
		},
		{
			name:   "azuremachinepool with valid MaxSurge and MaxUnavailable rolling upgrade configuration",
			oldAMP: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{}),
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ampw := &azureMachinePoolWebhook{}
			_, err := ampw.ValidateUpdate(context.Background(), tc.oldAMP, tc.amp)
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
		amp *AzureMachinePool
	}

	existingPublicKey := validSSHPublicKey
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey("")}

	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"

	fakeSubscriptionID := guuid.New().String()
	fakeClusterName := "testcluster"
	fakeMachinePoolName := "testmachinepool"
	mockClient := mockDefaultClient{Name: fakeMachinePoolName, ClusterName: fakeClusterName, SubscriptionID: fakeSubscriptionID}

	roleAssignmentExistTest := test{amp: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
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

	emptyTest := test{amp: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Identity:                   "SystemAssigned",
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeMachinePoolName,
		},
	}}

	systemAssignedIdentityRoleExistTest := test{amp: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
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

	ampw := &azureMachinePoolWebhook{
		Client: mockClient,
	}

	err := ampw.Default(context.Background(), roleAssignmentExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(roleAssignmentExistTest.amp.Spec.SystemAssignedIdentityRole.Name).To(Equal(existingRoleAssignmentName))

	err = ampw.Default(context.Background(), publicKeyExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	err = ampw.Default(context.Background(), publicKeyNotExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyNotExistTest.amp.Spec.Template.SSHPublicKey).NotTo(BeEmpty())

	err = ampw.Default(context.Background(), systemAssignedIdentityRoleExistTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(systemAssignedIdentityRoleExistTest.amp.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal("testroledefinitionid"))
	g.Expect(systemAssignedIdentityRoleExistTest.amp.Spec.SystemAssignedIdentityRole.Scope).To(Equal("testscope"))

	err = ampw.Default(context.Background(), emptyTest.amp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole.Name).To(Not(BeEmpty()))
	_, err = guuid.Parse(emptyTest.amp.Spec.SystemAssignedIdentityRole.Name)
	g.Expect(err).To(Not(HaveOccurred()))
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole).To(Not(BeNil()))
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole.Scope).To(Equal(fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID)))
	g.Expect(emptyTest.amp.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal(fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, infrav1.ContributorRoleID)))
}

func createMachinePoolWithMarketPlaceImage(publisher, offer, sku, version string, terminateNotificationTimeout *int) *AzureMachinePool {
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

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithSharedImage(subscriptionID, resourceGroup, name, gallery, version string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := infrav1.Image{
		SharedGallery: &infrav1.AzureSharedGalleryImage{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Name:           name,
			Gallery:        gallery,
			Version:        version,
		},
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithNetworkConfig(subnetName string, interfaces []infrav1.NetworkInterface) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				SubnetName:        subnetName,
				NetworkInterfaces: interfaces,
			},
		},
	}
}

func createMachinePoolWithImageByID(imageID string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := infrav1.Image{
		ID: &imageID,
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithSystemAssignedIdentity(role string) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Identity: infrav1.VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         role,
				Scope:        "scope",
				DefinitionID: "definitionID",
			},
		},
	}
}

func createMachinePoolWithDiagnostics(diagnosticsType infrav1.BootDiagnosticsStorageAccountType, userManaged *infrav1.UserManagedBootDiagnostics) *AzureMachinePool {
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

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: diagnostics,
			},
		},
	}
}

func createMachinePoolWithUserAssignedIdentity(providerIds []string) *AzureMachinePool {
	userAssignedIdentities := make([]infrav1.UserAssignedIdentity, len(providerIds))

	for _, providerID := range providerIds {
		userAssignedIdentities = append(userAssignedIdentities, infrav1.UserAssignedIdentity{
			ProviderID: providerID,
		})
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Identity:               infrav1.VMIdentityUserAssigned,
			UserAssignedIdentities: userAssignedIdentities,
		},
	}
}

func generateSSHPublicKey(b64Enconded bool) string {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicRsaKey, _ := ssh.NewPublicKey(&privateKey.PublicKey)
	if b64Enconded {
		return base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}
	return string(ssh.MarshalAuthorizedKey(publicRsaKey))
}

func createMachinePoolWithStrategy(strategy AzureMachinePoolDeploymentStrategy) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Strategy: strategy,
		},
	}
}

func createMachinePoolWithOrchestrationMode(mode armcompute.OrchestrationMode) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			OrchestrationMode: infrav1.OrchestrationModeType(mode),
		},
	}
}

func TestAzureMachinePool_ValidateCreateFailure(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		amp         *AzureMachinePool
		deferFunc   func()
		expectError bool
	}{
		{
			name:        "feature gate explicitly disabled",
			amp:         getKnownValidAzureMachinePool(),
			deferFunc:   utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, false),
			expectError: true,
		},
		{
			name:        "feature gate implicitly enabled",
			amp:         getKnownValidAzureMachinePool(),
			deferFunc:   func() {},
			expectError: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.deferFunc()
			ampw := &azureMachinePoolWebhook{}
			_, err := ampw.ValidateCreate(context.Background(), tc.amp)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getKnownValidAzureMachinePool() *AzureMachinePool {
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
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: ptr.To(10),
			},
			Identity: infrav1.VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         string(uuid.NewUUID()),
				Scope:        "scope",
				DefinitionID: "definitionID",
			},
			Strategy: AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &one,
				},
			},
		},
	}
}
