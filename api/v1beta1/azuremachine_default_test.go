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
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAzureMachineSpec_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{machine: createMachineWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{machine: createMachineWithSSHPublicKey("")}

	err := publicKeyExistTest.machine.Spec.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.machine.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.machine.Spec.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyNotExistTest.machine.Spec.SSHPublicKey).To(Not(BeEmpty()))
}

func TestAzureMachineSpec_SetIdentityDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	fakeSubscriptionID := uuid.New().String()
	fakeClusterName := "testcluster"
	fakeRoleDefinitionID := "testroledefinitionid"
	fakeScope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", fakeSubscriptionID, fakeClusterName)
	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"
	roleAssignmentExistTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity: VMIdentitySystemAssigned,
		SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
			Name: existingRoleAssignmentName,
		},
	}}}
	notSystemAssignedTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity:                   VMIdentityUserAssigned,
		SystemAssignedIdentityRole: &SystemAssignedIdentityRole{},
	}}}
	systemAssignedIdentityRoleExistTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity: VMIdentitySystemAssigned,
		SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
			Scope:        fakeScope,
			DefinitionID: fakeRoleDefinitionID,
		},
	}}}
	deprecatedRoleAssignmentNameTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity:           VMIdentitySystemAssigned,
		RoleAssignmentName: existingRoleAssignmentName,
	}}}
	emptyTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity:                   VMIdentitySystemAssigned,
		SystemAssignedIdentityRole: &SystemAssignedIdentityRole{},
	}}}
	bothDeprecatedRoleAssignmentNameAndSystemAssignedIdentityRoleTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity:           VMIdentitySystemAssigned,
		RoleAssignmentName: existingRoleAssignmentName,
		SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
			Name: existingRoleAssignmentName,
		},
	}}}

	roleAssignmentExistTest.machine.Spec.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(roleAssignmentExistTest.machine.Spec.SystemAssignedIdentityRole.Name).To(Equal(existingRoleAssignmentName))

	notSystemAssignedTest.machine.Spec.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(notSystemAssignedTest.machine.Spec.SystemAssignedIdentityRole.Name).To(BeEmpty())

	systemAssignedIdentityRoleExistTest.machine.Spec.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(systemAssignedIdentityRoleExistTest.machine.Spec.SystemAssignedIdentityRole.Scope).To(Equal(fakeScope))
	g.Expect(systemAssignedIdentityRoleExistTest.machine.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal(fakeRoleDefinitionID))

	deprecatedRoleAssignmentNameTest.machine.Spec.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(deprecatedRoleAssignmentNameTest.machine.Spec.SystemAssignedIdentityRole.Name).To(Equal(existingRoleAssignmentName))
	g.Expect(deprecatedRoleAssignmentNameTest.machine.Spec.RoleAssignmentName).To(BeEmpty())

	emptyTest.machine.Spec.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(emptyTest.machine.Spec.SystemAssignedIdentityRole.Name).To(Not(BeEmpty()))
	_, err := uuid.Parse(emptyTest.machine.Spec.SystemAssignedIdentityRole.Name)
	g.Expect(err).To(Not(HaveOccurred()))
	g.Expect(emptyTest.machine.Spec.SystemAssignedIdentityRole.Scope).To(Equal(fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID)))
	g.Expect(emptyTest.machine.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal(fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, ContributorRoleID)))

	bothDeprecatedRoleAssignmentNameAndSystemAssignedIdentityRoleTest.machine.Spec.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(bothDeprecatedRoleAssignmentNameAndSystemAssignedIdentityRoleTest.machine.Spec.RoleAssignmentName).To(Not(BeEmpty()))
	g.Expect(bothDeprecatedRoleAssignmentNameAndSystemAssignedIdentityRoleTest.machine.Spec.SystemAssignedIdentityRole.Name).To(Not(BeEmpty()))
}

