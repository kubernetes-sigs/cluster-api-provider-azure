// +build e2e

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

package e2e

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureFailureDomainsSpecInput is the input for AzureFailureDomainSpec.
type AzureFailureDomainsSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Cluster               *clusterv1.Cluster
	Namespace             *corev1.Namespace
	ClusterName           string
}

// AzureFailureDomainsSpec implements a test that checks that control plane machines are spread
// across Azure failure domains.
func AzureFailureDomainsSpec(ctx context.Context, inputGetter func() AzureFailureDomainsSpecInput) {
	var (
		specName    = "azure-failuredomains"
		input       AzureFailureDomainsSpecInput
		machineType = os.Getenv("AZURE_CONTROL_PLANE_MACHINE_TYPE")
		location    = os.Getenv("AZURE_LOCATION")
		zones       []string
	)

	input = inputGetter()
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	zones, err := getAvailabilityZonesForRegion(location, machineType)
	Expect(err).NotTo(HaveOccurred())

	if zones != nil {
		// location supports zones for selected machine type
		By("Ensuring zones match CAPI failure domains")

		// fetch updated cluster object to ensure Status.FailureDomains is up-to-date
		err := input.BootstrapClusterProxy.GetClient().Get(context.TODO(), apimachinerytypes.NamespacedName{
			Namespace: input.Namespace.Name, Name: input.ClusterName}, input.Cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(input.Cluster.Status.FailureDomains)).To(Equal(len(zones)))
		for _, z := range zones {
			Expect(input.Cluster.Status.FailureDomains[z]).NotTo(BeNil())
		}

		// TODO: Find alternative when the number of zones is > 1 but doesn't equal to number of control plane machines.
		if len(input.Cluster.Status.FailureDomains) == 3 {
			By("Ensuring control planes are spread across availability zones.")
			key, err := client.ObjectKeyFromObject(input.Cluster)
			Expect(err).NotTo(HaveOccurred())
			failureDomainsInput := framework.AssertControlPlaneFailureDomainsInput{
				GetLister:  input.BootstrapClusterProxy.GetClient(),
				ClusterKey: key,
				ExpectedFailureDomains: map[string]int{
					"1": 1,
					"2": 1,
					"3": 1,
				},
			}
			framework.AssertControlPlaneFailureDomains(ctx, failureDomainsInput)
		}
	}
}
