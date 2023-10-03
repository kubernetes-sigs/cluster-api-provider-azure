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
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
)

func Test_newAzureMachinePoolService(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	cluster := newAzureCluster("fakeCluster")
	cluster.Spec.ResourceGroup = "resourceGroup"
	cluster.Spec.Location = "test-location"
	cluster.Spec.ResourceGroup = "my-rg"
	cluster.Spec.SubscriptionID = "123"
	cluster.Spec.NetworkSpec = infrav1.NetworkSpec{
		Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
	}

	clusterMock := mock_azure.NewMockClusterScoper(mockCtrl)
	clusterMock.EXPECT().SubscriptionID().AnyTimes()
	clusterMock.EXPECT().BaseURI().AnyTimes()
	clusterMock.EXPECT().CloudEnvironment().AnyTimes()
	clusterMock.EXPECT().Token().AnyTimes()
	clusterMock.EXPECT().Location().Return(cluster.Spec.Location)
	clusterMock.EXPECT().HashKey().Return("fakeCluster")
	clusterMock.EXPECT().CloudEnvironment().AnyTimes()
	clusterMock.EXPECT().Token().AnyTimes()

	mps := &scope.MachinePoolScope{
		ClusterScoper: clusterMock,
		MachinePool:   newMachinePool("fakeCluster", "poolName"),
		AzureMachinePool: &infrav1exp.AzureMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "poolName",
			},
		},
	}

	subject, err := newAzureMachinePoolService(mps)
	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(subject).NotTo(BeNil())
	g.Expect(subject.services).NotTo(BeEmpty())
	g.Expect(subject.skuCache).NotTo(BeNil())
}

func newScheme(g *GomegaWithT) *runtime.Scheme {
	scheme := runtime.NewScheme()
	for _, f := range []func(*runtime.Scheme) error{
		clusterv1.AddToScheme,
		expv1.AddToScheme,
		infrav1.AddToScheme,
		infrav1exp.AddToScheme,
	} {
		g.Expect(f(scheme)).To(Succeed())
	}
	return scheme
}

func newMachinePool(clusterName, poolName string) *expv1.MachinePool {
	return &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
		Spec: expv1.MachinePoolSpec{
			Replicas: ptr.To[int32](2),
		},
	}
}

func newAzureMachinePool(clusterName, poolName string) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
	}
}

func newMachinePoolWithInfrastructureRef(clusterName, poolName string) *expv1.MachinePool {
	m := newMachinePool(clusterName, poolName)
	m.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "AzureMachinePool",
		Namespace:  m.Namespace,
		Name:       "azure" + poolName,
		APIVersion: infrav1exp.GroupVersion.String(),
	}
	return m
}

func newAzureCluster(clusterName string) *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azure" + clusterName,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       clusterName,
				},
			},
		},
	}
}

func newCluster(name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}
