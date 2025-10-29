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

package agentpools

import (
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	asocontainerservicev1preview "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231102preview"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools/mock_agentpools"
)

func TestPostCreateOrUpdateResourceHook(t *testing.T) {
	t.Run("error creating or updating", func(t *testing.T) {
		g := NewGomegaWithT(t)
		mockCtrl := gomock.NewController(t)
		scope := mock_agentpools.NewMockAgentPoolScope(mockCtrl)

		err := postCreateOrUpdateResourceHook(t.Context(), scope, nil, errors.New("an error"))
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("successful create or update, autoscaling disabled", func(t *testing.T) {
		g := NewGomegaWithT(t)
		mockCtrl := gomock.NewController(t)
		scope := mock_agentpools.NewMockAgentPoolScope(mockCtrl)

		scope.EXPECT().RemoveCAPIMachinePoolAnnotation(clusterv1.ReplicasManagedByAnnotation)

		managedCluster := &asocontainerservicev1.ManagedClustersAgentPool{
			Status: asocontainerservicev1.ManagedClustersAgentPool_STATUS{
				EnableAutoScaling: ptr.To(false),
			},
		}

		err := postCreateOrUpdateResourceHook(t.Context(), scope, managedCluster, nil)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("successful create or update, autoscaling enabled", func(t *testing.T) {
		g := NewGomegaWithT(t)
		mockCtrl := gomock.NewController(t)
		scope := mock_agentpools.NewMockAgentPoolScope(mockCtrl)

		scope.EXPECT().SetCAPIMachinePoolAnnotation(clusterv1.ReplicasManagedByAnnotation, "true")
		scope.EXPECT().SetCAPIMachinePoolReplicas(ptr.To(1234))

		managedCluster := &asocontainerservicev1.ManagedClustersAgentPool{
			Status: asocontainerservicev1.ManagedClustersAgentPool_STATUS{
				EnableAutoScaling: ptr.To(true),
				Count:             ptr.To(1234),
			},
		}

		err := postCreateOrUpdateResourceHook(t.Context(), scope, managedCluster, nil)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("successful create or update, preview enabled", func(t *testing.T) {
		g := NewGomegaWithT(t)
		mockCtrl := gomock.NewController(t)
		scope := mock_agentpools.NewMockAgentPoolScope(mockCtrl)

		scope.EXPECT().SetCAPIMachinePoolAnnotation(clusterv1.ReplicasManagedByAnnotation, "true")
		scope.EXPECT().SetCAPIMachinePoolReplicas(ptr.To(1234))

		agentPool := &asocontainerservicev1preview.ManagedClustersAgentPool{
			Status: asocontainerservicev1preview.ManagedClustersAgentPool_STATUS{
				EnableAutoScaling: ptr.To(true),
				Count:             ptr.To(1234),
			},
		}

		g.Expect(postCreateOrUpdateResourceHook(t.Context(), scope, agentPool, nil)).To(Succeed())
	})
}
