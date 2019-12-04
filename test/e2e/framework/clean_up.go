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

package framework

import (
	"context"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/framework/management/kind"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// CleanUpInput are all the dependencies needed to clean up a Cluster API cluster.
type CleanUpInput struct {
	Management    *kind.Cluster
	Cluster       *clusterv1.Cluster
	DeleteTimeout time.Duration
}

func (c *CleanUpInput) setDefaults() {
	if c.DeleteTimeout == 0*time.Second {
		c.DeleteTimeout = 20 * time.Minute
	}
}

// CleanUp deletes the cluster and waits for everything to be gone.
func CleanUp(input *CleanUpInput) {
	input.setDefaults()

	mgmtClient, err := input.Management.GetClient()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "stack: %+v", err)

	ginkgo.By("Deleting cluster")
	ctx := context.Background()
	gomega.Expect(mgmtClient.Delete(ctx, input.Cluster)).NotTo(gomega.HaveOccurred())

	gomega.Eventually(func() []clusterv1.Cluster {
		clusters := clusterv1.ClusterList{}
		c, err := input.Management.GetClient()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(c.List(ctx, &clusters)).NotTo(gomega.HaveOccurred())
		return clusters.Items
	}, input.DeleteTimeout, 20*time.Second).Should(gomega.HaveLen(0))
}
