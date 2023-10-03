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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	mock_scope "sigs.k8s.io/cluster-api-provider-azure/azure/scope/mocks"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesetvms"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomock2 "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	FakeProviderID = "/foo/bin/bazz"
)

func TestNewMachinePoolMachineScope(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

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
				MachinePool:             new(expv1.MachinePool),
				AzureMachinePool:        new(infrav1exp.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1exp.AzureMachinePoolMachine),
			},
		},
		{
			Name: "no client",
			Input: MachinePoolMachineScopeParams{
				ClusterScope:            new(ClusterScope),
				MachinePool:             new(expv1.MachinePool),
				AzureMachinePool:        new(infrav1exp.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1exp.AzureMachinePoolMachine),
			},
			Err: "client is required when creating a MachinePoolScope",
		},
		{
			Name: "no ClusterScope",
			Input: MachinePoolMachineScopeParams{
				Client:                  fake.NewClientBuilder().WithScheme(scheme).Build(),
				MachinePool:             new(expv1.MachinePool),
				AzureMachinePool:        new(infrav1exp.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1exp.AzureMachinePoolMachine),
			},
			Err: "cluster scope is required when creating a MachinePoolScope",
		},
		{
			Name: "no MachinePool",
			Input: MachinePoolMachineScopeParams{
				Client:                  fake.NewClientBuilder().WithScheme(scheme).Build(),
				ClusterScope:            new(ClusterScope),
				AzureMachinePool:        new(infrav1exp.AzureMachinePool),
				AzureMachinePoolMachine: new(infrav1exp.AzureMachinePoolMachine),
			},
			Err: "machine pool is required when creating a MachinePoolScope",
		},
		{
			Name: "no AzureMachinePool",
			Input: MachinePoolMachineScopeParams{
				Client:                  fake.NewClientBuilder().WithScheme(scheme).Build(),
				ClusterScope:            new(ClusterScope),
				MachinePool:             new(expv1.MachinePool),
				AzureMachinePoolMachine: new(infrav1exp.AzureMachinePoolMachine),
			},
			Err: "azure machine pool is required when creating a MachinePoolScope",
		},
		{
			Name: "no AzureMachinePoolMachine",
			Input: MachinePoolMachineScopeParams{
				Client:           fake.NewClientBuilder().WithScheme(scheme).Build(),
				ClusterScope:     new(ClusterScope),
				MachinePool:      new(expv1.MachinePool),
				AzureMachinePool: new(infrav1exp.AzureMachinePool),
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

func TestMachinePoolMachineScope_ScaleSetVMSpecs(t *testing.T) {
	tests := []struct {
		name                    string
		machinePoolMachineScope MachinePoolMachineScope
		want                    azure.ResourceSpecGetter
	}{
		{
			name: "return vmss vm spec for uniform vmss",
			machinePoolMachineScope: MachinePoolMachineScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Linux",
							},
						},
						OrchestrationMode: infrav1.UniformOrchestrationMode,
					},
				},
				AzureMachinePoolMachine: &infrav1exp.AzureMachinePoolMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepoolmachine-name",
					},
					Spec: infrav1exp.AzureMachinePoolMachineSpec{
						ProviderID: "azure:///subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/machinepool-name/virtualMachines/0",
						InstanceID: "0",
					},
				},
				ClusterScoper: &ClusterScope{
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				MachinePoolScope: &MachinePoolScope{
					AzureMachinePool: &infrav1exp.AzureMachinePool{
						ObjectMeta: metav1.ObjectMeta{
							Name: "machinepool-name",
						},
					},
				},
			},
			want: &scalesetvms.ScaleSetVMSpec{
				Name:          "machinepoolmachine-name",
				InstanceID:    "0",
				ResourceGroup: "my-rg",
				ScaleSetName:  "machinepool-name",
				ProviderID:    "azure:///subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/machinepool-name/virtualMachines/0",
				IsFlex:        false,
				ResourceID:    "",
			},
		},
		{
			name: "return vmss vm spec for vmss flex",
			machinePoolMachineScope: MachinePoolMachineScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Linux",
							},
						},
						OrchestrationMode: infrav1.FlexibleOrchestrationMode,
					},
				},
				AzureMachinePoolMachine: &infrav1exp.AzureMachinePoolMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepoolmachine-name",
					},
					Spec: infrav1exp.AzureMachinePoolMachineSpec{
						ProviderID: "azure:///subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/machinepool-name/virtualMachines/0",
						InstanceID: "0",
					},
				},
				ClusterScoper: &ClusterScope{
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				MachinePoolScope: &MachinePoolScope{
					AzureMachinePool: &infrav1exp.AzureMachinePool{
						ObjectMeta: metav1.ObjectMeta{
							Name: "machinepool-name",
						},
					},
				},
			},
			want: &scalesetvms.ScaleSetVMSpec{
				Name:          "machinepoolmachine-name",
				InstanceID:    "0",
				ResourceGroup: "my-rg",
				ScaleSetName:  "machinepool-name",
				ProviderID:    "azure:///subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/machinepool-name/virtualMachines/0",
				IsFlex:        true,
				ResourceID:    "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/machinepool-name/virtualMachines/0",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.machinePoolMachineScope.ScaleSetVMSpec(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Diff between expected result and actual result: %+v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestMachineScope_UpdateNodeStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	clusterScope := mock_azure.NewMockClusterScoper(mockCtrl)
	clusterScope.EXPECT().BaseURI().AnyTimes()
	clusterScope.EXPECT().Location().AnyTimes()
	clusterScope.EXPECT().SubscriptionID().AnyTimes()
	clusterScope.EXPECT().ClusterName().Return("cluster-foo").AnyTimes()

	cases := []struct {
		Name   string
		Setup  func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1exp.AzureMachinePoolMachine)
		Verify func(g *WithT, scope *MachinePoolMachineScope)
		Err    string
	}{
		{
			Name: "should set kubernetes version, ready, and node reference upon finding the node",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1exp.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(getReadyNode(), nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status.Ready).To(Equal(true))
				g.Expect(scope.AzureMachinePoolMachine.Status.Version).To(Equal("1.2.3"))
				g.Expect(scope.AzureMachinePoolMachine.Status.NodeRef).To(Equal(&corev1.ObjectReference{
					Name: "node1",
				}))
				assertCondition(t, scope.AzureMachinePoolMachine, conditions.TrueCondition(clusterv1.MachineNodeHealthyCondition))
			},
		},
		{
			Name: "should not mark AMPM ready if node is not ready",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1exp.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(getNotReadyNode(), nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status.Ready).To(Equal(false))
				g.Expect(scope.AzureMachinePoolMachine.Status.Version).To(Equal("1.2.3"))
				g.Expect(scope.AzureMachinePoolMachine.Status.NodeRef).To(Equal(&corev1.ObjectReference{
					Name: "node1",
				}))
				assertCondition(t, scope.AzureMachinePoolMachine, conditions.FalseCondition(clusterv1.MachineNodeHealthyCondition, clusterv1.NodeConditionsFailedReason, clusterv1.ConditionSeverityWarning, ""))
			},
		},
		{
			Name: "fails fetching the node",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1exp.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, errors.New("boom"))
				return nil, ampm
			},
			Err: "failed to get node by providerID: boom",
		},
		{
			Name: "node is not found by providerID without error",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1exp.AzureMachinePoolMachine) {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				assertCondition(t, scope.AzureMachinePoolMachine, conditions.FalseCondition(clusterv1.MachineNodeHealthyCondition, clusterv1.NodeProvisioningReason, clusterv1.ConditionSeverityInfo, ""))
			},
		},
		{
			Name: "node is found by ObjectReference",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) (*azure.VMSSVM, *infrav1exp.AzureMachinePoolMachine) {
				nodeRef := corev1.ObjectReference{
					Name: "node1",
				}
				ampm.Status.NodeRef = &nodeRef
				mockNodeGetter.EXPECT().GetNodeByObjectReference(gomock2.AContext(), nodeRef).Return(getReadyNode(), nil)
				return nil, ampm
			},
			Verify: func(g *WithT, scope *MachinePoolMachineScope) {
				g.Expect(scope.AzureMachinePoolMachine.Status.Ready).To(Equal(true))
				g.Expect(scope.AzureMachinePoolMachine.Status.Version).To(Equal("1.2.3"))
				g.Expect(scope.AzureMachinePoolMachine.Status.NodeRef).To(Equal(&corev1.ObjectReference{
					Name: "node1",
				}))
				assertCondition(t, scope.AzureMachinePoolMachine, conditions.TrueCondition(clusterv1.MachineNodeHealthyCondition))
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
					MachinePool: &expv1.MachinePool{
						Spec: expv1.MachinePoolSpec{
							Template: clusterv1.MachineTemplateSpec{
								Spec: clusterv1.MachineSpec{
									Version: ptr.To("v1.19.11"),
								},
							},
						},
					},
					AzureMachinePool: new(infrav1exp.AzureMachinePool),
				}
			)

			defer controller.Finish()

			instance, ampm := c.Setup(mockClient, &infrav1exp.AzureMachinePoolMachine{
				Spec: infrav1exp.AzureMachinePoolMachineSpec{
					ProviderID: FakeProviderID,
				},
			})
			params.AzureMachinePoolMachine = ampm
			s, err := NewMachinePoolMachineScope(params)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s).NotTo(BeNil())
			s.instance = instance
			s.workloadNodeGetter = mockClient

			err = s.UpdateNodeStatus(context.TODO())
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
	_ = expv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

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
		Setup func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) *infrav1exp.AzureMachinePoolMachine
		Err   string
	}{
		{
			Name: "should skip cordon and drain if the node does not exist with provider ID",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) *infrav1exp.AzureMachinePoolMachine {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, nil)
				return ampm
			},
		},
		{
			Name: "should skip cordon and drain if the node does not exist with node reference",
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) *infrav1exp.AzureMachinePoolMachine {
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
			Setup: func(mockNodeGetter *mock_scope.MocknodeGetter, ampm *infrav1exp.AzureMachinePoolMachine) *infrav1exp.AzureMachinePoolMachine {
				mockNodeGetter.EXPECT().GetNodeByProviderID(gomock2.AContext(), FakeProviderID).Return(nil, errors.New("boom"))
				return ampm
			},
			Err: "failed to get node: failed to get node by providerID: boom",
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
					MachinePool: &expv1.MachinePool{
						Spec: expv1.MachinePoolSpec{
							Template: clusterv1.MachineTemplateSpec{
								Spec: clusterv1.MachineSpec{
									Version: ptr.To("v1.19.11"),
								},
							},
						},
					},
					AzureMachinePool: new(infrav1exp.AzureMachinePool),
				}
			)

			defer controller.Finish()

			ampm := c.Setup(mockClient, &infrav1exp.AzureMachinePoolMachine{
				Spec: infrav1exp.AzureMachinePoolMachineSpec{
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

// asserts whether a condition of type is set on the Getter object
// when the condition is true, asserting the reason/severity/message
// for the condition are avoided.
func assertCondition(t *testing.T, from conditions.Getter, condition *clusterv1.Condition) {
	t.Helper()

	g := NewWithT(t)
	g.Expect(conditions.Has(from, condition.Type)).To(BeTrue())

	if condition.Status == corev1.ConditionTrue {
		conditions.IsTrue(from, condition.Type)
	} else {
		conditionToBeAsserted := conditions.Get(from, condition.Type)
		g.Expect(conditionToBeAsserted.Status).To(Equal(condition.Status))
		g.Expect(conditionToBeAsserted.Severity).To(Equal(condition.Severity))
		g.Expect(conditionToBeAsserted.Reason).To(Equal(condition.Reason))
		if condition.Message != "" {
			g.Expect(conditionToBeAsserted.Message).To(Equal(condition.Message))
		}
	}
}
