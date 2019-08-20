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
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/certificates"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachines"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/fake"
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
)

var (
	_ machine.Actuator = (*Actuator)(nil)
)

func contains(s []*clusterv1.Machine, e clusterv1.Machine) bool {
	exists := false
	for _, em := range s {
		if em.Name == e.Name && em.Namespace == e.Namespace {
			exists = true
			break
		}
	}
	return exists
}

func TestGetControlPlaneMachines(t *testing.T) {
	testCases := []struct {
		name        string
		input       *clusterv1.MachineList
		expectedOut []clusterv1.Machine
	}{
		{
			name: "0 machines",
			input: &clusterv1.MachineList{
				Items: []clusterv1.Machine{},
			},
			expectedOut: []clusterv1.Machine{},
		},
		{
			name: "only 2 controlplane machines",
			input: &clusterv1.MachineList{
				Items: []clusterv1.Machine{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "master-0",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet:      "v1.13.0",
								ControlPlane: "v1.13.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "master-1",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet:      "v1.13.0",
								ControlPlane: "v1.13.0",
							},
						},
					},
				},
			},
			expectedOut: []clusterv1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-0",
						Namespace: "awesome-ns",
					},
					Spec: clusterv1.MachineSpec{
						Versions: clusterv1.MachineVersionInfo{
							Kubelet:      "v1.13.0",
							ControlPlane: "v1.13.0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-1",
						Namespace: "awesome-ns",
					},
					Spec: clusterv1.MachineSpec{
						Versions: clusterv1.MachineVersionInfo{
							Kubelet:      "v1.13.0",
							ControlPlane: "v1.13.0",
						},
					},
				},
			},
		},
		{
			name: "2 controlplane machines, 1 deleted",
			input: &clusterv1.MachineList{
				Items: []clusterv1.Machine{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "master-0",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet:      "v1.13.0",
								ControlPlane: "v1.13.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "master-1",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet:      "v1.13.0",
								ControlPlane: "v1.13.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "master-2",
							Namespace: "awesome-ns",
							DeletionTimestamp: &metav1.Time{
								Time: time.Now(),
							},
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet:      "v1.13.0",
								ControlPlane: "v1.13.0",
							},
						},
					},
				},
			},
			expectedOut: []clusterv1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-0",
						Namespace: "awesome-ns",
					},
					Spec: clusterv1.MachineSpec{
						Versions: clusterv1.MachineVersionInfo{
							Kubelet:      "v1.13.0",
							ControlPlane: "v1.13.0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-1",
						Namespace: "awesome-ns",
					},
					Spec: clusterv1.MachineSpec{
						Versions: clusterv1.MachineVersionInfo{
							Kubelet:      "v1.13.0",
							ControlPlane: "v1.13.0",
						},
					},
				},
			},
		},
		{
			name: "2 controlplane machines and 2 worker machines",
			input: &clusterv1.MachineList{
				Items: []clusterv1.Machine{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "master-0",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet:      "v1.13.0",
								ControlPlane: "v1.13.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "master-1",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet:      "v1.13.0",
								ControlPlane: "v1.13.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "worker-0",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet: "v1.13.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "worker-1",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet: "v1.13.0",
							},
						},
					},
				},
			},
			expectedOut: []clusterv1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-0",
						Namespace: "awesome-ns",
					},
					Spec: clusterv1.MachineSpec{
						Versions: clusterv1.MachineVersionInfo{
							Kubelet:      "v1.13.0",
							ControlPlane: "v1.13.0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-1",
						Namespace: "awesome-ns",
					},
					Spec: clusterv1.MachineSpec{
						Versions: clusterv1.MachineVersionInfo{
							Kubelet:      "v1.13.0",
							ControlPlane: "v1.13.0",
						},
					},
				}},
		},
		{
			name: "only 2 worker machines",
			input: &clusterv1.MachineList{
				Items: []clusterv1.Machine{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "worker-0",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet: "v1.13.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "worker-1",
							Namespace: "awesome-ns",
						},
						Spec: clusterv1.MachineSpec{
							Versions: clusterv1.MachineVersionInfo{
								Kubelet: "v1.13.0",
							},
						},
					},
				},
			},
			expectedOut: []clusterv1.Machine{},
		},
	}

	for _, tc := range testCases {
		actual := GetControlPlaneMachines(tc.input)
		if len(actual) != len(tc.expectedOut) {
			t.Fatalf("[%s] Unexpected number of controlplane machines returned. Got: %d, Want: %d", tc.name, len(actual), len(tc.expectedOut))
		}
		if len(tc.expectedOut) > 1 {
			for _, em := range tc.expectedOut {
				if !contains(actual, em) {
					t.Fatalf("[%s] Expected controlplane machine %q in namespace %q not found", tc.name, em.Name, em.Namespace)
				}
			}
		}
	}
}

