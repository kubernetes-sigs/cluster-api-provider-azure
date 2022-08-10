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
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networkinterfaces"
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
	fakeExistingVM = compute.VirtualMachine{
		ID:   to.StringPtr("subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm"),
		Name: to.StringPtr("test-vm-name"),
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
	}
	fakeNetworkInterfaceGetterSpec = networkinterfaces.NICSpec{
		Name:          "nic-1",
		ResourceGroup: "test-group",
	}
	fakeNetworkInterface = network.Interface{
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
	}
	fakePublicIPs = network.PublicIPAddress{
		PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
			IPAddress: to.StringPtr("10.0.0.6"),
		},
	}
	fakeNodeAddresses = []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalDNS,
			Address: "test-vm-name",
		},
		{
			Type:    corev1.NodeInternalIP,
			Address: "10.0.0.5",
		},
		{
			Type:    corev1.NodeExternalIP,
			Address: "10.0.0.6",
		},
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
)

func TestReconcileVM(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_publicips.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no vm spec is found",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_publicips.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(nil)
			},
		},
		{
			name:          "create vm succeeds",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_publicips.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(fakeNetworkInterface, nil)
				mpip.Get(gomockinternal.AContext(), "test-group", "pip-1").Return(fakePublicIPs, nil)
				s.SetAddresses(fakeNodeAddresses)
				s.SetVMState(infrav1.Succeeded)
			},
		},
		{
			name:          "creating vm fails",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_publicips.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, internalError)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "create vm succeeds but failed to get network interfaces",
			expectedError: "failed to fetch VM addresses: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_publicips.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(network.Interface{}, internalError)
			},
		},
		{
			name:          "create vm succeeds but failed to get public IPs",
			expectedError: "failed to fetch VM addresses: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_publicips.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(fakeNetworkInterface, nil)
				mpip.Get(gomockinternal.AContext(), "test-group", "pip-1").Return(network.PublicIPAddress{}, internalError)
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
			interfaceMock := mock_async.NewMockGetter(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				interfacesGetter: interfaceMock,
				publicIPsClient:  publicIPMock,
				Reconciler:       asyncMock,
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
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no vm spec is found",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(nil)
			},
		},
		{
			name:          "vm doesn't exist",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(nil)
				s.SetVMState(infrav1.Deleted)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, nil)
			},
		},
		{
			name:          "error occurs when deleting vm",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(internalError)
				s.SetVMState(infrav1.Deleting)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, internalError)
			},
		},
		{
			name:          "delete the vm successfully",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(nil)
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
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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
