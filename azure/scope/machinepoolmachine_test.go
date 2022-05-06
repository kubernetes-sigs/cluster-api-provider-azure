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

package scope

import (
	"context"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	mock_scope "sigs.k8s.io/cluster-api-provider-azure/azure/scope/mocks"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomock2 "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	FakeProviderID = "/foo/bin/bazz"
)

func TestNewMachinePoolMachineScope(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name  string
		Input MachinePoolMachineScopeParams
		Err   string
	}{
		{
			Name: "successfully create machine scope",
			Input: MachinePoolMachineScopeParams{
				Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
				ClusterScope: &ClusterScope{
					Cluster: &clusterv1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "clusterName",
						},
					},
				},
				MachinePool:             new(capiv1exp.MachinePool),
				AzureMachinePool:        new(infrav1.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1.AzureMachinePoolMachine),
			},
		},
		{
			Name: "no client",
			Input: MachinePoolMachineScopeParams{
				ClusterScope:            new(ClusterScope),
				MachinePool:             new(capiv1exp.MachinePool),
				AzureMachinePool:        new(infrav1.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1.AzureMachinePoolMachine),
			},
			Err: "client is required when creating a MachinePoolScope",
		},
		{
			Name: "no ClusterScope",
			Input: MachinePoolMachineScopeParams{
				Client:                  fake.NewClientBuilder().WithScheme(scheme).Build(),
				MachinePool:             new(capiv1exp.MachinePool),
				AzureMachinePool:        new(infrav1.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1.AzureMachinePoolMachine),
			},
			Err: "cluster scope is required when creating a MachinePoolScope",
		},
		{
			Name: "no MachinePool",
			Input: MachinePoolMachineScopeParams{
				Client:                  fake.NewClientBuilder().WithScheme(scheme).Build(),
				ClusterScope:            new(ClusterScope),
				AzureMachinePool:        new(infrav1.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1.AzureMachinePoolMachine),
			},
			Err: "machine pool is required when creating a MachinePoolScope",
		},
		{
			Name: "no AzureMachinePool",
			Input: MachinePoolMachineScopeParams{
				Client:                  fake.NewClientBuilder().WithScheme(scheme).Build(),
				ClusterScope:            new(ClusterScope),
				MachinePool:             new(capiv1exp.MachinePool),
				AzureMachinePoolMachine: new(infrav1.AzureMachinePoolMachine),
			},
			Err: "azure machine pool is required when creating a MachinePoolScope",
		},
		{
			Name: "no AzureMachinePoolMachine",
			Input: MachinePoolMachineScopeParams{
				Client:           fake.NewClientBuilder().WithScheme(scheme).Build(),
				ClusterScope:     new(ClusterScope),
				MachinePool:      new(capiv1exp.MachinePool),
				AzureMachinePool: new(infrav1.AzureMachinePool),
			},
			Err: "azure machine pool machine is required when creating a MachinePoolScope",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			s, err := NewMachinePoolMachineScope(c.Input)
			if c.Err != "" {
				g.Expect(err).To(MatchError(c.Err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(s).NotTo(BeNil())
			}
		})
	}
}

