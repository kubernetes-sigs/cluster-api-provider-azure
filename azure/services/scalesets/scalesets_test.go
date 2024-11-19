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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets/mock_scalesets"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

const (
	defaultSubscriptionID = "123"
	defaultResourceGroup  = "my-rg"
	defaultVMSSName       = "my-vmss"
	vmSizeEPH             = "VM_SIZE_EPH"
	vmSizeUSSD            = "VM_SIZE_USSD"
	defaultVMSSID         = "subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm-id"
	sshKeyData            = "ZmFrZXNzaGtleQo="
)

var (
	defaultImage = infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			ImagePlan: infrav1.ImagePlan{
				Publisher: "fake-publisher",
				Offer:     "my-offer",
				SKU:       "sku-id",
			},
			Version: "1.0",
		},
	}

	notFoundError = &azcore.ResponseError{StatusCode: http.StatusNotFound}
)

func internalError() *azcore.ResponseError {
	return &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
}

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func getDefaultVMSSSpec() azure.ResourceSpecGetter {
	defaultSpec := newDefaultVMSSSpec()
	defaultSpec.DataDisks = append(defaultSpec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})

	return &defaultSpec
}

func getResultVMSS() armcompute.VirtualMachineScaleSet {
	resultVMSS := newDefaultVMSS("VM_SIZE")
	resultVMSS.ID = ptr.To(defaultVMSSID)

	return resultVMSS
}

