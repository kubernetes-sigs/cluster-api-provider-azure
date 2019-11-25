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

package e2e

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/framework"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/generators"

	corev1 "k8s.io/api/core/v1"
	bootstrapv1 "sigs.k8s.io/cluster-api-bootstrap-provider-kubeadm/api/v1alpha2"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "capz e2e suite")
}

var (
	mgmt *ManagementCluster
	ctx  = context.Background()

	location = "westus2"
	vmSize   = "Standard_B2ms"

	namespace  = "default"
	k8sVersion = "v1.16.2"

	imageOffer     = "capi"
	imagePublisher = "cncf-upstream"
	imageSKU       = "k8s-1dot16-ubuntu-1804"
	imageVersion   = "latest"
)

var _ = BeforeSuite(func() {
	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		managerImage = "us.gcr.io/k8s-artifacts-prod/cluster-api-azure/cluster-api-azure-controller:v0.3.0-alpha.1"
	}

	By("creating management cluster")
	var err error

	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(capiv1.AddToScheme(scheme)).To(Succeed())
	Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())
	Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	// Create the management cluster
	capi := &generators.ClusterAPI{Version: "v0.2.3"}
	cabpk := &generators.Bootstrap{Version: "v0.1.1"}
	infra := &generators.Infra{}

	mgmt, err = NewManagementCluster(ctx, "mgmt", scheme, managerImage)
	Expect(err).NotTo(HaveOccurred())
	Expect(mgmt).NotTo(BeNil())

	framework.InstallComponents(ctx, mgmt, capi, cabpk, infra)

	// TODO: consider watching/persisting components logs
	// TODO: maybe wait for controller components to be ready
})

var _ = AfterSuite(func() {
	By("tearing down management cluster")
	Expect(mgmt.Teardown(ctx)).NotTo(HaveOccurred())
})
