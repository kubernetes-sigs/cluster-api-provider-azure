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

package controllers

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privatedns"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vnetpeerings"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

func TestAzureClusterServiceReconcile(t *testing.T) {
	cases := map[string]struct {
		expectedError string
		expect        func(one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder)
	}{
		"all services are reconciled in order": {
			expectedError: "",
			expect: func(one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					one.Reconcile(gomockinternal.AContext()).Return(nil),
					two.Reconcile(gomockinternal.AContext()).Return(nil),
					three.Reconcile(gomockinternal.AContext()).Return(nil))
			},
		},
		"service reconcile fails": {
			expectedError: "failed to reconcile AzureCluster service two: some error happened",
			expect: func(one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					one.Reconcile(gomockinternal.AContext()).Return(nil),
					two.Reconcile(gomockinternal.AContext()).Return(errors.New("some error happened")),
					two.Name().Return("two"))
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			svcOneMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcTwoMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcThreeMock := mock_azure.NewMockServiceReconciler(mockCtrl)

			tc.expect(svcOneMock.EXPECT(), svcTwoMock.EXPECT(), svcThreeMock.EXPECT())

			s := &azureClusterService{
				scope: &scope.ClusterScope{
					Cluster:      &clusterv1.Cluster{},
					AzureCluster: &infrav1.AzureCluster{},
				},
				services: []azure.ServiceReconciler{
					svcOneMock,
					svcTwoMock,
					svcThreeMock,
				},
				skuCache: resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, ""),
			}

			err := s.reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterServicePause(t *testing.T) {
	type pausingServiceReconciler struct {
		*mock_azure.MockServiceReconciler
		*mock_azure.MockPauser
	}

	cases := map[string]struct {
		expectedError string
		expect        func(one pausingServiceReconciler, two pausingServiceReconciler, three pausingServiceReconciler)
	}{
		"all services are paused in order": {
			expectedError: "",
			expect: func(one pausingServiceReconciler, two pausingServiceReconciler, three pausingServiceReconciler) {
				gomock.InOrder(
					one.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil),
					two.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil),
					three.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil))
			},
		},
		"service pause fails": {
			expectedError: "failed to pause AzureCluster service two: some error happened",
			expect: func(one pausingServiceReconciler, two pausingServiceReconciler, _ pausingServiceReconciler) {
				gomock.InOrder(
					one.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil),
					two.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(errors.New("some error happened")),
					two.MockServiceReconciler.EXPECT().Name().Return("two"))
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			newPausingServiceReconciler := func() pausingServiceReconciler {
				return pausingServiceReconciler{
					mock_azure.NewMockServiceReconciler(mockCtrl),
					mock_azure.NewMockPauser(mockCtrl),
				}
			}
			svcOneMock := newPausingServiceReconciler()
			svcTwoMock := newPausingServiceReconciler()
			svcThreeMock := newPausingServiceReconciler()

			tc.expect(svcOneMock, svcTwoMock, svcThreeMock)

			s := &azureClusterService{
				services: []azure.ServiceReconciler{
					svcOneMock,
					svcTwoMock,
					svcThreeMock,
				},
			}

			err := s.pause(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterServiceDelete(t *testing.T) {
	clusterName := "cluster"
	azClusterName := "azCluster"
	namespace := "ns"
	resourceGroup := "rg"

	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion:         infrav1.GroupVersion.String(),
			Kind:               infrav1.AzureClusterKind,
			Name:               azClusterName,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
	}

	cases := map[string]struct {
		expectedError string
		clientBuilder func(g Gomega) client.Client
		expect        func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder)
	}{
		"Resource Group is deleted successfully": {
			expectedError: "",
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())

				rg := &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:            resourceGroup,
						Namespace:       namespace,
						OwnerReferences: ownerRefs,
						Annotations: map[string]string{
							asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
						},
					},
				}

				c := fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(rg).
					Build()

				return c
			},
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					grp.Name().Return(groups.ServiceName),
					vpr.Name().Return(vnetpeerings.ServiceName),
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Name().Return(groups.ServiceName),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"Resource Group delete fails": {
			expectedError: "failed to delete resource group: internal error",
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())

				rg := &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:            resourceGroup,
						Namespace:       namespace,
						OwnerReferences: ownerRefs,
						Annotations: map[string]string{
							asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
						},
					},
				}

				c := fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(rg).
					Build()

				return c
			},
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					grp.Name().Return(groups.ServiceName),
					vpr.Name().Return(vnetpeerings.ServiceName),
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Name().Return(groups.ServiceName),
					grp.Delete(gomockinternal.AContext()).Return(errors.New("internal error")))
			},
		},
		"Resource Group not owned by cluster": {
			expectedError: "",
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())

				rg := &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:            resourceGroup,
						Namespace:       namespace,
						OwnerReferences: []metav1.OwnerReference{},
					},
				}

				c := fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(rg).
					Build()

				return c
			},
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					three.Delete(gomockinternal.AContext()).Return(nil),
					two.Delete(gomockinternal.AContext()).Return(nil),
					one.Delete(gomockinternal.AContext()).Return(nil),
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"service delete fails": {
			expectedError: "failed to delete AzureCluster service two: some error happened",
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())

				rg := &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:            resourceGroup,
						Namespace:       namespace,
						OwnerReferences: []metav1.OwnerReference{},
					},
				}

				c := fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(rg).
					Build()

				return c
			},
			expect: func(_ *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder, _ *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					three.Delete(gomockinternal.AContext()).Return(nil),
					two.Delete(gomockinternal.AContext()).Return(errors.New("some error happened")),
					two.Name().Return("two"))
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			groupsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			vnetpeeringsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcOneMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcTwoMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcThreeMock := mock_azure.NewMockServiceReconciler(mockCtrl)

			tc.expect(groupsMock.EXPECT(), vnetpeeringsMock.EXPECT(), svcOneMock.EXPECT(), svcTwoMock.EXPECT(), svcThreeMock.EXPECT())
			c := tc.clientBuilder(g)

			s := &azureClusterService{
				scope: &scope.ClusterScope{
					Client: c,
					AzureCluster: &infrav1.AzureCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      azClusterName,
							Namespace: namespace,
						},
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: resourceGroup,
							NetworkSpec: infrav1.NetworkSpec{
								Vnet: infrav1.VnetSpec{
									ResourceGroup: resourceGroup,
								},
							},
						},
					},
					Cluster: &clusterv1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:              clusterName,
							Namespace:         namespace,
							DeletionTimestamp: &metav1.Time{Time: time.Now()},
						},
					},
				},
				services: []azure.ServiceReconciler{
					groupsMock,
					vnetpeeringsMock,
					svcOneMock,
					svcTwoMock,
					svcThreeMock,
				},
				skuCache: resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, ""),
			}

			err := s.delete(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterServiceDeletePrivateDNS(t *testing.T) {
	clusterName := "cluster"
	azClusterName := "azCluster"
	namespace := "ns"
	resourceGroup := "rg"

	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion:         infrav1.GroupVersion.String(),
			Kind:               infrav1.AzureClusterKind,
			Name:               azClusterName,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
	}

	internalLB := &infrav1.LoadBalancerSpec{
		FrontendIPs: []infrav1.FrontendIP{
			{
				Name: "api-server-lb-internal-ip",
				FrontendIPClass: infrav1.FrontendIPClass{
					PrivateIPAddress: "10.0.0.100",
				},
			},
		},
		LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
			Type: infrav1.Internal,
		},
	}
	internalLBWithoutFrontendIPs := &infrav1.LoadBalancerSpec{
		LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
			Type: infrav1.Internal,
		},
	}
	publicLB := &infrav1.LoadBalancerSpec{
		LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
			Type: infrav1.Public,
		},
	}

	// Matches what async.Reconciler returns while a delete is in progress.
	notDoneErr := azure.WithTransientError(azure.NewOperationNotDoneError(&infrav1.Future{Type: "privatedns", ResourceGroup: "dns-rg", Name: "zone"}), 15*time.Second)

	cases := map[string]struct {
		vnetResourceGroup           string
		groupsNotManaged            bool
		apiServerLB                 *infrav1.LoadBalancerSpec
		privateDNSZone              *infrav1.PrivateDNSZoneMode
		privateDNSZoneResourceGroup string
		expectedError               string
		verifyError                 func(g Gomega, err error)
		expect                      func(grp, vpr, dns *mock_azure.MockServiceReconcilerMockRecorder)
	}{
		"zone in an external resource group is deleted before the resource groups": {
			apiServerLB:                 internalLB,
			privateDNSZoneResourceGroup: "dns-rg",
			expect: func(grp, vpr, dns *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					dns.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"zone in the cluster resource group is deleted with the resource group": {
			apiServerLB: internalLB,
			expect: func(grp, vpr, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"zone in the cluster resource group with different casing is deleted with the resource group": {
			apiServerLB:                 internalLB,
			privateDNSZoneResourceGroup: "RG",
			expect: func(grp, vpr, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"zone in the vnet resource group is deleted with the resource group": {
			vnetResourceGroup:           "vnet-rg",
			apiServerLB:                 internalLB,
			privateDNSZoneResourceGroup: "vnet-rg",
			expect: func(grp, vpr, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"zone in the vnet resource group with different casing is deleted with the resource group": {
			vnetResourceGroup:           "vnet-rg",
			apiServerLB:                 internalLB,
			privateDNSZoneResourceGroup: "VNet-RG",
			expect: func(grp, vpr, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"no private dns zone for a public api server": {
			apiServerLB:                 publicLB,
			privateDNSZoneResourceGroup: "dns-rg",
			expect: func(grp, vpr, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"no private dns zone when private dns zone mode is none": {
			apiServerLB:                 internalLB,
			privateDNSZone:              ptr.To(infrav1.PrivateDNSZoneModeNone),
			privateDNSZoneResourceGroup: "dns-rg",
			expect: func(grp, vpr, _ *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"private dns delete fails and resource groups are not deleted": {
			apiServerLB:                 internalLB,
			privateDNSZoneResourceGroup: "dns-rg",
			expectedError:               "failed to delete private DNS zone: internal error",
			expect: func(_, vpr, dns *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					dns.Delete(gomockinternal.AContext()).Return(errors.New("internal error")))
			},
		},
		"private dns delete in progress is retried and resource groups are not deleted": {
			apiServerLB:                 internalLB,
			privateDNSZoneResourceGroup: "dns-rg",
			verifyError: func(g Gomega, err error) {
				g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
				var reconcileErr azure.ReconcileError
				g.Expect(errors.As(err, &reconcileErr)).To(BeTrue())
				g.Expect(reconcileErr.IsTransient()).To(BeTrue())
			},
			expect: func(_, vpr, dns *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					dns.Delete(gomockinternal.AContext()).Return(notDoneErr))
			},
		},
		"zone in an external resource group is deleted when the internal LB has no frontend IPs": {
			apiServerLB:                 internalLBWithoutFrontendIPs,
			privateDNSZoneResourceGroup: "dns-rg",
			expect: func(grp, vpr, dns *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					dns.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"unmanaged resource group deletes private dns once with the other services": {
			groupsNotManaged:            true,
			apiServerLB:                 internalLB,
			privateDNSZoneResourceGroup: "dns-rg",
			expect: func(grp, vpr, dns *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					dns.Delete(gomockinternal.AContext()).Return(nil),
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			groupsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			vnetpeeringsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			privateDNSMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			groupsMock.EXPECT().Name().Return(groups.ServiceName).AnyTimes()
			vnetpeeringsMock.EXPECT().Name().Return(vnetpeerings.ServiceName).AnyTimes()
			privateDNSMock.EXPECT().Name().Return(privatedns.ServiceName).AnyTimes()
			tc.expect(groupsMock.EXPECT(), vnetpeeringsMock.EXPECT(), privateDNSMock.EXPECT())

			vnetResourceGroup := resourceGroup
			if tc.vnetResourceGroup != "" {
				vnetResourceGroup = tc.vnetResourceGroup
			}

			scheme := runtime.NewScheme()
			g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
			g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			rgNames := []string{resourceGroup}
			if vnetResourceGroup != resourceGroup {
				rgNames = append(rgNames, vnetResourceGroup)
			}
			for _, rgName := range rgNames {
				rg := &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rgName,
						Namespace: namespace,
					},
				}
				if !tc.groupsNotManaged {
					rg.OwnerReferences = ownerRefs
					rg.Annotations = map[string]string{
						asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
					}
				}
				builder = builder.WithObjects(rg)
			}

			s := &azureClusterService{
				scope: &scope.ClusterScope{
					Client: builder.Build(),
					AzureCluster: &infrav1.AzureCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      azClusterName,
							Namespace: namespace,
						},
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: resourceGroup,
							NetworkSpec: infrav1.NetworkSpec{
								NetworkClassSpec: infrav1.NetworkClassSpec{
									PrivateDNSZoneName:          "example.com",
									PrivateDNSZoneResourceGroup: tc.privateDNSZoneResourceGroup,
								},
								Vnet: infrav1.VnetSpec{
									ResourceGroup: vnetResourceGroup,
								},
								APIServerLB:    tc.apiServerLB,
								PrivateDNSZone: tc.privateDNSZone,
							},
						},
					},
					Cluster: &clusterv1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:              clusterName,
							Namespace:         namespace,
							DeletionTimestamp: &metav1.Time{Time: time.Now()},
						},
					},
				},
				services: []azure.ServiceReconciler{
					groupsMock,
					vnetpeeringsMock,
					privateDNSMock,
				},
				skuCache: resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, ""),
			}

			err := s.delete(t.Context())
			switch {
			case tc.expectedError != "":
				g.Expect(err).To(MatchError(tc.expectedError))
			case tc.verifyError != nil:
				tc.verifyError(g, err)
			default:
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterServiceDeletePrivateDNSWithoutFrontendIPs(t *testing.T) {
	clusterName := "cluster"
	azClusterName := "azCluster"
	namespace := "ns"
	resourceGroup := "rg"
	zoneName := "example.com"
	zoneID := azure.PrivateDNSZoneID("", "dns-rg", zoneName)
	linkID := azure.VirtualNetworkLinkID("", "dns-rg", zoneName, azure.GenerateVNetLinkName("vnet"))
	notFoundErr := &azcore.ResponseError{StatusCode: http.StatusNotFound}

	cases := map[string]struct {
		expectedError string
		expect        func(grp, vpr *mock_azure.MockServiceReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder)
	}{
		"user-owned zone and link are not deleted": {
			expect: func(grp, vpr *mock_azure.MockServiceReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					tg.GetAtScope(gomockinternal.AContext(), linkID).Return(armresources.TagsResource{}, nil),
					tg.GetAtScope(gomockinternal.AContext(), zoneID).Return(armresources.TagsResource{}, nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"zone and link are already deleted": {
			expect: func(grp, vpr *mock_azure.MockServiceReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					tg.GetAtScope(gomockinternal.AContext(), linkID).Return(armresources.TagsResource{}, notFoundErr),
					tg.GetAtScope(gomockinternal.AContext(), zoneID).Return(armresources.TagsResource{}, notFoundErr),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"ownership lookup fails and resource groups are not deleted": {
			expectedError: "failed to delete private DNS zone: could not get vnet link state of example.com in resource group dns-rg: internal error",
			expect: func(_, vpr *mock_azure.MockServiceReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				gomock.InOrder(
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					tg.GetAtScope(gomockinternal.AContext(), linkID).Return(armresources.TagsResource{}, errors.New("internal error")))
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			groupsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			vnetpeeringsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)
			groupsMock.EXPECT().Name().Return(groups.ServiceName).AnyTimes()
			vnetpeeringsMock.EXPECT().Name().Return(vnetpeerings.ServiceName).AnyTimes()
			tc.expect(groupsMock.EXPECT(), vnetpeeringsMock.EXPECT(), tagsGetterMock.EXPECT())

			scheme := runtime.NewScheme()
			g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
			g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
			rg := &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceGroup,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         infrav1.GroupVersion.String(),
							Kind:               infrav1.AzureClusterKind,
							Name:               azClusterName,
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					},
					Annotations: map[string]string{
						asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
					},
				},
			}

			clusterScope := &scope.ClusterScope{
				Client:          fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(rg).Build(),
				AsyncReconciler: reconciler.Timeouts{},
				AzureCluster: &infrav1.AzureCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      azClusterName,
						Namespace: namespace,
					},
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: resourceGroup,
						NetworkSpec: infrav1.NetworkSpec{
							NetworkClassSpec: infrav1.NetworkClassSpec{
								PrivateDNSZoneName:          zoneName,
								PrivateDNSZoneResourceGroup: "dns-rg",
							},
							Vnet: infrav1.VnetSpec{
								ResourceGroup: resourceGroup,
								Name:          "vnet",
							},
							APIServerLB: &infrav1.LoadBalancerSpec{
								LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
									Type: infrav1.Internal,
								},
							},
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:              clusterName,
						Namespace:         namespace,
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
				},
			}

			s := &azureClusterService{
				scope: clusterScope,
				services: []azure.ServiceReconciler{
					groupsMock,
					vnetpeeringsMock,
					&privatedns.Service{Scope: clusterScope, TagsGetter: tagsGetterMock},
				},
				skuCache: resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, ""),
			}

			err := s.delete(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
