/*
Copyright 2026 The Kubernetes Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"

	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav2exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

const (
	testNamespace        = "default"
	testClusterName      = "test-cluster"
	testControlPlaneName = "test-cp"
)

func createControlPlane(_ string, _ string, _ *metav1.Time, paused bool) *cplane.AROControlPlane {
	annotations := make(map[string]string)
	if paused {
		annotations[clusterv1.PausedAnnotation] = "true"
	}
	annotations[clusterctlv1.BlockMoveAnnotation] = "true"

	cp := &cplane.AROControlPlane{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav2exp.GroupVersion.String(),
			Kind:       cplane.AROControlPlaneKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              testControlPlaneName,
			Namespace:         testNamespace,
			UID:               "test-uid",
			Generation:        1,
			DeletionTimestamp: nil,
			Annotations:       annotations,
			Finalizers:        []string{cplane.AROControlPlaneFinalizer},
		},
		Spec: cplane.AROControlPlaneSpec{
			SubscriptionID:   "12345678-1234-1234-1234-123456789012",
			AzureEnvironment: "AzurePublicCloud",
			Resources: []runtime.RawExtension{{Raw: []byte(`{
				"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
				"kind": "OpenShiftCluster",
				"metadata": {"name": "test-aro-cluster"},
				"spec": {"location": "eastus", "properties": {
					"clusterProfile": {"version": "4.14.0"},
					"masterProfile": {
						"vmSize": "Standard_D8s_v3",
						"subnetId": "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
						"encryptionAtHost": "Disabled"
					}
				}}
			}`)}},
			IdentityRef: &corev1.ObjectReference{
				Name:      "test-identity",
				Namespace: testNamespace,
				Kind:      "AzureClusterIdentity",
			},
		},
	}

	return cp
}

func createCluster(_ string, _ string, paused bool) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav2exp.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       testClusterName,
			Namespace:  testNamespace,
			UID:        "test-cluster-uid",
			Generation: 1,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: &paused,
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrav2exp.GroupVersion.Group,
				Kind:     infrav2exp.AROClusterKind,
				Name:     "test-aro-cluster",
			},
		},
	}
}
