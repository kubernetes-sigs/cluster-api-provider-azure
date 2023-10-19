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
	"fmt"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAzureMachinePool_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		amp *AzureMachinePool
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey("")}

	err := publicKeyExistTest.amp.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.amp.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
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
		machinePool                        *AzureMachinePool
		wantErr                            bool
		expectedRoleAssignmentName         string
		expectedSystemAssignedIdentityRole *infrav1.SystemAssignedIdentityRole
	}{
		{
			name: "bothRoleAssignmentNamesPopulated",
			machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
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
			machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
				Identity: infrav1.VMIdentitySystemAssigned,
				SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
					Name: existingRoleAssignmentName,
				},
			}},
			expectedSystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         existingRoleAssignmentName,
				DefinitionID: fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, infrav1.ContributorRoleID),
				Scope:        fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID),
			},
		},
		{
			name: "notSystemAssigned",
			machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
				Identity: infrav1.VMIdentityUserAssigned,
			}},
			expectedSystemAssignedIdentityRole: nil,
		},
		{
			name: "systemAssignedIdentityRoleExist",
			machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
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
			machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
				Identity:           infrav1.VMIdentitySystemAssigned,
				RoleAssignmentName: existingRoleAssignmentName,
			}},
			expectedSystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         existingRoleAssignmentName,
				DefinitionID: fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, infrav1.ContributorRoleID),
				Scope:        fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			_ = AddToScheme(scheme)
			_ = infrav1.AddToScheme(scheme)
			_ = clusterv1.AddToScheme(scheme)
			_ = expv1.AddToScheme(scheme)

			machinePool := &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pool1",
					Namespace: "default",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "testcluster",
					},
				},
				Spec: expv1.MachinePoolSpec{
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
					InfrastructureRef: &corev1.ObjectReference{
						Name:      "testcluster",
						Namespace: "default",
					},
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.machinePool, machinePool, azureCluster, cluster).Build()
			err := tc.machinePool.SetIdentityDefaults(fakeClient)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(tc.machinePool.Spec.RoleAssignmentName).To(Equal(tc.expectedRoleAssignmentName))
				g.Expect(tc.machinePool.Spec.SystemAssignedIdentityRole).To(Equal(tc.expectedSystemAssignedIdentityRole))
			}
		})
	}
}

func TestAzureMachinePool_SetDiagnosticsDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *AzureMachinePool
	}

	bootDiagnosticsDefault := &infrav1.BootDiagnostics{
		StorageAccountType: infrav1.ManagedDiagnosticsStorage,
	}

	managedStorageDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.ManagedDiagnosticsStorage,
					},
				},
			},
		},
	}}

	disabledStorageDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.DisabledDiagnosticsStorage,
					},
				},
			},
		},
	}}

	userManagedDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
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

	nilDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: nil,
			},
		},
	}}

	// Test that when no diagnostics are specified, the defaults are set correctly
	nilBootDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{},
			},
		},
	}}

	nilBootDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(nilBootDiagnostics.machinePool.Spec.Template.Diagnostics.Boot).To(Equal(bootDiagnosticsDefault))

	managedStorageDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(managedStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))

	disabledStorageDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(disabledStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.DisabledDiagnosticsStorage))

	userManagedDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(userManagedDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.UserManagedDiagnosticsStorage))

	nilDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(nilDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))
}

func TestAzureMachinePool_SetSpotEvictionPolicyDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *AzureMachinePool
	}

	// test to Ensure the the default policy is set to Deallocate if EvictionPolicy is nil
	defaultEvictionPolicy := infrav1.SpotEvictionPolicyDeallocate
	nilDiffDiskSettingsPolicy := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				SpotVMOptions: &infrav1.SpotVMOptions{
					EvictionPolicy: nil,
				},
			},
		},
	}}
	nilDiffDiskSettingsPolicy.machinePool.SetSpotEvictionPolicyDefaults()
	g.Expect(nilDiffDiskSettingsPolicy.machinePool.Spec.Template.SpotVMOptions.EvictionPolicy).To(Equal(&defaultEvictionPolicy))

	// test to Ensure the the default policy is set to Delete if diffDiskSettings option is set to "Local"
	expectedEvictionPolicy := infrav1.SpotEvictionPolicyDelete
	diffDiskSettingsPolicy := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				SpotVMOptions: &infrav1.SpotVMOptions{},
				OSDisk: infrav1.OSDisk{
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: "Local",
					},
				},
			},
		},
	}}
	diffDiskSettingsPolicy.machinePool.SetSpotEvictionPolicyDefaults()
	g.Expect(diffDiskSettingsPolicy.machinePool.Spec.Template.SpotVMOptions.EvictionPolicy).To(Equal(&expectedEvictionPolicy))
}

func TestAzureMachinePool_SetNetworkInterfacesDefaults(t *testing.T) {
	testCases := []struct {
		name        string
		machinePool *AzureMachinePool
		want        *AzureMachinePool
	}{
		{
			name: "defaulting webhook updates MachinePool with deprecated subnetName field",
			machinePool: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName: "test-subnet",
					},
				},
			},
			want: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
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
			machinePool: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName:            "test-subnet",
						AcceleratedNetworking: ptr.To(true),
					},
				},
			},
			want: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
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
			machinePool: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName: "test-subnet",
						NetworkInterfaces: []infrav1.NetworkInterface{{
							SubnetName: "test-subnet",
						}},
					},
				},
			},
			want: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
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
			tc.machinePool.SetNetworkInterfacesDefaults()
			g.Expect(tc.machinePool).To(Equal(tc.want))
		})
	}
}

func createMachinePoolWithSSHPublicKey(sshPublicKey string) *AzureMachinePool {
	return hardcodedAzureMachinePoolWithSSHKey(sshPublicKey)
}

func hardcodedAzureMachinePoolWithSSHKey(sshPublicKey string) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				SSHPublicKey: sshPublicKey,
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "testmachinepool",
		},
	}
}
