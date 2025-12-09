/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta2

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestAROControlPlaneWebhook_Default(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	testCases := []struct {
		name            string
		inputVersion    string
		expectedVersion string
		description     string
	}{
		{
			name:            "openshift-v prefix removed",
			inputVersion:    "openshift-v4.14",
			expectedVersion: "4.14",
			description:     "should remove openshift-v prefix",
		},
		{
			name:            "v prefix removed",
			inputVersion:    "v4.14",
			expectedVersion: "4.14",
			description:     "should remove v prefix",
		},
		{
			name:            "plain X.Y version unchanged",
			inputVersion:    "4.14",
			expectedVersion: "4.14",
			description:     "should leave plain X.Y version unchanged",
		},
		{
			name:            "empty version unchanged",
			inputVersion:    "",
			expectedVersion: "",
			description:     "should leave empty version unchanged",
		},
		{
			name:            "semantic version with patch stripped",
			inputVersion:    "4.14.5",
			expectedVersion: "4.14.5",
			description:     "should leave semantic version as-is for defaulter",
		},
		{
			name:            "openshift-v with patch version",
			inputVersion:    "openshift-v4.14.5",
			expectedVersion: "4.14.5",
			description:     "should handle openshift-v prefix with patch version",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			controlPlane := &AROControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "default",
				},
				Spec: AROControlPlaneSpec{
					Version: tc.inputVersion,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			webhook := &aroControlPlaneWebhook{Client: fakeClient}

			err := webhook.Default(t.Context(), controlPlane)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(controlPlane.Spec.Version).To(Equal(tc.expectedVersion), tc.description)
		})
	}
}

func TestValidateOCPVersion(t *testing.T) {
	testCases := []struct {
		name        string
		version     string
		expectError bool
		description string
	}{
		{
			name:        "valid X.Y version",
			version:     "4.14",
			expectError: false,
			description: "should accept valid X.Y version format",
		},
		{
			name:        "valid X.Y version with higher numbers",
			version:     "4.20",
			expectError: false,
			description: "should accept valid X.Y version format with higher numbers",
		},
		{
			name:        "invalid semantic version with patch",
			version:     "4.14.5",
			expectError: true,
			description: "should reject full semantic version with patch",
		},
		{
			name:        "invalid version with pre-release",
			version:     "4.14.5-rc.1",
			expectError: true,
			description: "should reject version with pre-release",
		},
		{
			name:        "invalid version with build metadata",
			version:     "4.14.5+build.1",
			expectError: true,
			description: "should reject version with build metadata",
		},
		{
			name:        "invalid version with openshift-v prefix",
			version:     "openshift-v4.14",
			expectError: true,
			description: "should reject version with openshift-v prefix",
		},
		{
			name:        "invalid version with v prefix",
			version:     "v4.14",
			expectError: true,
			description: "should reject version with v prefix",
		},
		{
			name:        "invalid version format with single number",
			version:     "4",
			expectError: true,
			description: "should reject incomplete version with single number",
		},
		{
			name:        "invalid version format with letters",
			version:     "4.abc",
			expectError: true,
			description: "should reject version with letters in minor version",
		},
		{
			name:        "empty version",
			version:     "",
			expectError: true,
			description: "should reject empty version",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			controlPlane := &AROControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "default",
				},
				Spec: AROControlPlaneSpec{
					AroClusterName: "test-cluster",
					Version:        tc.version,
					Platform: AROPlatformProfileControlPlane{
						ResourceGroup: "test-rg",
						Location:      "eastus",
					},
					Network: &NetworkSpec{
						NetworkType: "OVNKubernetes",
						MachineCIDR: "10.0.0.0/16",
						ServiceCIDR: "172.30.0.0/16",
						PodCIDR:     "10.128.0.0/14",
						HostPrefix:  23,
					},
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
			err := controlPlane.Validate(fakeClient)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred(), tc.description)
			} else {
				g.Expect(err).NotTo(HaveOccurred(), tc.description)
			}
		})
	}
}

func TestSetDefaultOCPVersion(t *testing.T) {
	testCases := []struct {
		name            string
		inputVersion    string
		expectedVersion string
		description     string
	}{
		{
			name:            "openshift-v prefix removed",
			inputVersion:    "openshift-v4.14",
			expectedVersion: "4.14",
			description:     "should remove openshift-v prefix",
		},
		{
			name:            "v prefix removed",
			inputVersion:    "v4.14",
			expectedVersion: "4.14",
			description:     "should remove v prefix",
		},
		{
			name:            "plain version unchanged",
			inputVersion:    "4.14",
			expectedVersion: "4.14",
			description:     "should leave plain version unchanged",
		},
		{
			name:            "empty version unchanged",
			inputVersion:    "",
			expectedVersion: "",
			description:     "should leave empty version unchanged",
		},
		{
			name:            "semantic version with patch",
			inputVersion:    "4.14.5",
			expectedVersion: "4.14.5",
			description:     "should leave semantic version unchanged",
		},
		{
			name:            "openshift-v with patch version",
			inputVersion:    "openshift-v4.14.5-rc.1",
			expectedVersion: "4.14.5-rc.1",
			description:     "should handle openshift-v prefix with pre-release",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result := setDefaultOCPVersion(tc.inputVersion)
			g.Expect(result).To(Equal(tc.expectedVersion), tc.description)
		})
	}
}

