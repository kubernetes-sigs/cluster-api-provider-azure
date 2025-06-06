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

package coalescing

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mock_coalescing "sigs.k8s.io/cluster-api-provider-azure/pkg/coalescing/mocks"
)

func TestCoalescingReconciler_Reconcile(t *testing.T) {
	var (
		defaultRequest = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "aName",
				Namespace: "aNamespace",
			},
		}

		defaultRequestKey = "aNamespace/aName"
	)

	cases := []struct {
		Name       string
		Reconciler func(g *WithT, cacherMock *mock_coalescing.MockReconcileCacher, mockReconciler *mock_coalescing.MockReconciler[reconcile.Request]) reconcile.Reconciler
		Request    reconcile.Request
		MatchThis  gtypes.GomegaMatcher
		Error      string
	}{
		{
			Name: "should call upstream reconciler if key does not exist in cache",
			Reconciler: func(g *WithT, cacherMock *mock_coalescing.MockReconcileCacher, mockReconciler *mock_coalescing.MockReconciler[reconcile.Request]) reconcile.Reconciler {
				cacherMock.EXPECT().ShouldProcess(defaultRequestKey).Return(time.Now(), true)
				cacherMock.EXPECT().Reconciled(defaultRequestKey)
				mockReconciler.EXPECT().Reconcile(gomock.Any(), defaultRequest)
				return NewReconciler(mockReconciler, cacherMock, logr.New(log.NullLogSink{}))
			},
			Request:   defaultRequest,
			MatchThis: Equal(0 * time.Second),
		},
		{
			Name: "should not call upstream reconciler if key does exists in cache and is not expired",
			Reconciler: func(g *WithT, cacherMock *mock_coalescing.MockReconcileCacher, mockReconciler *mock_coalescing.MockReconciler[reconcile.Request]) reconcile.Reconciler {
				cacherMock.EXPECT().ShouldProcess(defaultRequestKey).Return(time.Now().Add(30*time.Second), false)
				return NewReconciler(mockReconciler, cacherMock, logr.New(log.NullLogSink{}))
			},
			Request:   defaultRequest,
			MatchThis: And(BeNumerically("<=", 30*time.Second), BeNumerically(">", 29*time.Second)),
		},
		{
			Name: "should call upstream reconciler if key does not exist in cache and return error",
			Reconciler: func(g *WithT, cacherMock *mock_coalescing.MockReconcileCacher, mockReconciler *mock_coalescing.MockReconciler[reconcile.Request]) reconcile.Reconciler {
				cacherMock.EXPECT().ShouldProcess(defaultRequestKey).Return(time.Now(), true)
				mockReconciler.EXPECT().Reconcile(gomock.Any(), defaultRequest).Return(reconcile.Result{}, errors.New("boom"))
				return NewReconciler(mockReconciler, cacherMock, logr.New(log.NullLogSink{}))
			},
			Request:   defaultRequest,
			MatchThis: Equal(0 * time.Second),
			Error:     "boom",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			cacherMock := mock_coalescing.NewMockReconcileCacher(mockCtrl)
			reconcilerMock := mock_coalescing.NewMockReconciler[reconcile.Request](mockCtrl)
			subject := c.Reconciler(g, cacherMock, reconcilerMock)
			result, err := subject.Reconcile(context.Background(), c.Request)
			if c.Error != "" || err != nil {
				g.Expect(err).To(And(HaveOccurred(), MatchError(c.Error)))
				return
			}

			g.Expect(result.RequeueAfter).To(c.MatchThis)
		})
	}
}