func TestMachineEqual(t *testing.T) {
	testCases := []struct {
		name          string
		inM1          clusterv1.Machine
		inM2          clusterv1.Machine
		expectedEqual bool
	}{
		{
			name: "machines are equal",
			inM1: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "my-awesome-ns",
				},
			},
			inM2: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "my-awesome-ns",
				},
			},
			expectedEqual: true,
		},
		{
			name: "machines are not equal: names are different",
			inM1: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine 1",
					Namespace: "my-awesome-ns",
				},
			},
			inM2: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine 2",
					Namespace: "my-azureesome-ns",
				},
			},
			expectedEqual: false,
		},
		{
			name: "machines are not equal: namespace are different",
			inM1: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "my-awesome-ns",
				},
			},
			inM2: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine1",
					Namespace: "your-azureesome-ns",
				},
			},
			expectedEqual: false,
		},
	}

	for _, tc := range testCases {
		actualEqual := machinesEqual(&tc.inM1, &tc.inM2)
		if tc.expectedEqual {
			if !actualEqual {
				t.Fatalf("[%s] Expected Machine1 [Name:%q, Namespace:%q], Equal Machine2 [Name:%q, Namespace:%q]",
					tc.name, tc.inM1.Name, tc.inM1.Namespace, tc.inM2.Name, tc.inM2.Namespace)
			}
		} else {
			if actualEqual {
				t.Fatalf("[%s] Expected Machine1 [Name:%q, Namespace:%q], NOT Equal Machine2 [Name:%q, Namespace:%q]",
					tc.name, tc.inM1.Name, tc.inM1.Namespace, tc.inM2.Name, tc.inM2.Namespace)
			}
		}
	}
}