func TestReconcileVMSS(t *testing.T) {
	defaultInstances := newDefaultInstances()
	resultVMSS := newDefaultVMSS("VM_SIZE")
	resultVMSS.ID = ptr.To(defaultVMSSID)
	fetchedVMSS := converters.SDKToVMSS(getResultVMSS(), defaultInstances)

	testcases := []struct {
		name          string
		expect        func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "update an existing vmss",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := getDefaultVMSSSpec()
				// Validate spec
				s.ScaleSetSpec(gomockinternal.AContext()).Return(spec).AnyTimes()
				m.Get(gomockinternal.AContext(), &defaultSpec).Return(resultVMSS, nil)
				m.ListInstances(gomockinternal.AContext(), defaultSpec.ResourceGroup, defaultSpec.Name).Return(defaultInstances, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), spec, serviceName).Return(getResultVMSS(), nil)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)

				s.ReconcileReplicas(gomockinternal.AContext(), &fetchedVMSS).Return(nil).Times(2)
				s.SetProviderID(azureutil.ProviderIDPrefix + defaultVMSSID).Times(2)
				s.SetVMSSState(&fetchedVMSS).Times(2)
			},
		},
		{
			name:          "create a vmss, skip list instances if vmss doesn't exist",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := getDefaultVMSSSpec()
				// Validate spec
				s.ScaleSetSpec(gomockinternal.AContext()).Return(spec).AnyTimes()
				m.Get(gomockinternal.AContext(), &defaultSpec).Return(nil, notFoundError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), spec, serviceName).Return(getResultVMSS(), nil)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)

				s.ReconcileReplicas(gomockinternal.AContext(), &fetchedVMSS).Return(nil)
				s.SetProviderID(azureutil.ProviderIDPrefix + defaultVMSSID)
				s.SetVMSSState(&fetchedVMSS)
			},
		},
		{
			name:          "error getting existing vmss",
			expectedError: "failed to get existing VMSS:.*#: Internal Server Error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := getDefaultVMSSSpec()
				// Validate spec
				s.ScaleSetSpec(gomockinternal.AContext()).Return(spec).AnyTimes()
				m.Get(gomockinternal.AContext(), &defaultSpec).Return(nil, internalError())
			},
		},
		{
			name:          "failed to list instances",
			expectedError: "failed to get existing VMSS instances:.*#: Internal Server Error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := getDefaultVMSSSpec()
				// Validate spec
				s.ScaleSetSpec(gomockinternal.AContext()).Return(spec).AnyTimes()
				m.Get(gomockinternal.AContext(), &defaultSpec).Return(&resultVMSS, nil)
				m.ListInstances(gomockinternal.AContext(), defaultSpec.ResourceGroup, defaultSpec.Name).Return(defaultInstances, internalError())
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, gomockinternal.ErrStrEq("failed to get existing VMSS instances: "+internalError().Error()))
			},
		},
		{
			name:          "failed to create a vmss",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := getDefaultVMSSSpec()
				s.ScaleSetSpec(gomockinternal.AContext()).Return(spec).AnyTimes()
				m.Get(gomockinternal.AContext(), &defaultSpec).Return(resultVMSS, nil)
				m.ListInstances(gomockinternal.AContext(), defaultSpec.ResourceGroup, defaultSpec.Name).Return(defaultInstances, nil)

				r.CreateOrUpdateResource(gomockinternal.AContext(), spec, serviceName).
					Return(nil, internalError())
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, internalError())

				s.ReconcileReplicas(gomockinternal.AContext(), &fetchedVMSS).Return(nil)
				s.SetProviderID(azureutil.ProviderIDPrefix + defaultVMSSID)
				s.SetVMSSState(&fetchedVMSS)
			},
		},
		{
			name:          "validate spec failure: less than 2 vCPUs",
			expectedError: "reconcile error that cannot be recovered occurred: vm size should be bigger or equal to at least 2 vCPUs. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := newDefaultVMSSSpec()
				spec.Size = "VM_SIZE_1_CPU"
				spec.Capacity = 2
				spec.SSHKeyData = sshKeyData
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&spec).AnyTimes()
			},
		},
		{
			name:          "validate spec failure: Memory is less than 2Gi",
			expectedError: "reconcile error that cannot be recovered occurred: vm memory should be bigger or equal to at least 2Gi. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := newDefaultVMSSSpec()
				spec.Size = "VM_SIZE_1_MEM"
				spec.Capacity = 2
				spec.SSHKeyData = sshKeyData
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&spec).AnyTimes()
			},
		},
		{
			name:          "validate spec failure: failed to get SKU",
			expectedError: "failed to get SKU INVALID_VM_SIZE in compute api: reconcile error that cannot be recovered occurred: resource sku with name 'INVALID_VM_SIZE' and category 'virtualMachines' not found in location 'test-location'. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := newDefaultVMSSSpec()
				spec.Size = "INVALID_VM_SIZE"
				spec.Capacity = 2
				spec.SSHKeyData = sshKeyData
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&spec).AnyTimes()
			},
		},
		{
			name:          "validate spec failure: fail to create a vm with ultra disk implicitly enabled by data disk, when location not supported",
			expectedError: "reconcile error that cannot be recovered occurred: vm size VM_SIZE_USSD does not support ultra disks in location test-location. select a different vm size or disable ultra disks. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := newDefaultVMSSSpec()
				spec.Size = vmSizeUSSD
				spec.Capacity = 2
				spec.SSHKeyData = sshKeyData
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&spec).AnyTimes()
			},
		},
		{
			name:          "validate spec failure: fail to create a vm with ultra disk explicitly enabled via additional capabilities, when location not supported",
			expectedError: "reconcile error that cannot be recovered occurred: vm size VM_SIZE_USSD does not support ultra disks in location test-location. select a different vm size or disable ultra disks. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := newDefaultVMSSSpec()
				spec.Size = vmSizeUSSD
				spec.Capacity = 2
				spec.SSHKeyData = sshKeyData
				spec.AdditionalCapabilities = &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: ptr.To(true),
				}
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&spec).AnyTimes()
			},
		},
		{
			name:          "validate spec failure: fail to create a vm with ultra disk explicitly enabled via additional capabilities, when location not supported",
			expectedError: "reconcile error that cannot be recovered occurred: vm size VM_SIZE_USSD does not support ultra disks in location test-location. select a different vm size or disable ultra disks. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := newDefaultVMSSSpec()
				spec.Size = vmSizeUSSD
				spec.Capacity = 2
				spec.SSHKeyData = sshKeyData
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				spec.AdditionalCapabilities = &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: ptr.To(true),
				}
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&spec).AnyTimes()
			},
		},
		{
			name:          "validate spec failure: fail to create a vm with diagnostics set to User Managed but empty StorageAccountURI",
			expectedError: "reconcile error that cannot be recovered occurred: userManaged must be specified when storageAccountType is 'UserManaged'. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				spec := newDefaultVMSSSpec()
				spec.Size = vmSizeUSSD
				spec.Capacity = 2
				spec.SSHKeyData = sshKeyData
				spec.DiagnosticsProfile = &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
						UserManaged:        nil,
					},
				}
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&spec).AnyTimes()
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)
			clientMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(g, scopeMock.EXPECT(), asyncMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				Reconciler:       asyncMock,
				Client:           clientMock,
				resourceSKUCache: resourceskus.NewStaticCache(getFakeSkus(), "test-location"),
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.ReplaceAll(err.Error(), "\n", "")).To(MatchRegexp(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVMSS(t *testing.T) {
	defaultSpec := newDefaultVMSSSpec()
	defaultInstances := newDefaultInstances()
	resultVMSS := newDefaultVMSS("VM_SIZE")
	resultVMSS.ID = ptr.To(defaultVMSSID)
	fetchedVMSS := converters.SDKToVMSS(getResultVMSS(), defaultInstances)
	// Be careful about race conditions if you need modify these.

	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing vmss",
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&defaultSpec).AnyTimes()
				r.DeleteResource(gomockinternal.AContext(), &defaultSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)

				m.Get(gomockinternal.AContext(), &defaultSpec).Return(resultVMSS, nil)
				m.ListInstances(gomockinternal.AContext(), defaultSpec.ResourceGroup, defaultSpec.Name).Return(defaultInstances, nil)
				s.SetVMSSState(&fetchedVMSS)
			},
		},
		{
			name:          "successfully delete an existing vmss, fetch call returns error",
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&defaultSpec).AnyTimes()
				r.DeleteResource(gomockinternal.AContext(), &defaultSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				m.Get(gomockinternal.AContext(), &defaultSpec).Return(armcompute.VirtualMachineScaleSet{}, notFoundError)
			},
		},
		{
			name:          "failed to delete an existing vmss",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.ScaleSetSpec(gomockinternal.AContext()).Return(&defaultSpec).AnyTimes()
				r.DeleteResource(gomockinternal.AContext(), &defaultSpec, serviceName).Return(internalError())
				s.UpdateDeleteStatus(infrav1.BootstrapSucceededCondition, serviceName, internalError())
				m.Get(gomockinternal.AContext(), &defaultSpec).Return(armcompute.VirtualMachineScaleSet{}, notFoundError)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)
			mockClient := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT(), mockClient.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
				Client:     mockClient,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getFakeSkus() []armcompute.ResourceSKU {
	return []armcompute.ResourceSKU{
		{
			Name:         ptr.To("VM_SIZE"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: []*string{
				ptr.To("test-location"),
			},
			LocationInfo: []*armcompute.ResourceSKULocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    []*string{ptr.To("1"), ptr.To("3")},
					ZoneDetails: []*armcompute.ResourceSKUZoneDetails{
						{
							Capabilities: []*armcompute.ResourceSKUCapabilities{
								{
									Name:  ptr.To("UltraSSDAvailable"),
									Value: ptr.To("True"),
								},
							},
							Name: []*string{ptr.To("1"), ptr.To("3")},
						},
					},
				},
			},
			Capabilities: []*armcompute.ResourceSKUCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("4"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_AN"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: []*string{
				ptr.To("test-location"),
			},
			LocationInfo: []*armcompute.ResourceSKULocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    []*string{ptr.To("1"), ptr.To("3")},
				},
			},
			Capabilities: []*armcompute.ResourceSKUCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("6"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_1_CPU"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: []*string{
				ptr.To("test-location"),
			},
			LocationInfo: []*armcompute.ResourceSKULocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    []*string{ptr.To("1"), ptr.To("3")},
				},
			},
			Capabilities: []*armcompute.ResourceSKUCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("1"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("4"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_1_MEM"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: []*string{
				ptr.To("test-location"),
			},
			LocationInfo: []*armcompute.ResourceSKULocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    []*string{ptr.To("1"), ptr.To("3")},
				},
			},
			Capabilities: []*armcompute.ResourceSKUCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("2"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("1"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_EAH"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: []*string{
				ptr.To("test-location"),
			},
			LocationInfo: []*armcompute.ResourceSKULocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    []*string{ptr.To("1"), ptr.To("3")},
				},
			},
			Capabilities: []*armcompute.ResourceSKUCapabilities{
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("8"),
				},
				{
					Name:  ptr.To(resourceskus.EncryptionAtHost),
					Value: ptr.To(string(resourceskus.CapabilitySupported)),
				},
			},
		},
		{
			Name:         ptr.To(vmSizeUSSD),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: []*string{
				ptr.To("test-location"),
			},
			LocationInfo: []*armcompute.ResourceSKULocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    []*string{ptr.To("1"), ptr.To("3")},
				},
			},
			Capabilities: []*armcompute.ResourceSKUCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("6"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_EPH"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: []*string{
				ptr.To("test-location"),
			},
			LocationInfo: []*armcompute.ResourceSKULocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    []*string{ptr.To("1"), ptr.To("3")},
					ZoneDetails: []*armcompute.ResourceSKUZoneDetails{
						{
							Capabilities: []*armcompute.ResourceSKUCapabilities{
								{
									Name:  ptr.To("UltraSSDAvailable"),
									Value: ptr.To("True"),
								},
							},
							Name: []*string{ptr.To("1"), ptr.To("3")},
						},
					},
				},
			},
			Capabilities: []*armcompute.ResourceSKUCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.EphemeralOSDisk),
					Value: ptr.To("True"),
				},
			},
		},
	}
}

