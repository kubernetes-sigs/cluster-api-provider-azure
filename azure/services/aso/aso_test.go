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
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso/mock_aso"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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

// TestCreateOrUpdateResource tests the CreateOrUpdateResource function.
func TestCreateOrUpdateResource(t *testing.T) {
	t.Run("ready status unknown", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
				},
			},
			Status: asoresourcesv1.ResourceGroup_STATUS{},
		})).To(Succeed())

		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("ready status unknown"))
	})

	t.Run("create resource that doesn't already exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
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
		g.Expect(err).NotTo(BeNil())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
		var recerr azure.ReconcileError
		g.Expect(errors.As(err, &recerr)).To(BeTrue())
		g.Expect(recerr.IsTransient()).To(BeTrue())

		created := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, created)).To(Succeed())
		g.Expect(created.Name).To(Equal("name"))
		g.Expect(created.Namespace).To(Equal("namespace"))
		g.Expect(created.Labels).To(Equal(map[string]string{
			infrav1.OwnedByClusterLabelKey: clusterName,
		}))
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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).NotTo(BeNil())
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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).NotTo(BeNil())
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
		s := New[*asoresourcesv1.ResourceGroup](ErroringGetClient{Client: c, err: errors.New("an error")}, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})

		ctx := context.Background()
		result, err := s.CreateOrUpdateResource(ctx, specMock, "service")
		g.Expect(result).To(BeNil())
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to get existing resource"))
	})

	t.Run("begin an update", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
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
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).NotTo(BeNil())
	})

	t.Run("adopt managed resource in not found state", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		clusterName := "cluster"
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).NotTo(BeNil())

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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
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
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).NotTo(BeNil())

		updated := &asoresourcesv1.ResourceGroup{}
		g.Expect(c.Get(ctx, types.NamespacedName{Name: "name", Namespace: "namespace"}, updated)).To(Succeed())
		g.Expect(updated.Annotations).To(Equal(map[string]string{
			asoannotations.ReconcilePolicy:   string(asoannotations.ReconcilePolicyManage),
			asoannotations.PerResourceSecret: "cluster-aso-secret",
		}))
	})

	t.Run("Parameters error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Not(gomock.Nil())).Return(nil, errors.New("parameters error"))

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("parameters error"))
	})

	t.Run("skip update for unmanaged resource", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
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
		g.Expect(err).To(BeNil())
	})

	t.Run("resource up to date", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).To(BeNil())

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
		s := New[*asoresourcesv1.ResourceGroup](ErroringPatchClient{Client: c, err: errors.New("an error")}, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
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
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to update resource"))
	})

	t.Run("with tags success", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

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
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.MockASOResourceSpecGetter.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.MockASOResourceSpecGetter.EXPECT().WasManaged(gomock.Any()).Return(false)

		specMock.MockTagsGetterSetter.EXPECT().GetActualTags(gomock.Any()).Return(nil)
		specMock.MockTagsGetterSetter.EXPECT().GetAdditionalTags().Return(nil)
		specMock.MockTagsGetterSetter.EXPECT().GetDesiredTags(gomock.Any()).Return(nil).Times(2)
		specMock.MockTagsGetterSetter.EXPECT().SetTags(gomock.Any(), gomock.Any())

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

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
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.MockASOResourceSpecGetter.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specMock.EXPECT().Parameters(gomockinternal.AContext(), gomock.Any()).DoAndReturn(func(_ context.Context, group *asoresourcesv1.ResourceGroup) (*asoresourcesv1.ResourceGroup, error) {
			return group, nil
		})
		specMock.EXPECT().WasManaged(gomock.Any()).Return(false)

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		}).AnyTimes()

		ctx := context.Background()
		err := s.DeleteResource(ctx, specMock, "service")
		g.Expect(err).To(BeNil())
	})

	t.Run("delete in progress", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		}).AnyTimes()

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
				},
			},
		})).To(Succeed())

		err := s.DeleteResource(ctx, specMock, "service")
		g.Expect(err).NotTo(BeNil())
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
		s := New[*asoresourcesv1.ResourceGroup](c, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		}).AnyTimes()

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})).To(Succeed())

		err := s.DeleteResource(ctx, specMock, "service")
		g.Expect(err).To(BeNil())
	})

	t.Run("error checking if resource is managed", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](ErroringGetClient{Client: c, err: errors.New("a get error")}, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		}).AnyTimes()

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})).To(Succeed())

		err := s.DeleteResource(ctx, specMock, "service")
		g.Expect(err).To(MatchError(ContainSubstring("a get error")))
	})

	t.Run("error deleting", func(t *testing.T) {
		g := NewGomegaWithT(t)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(sch).
			Build()
		s := New[*asoresourcesv1.ResourceGroup](ErroringDeleteClient{Client: c, err: errors.New("an error")}, clusterName)

		mockCtrl := gomock.NewController(t)
		specMock := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		specMock.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		}).AnyTimes()

		ctx := context.Background()
		g.Expect(c.Create(ctx, &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
				Labels: map[string]string{
					infrav1.OwnedByClusterLabelKey: clusterName,
				},
			},
		})).To(Succeed())

		err := s.DeleteResource(ctx, specMock, "service")
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to delete resource"))
	})
}