func TestAROControlPlaneWebhook_ValidateUpdate_ImmutableFields(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	baseControlPlane := &AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "default",
		},
		Spec: AROControlPlaneSpec{
			AroClusterName: "test-cluster",
			Version:        "4.14",
			ChannelGroup:   "stable",
			Platform: AROPlatformProfileControlPlane{
				ResourceGroup:          "test-rg",
				Location:               "eastus",
				NetworkSecurityGroupID: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
				Subnet:                 "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
				OutboundType:           "Loadbalancer",
			},
			Visibility: "Public",
			Network: &NetworkSpec{
				NetworkType: "OVNKubernetes",
				MachineCIDR: "10.0.0.0/16",
				ServiceCIDR: "172.30.0.0/16",
				PodCIDR:     "10.128.0.0/14",
				HostPrefix:  23,
			},
		},
	}

	testCases := []struct {
		name        string
		modify      func(*AROControlPlane)
		expectError bool
		errorField  string
	}{
		{
			name: "version change should fail",
			modify: func(cp *AROControlPlane) {
				cp.Spec.Version = "4.15"
			},
			expectError: true,
			errorField:  "spec.version",
		},
		{
			name: "channelGroup change should pass",
			modify: func(cp *AROControlPlane) {
				cp.Spec.ChannelGroup = "fast"
			},
			expectError: false,
		},
		{
			name: "aroClusterName change should fail",
			modify: func(cp *AROControlPlane) {
				cp.Spec.AroClusterName = "different-cluster"
			},
			expectError: true,
			errorField:  "spec.aroClusterName",
		},
		{
			name: "networkSecurityGroupID change should fail",
			modify: func(cp *AROControlPlane) {
				cp.Spec.Platform.NetworkSecurityGroupID = "different-nsg"
			},
			expectError: true,
			errorField:  "spec.platform.networkSecurityGroupID",
		},
		{
			name: "no changes should pass",
			modify: func(cp *AROControlPlane) {
				// No changes
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create a copy of the base control plane
			old := baseControlPlane.DeepCopy()
			upd := baseControlPlane.DeepCopy()

			// Apply the modification
			tc.modify(upd)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			webhook := &aroControlPlaneWebhook{Client: fakeClient}

			_, err := webhook.ValidateUpdate(t.Context(), old, upd)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(apierrors.IsInvalid(err)).To(BeTrue())
				if tc.errorField != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorField))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAROControlPlane_ValidateAzureResourceID(t *testing.T) {
	testCases := []struct {
		name         string
		resourceID   string
		resourceType string
		expectError  bool
		errorMsg     string
	}{
		// Valid resource IDs
		{
			name:         "valid KeyVault resource ID",
			resourceID:   "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-kv",
			resourceType: "KeyVault",
			expectError:  false,
		},
		{
			name:         "valid subnet resource ID",
			resourceID:   "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
			resourceType: "subnet",
			expectError:  false,
		},
		{
			name:         "valid network security group resource ID",
			resourceID:   "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
			resourceType: "networkSecurityGroup",
			expectError:  false,
		},
		{
			name:         "valid user assigned identity resource ID",
			resourceID:   "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/test-identity",
			resourceType: "userAssignedIdentity",
			expectError:  false,
		},
		{
			name:         "empty resource ID (should be valid)",
			resourceID:   "",
			resourceType: "KeyVault",
			expectError:  false,
		},
		// Invalid resource IDs
		{
			name:         "invalid resource ID format",
			resourceID:   "invalid-resource-id",
			resourceType: "KeyVault",
			expectError:  true,
			errorMsg:     "must be a valid Azure KeyVault resource ID",
		},
		{
			name:         "wrong provider type for KeyVault",
			resourceID:   "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-resource",
			resourceType: "KeyVault",
			expectError:  true,
			errorMsg:     "provider/type must be one of: Microsoft.KeyVault/vaults",
		},
		{
			name:         "wrong provider type for subnet",
			resourceID:   "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-resource",
			resourceType: "subnet",
			expectError:  true,
			errorMsg:     "provider/type must be one of: Microsoft.Network/virtualNetworks",
		},
		{
			name:         "invalid subscription ID (not a GUID)",
			resourceID:   "/subscriptions/invalid-guid/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-kv",
			resourceType: "KeyVault",
			expectError:  true,
			errorMsg:     "subscription ID must be a valid GUID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			errs := validateAzureResourceID(tc.resourceID, nil, tc.resourceType)

			if tc.expectError {
				g.Expect(errs).NotTo(BeEmpty())
				if tc.errorMsg != "" {
					found := false
					for _, err := range errs {
						if err.Error() != "" && (tc.errorMsg == "" || err.Detail == tc.errorMsg) {
							found = true
							break
						}
					}
					if !found {
						// Check if any error contains the expected message
						for _, err := range errs {
							if strings.Contains(err.Detail, tc.errorMsg) {
								found = true
								break
							}
						}
					}
					g.Expect(found).To(BeTrue(), "Expected error message not found. Got errors: %v", errs)
				}
			} else {
				g.Expect(errs).To(BeEmpty())
			}
		})
	}
}

