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

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("AzureMachinePoolReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an AzureMachinePool", func() {
		It("should not error with minimal set up", func() {
			reconciler := NewAzureMachinePoolReconciler(testEnv, testEnv.GetEventRecorderFor("azuremachinepool-reconciler"),
				reconciler.DefaultLoopTimeout, "")
			By("Calling reconcile")
			instance := &infrav1exp.AzureMachinePool{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
			result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})
			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})
})