func TestPauseResource(t *testing.T) {
	tests := []struct {
		name          string
		expect        func(*mock_azure.MockASOResourceSpecGetterMockRecorder[*asoresourcesv1.ResourceGroup])
		clientBuilder func(g Gomega) client.Client
		expectedErr   string
		verify        func(g Gomega, ctrlClient client.Client, spec azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup])
	}{
		{
			name: "success, not already paused",
			expect: func(spec *mock_azure.MockASOResourceSpecGetterMockRecorder[*asoresourcesv1.ResourceGroup]) {
				spec.ResourceRef().Return(&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
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
							Labels: map[string]string{
								infrav1.OwnedByClusterLabelKey: clusterName,
							},
						},
					}).
					Build()
			},
			verify: func(g Gomega, ctrlClient client.Client, spec azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]) {
				ctx := context.Background()
				actual := &asoresourcesv1.ResourceGroup{}
				g.Expect(ctrlClient.Get(ctx, client.ObjectKeyFromObject(spec.ResourceRef()), actual)).To(Succeed())
				g.Expect(actual.Annotations).To(HaveKeyWithValue(prePauseReconcilePolicyAnnotation, string(asoannotations.ReconcilePolicyManage)))
				g.Expect(actual.Annotations).To(HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicySkip)))
			},
		},
		{
			name: "success, already paused",
			expect: func(spec *mock_azure.MockASOResourceSpecGetterMockRecorder[*asoresourcesv1.ResourceGroup]) {
				spec.ResourceRef().Return(&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
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
							Labels: map[string]string{
								infrav1.OwnedByClusterLabelKey: clusterName,
							},
						},
					}).
					Build()
			},
			verify: func(g Gomega, ctrlClient client.Client, spec azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]) {
				ctx := context.Background()
				actual := &asoresourcesv1.ResourceGroup{}
				g.Expect(ctrlClient.Get(ctx, client.ObjectKeyFromObject(spec.ResourceRef()), actual)).To(Succeed())
				g.Expect(actual.Annotations).To(HaveKeyWithValue(prePauseReconcilePolicyAnnotation, string(asoannotations.ReconcilePolicySkip)))
				g.Expect(actual.Annotations).To(HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicySkip)))
			},
		},
		{
			name: "failure getting existing resource",
			expect: func(spec *mock_azure.MockASOResourceSpecGetterMockRecorder[*asoresourcesv1.ResourceGroup]) {
				spec.ResourceRef().Return(&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
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
			expect: func(spec *mock_azure.MockASOResourceSpecGetterMockRecorder[*asoresourcesv1.ResourceGroup]) {
				spec.ResourceRef().Return(&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
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
							Labels: map[string]string{
								infrav1.OwnedByClusterLabelKey: clusterName,
							},
						},
					}).
					Build()
				return ErroringPatchClient{Client: c, err: errors.New("test patch error")}
			},
			expectedErr: "test patch error",
		},
		{
			name: "success, unmanaged resource",
			expect: func(spec *mock_azure.MockASOResourceSpecGetterMockRecorder[*asoresourcesv1.ResourceGroup]) {
				spec.ResourceRef().Return(&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
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
							Labels: map[string]string{
								infrav1.OwnedByClusterLabelKey: "not-" + clusterName,
							},
						},
					}).
					Build()
			},
			verify: func(g Gomega, ctrlClient client.Client, spec azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]) {
				ctx := context.Background()
				actual := &asoresourcesv1.ResourceGroup{}
				g.Expect(ctrlClient.Get(ctx, client.ObjectKeyFromObject(spec.ResourceRef()), actual)).To(Succeed())
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

			mockCtrl := gomock.NewController(t)
			spec := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
			test.expect(spec.EXPECT())

			s := New[*asoresourcesv1.ResourceGroup](ctrlClient, clusterName)

			err := s.PauseResource(ctx, spec, svcName)
			if test.expectedErr != "" {
				g.Expect(err.Error()).To(ContainSubstring(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if test.verify != nil {
				test.verify(g, ctrlClient, spec)
			}
		})
	}
}
