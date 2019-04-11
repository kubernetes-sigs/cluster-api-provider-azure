/*
Copyright 2018 The Kubernetes Authors.

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

package cluster

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func newFakeScope() *actuators.Scope {
	return &actuators.Scope{
		Context: context.Background(),
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dummyClusterName",
			},
		},
		ClusterConfig: &v1alpha1.AzureClusterProviderSpec{},
		ClusterStatus: &v1alpha1.AzureClusterProviderStatus{},
	}
}

func TestReconcileSuccess(t *testing.T) {
	fakeSuccessSvc := &azure.FakeSuccessService{}
	fakeNotFoundSvc := &azure.FakeNotFoundService{}

	fakeReconciler := &Reconciler{
		scope:            newFakeScope(),
		groupsSvc:        fakeSuccessSvc,
		certificatesSvc:  fakeSuccessSvc,
		vnetSvc:          fakeSuccessSvc,
		securityGroupSvc: fakeSuccessSvc,
		routeTableSvc:    fakeSuccessSvc,
		subnetsSvc:       fakeSuccessSvc,
		internalLBSvc:    fakeSuccessSvc,
		publicIPSvc:      fakeSuccessSvc,
		publicLBSvc:      fakeSuccessSvc,
	}

	if err := fakeReconciler.Reconcile(); err != nil {
		t.Errorf("failed to reconcile cluster services: %+v", err)
	}

	if err := fakeReconciler.Delete(); err != nil {
		t.Errorf("failed to delete cluster services: %+v", err)
	}

	fakeReconciler.groupsSvc = fakeNotFoundSvc

	if err := fakeReconciler.Delete(); err != nil {
		t.Errorf("failed to delete cluster services: %+v", err)
	}
}

func TestReconcileFailure(t *testing.T) {
	fakeSuccessSvc := &azure.FakeSuccessService{}
	fakeFailureSvc := &azure.FakeFailureService{}

	fakeReconciler := &Reconciler{
		scope:            newFakeScope(),
		certificatesSvc:  fakeFailureSvc,
		groupsSvc:        fakeSuccessSvc,
		vnetSvc:          fakeSuccessSvc,
		securityGroupSvc: fakeFailureSvc,
		routeTableSvc:    fakeSuccessSvc,
		subnetsSvc:       fakeSuccessSvc,
		internalLBSvc:    fakeFailureSvc,
		publicIPSvc:      fakeSuccessSvc,
		publicLBSvc:      fakeSuccessSvc,
	}

	if err := fakeReconciler.Reconcile(); err == nil {
		t.Errorf("expected reconcile to fail")
	}

	if err := fakeReconciler.Delete(); err == nil {
		t.Errorf("expected delete to fail")
	}
}

func TestPublicIPNonEmpty(t *testing.T) {
	fakeSuccessSvc := &azure.FakeSuccessService{}

	fakeReconciler := &Reconciler{
		scope:            newFakeScope(),
		groupsSvc:        fakeSuccessSvc,
		certificatesSvc:  fakeSuccessSvc,
		vnetSvc:          fakeSuccessSvc,
		securityGroupSvc: fakeSuccessSvc,
		routeTableSvc:    fakeSuccessSvc,
		subnetsSvc:       fakeSuccessSvc,
		internalLBSvc:    fakeSuccessSvc,
		publicIPSvc:      fakeSuccessSvc,
		publicLBSvc:      fakeSuccessSvc,
	}

	if err := fakeReconciler.Reconcile(); err != nil {
		t.Errorf("failed to reconcile cluster services: %+v", err)
	}

	ipName := fakeReconciler.scope.Network().APIServerIP.Name

	if ipName == "" {
		t.Errorf("public ip still empty, expected to be refilled")
	}

	if err := fakeReconciler.Reconcile(); err != nil {
		t.Errorf("failed to reconcile cluster services: %+v", err)
	}

	if fakeReconciler.scope.Network().APIServerIP.Name != ipName {
		t.Errorf("expected public ip to be not generated again")
	}
}

func TestServicesCreatedCount(t *testing.T) {
	cache := make(map[string]int)
	fakeSuccessSvc := &azure.FakeCachedService{Cache: &cache}

	fakeReconciler := &Reconciler{
		scope:            newFakeScope(),
		groupsSvc:        fakeSuccessSvc,
		certificatesSvc:  fakeSuccessSvc,
		vnetSvc:          fakeSuccessSvc,
		securityGroupSvc: fakeSuccessSvc,
		routeTableSvc:    fakeSuccessSvc,
		subnetsSvc:       fakeSuccessSvc,
		internalLBSvc:    fakeSuccessSvc,
		publicIPSvc:      fakeSuccessSvc,
		publicLBSvc:      fakeSuccessSvc,
	}

	if err := fakeReconciler.Reconcile(); err != nil {
		t.Errorf("failed to reconcile cluster services: %+v", err)
	}

	if cache[azure.GenerateVnetName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GenerateVnetName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[azure.GenerateControlPlaneSecurityGroupName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GenerateControlPlaneSecurityGroupName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[azure.GenerateNodeSecurityGroupName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GenerateNodeSecurityGroupName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[azure.GenerateNodeRouteTableName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GenerateNodeRouteTableName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[azure.GenerateControlPlaneSubnetName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GenerateControlPlaneSubnetName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[azure.GenerateNodeSubnetName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GenerateNodeSubnetName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[azure.GenerateInternalLBName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GenerateInternalLBName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[azure.GeneratePublicLBName(fakeReconciler.scope.Cluster.Name)] != 1 {
		t.Errorf("Expected 1 count of %s service", azure.GeneratePublicLBName(fakeReconciler.scope.Cluster.Name))
	}

	if cache[fakeReconciler.scope.Network().APIServerIP.Name] != 1 {
		t.Errorf("Expected 1 count of %s service", fakeReconciler.scope.Network().APIServerIP.Name)
	}
}
