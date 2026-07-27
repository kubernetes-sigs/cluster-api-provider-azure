//go:build e2e
// +build e2e

/*
Copyright The Kubernetes Authors.

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
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLabelASOCRDsForClusterctlUpgrade(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	asoCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "virtualnetworks.network.azure.com",
			Labels: map[string]string{
				asoCRDAppLabel: asoCRDAppValue,
				"existing":     "label",
			},
		},
	}
	mislabeledASOCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "managedclusters.containerservice.azure.com",
			Labels: map[string]string{
				asoCRDAppLabel:              asoCRDAppValue,
				clusterv1.ProviderNameLabel: "other-provider",
			},
		},
	}
	nonASOCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "clusters.cluster.x-k8s.io"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(asoCRD, mislabeledASOCRD, nonASOCRD).Build()
	ctx := context.Background()
	g.Expect(labelASOCRDsForClusterctlUpgrade(ctx, c)).To(Succeed())
	g.Expect(labelASOCRDsForClusterctlUpgrade(ctx, c)).To(Succeed())

	got := &apiextensionsv1.CustomResourceDefinition{}
	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoCRD), got)).To(Succeed())
	g.Expect(got.Labels).To(HaveKeyWithValue(clusterv1.ProviderNameLabel, infrastructureProviderLabel))
	g.Expect(got.Labels).To(HaveKeyWithValue("existing", "label"))

	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(mislabeledASOCRD), got)).To(Succeed())
	g.Expect(got.Labels).To(HaveKeyWithValue(clusterv1.ProviderNameLabel, infrastructureProviderLabel))

	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(nonASOCRD), got)).To(Succeed())
	g.Expect(got.Labels).NotTo(HaveKey(clusterv1.ProviderNameLabel))
}

func TestLabelASOCRDsForClusterctlUpgradeRequiresASOCRDs(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	err := labelASOCRDsForClusterctlUpgrade(context.Background(), c)
	g.Expect(err).To(MatchError("no ASO-managed CRDs found"))
}
