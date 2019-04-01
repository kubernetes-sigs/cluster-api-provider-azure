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

package machine

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-03-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachines"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/fake"
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
)

var (
	_ machine.Actuator = (*Actuator)(nil)
)

func newClusterProviderSpec() v1alpha1.AzureClusterProviderSpec {
	return v1alpha1.AzureClusterProviderSpec{
		ResourceGroup: v1alpha1.ResourceGroup{
			Name: "resource-group-test",
		},
		Location: "southcentralus",
	}
}

func providerSpecFromMachine(in *v1alpha1.AzureMachineProviderSpec) (*clusterv1.ProviderSpec, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func providerSpecFromCluster(in *v1alpha1.AzureClusterProviderSpec) (*clusterv1.ProviderSpec, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func newMachine(t *testing.T, machineConfig v1alpha1.AzureMachineProviderSpec, labels map[string]string) *clusterv1.Machine {
	providerSpec, err := providerSpecFromMachine(&machineConfig)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "machine-test",
			Labels: labels,
		},
		Spec: clusterv1.MachineSpec{
			ProviderSpec: *providerSpec,
			Versions: clusterv1.MachineVersionInfo{
				Kubelet:      "1.14.1",
				ControlPlane: "1.14.1",
			},
		},
	}
}

func newCluster(t *testing.T) *clusterv1.Cluster {
	clusterProviderSpec := newClusterProviderSpec()
	providerSpec, err := providerSpecFromCluster(&clusterProviderSpec)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}

	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-test",
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: clusterv1.ClusterNetworkingConfig{
				Services: clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"10.96.0.0/12",
					},
				},
				Pods: clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			ProviderSpec: *providerSpec,
		},
	}
}

func newFakeScope(t *testing.T, label string) *actuators.MachineScope {
	scope := &actuators.Scope{
		Context: context.Background(),
		Cluster: newCluster(t),
		ClusterConfig: &v1alpha1.AzureClusterProviderSpec{
			ResourceGroup: v1alpha1.ResourceGroup{
				Name: "dummyResourceGroup",
			},
			Location:            "dummyLocation",
			CAKeyPair:           v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			EtcdCAKeyPair:       v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			FrontProxyCAKeyPair: v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			SAKeyPair:           v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			DiscoveryHashes:     []string{"discoveryhash0"},
		},
		ClusterStatus: &v1alpha1.AzureClusterProviderStatus{},
	}
	scope.Network().APIServerIP.DNSName = "DummyDNSName"
	labels := make(map[string]string)
	labels["set"] = label
	machineConfig := v1alpha1.AzureMachineProviderSpec{}
	m := newMachine(t, machineConfig, labels)
	c := fake.NewSimpleClientset(m).ClusterV1alpha1()
	return &actuators.MachineScope{
		Scope:         scope,
		Machine:       m,
		MachineClient: c.Machines("dummyNamespace"),
		MachineConfig: &v1alpha1.AzureMachineProviderSpec{},
		MachineStatus: &v1alpha1.AzureMachineProviderStatus{},
	}
}

func newFakeReconciler(t *testing.T) *Reconciler {
	fakeSuccessSvc := &azure.FakeSuccessService{}
	fakeVMSuccessSvc := &FakeVMService{
		Name:              "machine-test",
		ID:                "machine-test-ID",
		ProvisioningState: "Succeeded",
	}

	return &Reconciler{
		scope:                 newFakeScope(t, v1alpha1.ControlPlane),
		availabilityZonesSvc:  fakeSuccessSvc,
		networkInterfacesSvc:  fakeSuccessSvc,
		virtualMachinesSvc:    fakeVMSuccessSvc,
		virtualMachinesExtSvc: fakeSuccessSvc,
		disksSvc:              fakeSuccessSvc,
	}
}

func newFakeReconcilerWithScope(t *testing.T, scope *actuators.MachineScope) *Reconciler {
	fakeSuccessSvc := &azure.FakeSuccessService{}
	fakeVMSuccessSvc := &FakeVMService{
		Name:              "machine-test",
		ID:                "machine-test-ID",
		ProvisioningState: "Succeeded",
	}

	return &Reconciler{
		scope:                 scope,
		availabilityZonesSvc:  fakeSuccessSvc,
		networkInterfacesSvc:  fakeSuccessSvc,
		virtualMachinesSvc:    fakeVMSuccessSvc,
		virtualMachinesExtSvc: fakeSuccessSvc,
		disksSvc:              fakeSuccessSvc,
	}
}