func newDefaultVMSSSpec() ScaleSetSpec {
	return ScaleSetSpec{
		Name:       defaultVMSSName,
		Size:       "VM_SIZE",
		Capacity:   2,
		SSHKeyData: sshKeyData,
		OSDisk: infrav1.OSDisk{
			OSType:     "Linux",
			DiskSizeGB: ptr.To[int32](120),
			ManagedDisk: &infrav1.ManagedDiskParameters{
				StorageAccountType: "Premium_LRS",
			},
		},
		DataDisks: []infrav1.DataDisk{
			{
				NameSuffix: "my_disk",
				DiskSizeGB: 128,
				Lun:        ptr.To[int32](0),
			},
			{
				NameSuffix: "my_disk_with_managed_disk",
				DiskSizeGB: 128,
				Lun:        ptr.To[int32](1),
				ManagedDisk: &infrav1.ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				NameSuffix: "managed_disk_with_encryption",
				DiskSizeGB: 128,
				Lun:        ptr.To[int32](2),
				ManagedDisk: &infrav1.ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
						ID: "encryption_id",
					},
				},
			},
		},
		DiagnosticsProfile: &infrav1.Diagnostics{
			Boot: &infrav1.BootDiagnostics{
				StorageAccountType: infrav1.ManagedDiagnosticsStorage,
			},
		},
		SubnetName:                   "my-subnet",
		VNetName:                     "my-vnet",
		VNetResourceGroup:            defaultResourceGroup,
		PublicLBName:                 "capz-lb",
		PublicLBAddressPoolName:      "backendPool",
		AcceleratedNetworking:        nil,
		TerminateNotificationTimeout: ptr.To(7),
		FailureDomains:               []string{"1", "3"},
		NetworkInterfaces: []infrav1.NetworkInterface{
			{
				SubnetName:       "my-subnet",
				PrivateIPConfigs: 1,
			},
		},
		SubscriptionID: "123",
		ResourceGroup:  "my-rg",
		Location:       "test-location",
		ClusterName:    "my-cluster",
		VMSSInstances:  newDefaultInstances(),
		VMImage:        &defaultImage,
		BootstrapData:  "fake-bootstrap-data",
		VMSSExtensionSpecs: []azure.ResourceSpecGetter{
			&VMSSExtensionSpec{
				ExtensionSpec: azure.ExtensionSpec{
					Name:      "someExtension",
					VMName:    "my-vmss",
					Publisher: "somePublisher",
					Version:   "someVersion",
					Settings: map[string]string{
						"someSetting": "someValue",
					},
					ProtectedSettings: map[string]string{
						"commandToExecute": "echo hello",
					},
				},
				ResourceGroup: "my-rg",
			},
		},
		PlatformFaultDomainCount: ptr.To(int32(1)),
		ZoneBalance:              ptr.To(true),
	}
}

