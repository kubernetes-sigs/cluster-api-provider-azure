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
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/record"
)

var _ = Describe("AzureClusterReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an AzureCluster", func() {
		It("should reconcile and exit early due to the cluster not having an OwnerRef", func() {
			ctx := context.Background()
			logListener := record.NewListener(testEnv.LogRecorder)
			del := logListener.Listen()
			defer del()

			randName := test.RandomName("foo", 10)
			instance := &infrav1.AzureCluster{ObjectMeta: metav1.ObjectMeta{Name: randName, Namespace: "default"}}
			Expect(testEnv.Create(ctx, instance)).To(Succeed())
			defer func() {
				err := testEnv.Delete(ctx, instance)
				Expect(err).NotTo(HaveOccurred())
			}()

			// Make sure the Cluster exists.
			Eventually(logListener.GetEntries, 10*time.Second).
				Should(ContainElement(record.LogEntry{
					LogFunc: "Info",
					Values: []interface{}{
						"namespace",
						instance.Namespace,
						"azureCluster",
						randName,
						"msg",
						"Cluster Controller has not yet set OwnerRef",
					},
				}))
		})

		It("should fail with context timeout error if context expires", func() {
			mockCtrl := gomock.NewController(GinkgoT())
			defer mockCtrl.Finish()

			log := mock_log.NewMockLogger(mockCtrl)
			log.EXPECT().WithValues(gomock.Any()).DoAndReturn(func(args ...interface{}) logr.Logger {
				time.Sleep(3 * time.Second)
				return log
			})

			c, err := client.New(testEnv.Config, client.Options{Scheme: testEnv.GetScheme()})
			Expect(err).ToNot(HaveOccurred())
			reconciler := NewAzureClusterReconciler(c, log, testEnv.GetEventRecorderFor("azurecluster-reconciler"), 1*time.Second)

			instance := &infrav1.AzureCluster{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
			_, err = reconciler.Reconcile(ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Or(Equal("context deadline exceeded"), Equal("rate: Wait(n=1) would exceed context deadline")))
		})
	})
})