// FakeVMService generic vm service
type FakeVMService struct {
	Name                    string
	ID                      string
	ProvisioningState       string
	GetCallCount            int
	CreateOrUpdateCallCount int
	DeleteCallCount         int
}

// Get returns fake success.
func (s *FakeVMService) Get(ctx context.Context, spec v1alpha1.ResourceSpec) (interface{}, error) {
	s.GetCallCount++
	return compute.VirtualMachine{
		ID:   to.StringPtr(s.ID),
		Name: to.StringPtr(s.Name),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			ProvisioningState: to.StringPtr(s.ProvisioningState),
		},
	}, nil
}

// Reconcile returns fake success.
func (s *FakeVMService) Reconcile(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	s.CreateOrUpdateCallCount++
	return nil
}

// Delete returns fake success.
func (s *FakeVMService) Delete(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	s.DeleteCallCount++
	return nil
}

func TestReconcilerSuccess(t *testing.T) {
	fakeReconciler := newFakeReconciler(t)

	if err := fakeReconciler.Create(context.Background()); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	if err := fakeReconciler.Update(context.Background()); err != nil {
		t.Errorf("failed to update machine: %+v", err)
	}

	if _, err := fakeReconciler.Exists(context.Background()); err != nil {
		t.Errorf("failed to check if machine exists: %+v", err)
	}

	if err := fakeReconciler.Delete(context.Background()); err != nil {
		t.Errorf("failed to delete machine: %+v", err)
	}
}

func TestReconcileFailure(t *testing.T) {
	fakeFailureSvc := &azure.FakeFailureService{}
	fakeReconciler := newFakeReconciler(t)
	fakeReconciler.networkInterfacesSvc = fakeFailureSvc
	fakeReconciler.virtualMachinesSvc = fakeFailureSvc
	fakeReconciler.virtualMachinesExtSvc = fakeFailureSvc

	if err := fakeReconciler.Create(context.Background()); err == nil {
		t.Errorf("expected create to fail")
	}

	if err := fakeReconciler.Update(context.Background()); err == nil {
		t.Errorf("expected update to fail")
	}

	if _, err := fakeReconciler.Exists(context.Background()); err == nil {
		t.Errorf("expected exists to fail")
	}

	if err := fakeReconciler.Delete(context.Background()); err == nil {
		t.Errorf("expected delete to fail")
	}
}

func TestReconcileVMFailedState(t *testing.T) {
	fakeReconciler := newFakeReconciler(t)
	fakeVMService := &FakeVMService{
		Name:              "machine-test",
		ID:                "machine-test-ID",
		ProvisioningState: "Failed",
	}
	fakeReconciler.virtualMachinesSvc = fakeVMService

	if err := fakeReconciler.Create(context.Background()); err == nil {
		t.Errorf("expected create to fail")
	}

	if fakeVMService.GetCallCount != 1 {
		t.Errorf("expected get to be called just once")
	}

	if fakeVMService.DeleteCallCount != 1 {
		t.Errorf("expected delete to be called just once")
	}

	if fakeVMService.CreateOrUpdateCallCount != 0 {
		t.Errorf("expected reconcile not to be called")
	}
}

func TestReconcileVMUpdatingState(t *testing.T) {
	fakeReconciler := newFakeReconciler(t)
	fakeVMService := &FakeVMService{
		Name:              "machine-test",
		ID:                "machine-test-ID",
		ProvisioningState: "Updating",
	}
	fakeReconciler.virtualMachinesSvc = fakeVMService

	if err := fakeReconciler.Create(context.Background()); err == nil {
		t.Errorf("expected create to fail")
	}

	if fakeVMService.GetCallCount != 1 {
		t.Errorf("expected get to be called just once")
	}

	if fakeVMService.DeleteCallCount != 0 {
		t.Errorf("expected delete not to be called")
	}

	if fakeVMService.CreateOrUpdateCallCount != 0 {
		t.Errorf("expected reconcile not to be called")
	}
}

func TestReconcileVMSuceededState(t *testing.T) {
	fakeReconciler := newFakeReconciler(t)
	fakeVMService := &FakeVMService{
		Name:              "machine-test",
		ID:                "machine-test-ID",
		ProvisioningState: "Succeeded",
	}
	fakeReconciler.virtualMachinesSvc = fakeVMService

	if err := fakeReconciler.Create(context.Background()); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	if fakeVMService.GetCallCount != 1 {
		t.Errorf("expected get to be called just once")
	}

	if fakeVMService.DeleteCallCount != 0 {
		t.Errorf("expected delete not to be called")
	}

	if fakeVMService.CreateOrUpdateCallCount != 0 {
		t.Errorf("expected reconcile not to be called")
	}
}

