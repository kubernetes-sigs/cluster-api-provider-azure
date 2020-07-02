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
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers/mock_publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus/mock_resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets/mock_scalesets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestNewService(t *testing.T) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}
	client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)
	s, err := scope.NewClusterScope(scope.ClusterScopeParams{
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
	g := gomega.NewGomegaWithT(t)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	mps, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Client:           client,
		Logger:           s.Logger,
		MachinePool:      new(clusterv1exp.MachinePool),
		AzureMachinePool: new(infrav1exp.AzureMachinePool),
		ClusterDescriber: s,
	})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	actual := NewService(mps)
	g.Expect(actual).ToNot(gomega.BeNil())
}

func TestService_Get(t *testing.T) {
	cases := []struct {
		Name        string
		SpecFactory func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *Spec
		Setup       func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *gomock.Controller
		Expect      func(ctx context.Context, g *gomega.GomegaWithT, result interface{}, err error)
	}{
		{
			Name: "WithValidSpecBut404FromAzureOnVMSS",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *Spec {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *gomock.Controller {
				mockCtrl := gomock.NewController(t)

				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock
				vmssMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, mpScope.Name()).Return(compute.VirtualMachineScaleSet{}, autorest.DetailedError{
					StatusCode: 404,
				})

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, result interface{}, err error) {
				g.Expect(err).To(gomega.Equal(autorest.DetailedError{
					StatusCode: 404,
				}))
			},
		},
		{
			Name: "WithValidSpecBut404FromAzureOnInstances",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *Spec {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *gomock.Controller {
				mockCtrl := gomock.NewController(t)

				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock
				vmssMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, mpScope.Name()).Return(compute.VirtualMachineScaleSet{}, nil)
				vmssMock.EXPECT().ListInstances(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, mpScope.Name()).Return([]compute.VirtualMachineScaleSetVM{}, autorest.DetailedError{
					StatusCode: 404,
				})

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, result interface{}, err error) {
				g.Expect(err).To(gomega.Equal(autorest.DetailedError{
					StatusCode: 404,
				}))
			},
		},
		{
			Name: "WithValidSpecWithVMSSAndInstancesReturned",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *Spec {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *gomock.Controller {
				mockCtrl := gomock.NewController(t)

				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock
				vmssMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, mpScope.Name()).Return(compute.VirtualMachineScaleSet{
					Name: to.StringPtr(mpScope.Name()),
					Sku: &compute.Sku{
						Capacity: to.Int64Ptr(1),
						Name:     to.StringPtr("Standard"),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						ProvisioningState: to.StringPtr("Succeeded"),
					},
				}, nil)
				vmssMock.EXPECT().ListInstances(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, mpScope.Name()).Return([]compute.VirtualMachineScaleSetVM{
					{
						Name:       to.StringPtr("vm0"),
						InstanceID: to.StringPtr("0"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					},
				}, nil)

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, result interface{}, err error) {
				g.Expect(result).To(gomega.Equal(&infrav1exp.VMSS{
					Name:     "capz-mp-0",
					Sku:      "Standard",
					Capacity: 1,
					Image:    infrav1.Image{},
					State:    "Succeeded",
					Instances: []infrav1exp.VMSSVM{
						{
							InstanceID: "0",
							Name:       "vm0",
							State:      "Succeeded",
						},
					},
				}))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			s, mps := getScopes(g)
			svc := NewService(s)
			spec := c.SpecFactory(g, s, mps)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if c.Setup != nil {
				mockctrl := c.Setup(ctx, g, svc, s, mps)
				defer mockctrl.Finish()
			}
			res, err := svc.Get(context.Background(), spec)
			c.Expect(ctx, g, res, err)
		})
	}
}

func TestService_Reconcile(t *testing.T) {
	cases := []struct {
		Name        string
		SpecFactory func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{}
		Setup       func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope, spec *Spec) *gomock.Controller
		Expect      func(ctx context.Context, g *gomega.GomegaWithT, err error)
	}{
		{
			Name: "WithInvalidSepcType",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{} {
				return "bazz"
			},
			Expect: func(_ context.Context, g *gomega.GomegaWithT, err error) {
				g.Expect(err).To(gomega.MatchError("invalid VMSS specification"))
			},
		},
		{
			Name: "WithValidSpec",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{} {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
					Sku:                    "skuName",
					Capacity:               2,
					SSHKeyData:             "sshKeyData",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "accountType",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					Image: &infrav1.Image{
						ID: to.StringPtr("image"),
					},
					CustomData: "customData",
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope, spec *Spec) *gomock.Controller {
				mockCtrl := gomock.NewController(t)
				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock
				skusMock := mock_resourceskus.NewMockClient(mockCtrl)
				svc.ResourceSkusClient = skusMock
				lbMock := mock_publicloadbalancers.NewMockClient(mockCtrl)
				svc.PublicLoadBalancersClient = lbMock

				storageProfile, err := generateStorageProfile(*spec)
				g.Expect(err).ToNot(gomega.HaveOccurred())

				vmss := compute.VirtualMachineScaleSet{
					Location: to.StringPtr(scope.Location()),
					Tags: map[string]*string{
						"Name":                            to.StringPtr("capz-mp-0"),
						"kubernetes.io_cluster_capz-mp-0": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr(spec.Sku),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(spec.Capacity),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeManual,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr(spec.Name),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr(spec.CustomData),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
												KeyData: to.StringPtr(spec.SSHKeyData),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: storageProfile,
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr(spec.Name + "-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(false),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr(spec.Name + "-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr(scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				skusMock.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any()).Return(false, nil)
				lbMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.ClusterName).Return(getFakeNodeOutboundLoadBalancer(), nil)
				vmssMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.Name).Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				vmssMock.EXPECT().CreateOrUpdate(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.Name, matchers.DiffEq(vmss)).Return(nil)

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, err error) {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			},
		},
		{
			Name: "WithAcceleratedNetworking",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{} {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
					Sku:                    "skuName",
					Capacity:               2,
					SSHKeyData:             "sshKeyData",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "accountType",
						},
					},
					Image: &infrav1.Image{
						ID: to.StringPtr("image"),
					},
					CustomData: "customData",
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope, spec *Spec) *gomock.Controller {
				mockCtrl := gomock.NewController(t)
				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock
				skusMock := mock_resourceskus.NewMockClient(mockCtrl)
				svc.ResourceSkusClient = skusMock
				lbMock := mock_publicloadbalancers.NewMockClient(mockCtrl)
				svc.PublicLoadBalancersClient = lbMock

				storageProfile, err := generateStorageProfile(*spec)
				g.Expect(err).ToNot(gomega.HaveOccurred())

				vmss := compute.VirtualMachineScaleSet{
					Location: to.StringPtr(scope.Location()),
					Tags: map[string]*string{
						"Name":                            to.StringPtr("capz-mp-0"),
						"kubernetes.io_cluster_capz-mp-0": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr(spec.Sku),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(spec.Capacity),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeManual,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr(spec.Name),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr(spec.CustomData),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
												KeyData: to.StringPtr(spec.SSHKeyData),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: storageProfile,
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr(spec.Name + "-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(true),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr(spec.Name + "-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr(scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				skusMock.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any()).Return(true, nil)
				lbMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.ClusterName).Return(getFakeNodeOutboundLoadBalancer(), nil)
				vmssMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.Name).Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				vmssMock.EXPECT().CreateOrUpdate(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.Name, matchers.DiffEq(vmss)).Return(nil)

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, err error) {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			},
		},
		{
			Name: "Scale Set already exists",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{} {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
					Sku:                    "skuName",
					Capacity:               2,
					SSHKeyData:             "sshKeyData",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "accountType",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					Image: &infrav1.Image{
						ID: to.StringPtr("image"),
					},
					CustomData: "customData",
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope, spec *Spec) *gomock.Controller {
				mockCtrl := gomock.NewController(t)
				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock
				skusMock := mock_resourceskus.NewMockClient(mockCtrl)
				svc.ResourceSkusClient = skusMock
				lbMock := mock_publicloadbalancers.NewMockClient(mockCtrl)
				svc.PublicLoadBalancersClient = lbMock

				storageProfile, err := generateStorageProfile(*spec)
				g.Expect(err).ToNot(gomega.HaveOccurred())

				vmss := compute.VirtualMachineScaleSet{
					Location: to.StringPtr(scope.Location()),
					Tags: map[string]*string{
						"Name":                            to.StringPtr("capz-mp-0"),
						"kubernetes.io_cluster_capz-mp-0": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr(spec.Sku),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(spec.Capacity),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeManual,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr(spec.Name),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr(spec.CustomData),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
												KeyData: to.StringPtr(spec.SSHKeyData),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: storageProfile,
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr(spec.Name + "-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(false),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr(spec.Name + "-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr(scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				update := compute.VirtualMachineScaleSetUpdate{
					Tags: map[string]*string{
						"Name":                            to.StringPtr("capz-mp-0"),
						"kubernetes.io_cluster_capz-mp-0": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr(spec.Sku),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(spec.Capacity),
					},
					VirtualMachineScaleSetUpdateProperties: &compute.VirtualMachineScaleSetUpdateProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeManual,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetUpdateVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetUpdateOSProfile{
								CustomData: to.StringPtr(spec.CustomData),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
												KeyData: to.StringPtr(spec.SSHKeyData),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: &compute.VirtualMachineScaleSetUpdateStorageProfile{
								ImageReference: &compute.ImageReference{ID: to.StringPtr("image")},
								OsDisk: &compute.VirtualMachineScaleSetUpdateOSDisk{
									DiskSizeGB:  to.Int32Ptr(120),
									ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{StorageAccountType: "accountType"},
								},
								DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
									{
										Name:         to.StringPtr("capz-mp-0_my_disk"),
										Lun:          to.Int32Ptr(0),
										CreateOption: "Empty",
										DiskSizeGB:   to.Int32Ptr(128),
									},
								},
							},
						},
					},
				}

				skusMock.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any()).Return(false, nil)
				lbMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.ClusterName).Return(getFakeNodeOutboundLoadBalancer(), nil)
				vmssMock.EXPECT().Get(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.Name).Return(vmss, nil)
				vmssMock.EXPECT().Update(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, spec.Name, matchers.DiffEq(update)).Return(nil)

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, err error) {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			s, mps := getScopes(g)
			svc := NewService(s)
			spec := c.SpecFactory(g, s, mps)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if c.Setup != nil {
				mockCtrl := c.Setup(ctx, g, svc, s, mps, spec.(*Spec))
				defer mockCtrl.Finish()
			}
			err := svc.Reconcile(context.Background(), spec)
			c.Expect(ctx, g, err)
		})
	}
}

