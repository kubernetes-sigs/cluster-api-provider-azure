/*
Copyright 2020 The Kubernetes Authors.

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

package scalesets

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets/mock_scalesets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

const (
	defaultSubscriptionID = "123"
	defaultResourceGroup  = "my-rg"
	defaultVMSSName       = "my-vmss"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestNewService(t *testing.T) {
	g := NewGomegaWithT(t)
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	s, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "test-location",
				ResourceGroup:  "my-rg",
				SubscriptionID: "123",
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	mps, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Client:           client,
		Logger:           s.Logger,
		MachinePool:      new(clusterv1exp.MachinePool),
		AzureMachinePool: new(infrav1exp.AzureMachinePool),
		ClusterScope:     s,
	})
	g.Expect(err).ToNot(HaveOccurred())
	actual := NewService(mps, resourceskus.NewStaticCache(nil, ""))
	g.Expect(actual).ToNot(BeNil())
}

func TestGetExistingVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		vmssName      string
		result        *azure.VMSS
		expectedError string
		expect        func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
	}{
		{
			name:          "scale set not found",
			vmssName:      "my-vmss",
			result:        &azure.VMSS{},
			expectedError: "failed to get existing vmss: #: Not found: StatusCode=404",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:     "get existing vmss",
			vmssName: "my-vmss",
			result: &azure.VMSS{
				ID:       "my-id",
				Name:     "my-vmss",
				State:    "Succeeded",
				Sku:      "Standard_D2",
				Identity: "",
				Tags:     nil,
				Capacity: int64(1),
				Zones:    []string{"1", "3"},
				Instances: []azure.VMSSVM{
					{
						ID:         "my-vm-id",
						InstanceID: "my-vm-1",
						Name:       "instance-000001",
						State:      "Succeeded",
					},
				},
			},
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{
					ID:   to.StringPtr("my-id"),
					Name: to.StringPtr("my-vmss"),
					Sku: &compute.Sku{
						Capacity: to.Int64Ptr(1),
						Name:     to.StringPtr("Standard_D2"),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						SinglePlacementGroup: to.BoolPtr(false),
						ProvisioningState:    to.StringPtr("Succeeded"),
					},
					Zones: &[]string{"1", "3"},
				}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         to.StringPtr("my-vm-id"),
						InstanceID: to.StringPtr("my-vm-1"),
						Name:       to.StringPtr("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
							OsProfile: &compute.OSProfile{
								ComputerName: to.StringPtr("instance-000001"),
							},
						},
					},
				}, nil)
			},
		},
		{
			name:          "list instances fails",
			vmssName:      "my-vmss",
			result:        &azure.VMSS{},
			expectedError: "failed to list instances: #: Not found: StatusCode=404",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{
					ID:   to.StringPtr("my-id"),
					Name: to.StringPtr("my-vmss"),
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						SinglePlacementGroup: to.BoolPtr(false),
						ProvisioningState:    to.StringPtr("Succeeded"),
					},
				}, nil)
				m.ListInstances(gomockinternal.AContext(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			clientMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}

			result, err := s.getVirtualMachineScaleSet(context.TODO(), tc.vmssName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				t.Log(err.Error())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(BeEquivalentTo(tc.result))
			}
		})
	}
}

func TestReconcileVMSS(t *testing.T) {
	var (
		putFuture = &infrav1.Future{
			Type:          PutFuture,
			ResourceGroup: defaultResourceGroup,
			Name:          defaultVMSSName,
		}

		patchFuture = &infrav1.Future{
			Type:          PatchFuture,
			ResourceGroup: defaultResourceGroup,
			Name:          defaultVMSSName,
		}
	)

	testcases := []struct {
		name          string
		expect        func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "should start creating a vmss",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				defaultSpec := newDefaultVMSSSpec()
				s.ScaleSetSpec().Return(defaultSpec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS()
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(), putFuture)
			},
		},
		{
			name:          "should finish creating a vmss when long running operation is done",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				defaultSpec := newDefaultVMSSSpec()
				s.ScaleSetSpec().Return(defaultSpec, nil).AnyTimes()
				createdVMSS := newDefaultVMSS()
				instances := newDefaultInstances()
				_ = setupDefaultVMSSInProgressOperationDoneExpectations(s, m, createdVMSS, instances)
				s.SetLongRunningOperationState(nil)
			},
		},
		{
			name:          "Windows VMSS should not get patched",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				defaultSpec := newWindowsVMSSSpec()
				s.ScaleSetSpec().Return(defaultSpec, nil).AnyTimes()
				createdVMSS := newDefaultWindowsVMSS()
				instances := newDefaultInstances()
				_ = setupDefaultVMSSInProgressOperationDoneExpectations(s, m, createdVMSS, instances)
				s.SetLongRunningOperationState(nil)
			},
		},
		{
			name:          "should start creating vmss with defaulted accelerated networking when size allows",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Size = "VM_SIZE_AN"
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS()
				netConfigs := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations
				(*netConfigs)[0].EnableAcceleratedNetworking = to.BoolPtr(true)
				vmss.Sku.Name = to.StringPtr(spec.Size)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with spot vm",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.SpotVMOptions = &infrav1.SpotVMOptions{}
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS()
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.Priority = compute.Spot
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.EvictionPolicy = compute.Deallocate
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with spot vm and a maximum price",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				maxPrice := resource.MustParse("0.001")
				spec.SpotVMOptions = &infrav1.SpotVMOptions{
					MaxPrice: &maxPrice,
				}
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS()
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.Priority = compute.Spot
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.BillingProfile = &compute.BillingProfile{
					MaxPrice: to.Float64Ptr(0.001),
				}
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.EvictionPolicy = compute.Deallocate
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with encryption",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.OSDisk.ManagedDisk.DiskEncryptionSet = &infrav1.DiskEncryptionSetParameters{
					ID: "my-diskencryptionset-id",
				}
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS()
				osdisk := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk
				osdisk.ManagedDisk = &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Premium_LRS",
					DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
						ID: to.StringPtr("my-diskencryptionset-id"),
					},
				}
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(), putFuture)
			},
		},
		{
			name:          "can start creating a vmss with user assigned identity",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Identity = infrav1.VMIdentityUserAssigned
				spec.UserAssignedIdentities = []infrav1.UserAssignedIdentity{
					{
						ProviderID: "azure:///subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1",
					},
				}
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS()
				vmss.Identity = &compute.VirtualMachineScaleSetIdentity{
					Type: compute.ResourceIdentityTypeUserAssigned,
					UserAssignedIdentities: map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue{
						"/subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1": {},
					},
				}
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with encryption at host enabled",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Size = "VM_SIZE_EAH"
				spec.SecurityProfile = &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)}
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS()
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.SecurityProfile = &compute.SecurityProfile{
					EncryptionAtHost: to.BoolPtr(true),
				}
				vmss.Sku.Name = to.StringPtr(spec.Size)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(), putFuture)
			},
		},
		{
			name:          "creating a vmss with encryption at host enabled for unsupported VM type fails",
			expectedError: "reconcile error that cannot be recovered occurred: encryption at host is not supported for VM type VM_SIZE. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:            defaultVMSSName,
					Size:            "VM_SIZE",
					Capacity:        2,
					SSHKeyData:      "ZmFrZXNzaGtleQo=",
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)},
				}, nil)
			},
		},
		{
			name:          "should start updating when scale set already exists and not currently in a long running operation",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PATCH on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Capacity = 2
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()

				setupDefaultVMSSUpdateExpectations(s)
				existingVMSS := newDefaultExistingVMSS()
				existingVMSS.Sku.Capacity = to.Int64Ptr(2)
				instances := newDefaultInstances()
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(existingVMSS, nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil)

				clone := newDefaultExistingVMSS()
				clone.Sku.Capacity = to.Int64Ptr(3)
				patchVMSS, err := getVMSSUpdateFromVMSS(clone)
				g.Expect(err).NotTo(HaveOccurred())
				patchVMSS.VirtualMachineProfile.StorageProfile.ImageReference.Version = to.StringPtr("2.0")
				patchVMSS.VirtualMachineProfile.NetworkProfile = nil
				m.UpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(patchVMSS)).
					Return(patchFuture, nil)
				s.SetLongRunningOperationState(patchFuture)
				m.GetResultIfDone(gomockinternal.AContext(), patchFuture).Return(compute.VirtualMachineScaleSet{}, azure.NewOperationNotDoneError(patchFuture))
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(clone, nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil)
			},
		},
		{
			name:          "less than 2 vCPUs",
			expectedError: "reconcile error that cannot be recovered occurred: vm size should be bigger or equal to at least 2 vCPUs. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE_1_CPU",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				}, nil)
			},
		},
		{
			name:          "Memory is less than 2Gi",
			expectedError: "reconcile error that cannot be recovered occurred: vm memory should be bigger or equal to at least 2Gi. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE_1_MEM",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				}, nil)
			},
		},
		{
			name:          "failed to get SKU",
			expectedError: "reconcile error that cannot be recovered occurred: failed to get SKU INVALID_VM_SIZE in compute api: resource sku with name 'INVALID_VM_SIZE' and category 'virtualMachines' not found in location 'test-location'. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "INVALID_VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				}, nil)
			},
		},
		{
			name:          "fails with internal error",
			expectedError: "failed to start creating VMSS: cannot create VMSS: #: Internal error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				s.ScaleSetSpec().Return(spec, nil).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomock.AssignableToTypeOf(compute.VirtualMachineScaleSet{})).
					Return(nil, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal error"))
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			clientMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(g, scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				Client:           clientMock,
				resourceSKUCache: resourceskus.NewStaticCache(getFakeSkus(), "test-location"),
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVMSS(t *testing.T) {
	const (
		resourceGroup = "my-rg"
		name          = "my-vmss"
	)

	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing vmss",
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     "my-existing-vmss",
					Size:     "VM_SIZE",
					Capacity: 3,
				}, nil).AnyTimes()
				s.ResourceGroup().AnyTimes().Return("my-existing-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				future := &infrav1.Future{}
				s.GetLongRunningOperationState().Return(future)
				m.GetResultIfDone(gomockinternal.AContext(), future).Return(compute.VirtualMachineScaleSet{}, nil)
				m.Get(gomockinternal.AContext(), "my-existing-rg", "my-existing-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.SetLongRunningOperationState(nil)
			},
		},
		{
			name:          "vmss already deleted",
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     name,
					Size:     "VM_SIZE",
					Capacity: 3,
				}, nil).AnyTimes()
				s.ResourceGroup().AnyTimes().Return(resourceGroup)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetLongRunningOperationState().Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), resourceGroup, name).
					Return(nil, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), resourceGroup, name).
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "vmss deletion fails",
			expectedError: "failed to delete VMSS my-vmss in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     name,
					Size:     "VM_SIZE",
					Capacity: 3,
				}, nil).AnyTimes()
				s.ResourceGroup().AnyTimes().Return(resourceGroup)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetLongRunningOperationState().Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), resourceGroup, name).
					Return(nil, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				m.Get(gomockinternal.AContext(), resourceGroup, name).
					Return(newDefaultVMSS(), nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(newDefaultInstances(), nil).AnyTimes()
				s.SetVMSSState(gomock.AssignableToTypeOf(&azure.VMSS{}))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			clientMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getFakeSkus() []compute.ResourceSku {
	return []compute.ResourceSku{
		{
			Name:         to.StringPtr("VM_SIZE"),
			ResourceType: to.StringPtr(string(resourceskus.VirtualMachines)),
			Kind:         to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("4"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("4"),
				},
			},
		},
		{
			Name:         to.StringPtr("VM_SIZE_AN"),
			ResourceType: to.StringPtr(string(resourceskus.VirtualMachines)),
			Kind:         to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("4"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("6"),
				},
			},
		},
		{
			Name:         to.StringPtr("VM_SIZE_1_CPU"),
			ResourceType: to.StringPtr(string(resourceskus.VirtualMachines)),
			Kind:         to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("1"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("4"),
				},
			},
		},
		{
			Name:         to.StringPtr("VM_SIZE_1_MEM"),
			ResourceType: to.StringPtr(string(resourceskus.VirtualMachines)),
			Kind:         to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("2"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("1"),
				},
			},
		},
		{
			Name:         to.StringPtr("VM_SIZE_EAH"),
			ResourceType: to.StringPtr(string(resourceskus.VirtualMachines)),
			Kind:         to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("4"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("8"),
				},
				{
					Name:  to.StringPtr(resourceskus.EncryptionAtHost),
					Value: to.StringPtr(string(resourceskus.CapabilitySupported)),
				},
			},
		},
	}
}

func newDefaultVMSSSpec() azure.ScaleSetSpec {
	return azure.ScaleSetSpec{
		Name:       defaultVMSSName,
		Size:       "VM_SIZE",
		Capacity:   2,
		SSHKeyData: "ZmFrZXNzaGtleQo=",
		OSDisk: infrav1.OSDisk{
			OSType:     "Linux",
			DiskSizeGB: to.Int32Ptr(120),
			ManagedDisk: &infrav1.ManagedDiskParameters{
				StorageAccountType: "Premium_LRS",
			},
		},
		DataDisks: []infrav1.DataDisk{
			{
				NameSuffix: "my_disk",
				DiskSizeGB: 128,
				Lun:        to.Int32Ptr(0),
			},
			{
				NameSuffix: "my_disk_with_managed_disk",
				DiskSizeGB: 128,
				Lun:        to.Int32Ptr(1),
				ManagedDisk: &infrav1.ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				NameSuffix: "managed_disk_with_encryption",
				DiskSizeGB: 128,
				Lun:        to.Int32Ptr(2),
				ManagedDisk: &infrav1.ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
						ID: "encryption_id",
					},
				},
			},
		},
		SubnetName:                   "my-subnet",
		VNetName:                     "my-vnet",
		VNetResourceGroup:            defaultResourceGroup,
		PublicLBName:                 "capz-lb",
		PublicLBAddressPoolName:      "backendPool",
		AcceleratedNetworking:        nil,
		TerminateNotificationTimeout: to.IntPtr(7),
		FailureDomains:               []string{"1", "3"},
	}
}

func newWindowsVMSSSpec() azure.ScaleSetSpec {
	vmss := newDefaultVMSSSpec()
	vmss.OSDisk.OSType = azure.WindowsOS
	return vmss
}

func newDefaultExistingVMSS() compute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS()
	vmss.ID = to.StringPtr("vmss-id")
	return vmss
}

func newDefaultWindowsVMSS() compute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS()
	vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk.OsType = compute.Windows
	vmss.VirtualMachineProfile.OsProfile.LinuxConfiguration = nil
	vmss.VirtualMachineProfile.OsProfile.WindowsConfiguration = &compute.WindowsConfiguration{
		EnableAutomaticUpdates: to.BoolPtr(false),
	}
	return vmss
}

func newDefaultVMSS() compute.VirtualMachineScaleSet {
	return compute.VirtualMachineScaleSet{
		Location: to.StringPtr("test-location"),
		Tags: map[string]*string{
			"Name": to.StringPtr(defaultVMSSName),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
		},
		Sku: &compute.Sku{
			Name:     to.StringPtr("VM_SIZE"),
			Tier:     to.StringPtr("Standard"),
			Capacity: to.Int64Ptr(2),
		},
		Zones: &[]string{"1", "3"},
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			SinglePlacementGroup: to.BoolPtr(false),
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.UpgradeModeManual,
			},
			Overprovision: to.BoolPtr(false),
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile: &compute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: to.StringPtr(defaultVMSSName),
					AdminUsername:      to.StringPtr(azure.DefaultUserName),
					CustomData:         to.StringPtr("fake-bootstrap-data"),
					LinuxConfiguration: &compute.LinuxConfiguration{
						SSH: &compute.SSHConfiguration{
							PublicKeys: &[]compute.SSHPublicKey{
								{
									Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
									KeyData: to.StringPtr("fakesshkey\n"),
								},
							},
						},
						DisablePasswordAuthentication: to.BoolPtr(true),
					},
				},
				StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: to.StringPtr("fake-publisher"),
						Offer:     to.StringPtr("my-offer"),
						Sku:       to.StringPtr("sku-id"),
						Version:   to.StringPtr("1.0"),
					},
					OsDisk: &compute.VirtualMachineScaleSetOSDisk{
						OsType:       "Linux",
						CreateOption: "FromImage",
						DiskSizeGB:   to.Int32Ptr(120),
						ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
						{
							Lun:          to.Int32Ptr(0),
							Name:         to.StringPtr("my-vmss_my_disk"),
							CreateOption: "Empty",
							DiskSizeGB:   to.Int32Ptr(128),
						},
						{
							Lun:          to.Int32Ptr(1),
							Name:         to.StringPtr("my-vmss_my_disk_with_managed_disk"),
							CreateOption: "Empty",
							DiskSizeGB:   to.Int32Ptr(128),
							ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
								StorageAccountType: "Standard_LRS",
							},
						},
						{
							Lun:          to.Int32Ptr(2),
							Name:         to.StringPtr("my-vmss_managed_disk_with_encryption"),
							CreateOption: "Empty",
							DiskSizeGB:   to.Int32Ptr(128),
							ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
								StorageAccountType: "Standard_LRS",
								DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
									ID: to.StringPtr("encryption_id"),
								},
							},
						},
					},
				},
				DiagnosticsProfile: &compute.DiagnosticsProfile{
					BootDiagnostics: &compute.BootDiagnostics{
						Enabled: to.BoolPtr(true),
					},
				},
				NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: to.StringPtr("my-vmss-netconfig"),
							VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:                     to.BoolPtr(true),
								EnableAcceleratedNetworking: to.BoolPtr(false),
								EnableIPForwarding:          to.BoolPtr(true),
								IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: to.StringPtr("my-vmss-ipconfig"),
										VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
											Subnet: &compute.APIEntityReference{
												ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
											},
											Primary:                         to.BoolPtr(true),
											PrivateIPAddressVersion:         compute.IPv4,
											LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
										},
									},
								},
							},
						},
					},
				},
				ExtensionProfile: &compute.VirtualMachineScaleSetExtensionProfile{
					Extensions: &[]compute.VirtualMachineScaleSetExtension{
						{
							Name: to.StringPtr("someExtension"),
							VirtualMachineScaleSetExtensionProperties: &compute.VirtualMachineScaleSetExtensionProperties{
								Publisher:          to.StringPtr("somePublisher"),
								Type:               to.StringPtr("someExtension"),
								TypeHandlerVersion: to.StringPtr("someVersion"),
								ProtectedSettings: map[string]string{
									"commandToExecute": "echo hello",
								},
							},
						},
					},
				},
			},
		},
	}
}

func newDefaultInstances() []compute.VirtualMachineScaleSetVM {
	return []compute.VirtualMachineScaleSetVM{
		{
			ID:         to.StringPtr("my-vm-id"),
			InstanceID: to.StringPtr("my-vm-1"),
			Name:       to.StringPtr("my-vm"),
			VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: to.StringPtr("Succeeded"),
				OsProfile: &compute.OSProfile{
					ComputerName: to.StringPtr("instance-000001"),
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: to.StringPtr("fake-publisher"),
						Offer:     to.StringPtr("my-offer"),
						Sku:       to.StringPtr("sku-id"),
						Version:   to.StringPtr("1.0"),
					},
				},
			},
		},
		{
			ID:         to.StringPtr("my-vm-id"),
			InstanceID: to.StringPtr("my-vm-2"),
			Name:       to.StringPtr("my-vm"),
			VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: to.StringPtr("Succeeded"),
				OsProfile: &compute.OSProfile{
					ComputerName: to.StringPtr("instance-000002"),
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: to.StringPtr("fake-publisher"),
						Offer:     to.StringPtr("my-offer"),
						Sku:       to.StringPtr("sku-id"),
						Version:   to.StringPtr("1.0"),
					},
				},
			},
		},
	}
}

func setupDefaultVMSSInProgressOperationDoneExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder, createdVMSS compute.VirtualMachineScaleSet, instances []compute.VirtualMachineScaleSetVM) compute.VirtualMachineScaleSet {
	createdVMSS.ID = to.StringPtr("vmss-id")
	createdVMSS.ProvisioningState = to.StringPtr(string(infrav1.Succeeded))
	setupDefaultVMSSExpectations(s)
	future := &infrav1.Future{
		Type:          PutFuture,
		ResourceGroup: defaultResourceGroup,
		Name:          defaultVMSSName,
		FutureData:    "",
	}
	s.GetLongRunningOperationState().Return(future)
	m.GetResultIfDone(gomockinternal.AContext(), future).Return(createdVMSS, nil).AnyTimes()
	m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil).AnyTimes()
	s.MaxSurge().Return(1, nil)
	s.SetVMSSState(gomock.Any())
	s.SetProviderID(azure.ProviderIDPrefix + *createdVMSS.ID)
	return createdVMSS
}

func setupDefaultVMSSStartCreatingExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
	setupDefaultVMSSExpectations(s)
	s.GetLongRunningOperationState().Return(nil)
	m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).
		Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
}

func setupCreatingSucceededExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder, vmss compute.VirtualMachineScaleSet, future *infrav1.Future) {
	s.SetLongRunningOperationState(future)
	m.GetResultIfDone(gomockinternal.AContext(), future).Return(compute.VirtualMachineScaleSet{}, azure.NewOperationNotDoneError(future))
	m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(vmss, nil)
	m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(newDefaultInstances(), nil).AnyTimes()
	s.SetVMSSState(gomock.Any())
	s.SetProviderID(azure.ProviderIDPrefix + *vmss.ID)
}

func setupDefaultVMSSExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	setupVMSSExpectationsWithoutVMImage(s)
	image := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			Publisher: "fake-publisher",
			Offer:     "my-offer",
			SKU:       "sku-id",
			Version:   "1.0",
		},
	}
	s.GetVMImage().Return(image, nil).AnyTimes()
	s.SaveVMImageToStatus(image)
}

func setupUpdateVMSSExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	setupVMSSExpectationsWithoutVMImage(s)
	image := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			Publisher: "fake-publisher",
			Offer:     "my-offer",
			SKU:       "sku-id",
			Version:   "2.0",
		},
	}
	s.GetVMImage().Return(image, nil).AnyTimes()
	s.SaveVMImageToStatus(image)
}

func setupVMSSExpectationsWithoutVMImage(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	s.SubscriptionID().AnyTimes().Return(defaultSubscriptionID)
	s.ResourceGroup().AnyTimes().Return(defaultResourceGroup)
	s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
	s.AdditionalTags()
	s.Location().AnyTimes().Return("test-location")
	s.ClusterName().Return("my-cluster")
	s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
	s.VMSSExtensionSpecs().Return([]azure.VMSSExtensionSpec{
		{
			Name:         "someExtension",
			ScaleSetName: "my-vmss",
			Publisher:    "somePublisher",
			Version:      "someVersion",
			ProtectedSettings: map[string]string{
				"commandToExecute": "echo hello",
			},
		},
	}).AnyTimes()
}

func setupDefaultVMSSUpdateExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	setupUpdateVMSSExpectations(s)
	s.SetProviderID(azure.ProviderIDPrefix + "vmss-id")
	s.GetLongRunningOperationState().Return(nil)
	s.MaxSurge().Return(1, nil)
	s.SetVMSSState(gomock.Any())
}