func TestMachineScope_UpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	clusterScope := mock_azure.NewMockClusterScoper(mockCtrl)
	clusterScope.EXPECT().Authorizer().AnyTimes()
	clusterScope.EXPECT().BaseURI().AnyTimes()
	clusterScope.EXPECT().Location().AnyTimes()
	clusterScope.EXPECT().SubscriptionID().AnyTimes()
	clusterScope.EXPECT().ClusterName().Return("cluster-foo").AnyTimes()

	cases := []struct {
		Name   string
		Setup  func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine)
		Verify func(g *WithT, scope *MachinePoolMachineScope)
		Err    string
	}{
		{
			Name: "should set kubernetes version, ready, and node reference upon finding the node",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(getReadyNode(), nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status).To(Equal(infrav1.AzureMachinePoolMachineStatus{
					Ready:   true,
					Version: "1.2.3",
					NodeRef: &corev1.ObjectReference{
						Name: "node1",
					},
				}))
			},
		},
		{
			Name: "should not mark AMPM ready if node is not ready",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(getNotReadyNode(), nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status).To(Equal(infrav1.AzureMachinePoolMachineStatus{
					Ready:   false,
					Version: "1.2.3",
					NodeRef: &corev1.ObjectReference{
						Name: "node1",
					},
				}))
			},
		},
		{
			Name: "fails fetching the node",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, errors.New("boom"))
				return nil, ampm
			},
			Err: "failed to get node by providerID or object reference: boom",
		},
		{
			Name: "should not mark AMPM ready if node is not ready",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(getNotReadyNode(), nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status).To(Equal(infrav1.AzureMachinePoolMachineStatus{
					Ready:   false,
					Version: "1.2.3",
					NodeRef: &corev1.ObjectReference{
						Name: "node1",
					},
				}))
			},
		},
		{
			Name: "node is not found",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status).To(Equal(infrav1.AzureMachinePoolMachineStatus{}))
			},
		},
		{
			Name: "node is found by ObjectReference",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine) {
				nodeRef := corev1.ObjectReference{
					Name: "node1",
				}
				ampm.Status.NodeRef = &nodeRef
				mockNodeGetter.EXPECT().GetNodeByObjectReference(gomock2.AContext(), nodeRef).Return(getReadyNode(), nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status).To(Equal(infrav1.AzureMachinePoolMachineStatus{
					NodeRef: &corev1.ObjectReference{
						Name: "node1",
					},
					Version: "1.2.3",
					Ready:   true,
				}))
			},
		},
		{
			Name: "instance information with latest model populates the AMPM status",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, nil)
				return &azure.VMSSVM{
					State: v1beta1.Succeeded,
					Image: v1beta1.Image{
						Marketplace: &v1beta1.AzureMarketplaceImage{
							ImagePlan: v1beta1.ImagePlan{
								Publisher: "cncf-upstream",
								Offer:     "capi",
								SKU:       "k8s-1dot19dot11-ubuntu-1804",
							},
							Version: "latest",
						},
					},
				}, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				succeeded := v1beta1.Succeeded
				g.Expect(scope.AzureMachinePoolMachine.Status).To(Equal(infrav1.AzureMachinePoolMachineStatus{
					ProvisioningState:  &succeeded,
					LatestModelApplied: true,
				}))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				controller = gomock.NewController(t)
				mockClient = mock_scope.NewMocknodeGetter(controller)
				g          = NewWithT(t)
				params     = MachinePoolMachineScopeParams{
					Client:       fake.NewClientBuilder().WithScheme(scheme).Build(),
					ClusterScope: clusterScope,
					MachinePool: &capiv1exp.MachinePool{
						Spec: capiv1exp.MachinePoolSpec{
							Template: clusterv1.MachineTemplateSpec{
								Spec: clusterv1.MachineSpec{
									Version: to.StringPtr("v1.19.11"),
								},
							},
						},
					},
					AzureMachinePool: new(infrav1.AzureMachinePool),
				}
			)

			defer controller.Finish()

			instance, ampm := c.Setup(mockClient, &infrav1.AzureMachinePoolMachine{
				Spec: infrav1.AzureMachinePoolMachineSpec{
					ProviderID: FakeProviderID,
				},
			})
			params.AzureMachinePoolMachine = ampm
			s, err := NewMachinePoolMachineScope(params)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s).NotTo(BeNil())
			s.instance = instance
			s.workloadNodeGetter = mockClient

			err = s.UpdateStatus(context.TODO())
			if c.Err == "" {
				g.Expect(err).To(Succeed())
			} else {
				g.Expect(err).To(MatchError(c.Err))
			}

			if c.Verify != nil {
				c.Verify(g, s)
			}
		})
	}
}

func TestMachinePoolMachineScope_CordonAndDrain(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	var (
		clusterScope = ClusterScope{
			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-foo",
				},
			},
		}
	)

	cases := []struct {
		Name  string
		Setup func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) *infrav1.AzureMachinePoolMachine
		Err   string
	}{
		{
			Name: "should skip cordon and drain if the node does not exist with provider ID",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) *infrav1.AzureMachinePoolMachine {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, nil)
				return ampm
			},
		},
		{
			Name: "should skip cordon and drain if the node does not exist with node reference",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) *infrav1.AzureMachinePoolMachine {
				nodeRef := corev1.ObjectReference{
					Name: "node1",
				}
				ampm.Status.NodeRef = &nodeRef
				mockNodeGetter.EXPECT().GetNodeByObjectReference(gomock2.AContext(), nodeRef).Return(nil, nil)
				return ampm
			},
		},
		{
			Name: "if GetNodeByProviderID fails with an error, an error will be returned",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1.AzureMachinePoolMachine) *infrav1.AzureMachinePoolMachine {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, errors.New("boom"))
				return ampm
			},
			Err: "failed to find node: boom",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				controller = gomock.NewController(t)
				mockClient = mock_scope.NewMocknodeGetter(controller)
				g          = NewWithT(t)
				params     = MachinePoolMachineScopeParams{
					Client:       fake.NewClientBuilder().WithScheme(scheme).Build(),
					ClusterScope: &clusterScope,
					MachinePool: &capiv1exp.MachinePool{
						Spec: capiv1exp.MachinePoolSpec{
							Template: clusterv1.MachineTemplateSpec{
								Spec: clusterv1.MachineSpec{
									Version: to.StringPtr("v1.19.11"),
								},
							},
						},
					},
					AzureMachinePool: new(infrav1.AzureMachinePool),
				}
			)

			defer controller.Finish()

			ampm := c.Setup(mockClient, &infrav1.AzureMachinePoolMachine{
				Spec: infrav1.AzureMachinePoolMachineSpec{
					ProviderID: FakeProviderID,
				},
			})
			params.AzureMachinePoolMachine = ampm
			s, err := NewMachinePoolMachineScope(params)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s).NotTo(BeNil())
			s.workloadNodeGetter = mockClient

			err = s.CordonAndDrain(context.TODO())
			if c.Err == "" {
				g.Expect(err).To(Succeed())
			} else {
				g.Expect(err).To(MatchError(c.Err))
			}
		})
	}
}

func getReadyNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion: "1.2.3",
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func getNotReadyNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion: "1.2.3",
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}
}