func TestService_Delete(t *testing.T) {
	cases := []struct {
		Name        string
		SpecFactory func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{}
		Setup       func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *gomock.Controller
		Expect      func(ctx context.Context, g *gomega.GomegaWithT, err error)
	}{
		{
			Name: "WithInvalidSepcType",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{} {
				return "foo"
			},
			Expect: func(_ context.Context, g *gomega.GomegaWithT, err error) {
				g.Expect(err).To(gomega.MatchError("invalid VMSS specification"))
			},
		},
		{
			Name: "WithValidSpecBut404FromAzureOnVMSSAssumeAlreadyDeleted",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{} {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *gomock.Controller {
				mockCtrl := gomock.NewController(t)
				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock

				vmssMock.EXPECT().Delete(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, mpScope.Name()).Return(autorest.DetailedError{
					StatusCode: 404,
				})

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, err error) {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			},
		},
		{
			Name: "WithValidSpecAndSuccessfulDelete",
			SpecFactory: func(g *gomega.GomegaWithT, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) interface{} {
				return &Spec{
					Name:                   mpScope.Name(),
					ResourceGroup:          scope.AzureCluster.Spec.ResourceGroup,
					Location:               scope.AzureCluster.Spec.Location,
					ClusterName:            scope.Cluster.Name,
					SubnetID:               scope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
					PublicLoadBalancerName: scope.Cluster.Name,
					MachinePoolName:        mpScope.Name(),
				}
			},
			Setup: func(ctx context.Context, g *gomega.GomegaWithT, svc *Service, scope *scope.ClusterScope, mpScope *scope.MachinePoolScope) *gomock.Controller {
				mockCtrl := gomock.NewController(t)
				vmssMock := mock_scalesets.NewMockClient(mockCtrl)
				svc.Client = vmssMock
				vmssMock.EXPECT().Delete(gomock.Any(), scope.AzureCluster.Spec.ResourceGroup, mpScope.Name()).Return(nil)

				return mockCtrl
			},
			Expect: func(ctx context.Context, g *gomega.GomegaWithT, err error) {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			s, mps := getScopes(g)
			svc := NewService(s)
			spec := c.SpecFactory(g, s, mps)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if c.Setup != nil {
				mockCtrl := c.Setup(ctx, g, svc, s, mps)
				defer mockCtrl.Finish()
			}
			err := svc.Delete(context.Background(), spec)
			c.Expect(ctx, g, err)
		})
	}
}

func TestGetVMSSUpdateFromVMSS(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	vmss := compute.VirtualMachineScaleSet{
		Location: to.StringPtr("eastus"),
		Tags: map[string]*string{
			"Name": to.StringPtr("capz-mp-0"),
		},
		Sku: &compute.Sku{
			Name: to.StringPtr("sku"),
		},
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.UpgradeModeManual,
			},
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile: &compute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: to.StringPtr("vmss"),
					CustomData:         to.StringPtr("data"),
				},
				StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &compute.ImageReference{
						ID: to.StringPtr("image-id"),
					},
				},
				NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: to.StringPtr("netconfig"),
						},
					},
				},
			},
		},
	}

	expectedUpdate := compute.VirtualMachineScaleSetUpdate{
		Tags: map[string]*string{
			"Name": to.StringPtr("capz-mp-0"),
		},
		Sku: &compute.Sku{
			Name: to.StringPtr("sku"),
		},
		VirtualMachineScaleSetUpdateProperties: &compute.VirtualMachineScaleSetUpdateProperties{
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.UpgradeModeManual,
			},
			VirtualMachineProfile: &compute.VirtualMachineScaleSetUpdateVMProfile{
				OsProfile: &compute.VirtualMachineScaleSetUpdateOSProfile{
					CustomData: to.StringPtr("data"),
				},
				StorageProfile: &compute.VirtualMachineScaleSetUpdateStorageProfile{
					ImageReference: &compute.ImageReference{
						ID: to.StringPtr("image-id"),
					},
				},
				NetworkProfile: &compute.VirtualMachineScaleSetUpdateNetworkProfile{
					NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetUpdateNetworkConfiguration{
						{
							Name: to.StringPtr("netconfig"),
						},
					},
				},
			},
		},
	}
	result, err := getVMSSUpdateFromVMSS(vmss)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(expectedUpdate))
}

func getScopes(g *gomega.GomegaWithT) (*scope.ClusterScope, *scope.MachinePoolScope) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}
	client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)
	s, err := scope.NewClusterScope(scope.ClusterScopeParams{
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
					Subnets: infrav1.Subnets{
						{
							ID: "subnet0.id",
						},
					},
				},
			},
		},
	})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	mps, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Client:      client,
		Logger:      s.Logger,
		MachinePool: new(clusterv1exp.MachinePool),
		AzureMachinePool: &infrav1exp.AzureMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "capz-mp-0",
			},
		},
		ClusterDescriber: s,
	})
	g.Expect(err).ToNot(gomega.HaveOccurred())

	return s, mps
}

func getFakeNodeOutboundLoadBalancer() network.LoadBalancer {
	return network.LoadBalancer{
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: to.StringPtr("frontend-ip-config-id"),
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					ID: pointer.StringPtr("cluster-name-outboundBackendPool"),
				},
			},
		}}
}
