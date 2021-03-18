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

package scope

import (
	"context"
	"testing"

	"github.com/Azure/go-autorest/autorest"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGettingIngressRules(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
	}
	cluster.Default()

	azureCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-azure-cluster",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1alpha4",
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		},
		Spec: infrav1.AzureClusterSpec{
			SubscriptionID: "123",
			NetworkSpec: infrav1.NetworkSpec{
				Subnets: infrav1.Subnets{
					{
						Name: "node",
						Role: infrav1.SubnetNode,
					},
				},
			},
		},
	}
	azureCluster.Default()

	initObjects := []runtime.Object{cluster, azureCluster}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	clusterScope, err := NewClusterScope(context.TODO(), ClusterScopeParams{
		AzureClients: AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Cluster:      cluster,
		AzureCluster: azureCluster,
		Client:       fakeClient,
	})
	g.Expect(err).ToNot(HaveOccurred())

	clusterScope.SetControlPlaneIngressRules()

	subnet, err := clusterScope.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(subnet.SecurityGroup.IngressRules)).To(Equal(2))
}
