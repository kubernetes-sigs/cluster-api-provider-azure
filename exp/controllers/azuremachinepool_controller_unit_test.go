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

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
)

func Test_newAzureMachinePoolService(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	cluster := newAzureCluster("foo")
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
	clusterMock.EXPECT().Authorizer().AnyTimes()
	clusterMock.EXPECT().Location().Return(cluster.Spec.Location)
	clusterMock.EXPECT().HashKey().Return("foo")

	mps := &scope.MachinePoolScope{
		ClusterScoper: clusterMock,
		MachinePool:   newMachinePool("foo", "poolName"),
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
	g.Expect(subject.virtualMachinesScaleSetSvc).NotTo(BeNil())
	g.Expect(subject.skuCache).NotTo(BeNil())
}

func newScheme(g *GomegaWithT) *runtime.Scheme {
	scheme := runtime.NewScheme()
	for _, f := range []func(*runtime.Scheme) error{
		clusterv1.AddToScheme,
		clusterv1exp.AddToScheme,
		infrav1.AddToScheme,
		infrav1exp.AddToScheme,
	} {
		g.Expect(f(scheme)).ToNot(HaveOccurred())
	}
	return scheme
}

func newMachinePool(clusterName, poolName string) *clusterv1exp.MachinePool {
	return &clusterv1exp.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
		Spec: clusterv1exp.MachinePoolSpec{
			Replicas: to.Int32Ptr(2),
		},
	}
}

func newAzureMachinePool(clusterName, poolName string) *infrav1exp.AzureMachinePool {
	return &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
	}
}

func newMachinePoolWithInfrastructureRef(clusterName, poolName string) *clusterv1exp.MachinePool {
	m := newMachinePool(clusterName, poolName)
	m.Spec.Template.Spec.InfrastructureRef = v1.ObjectReference{
		Kind:       "AzureMachinePool",
		Namespace:  m.Namespace,
		Name:       "azure" + poolName,
		APIVersion: infrav1exp.GroupVersion.String(),
	}
	return m
}

func newManagedMachinePoolWithInfrastructureRef(clusterName, poolName string) *clusterv1exp.MachinePool {
	m := newMachinePool(clusterName, poolName)
	m.Spec.Template.Spec.InfrastructureRef = v1.ObjectReference{
		Kind:       "AzureManagedMachinePool",
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
