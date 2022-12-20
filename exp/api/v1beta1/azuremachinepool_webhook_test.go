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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	guuid "github.com/google/uuid"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	utilfeature "k8s.io/component-base/featuregate/testing"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	validSSHPublicKey = generateSSHPublicKey(true)
	zero              = intstr.FromInt(0)
	one               = intstr.FromInt(1)
)

func TestAzureMachinePool_ValidateCreate(t *testing.T) {
	// NOTE: AzureMachinePool is behind MachinePool feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()

	g := NewWithT(t)

	tests := []struct {
		name    string
		amp     *AzureMachinePool
		version string
		wantErr bool
	}{
		{
			name:    "valid",
			amp:     getKnownValidAzureMachinePool(),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - full",
			amp:     createMachinePoolWithMarketPlaceImage("PUB1234", "OFFER1234", "SKU1234", "1.0.0", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing publisher",
			amp:     createMachinePoolWithMarketPlaceImage("", "OFFER1234", "SKU1234", "1.0.0", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with shared gallery image - full",
			amp:     createMachinePoolWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing subscription",
			amp:     createMachinePoolWithSharedImage("", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with image by - with id",
			amp:     createMachinePoolWithImageByID("ID123", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with image by - without id",
			amp:     createMachinePoolWithImageByID("", to.IntPtr(10)),
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
			amp:     createMachinePoolWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(35)),
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
			name:    "azuremachinepool with user assigned identity",
			amp:     createMachinePoolWithUserAssignedIdentity([]string{"azure:://id1", "azure:://id2"}),
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
			amp:     createMachinePoolWithOrchestrationMode(compute.OrchestrationModeFlexible),
			version: "v1.26.0",
			wantErr: false,
		},
		{
			name:    "azuremachinepool with Flexible orchestration mode and invalid Kubernetes version",
			amp:     createMachinePoolWithOrchestrationMode(compute.OrchestrationModeFlexible),
			version: "v1.25.6",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		client := mockClient{Version: tc.version}
		t.Run(tc.name, func(t *testing.T) {
			err := tc.amp.ValidateCreate(client)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

type mockClient struct {
	client.Client
	Version string
}

func (m mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	obj.(*expv1.MachinePool).Spec.Template.Spec.Version = &m.Version
	return nil
}

func TestAzureMachinePool_ValidateUpdate(t *testing.T) {
	// NOTE: AzureMachinePool is behind MachinePool feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()

	g := NewWithT(t)

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
			err := tc.amp.ValidateUpdate(tc.oldAMP, nil)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachinePool_Default(t *testing.T) {
	// NOTE: AzureMachinePool is behind MachinePool feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()

	g := NewWithT(t)

	type test struct {
		amp *AzureMachinePool
	}

	existingPublicKey := validSSHPublicKey
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey("")}

	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"
	roleAssignmentExistTest := test{amp: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity:           "SystemAssigned",
		RoleAssignmentName: existingRoleAssignmentName,
	}}}

	roleAssignmentEmptyTest := test{amp: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity:           "SystemAssigned",
		RoleAssignmentName: "",
	}}}

	roleAssignmentExistTest.amp.Default(nil)
	g.Expect(roleAssignmentExistTest.amp.Spec.RoleAssignmentName).To(Equal(existingRoleAssignmentName))

	roleAssignmentEmptyTest.amp.Default(nil)
	g.Expect(roleAssignmentEmptyTest.amp.Spec.RoleAssignmentName).To(Not(BeEmpty()))
	_, err := guuid.Parse(roleAssignmentEmptyTest.amp.Spec.RoleAssignmentName)
	g.Expect(err).To(Not(HaveOccurred()))

	publicKeyExistTest.amp.Default(nil)
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	publicKeyNotExistTest.amp.Default(nil)
	g.Expect(publicKeyNotExistTest.amp.Spec.Template.SSHPublicKey).NotTo(BeEmpty())
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
			Identity:           infrav1.VMIdentitySystemAssigned,
			RoleAssignmentName: role,
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

func createMachinePoolWithOrchestrationMode(mode compute.OrchestrationMode) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			OrchestrationMode: infrav1.OrchestrationModeType(mode),
		},
	}
}

func TestAzureMachinePool_ValidateCreateFailure(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name      string
		amp       *AzureMachinePool
		deferFunc func()
	}{
		{
			name:      "feature gate explicitly disabled",
			amp:       getKnownValidAzureMachinePool(),
			deferFunc: utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, false),
		},
		{
			name:      "feature gate implicitly disabled",
			amp:       getKnownValidAzureMachinePool(),
			deferFunc: func() {},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.deferFunc()
			err := tc.amp.ValidateCreate(nil)
			g.Expect(err).To(HaveOccurred())
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
				TerminateNotificationTimeout: to.IntPtr(10),
			},
			Identity:           infrav1.VMIdentitySystemAssigned,
			RoleAssignmentName: string(uuid.NewUUID()),
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
