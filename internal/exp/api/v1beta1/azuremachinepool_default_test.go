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

package v1beta1

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	apiinternal "sigs.k8s.io/cluster-api-provider-azure/internal/api/v1beta1"
)

func TestAzureMachinePool_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		amp *infrav1exp.AzureMachinePool
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey("")}

	g.Expect(SetDefaultSSHPublicKey(publicKeyExistTest.amp)).To(Succeed())
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	g.Expect(SetDefaultSSHPublicKey(publicKeyNotExistTest.amp)).To(Succeed())
	g.Expect(publicKeyNotExistTest.amp.Spec.Template.SSHPublicKey).NotTo(BeEmpty())
}

func TestAzureMachinePool_SetIdentityDefaults(t *testing.T) {
	fakeSubscriptionID := uuid.New().String()
	fakeClusterName := "testcluster"
	fakeRoleDefinitionID := "testroledefinitionid"
	fakeScope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", fakeSubscriptionID, fakeClusterName)
	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"

	tests := []struct {
		name                               string
		machinePool                        *infrav1exp.AzureMachinePool
		wantErr                            bool
		expectedRoleAssignmentName         string
		expectedSystemAssignedIdentityRole *infrav1.SystemAssignedIdentityRole
	}{
		{
			name: "bothRoleAssignmentNamesPopulated",
			machinePool: &infrav1exp.AzureMachinePool{Spec: infrav1exp.AzureMachinePoolSpec{
				Identity:           infrav1.VMIdentitySystemAssigned,
				RoleAssignmentName: existingRoleAssignmentName,
				SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
					Name: existingRoleAssignmentName,
				},
			}},
			expectedRoleAssignmentName: existingRoleAssignmentName,
			expectedSystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name: existingRoleAssignmentName,
			},
		},
		{
			name: "roleAssignmentExist",
			machinePool: &infrav1exp.AzureMachinePool{Spec: infrav1exp.AzureMachinePoolSpec{
				Identity: infrav1.VMIdentitySystemAssigned,
				SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
					Name: existingRoleAssignmentName,
				},
			}},
			expectedSystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         existingRoleAssignmentName,
				DefinitionID: fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, apiinternal.ContributorRoleID),
				Scope:        fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID),
			},
		},
		{
			name: "notSystemAssigned",
			machinePool: &infrav1exp.AzureMachinePool{Spec: infrav1exp.AzureMachinePoolSpec{
				Identity: infrav1.VMIdentityUserAssigned,
			}},
			expectedSystemAssignedIdentityRole: nil,
		},
		{
			name: "systemAssignedIdentityRoleExist",
			machinePool: &infrav1exp.AzureMachinePool{Spec: infrav1exp.AzureMachinePoolSpec{
				Identity: infrav1.VMIdentitySystemAssigned,
				SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
					Name:         existingRoleAssignmentName,
					DefinitionID: fakeRoleDefinitionID,
					Scope:        fakeScope,
				},
			}},
			expectedSystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         existingRoleAssignmentName,
				DefinitionID: fakeRoleDefinitionID,
				Scope:        fakeScope,
			},
		},
		{
			name: "deprecatedRoleAssignmentName",
			machinePool: &infrav1exp.AzureMachinePool{Spec: infrav1exp.AzureMachinePoolSpec{
				Identity:           infrav1.VMIdentitySystemAssigned,
				RoleAssignmentName: existingRoleAssignmentName,
			}},
			expectedSystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         existingRoleAssignmentName,
				DefinitionID: fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, apiinternal.ContributorRoleID),
				Scope:        fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			_ = infrav1exp.AddToScheme(scheme)
			_ = infrav1.AddToScheme(scheme)
			_ = clusterv1.AddToScheme(scheme)

			machinePool := &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pool1",
					Namespace: "default",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "testcluster",
					},
				},
				Spec: clusterv1.MachinePoolSpec{
					ClusterName: "testcluster",
				},
			}
			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "default",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: fakeSubscriptionID,
					},
				},
			}
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Name: "testcluster",
					},
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.machinePool, machinePool, azureCluster, cluster).Build()
			err := SetIdentityDefaults(tc.machinePool, fakeClient)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(tc.machinePool.Spec.RoleAssignmentName).To(Equal(tc.expectedRoleAssignmentName)) //nolint:staticcheck
				g.Expect(tc.machinePool.Spec.SystemAssignedIdentityRole).To(Equal(tc.expectedSystemAssignedIdentityRole))
			}
		})
	}
}