// TODO: Add immutable state change tests
/*
func TestImmutableStateChange(t *testing.T) {
	testCases := []struct {
		name        string
		machineSpec v1alpha1.AzureMachineProviderSpec
		instance    v1alpha1.VM
		// expected length of returned errors
		expected int
	}{
		{
			name: "instance type is unchanged",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				InstanceType: "t2.micro",
			},
			instance: v1alpha1.VM{
				Type: "t2.micro",
			},
			expected: 0,
		},
		{
			name: "instance type is changed",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				InstanceType: "m5.large",
			},
			instance: v1alpha1.VM{
				Type: "t2.micro",
			},
			expected: 1,
		},
		{
			name: "iam profile is unchanged",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				IAMInstanceProfile: "test-profile",
			},
			instance: v1alpha1.VM{
				IAMProfile: "test-profile",
			},
			expected: 0,
		},
		{
			name: "iam profile is changed",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				IAMInstanceProfile: "test-profile-updated",
			},
			instance: v1alpha1.VM{
				IAMProfile: "test-profile",
			},
			expected: 1,
		},
		{
			name: "keyname is unchanged",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				KeyName: "SSHKey",
			},
			instance: v1alpha1.VM{
				KeyName: azure.String("SSHKey"),
			},
			expected: 0,
		},
		{
			name: "keyname is changed",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				KeyName: "SSHKey2",
			},
			instance: v1alpha1.VM{
				KeyName: azure.String("SSHKey"),
			},
			expected: 1,
		},
		{
			name: "instance with public ip is unchanged",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				PublicIP: azure.Bool(true),
			},
			instance: v1alpha1.VM{
				// This IP chosen from RFC5737 TEST-NET-1
				PublicIP: azure.String("192.0.2.1"),
			},
			expected: 0,
		},
		{
			name: "instance with public ip is changed",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				PublicIP: azure.Bool(false),
			},
			instance: v1alpha1.VM{
				// This IP chosen from RFC5737 TEST-NET-1
				PublicIP: azure.String("192.0.2.1"),
			},
			expected: 1,
		},
		{
			name: "instance without public ip is unchanged",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				PublicIP: azure.Bool(false),
			},
			instance: v1alpha1.VM{
				PublicIP: azure.String(""),
			},
			expected: 0,
		},
		{
			name: "instance without public ip is changed",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				PublicIP: azure.Bool(true),
			},
			instance: v1alpha1.VM{
				PublicIP: azure.String(""),
			},
			expected: 1,
		},
		{
			name: "subnetid is unchanged",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				Subnet: &v1alpha1.AzureResourceReference{
					ID: azure.String("subnet-abcdef"),
				},
			},
			instance: v1alpha1.VM{
				SubnetID: "subnet-abcdef",
			},
			expected: 0,
		},
		{
			name: "subnetid is changed",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				Subnet: &v1alpha1.AzureResourceReference{
					ID: azure.String("subnet-123456"),
				},
			},
			instance: v1alpha1.VM{
				SubnetID: "subnet-abcdef",
			},
			expected: 1,
		},
		{
			name:        "root device size is omitted",
			machineSpec: v1alpha1.AzureMachineProviderSpec{},
			instance: v1alpha1.VM{
				// All instances have a root device size, even when we don't set one
				RootDeviceSize: 12,
			},
			expected: 0,
		},
		{
			name: "root device size is unchanged",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				RootDeviceSize: 12,
			},
			instance: v1alpha1.VM{
				RootDeviceSize: 12,
			},
			expected: 0,
		},
		{
			name: "root device size is changed",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				RootDeviceSize: 12,
			},
			instance: v1alpha1.VM{
				RootDeviceSize: 16,
			},
			expected: 1,
		},
		{
			name: "multiple immutable changes",
			machineSpec: v1alpha1.AzureMachineProviderSpec{
				IAMInstanceProfile: "test-profile-updated",
				PublicIP:           azure.Bool(false),
			},
			instance: v1alpha1.VM{
				IAMProfile: "test-profile",
				// This IP chosen from RFC5737 TEST-NET-1
				PublicIP: azure.String("192.0.2.1"),
			},
			expected: 2,
		},
	}

	testActuator := NewActuator(ActuatorParams{})

	for _, tc := range testCases {
		changed := len(testActuator.isMachineOutdated(&tc.machineSpec, &tc.instance))

		if tc.expected != changed {
			t.Fatalf("[%s] Expected MachineSpec [%+v], NOT Equal Instance [%+v]",
				tc.name, tc.machineSpec, tc.instance)
		}
	}
}
*/

func TestIsNodeJoin(t *testing.T) {
	tests := []struct {
		name          string
		cluster       *clusterv1.Cluster
		machine       *clusterv1.Machine
		actualAcquire bool
		expectJoin    bool
		expectError   bool
	}{
		{
			name: "control plane already ready",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1alpha1.AnnotationControlPlaneReady: v1alpha1.ValueReady},
				},
			},
			machine:     &clusterv1.Machine{},
			expectJoin:  true,
			expectError: false,
		},
		{
			name:        "not a control plane machine",
			cluster:     &clusterv1.Cluster{},
			machine:     &clusterv1.Machine{},
			expectJoin:  true,
			expectError: true,
		},
		{
			name:    "able to acquire lock",
			cluster: &clusterv1.Cluster{},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"set": "controlplane"},
				},
			},
			actualAcquire: true,
			expectJoin:    false,
			expectError:   false,
		},
		{
			name:    "unable to acquire lock",
			cluster: &clusterv1.Cluster{},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"set": "controlplane"},
				},
			},
			actualAcquire: false,
			expectError:   true,
			expectJoin:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log := klogr.New()

			a := &Actuator{
				controlPlaneInitLocker: &fakeControlPlaneInitLocker{succeed: tc.actualAcquire},
			}

			actual, err := a.isNodeJoin(log, tc.cluster, tc.machine)
			if tc.expectError && err == nil {
				t.Fatal("expected error but got nil")
			} else if !tc.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.expectJoin != actual {
				t.Errorf("join: expected %t, got %t", tc.expectJoin, actual)
			}
		})
	}
}