func TestNodeJoinFirstControlPlane(t *testing.T) {
	fakeReconciler := newFakeReconciler(t)

	if isNodeJoin, err := fakeReconciler.isNodeJoin(); err != nil {
		t.Errorf("isNodeJoin failed to create machine: %+v", err)
	} else if isNodeJoin {
		t.Errorf("Expected isNodeJoin to be false since its first VM")
	}
}

func TestNodeJoinNode(t *testing.T) {
	fakeScope := newFakeScope(t, v1alpha1.Node)
	fakeReconciler := newFakeReconcilerWithScope(t, fakeScope)

	if isNodeJoin, err := fakeReconciler.isNodeJoin(); err != nil {
		t.Errorf("isNodeJoin failed to create machine: %+v", err)
	} else if !isNodeJoin {
		t.Errorf("Expected isNodeJoin to be true since its a node")
	}
}

func TestNodeJoinSecondControlPlane(t *testing.T) {
	fakeScope := newFakeScope(t, v1alpha1.ControlPlane)
	fakeReconciler := newFakeReconcilerWithScope(t, fakeScope)

	if _, err := fakeScope.MachineClient.Create(fakeScope.Machine); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	if isNodeJoin, err := fakeReconciler.isNodeJoin(); err != nil {
		t.Errorf("isNodeJoin failed to create machine: %+v", err)
	} else if !isNodeJoin {
		t.Errorf("Expected isNodeJoin to be true since its second control plane vm")
	}
}

// FakeVMCheckZonesService generic fake vm zone service
type FakeVMCheckZonesService struct {
	checkZones []string
}

// Get returns fake success.
func (s *FakeVMCheckZonesService) Get(ctx context.Context, spec v1alpha1.ResourceSpec) (interface{}, error) {
	return nil, errors.New("vm not found")
}

// Reconcile returns fake success.
func (s *FakeVMCheckZonesService) Reconcile(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	vmSpec, ok := spec.(*virtualmachines.Spec)
	if !ok {
		return errors.New("invalid vm specification")
	}

	if len(s.checkZones) <= 0 {
		return nil
	}
	for _, zone := range s.checkZones {
		if strings.EqualFold(zone, vmSpec.Zone) {
			return nil
		}
	}

	return errors.New("invalid input zone")
}

// Delete returns fake success.
func (s *FakeVMCheckZonesService) Delete(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	return nil
}

// FakeAvailabilityZonesService generic fake availability zones
type FakeAvailabilityZonesService struct {
	zonesResponse []string
}

// Get returns fake success.
func (s *FakeAvailabilityZonesService) Get(ctx context.Context, spec v1alpha1.ResourceSpec) (interface{}, error) {
	return s.zonesResponse, nil
}

// Reconcile returns fake success.
func (s *FakeAvailabilityZonesService) Reconcile(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	return nil
}

// Delete returns fake success.
func (s *FakeAvailabilityZonesService) Delete(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	return nil
}

func TestAvailabilityZones(t *testing.T) {
	fakeScope := newFakeScope(t, v1alpha1.ControlPlane)
	fakeReconciler := newFakeReconcilerWithScope(t, fakeScope)

	zones := []string{"1", "2", "3"}

	fakeReconciler.availabilityZonesSvc = &FakeAvailabilityZonesService{
		zonesResponse: zones,
	}

	fakeReconciler.virtualMachinesSvc = &FakeVMCheckZonesService{
		checkZones: zones,
	}

	if err := fakeReconciler.Create(context.Background()); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	fakeReconciler.availabilityZonesSvc = &FakeAvailabilityZonesService{
		zonesResponse: []string{},
	}

	fakeReconciler.virtualMachinesSvc = &FakeVMCheckZonesService{
		checkZones: []string{},
	}

	if err := fakeReconciler.Create(context.Background()); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	fakeReconciler.availabilityZonesSvc = &FakeAvailabilityZonesService{
		zonesResponse: []string{"2"},
	}

	fakeReconciler.virtualMachinesSvc = &FakeVMCheckZonesService{
		checkZones: []string{"3"},
	}

	if err := fakeReconciler.Create(context.Background()); err == nil {
		t.Errorf("expected create to fail due to zone mismatch")
	}
}