func TestAzureMachinePool_SetDiagnosticsDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *infrav1exp.AzureMachinePool
	}

	bootDiagnosticsDefault := &infrav1.BootDiagnostics{
		StorageAccountType: infrav1.ManagedDiagnosticsStorage,
	}

	managedStorageDiagnostics := test{machinePool: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.ManagedDiagnosticsStorage,
					},
				},
			},
		},
	}}

	disabledStorageDiagnostics := test{machinePool: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.DisabledDiagnosticsStorage,
					},
				},
			},
		},
	}}

	userManagedDiagnostics := test{machinePool: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
						UserManaged: &infrav1.UserManagedBootDiagnostics{
							StorageAccountURI: "https://fakeurl",
						},
					},
				},
			},
		},
	}}

	nilDiagnostics := test{machinePool: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Diagnostics: nil,
			},
		},
	}}

	// Test that when no diagnostics are specified, the defaults are set correctly
	nilBootDiagnostics := test{machinePool: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{},
			},
		},
	}}

	SetDiagnosticsDefaults(nilBootDiagnostics.machinePool)
	g.Expect(nilBootDiagnostics.machinePool.Spec.Template.Diagnostics.Boot).To(Equal(bootDiagnosticsDefault))

	SetDiagnosticsDefaults(managedStorageDiagnostics.machinePool)
	g.Expect(managedStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))

	SetDiagnosticsDefaults(disabledStorageDiagnostics.machinePool)
	g.Expect(disabledStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.DisabledDiagnosticsStorage))

	SetDiagnosticsDefaults(userManagedDiagnostics.machinePool)
	g.Expect(userManagedDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.UserManagedDiagnosticsStorage))

	SetDiagnosticsDefaults(nilDiagnostics.machinePool)
	g.Expect(nilDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))
}

func TestAzureMachinePool_SetSpotEvictionPolicyDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *infrav1exp.AzureMachinePool
	}

	// test to Ensure the default policy is set to Deallocate if EvictionPolicy is nil
	defaultEvictionPolicy := infrav1.SpotEvictionPolicyDeallocate
	nilDiffDiskSettingsPolicy := test{machinePool: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				SpotVMOptions: &infrav1.SpotVMOptions{
					EvictionPolicy: nil,
				},
			},
		},
	}}
	SetSpotEvictionPolicyDefaults(nilDiffDiskSettingsPolicy.machinePool)
	g.Expect(nilDiffDiskSettingsPolicy.machinePool.Spec.Template.SpotVMOptions.EvictionPolicy).To(Equal(&defaultEvictionPolicy))

	// test to Ensure the default policy is set to Delete if diffDiskSettings option is set to "Local"
	expectedEvictionPolicy := infrav1.SpotEvictionPolicyDelete
	diffDiskSettingsPolicy := test{machinePool: &infrav1exp.AzureMachinePool{
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				SpotVMOptions: &infrav1.SpotVMOptions{},
				OSDisk: infrav1.OSDisk{
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: "Local",
					},
				},
			},
		},
	}}
	SetSpotEvictionPolicyDefaults(diffDiskSettingsPolicy.machinePool)
	g.Expect(diffDiskSettingsPolicy.machinePool.Spec.Template.SpotVMOptions.EvictionPolicy).To(Equal(&expectedEvictionPolicy))
}

func TestAzureMachinePool_SetNetworkInterfacesDefaults(t *testing.T) {
	testCases := []struct {
		name        string
		machinePool *infrav1exp.AzureMachinePool
		want        *infrav1exp.AzureMachinePool
	}{
		{
			name: "defaulting webhook updates MachinePool with deprecated subnetName field",
			machinePool: &infrav1exp.AzureMachinePool{
				Spec: infrav1exp.AzureMachinePoolSpec{
					Template: infrav1exp.AzureMachinePoolMachineTemplate{
						SubnetName: "test-subnet",
					},
				},
			},
			want: &infrav1exp.AzureMachinePool{
				Spec: infrav1exp.AzureMachinePoolSpec{
					Template: infrav1exp.AzureMachinePoolMachineTemplate{
						SubnetName: "",
						NetworkInterfaces: []infrav1.NetworkInterface{
							{
								SubnetName:       "test-subnet",
								PrivateIPConfigs: 1,
							},
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook updates MachinePool with deprecated acceleratedNetworking field",
			machinePool: &infrav1exp.AzureMachinePool{
				Spec: infrav1exp.AzureMachinePoolSpec{
					Template: infrav1exp.AzureMachinePoolMachineTemplate{
						SubnetName:            "test-subnet",
						AcceleratedNetworking: ptr.To(true),
					},
				},
			},
			want: &infrav1exp.AzureMachinePool{
				Spec: infrav1exp.AzureMachinePoolSpec{
					Template: infrav1exp.AzureMachinePoolMachineTemplate{
						SubnetName:            "",
						AcceleratedNetworking: nil,
						NetworkInterfaces: []infrav1.NetworkInterface{
							{
								SubnetName:            "test-subnet",
								PrivateIPConfigs:      1,
								AcceleratedNetworking: ptr.To(true),
							},
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook does nothing if both new and deprecated subnetName fields are set",
			machinePool: &infrav1exp.AzureMachinePool{
				Spec: infrav1exp.AzureMachinePoolSpec{
					Template: infrav1exp.AzureMachinePoolMachineTemplate{
						SubnetName: "test-subnet",
						NetworkInterfaces: []infrav1.NetworkInterface{{
							SubnetName: "test-subnet",
						}},
					},
				},
			},
			want: &infrav1exp.AzureMachinePool{
				Spec: infrav1exp.AzureMachinePoolSpec{
					Template: infrav1exp.AzureMachinePoolMachineTemplate{
						SubnetName:            "test-subnet",
						AcceleratedNetworking: nil,
						NetworkInterfaces: []infrav1.NetworkInterface{
							{
								SubnetName: "test-subnet",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			SetNetworkInterfacesDefaults(tc.machinePool)
			g.Expect(tc.machinePool).To(Equal(tc.want))
		})
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