func newWindowsVMSSSpec() ScaleSetSpec {
	vmss := newDefaultVMSSSpec()
	vmss.OSDisk.OSType = azure.WindowsOS
	return vmss
}

func newDefaultExistingVMSS() armcompute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.ID = ptr.To("subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
	return vmss
}

func newDefaultWindowsVMSS() armcompute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.OSType = ptr.To(armcompute.OperatingSystemTypesWindows)
	vmss.Properties.VirtualMachineProfile.OSProfile.LinuxConfiguration = nil
	vmss.Properties.VirtualMachineProfile.OSProfile.WindowsConfiguration = &armcompute.WindowsConfiguration{
		EnableAutomaticUpdates: ptr.To(false),
	}
	return vmss
}

func newDefaultVMSS(vmSize string) armcompute.VirtualMachineScaleSet {
	dataDisk := fetchDataDiskBasedOnSize(vmSize)
	return armcompute.VirtualMachineScaleSet{
		Location: ptr.To("test-location"),
		Tags: map[string]*string{
			"Name": ptr.To(defaultVMSSName),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               ptr.To("node"),
		},
		SKU: &armcompute.SKU{
			Name:     ptr.To(vmSize),
			Tier:     ptr.To("Standard"),
			Capacity: ptr.To[int64](2),
		},
		Zones: []*string{ptr.To("1"), ptr.To("3")},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			SinglePlacementGroup: ptr.To(false),
			UpgradePolicy: &armcompute.UpgradePolicy{
				Mode: ptr.To(armcompute.UpgradeModeManual),
			},
			Overprovision:     ptr.To(false),
			OrchestrationMode: ptr.To(armcompute.OrchestrationModeUniform),
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				OSProfile: &armcompute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: ptr.To(defaultVMSSName),
					AdminUsername:      ptr.To(azure.DefaultUserName),
					CustomData:         ptr.To("fake-bootstrap-data"),
					LinuxConfiguration: &armcompute.LinuxConfiguration{
						SSH: &armcompute.SSHConfiguration{
							PublicKeys: []*armcompute.SSHPublicKey{
								{
									Path:    ptr.To("/home/capi/.ssh/authorized_keys"),
									KeyData: ptr.To("fakesshkey\n"),
								},
							},
						},
						DisablePasswordAuthentication: ptr.To(true),
					},
				},
				ScheduledEventsProfile: &armcompute.ScheduledEventsProfile{
					TerminateNotificationProfile: &armcompute.TerminateNotificationProfile{
						NotBeforeTimeout: ptr.To("PT7M"),
						Enable:           ptr.To(true),
					},
				},
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &armcompute.ImageReference{
						Publisher: ptr.To("fake-publisher"),
						Offer:     ptr.To("my-offer"),
						SKU:       ptr.To("sku-id"),
						Version:   ptr.To("1.0"),
					},
					OSDisk: &armcompute.VirtualMachineScaleSetOSDisk{
						OSType:       ptr.To(armcompute.OperatingSystemTypesLinux),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesFromImage),
						DiskSizeGB:   ptr.To[int32](120),
						ManagedDisk: &armcompute.VirtualMachineScaleSetManagedDiskParameters{
							StorageAccountType: ptr.To(armcompute.StorageAccountTypesPremiumLRS),
						},
					},
					DataDisks: dataDisk,
				},
				DiagnosticsProfile: &armcompute.DiagnosticsProfile{
					BootDiagnostics: &armcompute.BootDiagnostics{
						Enabled: ptr.To(true),
					},
				},
				NetworkProfile: &armcompute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: []*armcompute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: ptr.To("my-vmss-nic-0"),
							Properties: &armcompute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:                     ptr.To(true),
								EnableAcceleratedNetworking: ptr.To(false),
								EnableIPForwarding:          ptr.To(true),
								IPConfigurations: []*armcompute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: ptr.To("ipConfig0"),
										Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
											Subnet: &armcompute.APIEntityReference{
												ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
											},
											Primary:                         ptr.To(true),
											PrivateIPAddressVersion:         ptr.To(armcompute.IPVersionIPv4),
											LoadBalancerBackendAddressPools: []*armcompute.SubResource{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
										},
									},
								},
							},
						},
					},
				},
				ExtensionProfile: &armcompute.VirtualMachineScaleSetExtensionProfile{
					Extensions: []*armcompute.VirtualMachineScaleSetExtension{
						{
							Name: ptr.To("someExtension"),
							Properties: &armcompute.VirtualMachineScaleSetExtensionProperties{
								Publisher:          ptr.To("somePublisher"),
								Type:               ptr.To("someExtension"),
								TypeHandlerVersion: ptr.To("someVersion"),
								Settings: map[string]string{
									"someSetting": "someValue",
								},
								ProtectedSettings: map[string]string{
									"commandToExecute": "echo hello",
								},
							},
						},
					},
				},
			},
			PlatformFaultDomainCount: ptr.To(int32(1)),
			ZoneBalance:              ptr.To(true),
			// AdditionalCapabilities: &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)},
		},
	}
}

