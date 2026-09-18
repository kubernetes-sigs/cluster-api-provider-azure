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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines/mock_virtualmachines"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
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
	fakeExistingVM = armcompute.VirtualMachine{
		ID:   ptr.To("subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm"),
		Name: ptr.To("test-vm-name"),
		Properties: &armcompute.VirtualMachineProperties{
			ProvisioningState: ptr.To("Succeeded"),
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
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
	fakePublicIPs = armnetwork.PublicIPAddress{
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
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
		ProviderID: "/subscriptions/123/resourceGroups/test-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/fake-provider-id",
	}
	fakeUserAssignedIdentityWithPrefix = infrav1.UserAssignedIdentity{
		ProviderID: azureutil.ProviderIDPrefix + fakeUserAssignedIdentity.ProviderID,
	}
	fakeUserAssignedIdentity2 = infrav1.UserAssignedIdentity{
		ProviderID: fakeUserAssignedIdentity.ProviderID + "-2",
	}
	fakeUserAssignedIdentity2WithPrefix = infrav1.UserAssignedIdentity{
		ProviderID: azureutil.ProviderIDPrefix + fakeUserAssignedIdentity2.ProviderID,
	}
)

func internalError() *azcore.ResponseError {
	return &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
}

// fakeReapplyPoller creates a fake runtime.Poller for VM Reapply operations, simulating
// an in-progress long-running operation returned by BeginReapply.
func fakeReapplyPoller() *runtime.Poller[armcompute.VirtualMachinesClientReapplyResponse] {
	header := http.Header{}
	header.Set("Location", "https://management.azure.com/fake-reapply-operation")
	response := &http.Response{
		Body:   io.NopCloser(strings.NewReader("")),
		Header: header,
		Request: &http.Request{
			Method: http.MethodPost,
			URL:    &url.URL{Path: "/"},
		},
		StatusCode: http.StatusAccepted,
	}
	pipeline := runtime.NewPipeline("testmodule", "v0.1.0", runtime.PipelineOptions{}, nil)
	poller, err := runtime.NewPoller[armcompute.VirtualMachinesClientReapplyResponse](response, pipeline, nil)
	if err != nil {
		panic(err)
	}
	return poller
}

func TestReconcileVM(t *testing.T) {
	fakeFailedVM := armcompute.VirtualMachine{
		ID:   ptr.To("subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-failed-vm"),
		Name: ptr.To("test-vm-name"),
		Properties: &armcompute.VirtualMachineProperties{
			ProvisioningState: ptr.To("Failed"),
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: ptr.To("/subscriptions/123/resourceGroups/test-rg/providers/Microsoft.Network/networkInterfaces/nic-1"),
					},
				},
			},
		},
	}
	fakeReappliedVM := armcompute.VirtualMachine{
		ID:   ptr.To("subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm"),
		Name: ptr.To("test-vm-name"),
		Properties: &armcompute.VirtualMachineProperties{
			ProvisioningState: ptr.To("Succeeded"),
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: ptr.To("/subscriptions/123/resourceGroups/test-rg/providers/Microsoft.Network/networkInterfaces/nic-1"),
					},
				},
			},
		},
	}

	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder)
	}{
		{
			name:          "noop if no vm spec is found",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(nil)
			},
		},
		{
			name:          "create vm succeeds",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(nil, &azcore.ResponseError{StatusCode: http.StatusNotFound})
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
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(nil, &azcore.ResponseError{StatusCode: http.StatusNotFound})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(nil, internalError())
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, internalError())
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, internalError())
			},
		},
		{
			name:          "create vm succeeds but failed to get network interfaces",
			expectedError: "failed to fetch VM addresses:.*#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(nil, &azcore.ResponseError{StatusCode: http.StatusNotFound})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(armnetwork.Interface{}, internalError())
			},
		},
		{
			name:          "create vm succeeds but failed to get public IPs",
			expectedError: "failed to fetch VM addresses:.*#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(nil, &azcore.ResponseError{StatusCode: http.StatusNotFound})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(fakeExistingVM, nil)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, nil)
				s.SetProviderID("azure://subscriptions/123/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				mnic.Get(gomockinternal.AContext(), &fakeNetworkInterfaceGetterSpec).Return(fakeNetworkInterface, nil)
				mpip.Get(gomockinternal.AContext(), &fakePublicIPSpec).Return(armnetwork.PublicIPAddress{}, internalError())
			},
		},
		{
			name:          "reapply vm in Failed state succeeds",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(fakeFailedVM, nil)
				// First call: hasActiveReapplyFuture check in Reconcile (VM is Failed, no existing future)
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(nil)
				// Second call: resume token lookup inside reapplyVM
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(nil)
				c.ReapplyAsync(gomockinternal.AContext(), &fakeVMSpec, "").Return(nil, nil)
				s.DeleteLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(fakeReappliedVM, nil)
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
			name:          "reapply vm in Failed state fails",
			expectedError: "failed to reapply VM",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(fakeFailedVM, nil)
				// First call: hasActiveReapplyFuture check in Reconcile
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(nil)
				// Second call: resume token lookup inside reapplyVM
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(nil)
				c.ReapplyAsync(gomockinternal.AContext(), &fakeVMSpec, "").Return(nil, internalError())
				s.DeleteLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture)
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, gomock.Any())
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, gomock.Any())
			},
		},
		{
			name:          "reapply vm in Failed state times out and saves future for resume",
			expectedError: "operation type PUT on Azure resource.*is not done",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(fakeFailedVM, nil)
				// First call: hasActiveReapplyFuture check in Reconcile
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(nil)
				// Second call: resume token lookup inside reapplyVM
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(nil)
				c.ReapplyAsync(gomockinternal.AContext(), &fakeVMSpec, "").Return(fakeReapplyPoller(), context.DeadlineExceeded)
				s.DefaultedReconcilerRequeue().Return(reconciler.DefaultReconcilerRequeue)
				s.SetLongRunningOperationState(gomock.Any())
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, gomock.Any())
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, gomock.Any())
			},
		},
		{
			name:          "existing vm in Succeeded state uses normal flow",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(fakeExistingVM, nil)
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(nil)
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
			name:          "resume reapply when VM exited Failed state but future still exists",
			expectedError: "could not decode reapply future data",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, mnic *mock_async.MockGetterMockRecorder, mpip *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder, c *mock_virtualmachines.MockClientMockRecorder) {
				// The VM transitioned out of "Failed" (e.g. to "Succeeded") mid-reapply,
				// but the future is still stored. The reapply path must be entered so that
				// status conditions are updated correctly. Here the stored future has corrupted
				// data (empty Data field), so we verify the state is cleared and an error is
				// returned — the reconciler will retry on the next loop.
				staleFuture := &infrav1.Future{
					Type:          infrav1.PutFuture,
					ServiceName:   reapplyServiceName,
					Name:          fakeVMSpec.Name,
					ResourceGroup: fakeVMSpec.ResourceGroup,
					// Data is intentionally empty: FutureToResumeToken returns an error,
					// causing the state to be cleared and the error to propagate.
				}
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(&fakeVMSpec)
				c.Get(gomockinternal.AContext(), &fakeVMSpec).Return(fakeExistingVM, nil) // Succeeded, not Failed
				// First call: hasActiveReapplyFuture check in Reconcile — future present, enter reapply path.
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(staleFuture)
				// Second call: resume-token lookup inside reapplyVM.
				s.GetLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture).Return(staleFuture)
				// FutureToResumeToken fails (empty Data) → clear the bad state, return error.
				s.DeleteLongRunningOperationState(fakeVMSpec.Name, reapplyServiceName, infrav1.PutFuture)
				// reapplyVM returns early with an error; Reconcile updates status with that error.
				s.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, gomock.Any())
				s.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, gomock.Any())
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_virtualmachines.NewMockVMScope(mockCtrl)
			interfaceMock := mock_async.NewMockGetter(mockCtrl)
			publicIPMock := mock_async.NewMockGetter(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)
			clientMock := mock_virtualmachines.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT(), asyncMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				client:           clientMock,
				interfacesGetter: interfaceMock,
				publicIPsGetter:  publicIPMock,
				Reconciler:       asyncMock,
			}

			err := s.Reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.ReplaceAll(err.Error(), "\n", "")).To(MatchRegexp(tc.expectedError))
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
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().Return(nil)
			},
		},
		{
			name:          "vm doesn't exist",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
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
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(internalError())
				s.SetVMState(infrav1.Deleting)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, internalError())
			},
		},
		{
			name:          "delete the vm successfully",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.VMSpec().AnyTimes().Return(&fakeVMSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeVMSpec, serviceName).Return(nil)
				s.SetVMState(infrav1.Deleted)
				s.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, nil)
			},
		},
	}

	for _, tc := range testcases {
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

			err := s.Delete(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
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
		expectedKey      string
	}{
		{
			name:             "no user assigned identities",
			specIdentities:   []infrav1.UserAssignedIdentity{},
			actualIdentities: []infrav1.UserAssignedIdentity{},
		},
		{
			name:             "matching user assigned identities",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentityWithPrefix},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
		},
		{
			name:             "less user assigned identities than expected",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity, fakeUserAssignedIdentity2WithPrefix},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			expectedKey:      fakeUserAssignedIdentity2WithPrefix.ProviderID,
		},
		{
			name:             "more user assigned identities than expected",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity, fakeUserAssignedIdentity2},
		},
		{
			name:             "mismatched user assigned identities by content",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity2},
			expectedKey:      fakeUserAssignedIdentity.ProviderID,
		},
		{
			name:             "duplicate user assigned identity in spec",
			specIdentities:   []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity, fakeUserAssignedIdentity},
			actualIdentities: []infrav1.UserAssignedIdentity{fakeUserAssignedIdentity},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_virtualmachines.NewMockVMScope(mockCtrl)

			if tc.expectedKey != "" {
				scopeMock.EXPECT().SetConditionFalse(infrav1.VMIdentitiesReadyCondition, infrav1.UserAssignedIdentityMissingReason, clusterv1beta1.ConditionSeverityWarning, vmMissingUAI+tc.expectedKey).Times(1)
			}
			s := &Service{
				Scope: scopeMock,
			}

			s.checkUserAssignedIdentities(tc.specIdentities, tc.actualIdentities)
		})
	}
}