func TestAzureMachineSpec_SetSpotEvictionPolicyDefaults(t *testing.T) {
	deallocatePolicy := SpotEvictionPolicyDeallocate
	deletePolicy := SpotEvictionPolicyDelete

	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	spotVMOptionsExistTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		SpotVMOptions: &SpotVMOptions{
			MaxPrice: &resource.Quantity{Format: "vmoptions-0"},
		},
	}}}

	localDiffDiskSettingsExistTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		SpotVMOptions: &SpotVMOptions{
			MaxPrice: &resource.Quantity{},
		},
		OSDisk: OSDisk{
			DiffDiskSettings: &DiffDiskSettings{
				Option: "Local",
			},
		},
	}}}

	spotVMOptionsExistTest.machine.Spec.SetSpotEvictionPolicyDefaults()
	g.Expect(spotVMOptionsExistTest.machine.Spec.SpotVMOptions.EvictionPolicy).To(Equal(&deallocatePolicy))

	localDiffDiskSettingsExistTest.machine.Spec.SetSpotEvictionPolicyDefaults()
	g.Expect(localDiffDiskSettingsExistTest.machine.Spec.SpotVMOptions.EvictionPolicy).To(Equal(&deletePolicy))
}

func TestAzureMachineSpec_SetDataDisksDefaults(t *testing.T) {
	cases := []struct {
		name   string
		disks  []DataDisk
		output []DataDisk
	}{
		{
			name:   "no disks",
			disks:  []DataDisk{},
			output: []DataDisk{},
		},
		{
			name: "no LUNs specified",
			disks: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](1),
					CachingType: "ReadWrite",
				},
			},
		},
		{
			name: "All LUNs specified",
			disks: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](5),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](3),
					CachingType: "ReadWrite",
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](5),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](3),
					CachingType: "ReadWrite",
				},
			},
		},
		{
			name: "Some LUNs missing",
			disks: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk3",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](1),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk4",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](2),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk3",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](1),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk4",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](3),
					CachingType: "ReadWrite",
				},
			},
		},
		{
			name: "CachingType unspecified",
			disks: []DataDisk{
				{
					NameSuffix: "testdisk1",
					DiskSizeGB: 30,
					Lun:        ptr.To[int32](0),
				},
				{
					NameSuffix: "testdisk2",
					DiskSizeGB: 30,
					Lun:        ptr.To[int32](2),
				},
				{
					NameSuffix: "testdisk3",
					DiskSizeGB: 30,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
					Lun: ptr.To[int32](3),
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         ptr.To[int32](2),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix: "testdisk3",
					DiskSizeGB: 30,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
					CachingType: "None",
				},
			},
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			machine := hardcodedAzureMachineWithSSHKey(generateSSHPublicKey(true))
			machine.Spec.DataDisks = tc.disks
			machine.Spec.SetDataDisksDefaults()
			if !reflect.DeepEqual(machine.Spec.DataDisks, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(machine.Spec.DataDisks, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestAzureMachineSpec_SetNetworkInterfacesDefaults(t *testing.T) {
	tests := []struct {
		name    string
		machine *AzureMachine
		want    *AzureMachine
	}{
		{
			name: "defaulting webhook updates machine with deprecated subnetName field",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "test-subnet",
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "",
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName:       "test-subnet",
							PrivateIPConfigs: 1,
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook updates machine with deprecated subnetName field and empty NetworkInterfaces slice",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:        "test-subnet",
					NetworkInterfaces: []NetworkInterface{},
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "",
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName:       "test-subnet",
							PrivateIPConfigs: 1,
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook updates machine with deprecated acceleratedNetworking field",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:            "test-subnet",
					AcceleratedNetworking: ptr.To(true),
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:            "",
					AcceleratedNetworking: nil,
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName:            "test-subnet",
							PrivateIPConfigs:      1,
							AcceleratedNetworking: ptr.To(true),
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook does nothing if both new and deprecated subnetName fields are set",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "test-subnet",
					NetworkInterfaces: []NetworkInterface{{
						SubnetName: "test-subnet",
					}},
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:            "test-subnet",
					AcceleratedNetworking: nil,
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName: "test-subnet",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			tc.machine.Spec.SetNetworkInterfacesDefaults()
			g.Expect(tc.machine).To(Equal(tc.want))
		})
	}
}

func TestAzureMachineSpec_GetOwnerCluster(t *testing.T) {
	tests := []struct {
		name            string
		maxAttempts     int
		wantedName      string
		wantedNamespace string
		wantErr         bool
	}{
		{
			name:            "ownerCluster is returned",
			maxAttempts:     1,
			wantedName:      "test-cluster",
			wantedNamespace: "default",
			wantErr:         false,
		},
		{
			name:            "ownerCluster is returned after 2 attempts",
			maxAttempts:     2,
			wantedName:      "test-cluster",
			wantedNamespace: "default",
			wantErr:         false,
		},
		{
			name:            "ownerCluster is not returned after 5 attempts",
			maxAttempts:     5,
			wantedName:      "test-cluster",
			wantedNamespace: "default",
			wantErr:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			client := mockClient{ReturnError: tc.wantErr}
			name, namespace, err := GetOwnerAzureClusterNameAndNamespace(client, "test-cluster", "default", tc.maxAttempts)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(name).To(Equal(tc.wantedName))
				g.Expect(namespace).To(Equal(tc.wantedNamespace))
			}
		})
	}
}