type fakeControlPlaneInitLocker struct {
	succeed bool
}

func (f *fakeControlPlaneInitLocker) Acquire(cluster *clusterv1.Cluster) bool {
	return f.succeed
}

func newClusterProviderSpec() v1alpha1.AzureClusterProviderSpec {
	return v1alpha1.AzureClusterProviderSpec{
		ResourceGroup: "resource-group-test",
		Location:      "southcentralus",
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
				Kubelet:      "1.15.3",
				ControlPlane: "1.15.3",
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
			Name:      "cluster-test",
			Namespace: "dummy-namespace",
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
		Cluster: newCluster(t),
		ClusterConfig: &v1alpha1.AzureClusterProviderSpec{
			ResourceGroup: "dummyResourceGroup",
			Location:      "dummyLocation",
		},
		ClusterStatus: &v1alpha1.AzureClusterProviderStatus{},
		Context:       context.Background(),
	}

	if scope.Logger == nil {
		scope.Logger = klogr.New().WithName("default-logger")
	}

	scope.Network().APIServerIP.DNSName = "fakecluster.example.com"

	labels := make(map[string]string)
	labels["set"] = label
	machineConfig := v1alpha1.AzureMachineProviderSpec{}
	m := newMachine(t, machineConfig, labels)
	c := fake.NewSimpleClientset(m).ClusterV1alpha1()
	return &actuators.MachineScope{
		Scope:         scope,
		Machine:       m,
		MachineClient: c.Machines(scope.Cluster.Namespace),
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
	fakeScope := newFakeScope(t, v1alpha1.ControlPlane)
	fakeReconciler := newFakeReconcilerWithScope(t, fakeScope)

	certSvc := certificates.NewService(fakeScope.Scope)
	certSvc.Reconcile(fakeScope.Context, nil)

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err != nil {
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

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err == nil {
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

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err == nil {
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

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err == nil {
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

func TestReconcileVMSucceededState(t *testing.T) {
	fakeScope := newFakeScope(t, v1alpha1.ControlPlane)
	fakeReconciler := newFakeReconcilerWithScope(t, fakeScope)
	fakeVMService := &FakeVMService{
		Name:              "machine-test",
		ID:                "machine-test-ID",
		ProvisioningState: "Succeeded",
	}
	fakeReconciler.virtualMachinesSvc = fakeVMService

	certSvc := certificates.NewService(fakeScope.Scope)
	certSvc.Reconcile(fakeScope.Context, nil)

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err != nil {
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

	certSvc := certificates.NewService(fakeScope.Scope)
	certSvc.Reconcile(fakeScope.Context, nil)

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	fakeReconciler.availabilityZonesSvc = &FakeAvailabilityZonesService{
		zonesResponse: []string{},
	}

	fakeReconciler.virtualMachinesSvc = &FakeVMCheckZonesService{
		checkZones: []string{},
	}

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	fakeReconciler.availabilityZonesSvc = &FakeAvailabilityZonesService{
		zonesResponse: []string{"2"},
	}

	fakeReconciler.virtualMachinesSvc = &FakeVMCheckZonesService{
		checkZones: []string{"3"},
	}

	if err := fakeReconciler.Create(context.Background(), "fake-bootstrap-token"); err == nil {
		t.Errorf("expected create to fail due to zone mismatch")
	}
}
