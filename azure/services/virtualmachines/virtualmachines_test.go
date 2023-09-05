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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/identities/mock_identities"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines/mock_virtualmachines"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
		Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
		BootstrapData:     "fake data",
	}
	fakeExistingVM = compute.VirtualMachine{
		ID:   ptr.To("subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm"),
		Name: ptr.To("test-vm-name"),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			ProvisioningState: ptr.To("Succeeded"),
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: ptr.To("/subscriptions/123/resourceGroups/test-rg/providers/Microsoft.Network/networkInterfaces/nic-1"),
					},
				},
			},
		},
	}
	fakeNetworkInterfaceGetterSpec = networkinterfaces.NICSpec{
		Name:          "nic-1",
		ResourceGroup: "test-group",
	}
	fakeNetworkInterface = armnetwork.Interface{
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddress: ptr.To("10.0.0.5"),
						PublicIPAddress: &armnetwork.PublicIPAddress{
							ID: ptr.To("/subscriptions/123/resourceGroups/test-rg/providers/Microsoft.Network/publicIPAddresses/pip-1"),
						},
					},
				},
			},
		},
	}
	fakePublicIPSpec = publicips.PublicIPSpec{
		Name:          "pip-1",
		ResourceGroup: "test-group",
	}
	fakePublicIPs = network.PublicIPAddress{
		PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
			IPAddress: ptr.To("10.0.0.6"),
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
	fakeUserAssignedIdentity = infrav1.UserAssignedIdentity{
		ProviderID: "fake-provider-id",
	}
	fakeUserAssignedIdentity2 = infrav1.UserAssignedIdentity{
		ProviderID: "fake-provider-id-2",
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func TestReconcileVM(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no vm spec is found",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(nil)
			},
		},
		{
			name:          "create vm succeeds",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(fakeNetworkInterface, nil)
				mpip.Get(gomockinternal.AContext(), &fakePublicIPSpec).Return(fakePublicIPs, nil)
				s.SetAddresses(fakeNodeAddresses)
				s.SetVMState(infrav1.Succeeded)
			},
		},
		{
			name:          "creating vm fails",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, internalError)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "create vm succeeds but failed to get network interfaces",
			expectedError: "failed to fetch VM addresses: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(armnetwork.Interface{}, internalError)
			},
		},
		{
			name:          "create vm succeeds but failed to get public IPs",
			expectedError: "failed to fetch VM addresses: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMSpec().Return(&fakeVMSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(fakeNetworkInterface, nil)
				mpip.Get(gomockinternal.AContext(), &fakePublicIPSpec).Return(network.PublicIPAddress{}, internalError)
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
			publicIPMock := mock_async.NewMockGetter(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				interfacesGetter: interfaceMock,
				publicIPsGetter:  publicIPMock,
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

func TestCheckUserAssignedIdentities(t *testing.T) {
	testcases := []struct {
		name             string
		specIdentities   []infrav1.UserAssignedIdentity
		actualIdentities []infrav1.UserAssignedIdentity
		expect           func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder)
		expectedError    string
	}{
		{
			name:             "no user assigned identities",
			specIdentities:   []infrav1.UserAssignedIdentity{},
			actualIdentities: []infrav1.UserAssignedIdentity{},
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder) {
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity.ProviderID).AnyTimes().Return(fakeUserAssignedIdentity.ProviderID, nil)
			},
			expectedError: "",
		},
		{
			name:             "matching user assigned identities",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder) {
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity.ProviderID).AnyTimes().Return(fakeUserAssignedIdentity.ProviderID, nil)
			},
			expectedError: "",
		},
		{
			name:             "less user assigned identities than expected",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity, fakeUserAssignedIdentity2},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder) {
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity.ProviderID).AnyTimes().Return(fakeUserAssignedIdentity.ProviderID, nil)
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity2.ProviderID).AnyTimes().Return(fakeUserAssignedIdentity2.ProviderID, nil)
				s.SetConditionFalse(infrav1.VMIdentitiesReadyCondition, infrav1.UserAssignedIdentityMissingReason, clusterv1.ConditionSeverityWarning, "VM is missing expected user assigned identity with client ID: "+fakeUserAssignedIdentity2.ProviderID).Times(1)
			},
			expectedError: "",
		},
		{
			name:             "more user assigned identities than expected",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity, fakeUserAssignedIdentity2},
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder) {
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity.ProviderID).AnyTimes().Return(fakeUserAssignedIdentity.ProviderID, nil)
			},
			expectedError: "",
		},
		{
			name:             "mismatched user assigned identities by content",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity2},
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder) {
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity.ProviderID).AnyTimes().Return(fakeUserAssignedIdentity.ProviderID, nil)
				s.SetConditionFalse(infrav1.VMIdentitiesReadyCondition, infrav1.UserAssignedIdentityMissingReason, clusterv1.ConditionSeverityWarning, "VM is missing expected user assigned identity with client ID: "+fakeUserAssignedIdentity.ProviderID).Times(1)
			},
			expectedError: "",
		},
		{
			name:             "duplicate user assigned identity in spec",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity, fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder) {
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity.ProviderID).AnyTimes().Return(fakeUserAssignedIdentity.ProviderID, nil)
			},
			expectedError: "",
		},
		{
			name:             "invalid client id",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, i *mock_identities.MockClientMockRecorder) {
				i.GetClientID(gomockinternal.AContext(), fakeUserAssignedIdentity.ProviderID).AnyTimes().Return("", errors.New("failed to get client id"))
			},
			expectedError: "failed to get client id",
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
			identitiesMock := mock_identities.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), identitiesMock.EXPECT())
			s := &Service{
				Scope:            scopeMock,
				Reconciler:       asyncMock,
				identitiesGetter: identitiesMock,
			}

			err := s.checkUserAssignedIdentities(context.TODO(), tc.specIdentities, tc.actualIdentities)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
