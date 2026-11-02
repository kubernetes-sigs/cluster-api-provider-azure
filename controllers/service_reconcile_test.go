/*
Copyright 2026 The Kubernetes Authors.

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
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/aso"
)

type stubService struct {
	name   string
	err    error
	called *[]string
}

func (s stubService) Name() string { return s.name }

func (s stubService) Reconcile(context.Context) error {
	*s.called = append(*s.called, s.name)
	return s.err
}

func (s stubService) Delete(context.Context) error { return nil }

func TestReconcileServicesContinuesAfterSubscriptionMismatch(t *testing.T) {
	g := NewWithT(t)
	var called []string
	mismatch := &aso.SubscriptionMismatchError{ClusterSubscription: "cluster-sub", ASOSubscription: "aso-sub"}
	services := []azure.ServiceReconciler{
		stubService{name: "groups", err: mismatch, called: &called},
		stubService{name: "virtualnetworks", err: mismatch, called: &called},
		stubService{name: "subnets", err: nil, called: &called},
		stubService{name: "managedclusters", err: errors.Wrap(mismatch, "wrapped"), called: &called},
	}

	err := reconcileServices(t.Context(), services, "AzureManagedControlPlane")
	g.Expect(err).To(MatchError(ContainSubstring("does not match")))
	g.Expect(called).To(Equal([]string{"groups", "virtualnetworks", "subnets", "managedclusters"}))
}

func TestReconcileServicesStopsOnOtherErrors(t *testing.T) {
	g := NewWithT(t)
	var called []string
	services := []azure.ServiceReconciler{
		stubService{name: "groups", err: errors.New("boom"), called: &called},
		stubService{name: "virtualnetworks", called: &called},
	}

	err := reconcileServices(t.Context(), services, "AzureCluster")
	g.Expect(err).To(MatchError(ContainSubstring("boom")))
	g.Expect(called).To(Equal([]string{"groups"}))
}