func TestAROControlPlane_ExtractProviderTypeFromResourceID(t *testing.T) {
	testCases := []struct {
		name         string
		resourceID   string
		expectedType string
	}{
		{
			name:         "KeyVault resource ID",
			resourceID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/kv",
			expectedType: "Microsoft.KeyVault/vaults",
		},
		{
			name:         "Virtual Network resource ID",
			resourceID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet",
			expectedType: "Microsoft.Network/virtualNetworks",
		},
		{
			name:         "Network Security Group resource ID",
			resourceID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/networkSecurityGroups/nsg",
			expectedType: "Microsoft.Network/networkSecurityGroups",
		},
		{
			name:         "User Assigned Identity resource ID",
			resourceID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/identity",
			expectedType: "Microsoft.ManagedIdentity/userAssignedIdentities",
		},
		{
			name:         "Subnet resource ID",
			resourceID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet",
			expectedType: "Microsoft.Network/virtualNetworks",
		},
		{
			name:         "Invalid resource ID",
			resourceID:   "invalid-resource-id",
			expectedType: "",
		},
		{
			name:         "Resource ID without providers",
			resourceID:   "/subscriptions/sub/resourceGroups/rg/something/else",
			expectedType: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result := extractProviderTypeFromResourceID(tc.resourceID)
			g.Expect(result).To(Equal(tc.expectedType))
		})
	}
}

func TestAROControlPlane_ValidatePlatformFields(t *testing.T) {
	testCases := []struct {
		name           string
		controlPlane   *AROControlPlane
		expectError    bool
		expectedErrors []string
	}{
		{
			name: "valid platform fields",
			controlPlane: &AROControlPlane{
				Spec: AROControlPlaneSpec{
					SubscriptionID: "64f0619f-ebc2-4156-9d91-c4c781de7e54",
					Platform: AROPlatformProfileControlPlane{
						KeyVault:               "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-kv",
						Subnet:                 "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
						NetworkSecurityGroupID: "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty platform fields (should be valid)",
			controlPlane: &AROControlPlane{
				Spec: AROControlPlaneSpec{
					Platform: AROPlatformProfileControlPlane{},
				},
			},
			expectError: false,
		},
		{
			name: "invalid KeyVault resource ID",
			controlPlane: &AROControlPlane{
				Spec: AROControlPlaneSpec{
					Platform: AROPlatformProfileControlPlane{
						KeyVault: "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/wrong-type",
					},
				},
			},
			expectError:    true,
			expectedErrors: []string{"provider/type must be one of: Microsoft.KeyVault/vaults"},
		},
		{
			name: "invalid subscription ID",
			controlPlane: &AROControlPlane{
				Spec: AROControlPlaneSpec{
					SubscriptionID: "invalid-guid",
				},
			},
			expectError:    true,
			expectedErrors: []string{"must be a valid GUID"},
		},
		{
			name: "invalid subnet resource ID",
			controlPlane: &AROControlPlane{
				Spec: AROControlPlaneSpec{
					Platform: AROPlatformProfileControlPlane{
						Subnet: "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/wrong-type",
					},
				},
			},
			expectError:    true,
			expectedErrors: []string{"provider/type must be one of: Microsoft.Network/virtualNetworks"},
		},
		{
			name: "invalid network security group resource ID",
			controlPlane: &AROControlPlane{
				Spec: AROControlPlaneSpec{
					Platform: AROPlatformProfileControlPlane{
						NetworkSecurityGroupID: "/subscriptions/64f0619f-ebc2-4156-9d91-c4c781de7e54/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/wrong-type",
					},
				},
			},
			expectError:    true,
			expectedErrors: []string{"provider/type must be one of: Microsoft.Network/networkSecurityGroups"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			errs := tc.controlPlane.validatePlatformFields(nil)

			if tc.expectError {
				g.Expect(errs).NotTo(BeEmpty())
				for _, expectedError := range tc.expectedErrors {
					found := false
					for _, err := range errs {
						if strings.Contains(err.Detail, expectedError) {
							found = true
							break
						}
					}
					g.Expect(found).To(BeTrue(), "Expected error message '%s' not found. Got errors: %v", expectedError, errs)
				}
			} else {
				g.Expect(errs).To(BeEmpty())
			}
		})
	}
}
