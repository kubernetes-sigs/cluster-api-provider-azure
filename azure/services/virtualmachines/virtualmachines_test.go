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

package virtualmachines

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/availabilitysets/mock_availabilitysets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines/mock_virtualmachines"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeVMSpec = VMSpec{
		Name:              "test-vm",
		ResourceGroup:     "test-group",
		Location:          "test-location",
		ClusterName:       "test-cluster",
		Role:              infrav1.ControlPlane,
		NICIDs:            []string{"nic-id-1", "nic-id-2"},
		SSHKeyData:        "fake ssh key",
		Size:              "Standard_Fake_Size",
		AvailabilitySetID: "availability-set",
		Identity:          infrav1.VMIdentitySystemAssigned,
		AdditionalTags:    map[string]string{"foo": "bar"},
		Image:             &infrav1.Image{ID: to.StringPtr("fake-image-id")},
		BootstrapData:     "fake data",
	}
	fakeFuture = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   serviceName,
		Name:          "test-vm",
		ResourceGroup: "test-group",
		Data:          "eyJtZXRob2QiOiJERUxFVEUiLCJwb2xsaW5nTWV0aG9kIjoiTG9jYXRpb24iLCJscm9TdGF0ZSI6IkluUHJvZ3Jlc3MifQ==",
	}
	internalError  = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
	notFoundError  = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found")
	errCtxExceeded = errors.New("ctx exceeded")
)

func TestReconcileVM(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:          "create vm succeeds",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakeVMSpec).Return(compute.VirtualMachine{
					ID: to.StringPtr("test-vm-id"),
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						ProvisioningState: to.StringPtr("Succeeded"),
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									ID: to.StringPtr("/subscriptions/123/resourceGroups/test-rg/providers/Microsoft.Network/networkInterfaces/nic-1"),
								},
							},
						},
					},
				}, nil, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://test-vm-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), "test-group", "nic-1").Return(network.Interface{
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									PrivateIPAddress: to.StringPtr("10.0.0.5"),
									PublicIPAddress: &network.PublicIPAddress{
										ID: to.StringPtr("/subscriptions/123/resourceGroups/test-rg/providers/Microsoft.Network/publicIPAddresses/pip-1"),
									},
								},
							},
						},
					},
				}, nil)
				mpip.Get(gomockinternal.AContext(), "test-group", "pip-1").Return(network.PublicIPAddress{
					PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
						IPAddress: to.StringPtr("10.0.0.6"),
					},
				}, nil)
				s.SetAddresses([]corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "10.0.0.5",
					},
					{
						Type:    corev1.NodeExternalIP,
						Address: "10.0.0.6",
					},
				})
				s.SetVMState(infrav1.Succeeded)
			},
		},
		{
			name:          "create vm fails",
			expectedError: "failed to create resource test-group/test-vm (service: virtualmachine): #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakeVMSpec).Return(nil, nil, internalError)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, gomockinternal.ErrStrEq(fmt.Sprintf("failed to create resource test-group/test-vm (service: virtualmachine): %s", internalError.Error())))
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, gomockinternal.ErrStrEq(fmt.Sprintf("failed to create resource test-group/test-vm (service: virtualmachine): %s", internalError.Error())))
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

			scopeMock := mock_virtualmachines.NewMockVMScope(mockCtrl)
			clientMock := mock_virtualmachines.NewMockClient(mockCtrl)
			interfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)
			availabilitySetsMock := mock_availabilitysets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT())

			s := &Service{
				Scope:                  scopeMock,
				Client:                 clientMock,
				interfacesClient:       interfaceMock,
				publicIPsClient:        publicIPMock,
				availabilitySetsClient: availabilitySetsMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVM(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder)
	}{
		{
			name:          "long running delete operation is done",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName).Times(2).Return(&fakeFuture)
				m.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(true, nil)
				m.Result(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{}), infrav1.DeleteFuture).Return(nil, nil)
				s.DeleteLongRunningOperationState("test-vm", serviceName)
				s.SetVMState(infrav1.Deleted)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, nil)
			},
		},
		{
			name:          "long running delete operation is not done",
			expectedError: "operation type DELETE on Azure resource test-group/test-vm is not done. Object will be requeued after 15s",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName).Times(2).Return(&fakeFuture)
				m.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
				s.SetVMState(infrav1.Deleting)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, gomockinternal.ErrStrEq("operation type DELETE on Azure resource test-group/test-vm is not done. Object will be requeued after 15s"))
			},
		},
		{
			name:          "vm doesn't exist",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName)
				m.DeleteAsync(gomockinternal.AContext(), &fakeVMSpec).Return(nil, notFoundError)
				s.SetVMState(infrav1.Deleted)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, nil)
			},
		},
		{
			name:          "error occurs when deleting vm",
			expectedError: "failed to delete resource test-group/test-vm (service: virtualmachine): #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakeVMSpec).Return(nil, internalError)
				s.SetVMState(infrav1.Deleting)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource test-group/test-vm (service: virtualmachine): #: Internal Server Error: StatusCode=500"))
			},
		},
		{
			name:          "context deadline exceeded while deleting vm",
			expectedError: "operation type DELETE on Azure resource test-group/test-vm is not done. Object will be requeued after 15s",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakeVMSpec).Return(&azureautorest.Future{}, errCtxExceeded)
				s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{}))
				s.SetVMState(infrav1.Deleting)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, gomockinternal.ErrStrEq("operation type DELETE on Azure resource test-group/test-vm is not done. Object will be requeued after 15s"))
			},
		},
		{
			name:          "delete the vm successfully",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				s.GetLongRunningOperationState("test-vm", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakeVMSpec).Return(nil, nil)
				s.SetVMState(infrav1.Deleted)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, nil)
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
			scopeMock := mock_virtualmachines.NewMockVMScope(mockCtrl)
			clientMock := mock_virtualmachines.NewMockClient(mockCtrl)

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
