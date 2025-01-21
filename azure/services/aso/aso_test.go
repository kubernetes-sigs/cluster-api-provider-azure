/*
Copyright 2023 The Kubernetes Authors.

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

package aso

import (
	"context"
	"errors"
	"testing"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso/mock_aso"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

const clusterName = "cluster"

type ErroringGetClient struct {
	client.Client
	err error
}

func (e ErroringGetClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return e.err
}

type ErroringPatchClient struct {
	client.Client
	err error
}

func (e ErroringPatchClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return e.err
}

type ErroringDeleteClient struct {
	client.Client
	err error
}

func (e ErroringDeleteClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return e.err
}

func newOwner() *asoresourcesv1.ResourceGroup {
	return &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
		},
	}
}

func ownerRefs() []metav1.OwnerReference {
	s := runtime.NewScheme()
	if err := asoresourcesv1.AddToScheme(s); err != nil {
		panic(err)
	}
	gvk, err := apiutil.GVKForObject(&asoresourcesv1.ResourceGroup{}, s)
	if err != nil {
		panic(err)
	}
	return []metav1.OwnerReference{
		{
			APIVersion:         gvk.GroupVersion().String(),
			Kind:               gvk.Kind,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
	}
}

// TestCreateOrUpdateResource tests the CreateOrUpdateResource function.
func TestCreateOrUpdateResource(t *testing.T) {
	t.Run("ready status unknown", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("ready status unknown"))
	})

	t.Run("create resource that doesn't already exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Nil()).Return(&asoresourcesv1.ResourceGroup{
			Spec: asoresourcesv1.ResourceGroup_Spec{
				Location: ptr.To("location"),
			},
		}, nil)

		ctx := context.Background()
		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
		var recerr azure.ReconcileError
		g.Expect(errors.As(err, &recerr)).To(BeTrue())
		g.Expect(recerr.IsTransient()).To(BeTrue())

		created := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, created)).To(Succeed())
		g.Expect(created.Name).To(Equal("name"))
		g.Expect(created.Namespace).To(Equal("namespace"))
		g.Expect(created.OwnerReferences).To(Equal(ownerRefs()))
		g.Expect(created.Annotations).To(Equal(map[string]string{
			asoannotations.ReconcilePolicy:   string(asoannotations.ReconcilePolicySkip),
			asoannotations.PerResourceSecret: "cluster-aso-secret",
		}))
		g.Expect(created.Spec).To(Equal(asoresourcesv1.ResourceGroup_Spec{
			Location: ptr.To("location"),
		}))
	})

	t.Run("resource is not ready in non-terminal state", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.PerResourceSecret: "cluster-aso-secret",
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:     conditions.ConditionTypeReady,
						Status:   metav1.ConditionFalse,
						Severity: conditions.ConditionSeverityInfo,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("resource is not Ready"))
		var recerr azure.ReconcileError
		g.Expect(errors.As(err, &recerr)).To(BeTrue())
		g.Expect(recerr.IsTransient()).To(BeTrue())
		g.Expect(recerr.IsTerminal()).To(BeFalse())
	})

	t.Run("resource is not ready in reconciling state", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.PerResourceSecret: "cluster-aso-secret",
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:     conditions.ConditionTypeReady,
						Status:   metav1.ConditionFalse,
						Severity: conditions.ConditionSeverityInfo,
						Reason:   conditions.ReasonReconciling.Name,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
	})

	t.Run("resource is not ready in terminal state", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.PerResourceSecret: "cluster-aso-secret",
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:     conditions.ConditionTypeReady,
						Status:   metav1.ConditionFalse,
						Severity: conditions.ConditionSeverityError,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("resource is not Ready"))
		var recerr azure.ReconcileError
		g.Expect(errors.As(err, &recerr)).To(BeTrue())
		g.Expect(recerr.IsTerminal()).To(BeTrue())
		g.Expect(recerr.IsTransient()).To(BeFalse())
	})

	t.Run("error getting existing resource", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](ErroringGetClient{Client: c, err: errors.New("an error")}, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})

		ctx := context.Background()
		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get existing resource"))
	})

	t.Run("begin an update", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			group.Spec.Location = ptr.To("location")
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("adopt managed resource in not found state", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		clusterName := "cluster"
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicySkip),
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonAzureResourceNotFound.Name,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Annotations).To(Equal(map[string]string{
			asoannotations.ReconcilePolicy:   string(asoannotations.ReconcilePolicyManage),
			asoannotations.PerResourceSecret: "cluster-aso-secret",
		}))
	})

	t.Run("adopt previously managed resource", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		clusterName := "cluster"
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(true)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicySkip),
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Annotations).To(Equal(map[string]string{
			asoannotations.ReconcilePolicy:   string(asoannotations.ReconcilePolicyManage),
			asoannotations.PerResourceSecret: "cluster-aso-secret",
		}))
	})

	t.Run("adopt previously managed resource with label", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		clusterName := "cluster"
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(true)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					clusterv1.ClusterNameLabel:     clusterName,
					infrav1.OwnedByClusterLabelKey: clusterName, //nolint:staticcheck // Referencing this deprecated value is required for backwards compatibility.
				},
				Annotations: map[string]string{
					asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicySkip),
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Annotations).To(Equal(map[string]string{
			asoannotations.ReconcilePolicy:   string(asoannotations.ReconcilePolicyManage),
			asoannotations.PerResourceSecret: "cluster-aso-secret",
		}))
		g.Expect(updated.OwnerReferences).To(Equal(ownerRefs()))
	})

	t.Run("Parameters error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).Return(nil, errors.New("parameters error"))

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("parameters error"))
	})

	t.Run("skip update for unmanaged resource", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).NotTo(BeNil())
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("resource up to date", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.ReconcilePolicy:   string(asoannotations.ReconcilePolicyManage),
					asoannotations.PerResourceSecret: "cluster-aso-secret",
				},
			},
			Spec: asoresourcesv1.ResourceGroup_Spec{
				Location: ptr.To("location"),
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).NotTo(BeNil())
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(result.GetName()).To(Equal("name"))
		g.Expect(result.GetNamespace()).To(Equal("namespace"))
		g.Expect(result.Spec.Location).To(Equal(ptr.To("location")))
	})

	t.Run("error updating", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](ErroringPatchClient{Client: c, err: errors.New("an error")}, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			group.Spec.Location = ptr.To("location")
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to update resource"))
	})

	t.Run("with tags success", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := struct {
			*mock_azure.MockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]
			*mock_aso.MockTagsGetterSetter[*asoresourcesv1.ResourceGroup]
		}{
			MockASOResourceSpecGetter: mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			MockTagsGetterSetter:      mock_aso.NewMockTagsGetterSetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}
		specMock.MockASOResourceSpecGetter.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.MockASOResourceSpecGetter.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.MockASOResourceSpecGetter.EXPECT().WasManaged(gomock.Any()).Return(false)

		specMock.MockTagsGetterSetter.EXPECT().GetAdditionalTags().Return(nil)
		specMock.MockTagsGetterSetter.EXPECT().GetDesiredTags(gomock.Any()).Return(nil).Times(2)
		specMock.MockTagsGetterSetter.EXPECT().SetTags(gomock.Any(), gomock.Any())

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Annotations).To(HaveKey(tagsLastAppliedAnnotation))
	})

	t.Run("with tags failure", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := struct {
			*mock_azure.MockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]
			*mock_aso.MockTagsGetterSetter[*asoresourcesv1.ResourceGroup]
		}{
			MockASOResourceSpecGetter: mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			MockTagsGetterSetter:      mock_aso.NewMockTagsGetterSetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}
		specMock.MockASOResourceSpecGetter.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.MockASOResourceSpecGetter.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
					tagsLastAppliedAnnotation:      "{",
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to reconcile tags"))
	})

	t.Run("reconcile policy annotation resets after un-pause", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					prePauseReconcilePolicyAnnotation: string(asoannotations.ReconcilePolicyManage),
					asoannotations.ReconcilePolicy:    string(asoannotations.ReconcilePolicySkip),
				},
			},
			Spec: asoresourcesv1.ResourceGroup_Spec{
				Location: ptr.To("location"),
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Annotations).NotTo(HaveKey(prePauseReconcilePolicyAnnotation))
		g.Expect(updated.Annotations).To(HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicyManage)))
	})

	t.Run("patches applied on create", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := struct {
			*mock_azure.MockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]
			*mock_aso.MockPatcher
		}{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_aso.NewMockPatcher(mockCtrl),
		}
		specMock.MockASOResourceSpecGetter.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.MockASOResourceSpecGetter.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return &asoresourcesv1.ResourceGroup{
				Spec: asoresourcesv1.ResourceGroup_Spec{
					Location: ptr.To("location-from-parameters"),
				},
			}, nil
		})

		specMock.MockPatcher.EXPECT().ExtraPatches().Return([]string{
			`{"metadata": {"labels": {"extra-patch": "not-this-value"}}}`,
			`{"metadata": {"labels": {"extra-patch": "this-value"}}}`,
			`{"metadata": {"labels": {"another": "label"}}}`,
		})

		ctx := context.Background()

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue(), "expected not done error, got %v", err)

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Labels).To(HaveKeyWithValue("extra-patch", "this-value"))
		g.Expect(updated.Labels).To(HaveKeyWithValue("another", "label"))
		g.Expect(*updated.Spec.Location).To(Equal("location-from-parameters"))
	})

	t.Run("patches applied on update", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		mockCtrl := gomock.NewController(t)
		specMock := struct {
			*mock_azure.MockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]
			*mock_aso.MockPatcher
		}{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_aso.NewMockPatcher(mockCtrl),
		}
		specMock.MockASOResourceSpecGetter.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		})
		specMock.MockASOResourceSpecGetter.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			group.Spec.Location = ptr.To("location-from-parameters")
			return group, nil
		})
		specMock.MockASOResourceSpecGetter.EXPECT().WasManaged(gomock.Any()).Return(false)

		specMock.MockPatcher.EXPECT().ExtraPatches().Return([]string{
			`{"metadata": {"labels": {"extra-patch": "not-this-value"}}}`,
			`{"metadata": {"labels": {"extra-patch": "this-value"}}}`,
			`{"metadata": {"labels": {"another": "label"}}}`,
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				Annotations: map[string]string{
					asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
				},
			},
			Spec: asoresourcesv1.ResourceGroup_Spec{
				Location: ptr.To("location"),
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{
				Conditions: []conditions.Condition{
					{
						Type:   conditions.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue(), "expected not done error, got %v", err)

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Labels).To(HaveKeyWithValue("extra-patch", "this-value"))
		g.Expect(updated.Labels).To(HaveKeyWithValue("another", "label"))
		g.Expect(*updated.Spec.Location).To(Equal("location-from-parameters"))
	})
}

// TestDeleteResource tests the DeleteResource function.
func TestDeleteResource(t *testing.T) {
	t.Run("successful delete", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		resource := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "name",
			},
		}

		ctx := context.Background()
		g.Expect(s.DeleteResource(ctx, resource, "service")).To(Succeed())
	})

	t.Run("delete in progress", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		ctx := context.Background()
		resource := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
			},
		}
		g.Expect(c.Create(ctx, resource)).To(Succeed())

		err := s.DeleteResource(ctx, resource, "service")
		g.Expect(err).To(HaveOccurred())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
		var recerr azure.ReconcileError
		g.Expect(errors.As(err, &recerr)).To(BeTrue())
		g.Expect(recerr.IsTransient()).To(BeTrue())
	})

	t.Run("skip delete for unmanaged resource", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName, newOwner())

		resource := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		}

		ctx := context.Background()
		g.Expect(c.Create(ctx, resource)).To(Succeed())

		g.Expect(s.DeleteResource(ctx, resource, "service")).To(Succeed())
	})

	t.Run("error checking if resource is managed", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](ErroringGetClient{Client: c, err: errors.New("a get error")}, clusterName, newOwner())

		resource := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		}

		ctx := context.Background()
		g.Expect(c.Create(ctx, resource)).To(Succeed())

		err := s.DeleteResource(ctx, resource, "service")
		g.Expect(err).To(MatchError(ContainSubstring("a get error")))
	})

	t.Run("error deleting", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](ErroringDeleteClient{Client: c, err: errors.New("an error")}, clusterName, newOwner())

		resource := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "name",
				Namespace:       "namespace",
				OwnerReferences: ownerRefs(),
			},
		}

		ctx := context.Background()
		g.Expect(c.Create(ctx, resource)).To(Succeed())

		err := s.DeleteResource(ctx, resource, "service")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to delete resource"))
	})
}

func TestPauseResource(t *testing.T) {
	tests := []struct {
		name          string
		resource      *asoresourcesv1.ResourceGroup
		clientBuilder func(g Gomega) client.Client
		expectedErr   string
		verify        func(g Gomega, ctrlClient client.Client, resource *asoresourcesv1.ResourceGroup)
	}{
		{
			name: "success, not already paused",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
			},
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				return fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&asoresourcesv1.ResourceGroup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name",
							Namespace: "namespace",
							Annotations: map[string]string{
								asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
							},
							OwnerReferences: ownerRefs(),
						},
					}).
					Build()
			},
			verify: func(g Gomega, ctrlClient client.Client, resource *asoresourcesv1.ResourceGroup) {
				ctx := context.Background()
				actual := &asoresourcesv1.ResourceGroup{}
				g.Expect(ctrlClient.Get(ctx, client.ObjectKeyFromObject(resource), actual)).To(Succeed())
				g.Expect(actual.Annotations).To(HaveKeyWithValue(prePauseReconcilePolicyAnnotation, string(asoannotations.ReconcilePolicyManage)))
				g.Expect(actual.Annotations).To(HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicySkip)))
			},
		},
		{
			name: "success, already paused",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
			},
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				return fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&asoresourcesv1.ResourceGroup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name",
							Namespace: "namespace",
							Annotations: map[string]string{
								asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicySkip),
							},
							OwnerReferences: ownerRefs(),
						},
					}).
					Build()
			},
			verify: func(g Gomega, ctrlClient client.Client, resource *asoresourcesv1.ResourceGroup) {
				ctx := context.Background()
				actual := &asoresourcesv1.ResourceGroup{}
				g.Expect(ctrlClient.Get(ctx, client.ObjectKeyFromObject(resource), actual)).To(Succeed())
				g.Expect(actual.Annotations).To(HaveKeyWithValue(prePauseReconcilePolicyAnnotation, string(asoannotations.ReconcilePolicySkip)))
				g.Expect(actual.Annotations).To(HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicySkip)))
			},
		},
		{
			name: "success, no patch needed",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
			},
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				c := fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&asoresourcesv1.ResourceGroup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name",
							Namespace: "namespace",
							Annotations: map[string]string{
								asoannotations.ReconcilePolicy:    string(asoannotations.ReconcilePolicySkip),
								prePauseReconcilePolicyAnnotation: string(asoannotations.ReconcilePolicyManage),
							},
							OwnerReferences: ownerRefs(),
						},
					}).
					Build()
				return ErroringPatchClient{Client: c, err: errors.New("patch shouldn't be called")}
			},
			expectedErr: "",
		},
		{
			name: "failure getting existing resource",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
			},
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				return fakeclient.NewClientBuilder().
					WithScheme(scheme).
					Build()
			},
			expectedErr: "not found",
		},
		{
			name: "failure patching resource",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
			},
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				c := fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&asoresourcesv1.ResourceGroup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name",
							Namespace: "namespace",
							Annotations: map[string]string{
								asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicySkip),
							},
							OwnerReferences: ownerRefs(),
						},
					}).
					Build()
				return ErroringPatchClient{Client: c, err: errors.New("test patch error")}
			},
			expectedErr: "test patch error",
		},
		{
			name: "success, unmanaged resource",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
			},
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				return fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&asoresourcesv1.ResourceGroup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name",
							Namespace: "namespace",
							Annotations: map[string]string{
								asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
							},
							OwnerReferences: []metav1.OwnerReference{{Name: "other-owner"}},
						},
					}).
					Build()
			},
			verify: func(g Gomega, ctrlClient client.Client, resource *asoresourcesv1.ResourceGroup) {
				ctx := context.Background()
				actual := &asoresourcesv1.ResourceGroup{}
				g.Expect(ctrlClient.Get(ctx, client.ObjectKeyFromObject(resource), actual)).To(Succeed())
				g.Expect(actual.Annotations).NotTo(HaveKey(prePauseReconcilePolicyAnnotation))
				g.Expect(actual.Annotations).To(HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicyManage)))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			ctx := context.Background()
			svcName := "service"

			ctrlClient := test.clientBuilder(g)

			s := New[*asoresourcesv1.ResourceGroup](ctrlClient, clusterName, newOwner())

			err := s.PauseResource(ctx, test.resource, svcName)
			if test.expectedErr != "" {
				g.Expect(err.Error()).To(ContainSubstring(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if test.verify != nil {
				test.verify(g, ctrlClient, test.resource)
			}
		})
	}
}