func fetchDataDiskBasedOnSize(vmSize string) []*armcompute.VirtualMachineScaleSetDataDisk {
	var dataDisk []*armcompute.VirtualMachineScaleSetDataDisk
	if vmSize == "VM_SIZE" {
		dataDisk = []*armcompute.VirtualMachineScaleSetDataDisk{
			{
				Lun:          ptr.To[int32](0),
				Name:         ptr.To("my-vmss_my_disk"),
				CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
				DiskSizeGB:   ptr.To[int32](128),
			},
			{
				Lun:          ptr.To[int32](1),
				Name:         ptr.To("my-vmss_my_disk_with_managed_disk"),
				CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &armcompute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: ptr.To(armcompute.StorageAccountTypesStandardLRS),
				},
			},
			{
				Lun:          ptr.To[int32](2),
				Name:         ptr.To("my-vmss_managed_disk_with_encryption"),
				CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &armcompute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: ptr.To(armcompute.StorageAccountTypesStandardLRS),
					DiskEncryptionSet: &armcompute.DiskEncryptionSetParameters{
						ID: ptr.To("encryption_id"),
					},
				},
			},
			{
				Lun:          ptr.To[int32](3),
				Name:         ptr.To("my-vmss_my_disk_with_ultra_disks"),
				CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &armcompute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: ptr.To(armcompute.StorageAccountTypesUltraSSDLRS),
				},
			},
		}
	} else {
		dataDisk = []*armcompute.VirtualMachineScaleSetDataDisk{
			{
				Lun:          ptr.To[int32](0),
				Name:         ptr.To("my-vmss_my_disk"),
				CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
				DiskSizeGB:   ptr.To[int32](128),
			},
			{
				Lun:          ptr.To[int32](1),
				Name:         ptr.To("my-vmss_my_disk_with_managed_disk"),
				CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &armcompute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: ptr.To(armcompute.StorageAccountTypesStandardLRS),
				},
			},
			{
				Lun:          ptr.To[int32](2),
				Name:         ptr.To("my-vmss_managed_disk_with_encryption"),
				CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &armcompute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: ptr.To(armcompute.StorageAccountTypesStandardLRS),
					DiskEncryptionSet: &armcompute.DiskEncryptionSetParameters{
						ID: ptr.To("encryption_id"),
					},
				},
			},
		}
	}
	return dataDisk
}