func TestAzureMachineSpec_GetSubscriptionID(t *testing.T) {
	tests := []struct {
		name                       string
		maxAttempts                int
		ownerAzureClusterName      string
		ownerAzureClusterNamespace string
		want                       string
		wantErr                    bool
	}{
		{
			name:                  "empty owner cluster name returns error",
			maxAttempts:           1,
			ownerAzureClusterName: "",
			want:                  "test-subscription-id",
			wantErr:               true,
		},
		{
			name:                       "subscription ID is returned",
			maxAttempts:                1,
			ownerAzureClusterName:      "test-cluster",
			ownerAzureClusterNamespace: "default",
			want:                       "test-subscription-id",
			wantErr:                    false,
		},
		{
			name:                       "subscription ID is returned after 2 attempts",
			maxAttempts:                2,
			ownerAzureClusterName:      "test-cluster",
			ownerAzureClusterNamespace: "default",
			want:                       "test-subscription-id",
			wantErr:                    false,
		},
		{
			name:                       "subscription ID is not returned after 5 attempts",
			maxAttempts:                5,
			ownerAzureClusterName:      "test-cluster",
			ownerAzureClusterNamespace: "default",
			want:                       "",
			wantErr:                    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			client := mockClient{ReturnError: tc.wantErr}
			result, err := GetSubscriptionID(client, tc.ownerAzureClusterName, tc.ownerAzureClusterNamespace, tc.maxAttempts)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tc.want))
			}
		})
	}
}

type mockClient struct {
	client.Client
	ReturnError bool
}

func (m mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if m.ReturnError {
		return errors.New("AzureCluster not found: failed to find owner cluster for test-cluster")
	}
	// Check if we're calling Get on an AzureCluster or a Cluster
	switch obj := obj.(type) {
	case *AzureCluster:
		obj.Spec.SubscriptionID = "test-subscription-id"
	case *clusterv1.Cluster:
		obj.Spec = clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Kind:      "AzureCluster",
				Name:      "test-cluster",
				Namespace: "default",
			},
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/26"},
				},
			},
		}
	default:
		return errors.New("unexpected object type")
	}

	return nil
}

func createMachineWithSSHPublicKey(sshPublicKey string) *AzureMachine {
	machine := hardcodedAzureMachineWithSSHKey(sshPublicKey)
	return machine
}

func createMachineWithUserAssignedIdentities(identitiesList []UserAssignedIdentity) *AzureMachine {
	machine := hardcodedAzureMachineWithSSHKey(generateSSHPublicKey(true))
	machine.Spec.Identity = VMIdentityUserAssigned
	machine.Spec.UserAssignedIdentities = identitiesList
	return machine
}

func hardcodedAzureMachineWithSSHKey(sshPublicKey string) *AzureMachine {
	return &AzureMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "test-cluster",
			},
		},
		Spec: AzureMachineSpec{
			SSHPublicKey: sshPublicKey,
			OSDisk:       generateValidOSDisk(),
			Image: &Image{
				SharedGallery: &AzureSharedGalleryImage{
					SubscriptionID: "SUB123",
					ResourceGroup:  "RG123",
					Name:           "NAME123",
					Gallery:        "GALLERY1",
					Version:        "1.0.0",
				},
			},
		},
	}
}
