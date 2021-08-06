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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/logentries"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/record"
)

var (
	clusterControllers = []string{
		"AzureManagedCluster",
	}

	infraControllers = []string{
		"AzureMachinePool",
		"AzureManagedMachinePool",
	}
)

var _ = Describe("CommonReconcilerBehaviors", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	It("should trigger reconciliation if cluster is unpaused", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		logListener := record.NewListener(testEnv.LogRecorder)
		del := logListener.Listen()
		defer del()

		clusterName := test.RandomName("foo", 10)
		azManagedClusterName := test.RandomName("foo", 10)
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: "default",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Name:       azManagedClusterName,
					Namespace:  "default",
					Kind:       "AzureManagedCluster",
					APIVersion: infrav1exp.GroupVersion.Identifier(),
				},
			},
		}
		Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() {
			err := testEnv.Delete(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		}()

		cluster.Status.InfrastructureReady = true
		Expect(testEnv.Status().Update(ctx, cluster)).To(Succeed())
		ec := logentries.EntryCriteria{
			ClusterName:        cluster.Name,
			ClusterNamespace:   cluster.Namespace,
			InfraControllers:   infraControllers,
			ClusterControllers: clusterControllers,
		}
		logNotPausedEntries := logentries.GenerateCreateNotPausedLogEntries(ec)
		// check to make sure the cluster has reconciled and is not in paused state
		Eventually(logListener.GetEntries, test.DefaultEventualTimeout, 1*time.Second).Should(ContainElements(logNotPausedEntries))

		// we have tried to reconcile, and cluster was not paused
		// now, we will pause the cluster and we should trigger a watch event
		cluster.Spec.Paused = true
		Expect(testEnv.Update(ctx, cluster)).To(Succeed())
		logPausedEntries := logentries.GenerateUpdatePausedClusterLogEntries(ec)
		// check to make sure the cluster has reconciled and is paused
		Eventually(logListener.GetEntries, test.DefaultEventualTimeout, 1*time.Second).Should(ContainElements(logPausedEntries))

		// cluster was paused with an update
		// now, we will unpause the cluster and we should trigger an unpause watch event for all controllers
		cluster.Spec.Paused = false
		Expect(testEnv.Update(ctx, cluster)).To(Succeed())
		logUnpausedEntries := logentries.GenerateUpdateUnpausedClusterLogEntries(ec)
		Eventually(logListener.GetEntries, test.DefaultEventualTimeout, 1*time.Second).Should(ContainElements(logUnpausedEntries))
	})

})
