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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework"
)

// AzureServicePrincipalIdentitySpecInput is the input for AzureServicePrincipalIdentitySpec
type AzureServicePrincipalIdentitySpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
	IPv6                  bool
}

// AzureServicePrincipalIdentitySpec implements a test that verifies Azure identity can be added to
// an AzureCluster and used as the identity for that cluster.
func AzureServicePrincipalIdentitySpec(ctx context.Context, inputGetter func() AzureServicePrincipalIdentitySpecInput) {
	var (
		specName = "azure-identity"
		input    AzureServicePrincipalIdentitySpecInput
	)

	input = inputGetter()
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	By("creating Azure clients with the workload cluster's subscription")
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	vmsClient := compute.NewVirtualMachinesClient(subscriptionID)
	vmsClient.Authorizer = authorizer

	rgName := input.ClusterName
	machines, err := vmsClient.List(ctx, rgName)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(machines.Values())).To(BeNumerically(">", 0))
}