func newDefaultInstances() []armcompute.VirtualMachineScaleSetVM {
	return []armcompute.VirtualMachineScaleSetVM{
		{
			ID:         ptr.To("my-vm-id"),
			InstanceID: ptr.To("my-vm-1"),
			Name:       ptr.To("my-vm"),
			Properties: &armcompute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: ptr.To("Succeeded"),
				OSProfile: &armcompute.OSProfile{
					ComputerName: ptr.To("instance-000001"),
				},
				StorageProfile: &armcompute.StorageProfile{
					ImageReference: &armcompute.ImageReference{
						Publisher: ptr.To("fake-publisher"),
						Offer:     ptr.To("my-offer"),
						SKU:       ptr.To("sku-id"),
						Version:   ptr.To("1.0"),
					},
				},
			},
		},
		{
			ID:         ptr.To("my-vm-id"),
			InstanceID: ptr.To("my-vm-2"),
			Name:       ptr.To("my-vm"),
			Properties: &armcompute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: ptr.To("Succeeded"),
				OSProfile: &armcompute.OSProfile{
					ComputerName: ptr.To("instance-000002"),
				},
				StorageProfile: &armcompute.StorageProfile{
					ImageReference: &armcompute.ImageReference{
						Publisher: ptr.To("fake-publisher"),
						Offer:     ptr.To("my-offer"),
						SKU:       ptr.To("sku-id"),
						Version:   ptr.To("1.0"),
					},
				},
			},
		},
	}
}
