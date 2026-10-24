//go:build e2e

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

package e2e

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type fakeKineHardware struct {
	vms       map[string]armcompute.VirtualMachine
	disks     map[string]armcompute.Disk
	updates   []string
	updateErr error
	stale     bool
}

func (f *fakeKineHardware) virtualMachine(_ context.Context, id *arm.ResourceID) (armcompute.VirtualMachine, error) {
	return f.vms[id.String()], nil
}

func (f *fakeKineHardware) disk(_ context.Context, id *arm.ResourceID) (armcompute.Disk, error) {
	return f.disks[id.String()], nil
}

func (f *fakeKineHardware) updateDisk(_ context.Context, id *arm.ResourceID, iops, mbps int64) error {
	f.updates = append(f.updates, id.String())
	if f.updateErr != nil {
		return f.updateErr
	}
	if !f.stale {
		disk := f.disks[id.String()]
		disk.Properties.DiskIOPSReadWrite = ptr.To(iops)
		disk.Properties.DiskMBpsReadWrite = ptr.To(mbps)
		f.disks[id.String()] = disk
	}
	return nil
}

func kineHardwareFixture() (kineHardwareConfig, corev1.Node, *fakeKineHardware) {
	config := kineHardwareConfig{SubscriptionID: "sub", ClusterName: "test-cluster", Size: "Standard_E96bs_v5", Location: "westus2", Zone: "2", DiskSizeGB: 512, IOPS: 120000, MBps: 4000}
	vmID := "/subscriptions/sub/resourceGroups/test-cluster/providers/Microsoft.Compute/virtualMachines/cp-1"
	diskID := "/subscriptions/sub/resourceGroups/test-cluster/providers/Microsoft.Compute/disks/cp-1_etcddisk"
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "cp-1"}, Spec: corev1.NodeSpec{ProviderID: "azure://" + vmID}}
	vm := armcompute.VirtualMachine{
		ID: ptr.To(vmID), Location: ptr.To(config.Location), Zones: []*string{ptr.To(config.Zone)},
		Tags: map[string]*string{infrav1.ClusterTagKey(config.ClusterName): ptr.To("owned"), infrav1.NameAzureClusterAPIRole: ptr.To(infrav1.ControlPlane)},
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{VMSize: ptr.To(armcompute.VirtualMachineSizeTypes(config.Size))},
			StorageProfile: &armcompute.StorageProfile{
				DiskControllerType: ptr.To(armcompute.DiskControllerTypesSCSI),
				OSDisk: &armcompute.OSDisk{
					DiskSizeGB: ptr.To(int32(128)), Caching: ptr.To(armcompute.CachingTypesReadWrite),
					ManagedDisk: &armcompute.ManagedDiskParameters{StorageAccountType: ptr.To(armcompute.StorageAccountTypesPremiumLRS)},
				},
				DataDisks: []*armcompute.DataDisk{{
					Lun: ptr.To(int32(0)), Caching: ptr.To(armcompute.CachingTypesNone),
					ManagedDisk: &armcompute.ManagedDiskParameters{ID: ptr.To(diskID)},
				}},
			},
		},
	}
	disk := armcompute.Disk{
		ID: ptr.To(diskID), ManagedBy: ptr.To(vmID), Location: ptr.To(config.Location), Zones: []*string{ptr.To(config.Zone)},
		SKU: &armcompute.DiskSKU{Name: ptr.To(armcompute.DiskStorageAccountTypesUltraSSDLRS)},
		Properties: &armcompute.DiskProperties{
			DiskSizeGB: ptr.To(int32(512)), DiskIOPSReadWrite: ptr.To(int64(1000)), DiskMBpsReadWrite: ptr.To(int64(125)),
			ProvisioningState: ptr.To("Succeeded"),
		},
	}
	return config, node, &fakeKineHardware{vms: map[string]armcompute.VirtualMachine{vmID: vm}, disks: map[string]armcompute.Disk{diskID: disk}}
}

func TestKineControlPlaneDiskTuning(t *testing.T) {
	for _, mode := range []string{"update", "already-matched", "update-fails", "readback-mismatch"} {
		t.Run(mode, func(t *testing.T) {
			config, node, azure := kineHardwareFixture()
			switch mode {
			case "already-matched":
				for _, disk := range azure.disks {
					disk.Properties.DiskIOPSReadWrite = ptr.To(config.IOPS)
					disk.Properties.DiskMBpsReadWrite = ptr.To(config.MBps)
				}
			case "update-fails":
				azure.updateErr = errors.New("Azure update failed")
			case "readback-mismatch":
				azure.stale = true
			}
			ctx := t.Context()
			if mode == "readback-mismatch" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 20*time.Millisecond)
				defer cancel()
			}
			records, err := configureKineControlPlaneDisks(ctx, azure, config, []corev1.Node{node})
			wantOK := mode == "update" || mode == "already-matched"
			if (err == nil) != wantOK {
				t.Fatalf("unexpected tuning outcome: records=%+v err=%v", records, err)
			}
			if wantOK && (len(records) != 1 || !records[0].Verified || records[0].IOPS != config.IOPS || records[0].MBps != config.MBps) {
				t.Fatalf("hardware must be read back and verified before success: %+v", records)
			}
			wantUpdates := 1
			if mode == "already-matched" {
				wantUpdates = 0
			}
			if len(azure.updates) != wantUpdates {
				t.Fatalf("unexpected updates: %v", azure.updates)
			}
		})
	}
}

func TestKineControlPlaneDiskOwnership(t *testing.T) {
	for _, mode := range []string{"subscription", "resource-group", "resource-type", "unowned-vm", "worker-vm", "wrong-size", "wrong-zone", "nvme", "cached", "wrong-lun", "foreign-disk", "premium-disk", "disk-size", "shared-disk"} {
		t.Run(mode, func(t *testing.T) {
			config, node, azure := kineHardwareFixture()
			vmID := strings.TrimPrefix(node.Spec.ProviderID, "azure://")
			vm := azure.vms[vmID]
			diskID := *vm.Properties.StorageProfile.DataDisks[0].ManagedDisk.ID
			disk := azure.disks[diskID]
			switch mode {
			case "subscription":
				node.Spec.ProviderID = strings.Replace(node.Spec.ProviderID, "/sub/", "/other/", 1)
			case "resource-group":
				node.Spec.ProviderID = strings.Replace(node.Spec.ProviderID, "/test-cluster/", "/other/", 1)
			case "resource-type":
				node.Spec.ProviderID = strings.Replace(node.Spec.ProviderID, "/virtualMachines/", "/disks/", 1)
			case "unowned-vm":
				delete(vm.Tags, infrav1.ClusterTagKey(config.ClusterName))
			case "worker-vm":
				vm.Tags[infrav1.NameAzureClusterAPIRole] = ptr.To("node")
			case "wrong-size":
				vm.Properties.HardwareProfile.VMSize = ptr.To(armcompute.VirtualMachineSizeTypes("other"))
			case "wrong-zone":
				vm.Zones = []*string{ptr.To("1")}
			case "nvme":
				vm.Properties.StorageProfile.DiskControllerType = ptr.To(armcompute.DiskControllerTypesNVMe)
			case "cached":
				vm.Properties.StorageProfile.DataDisks[0].Caching = ptr.To(armcompute.CachingTypesReadOnly)
			case "wrong-lun":
				vm.Properties.StorageProfile.DataDisks[0].Lun = ptr.To(int32(1))
			case "foreign-disk":
				disk.ManagedBy = ptr.To(vmID + "-other")
			case "premium-disk":
				disk.SKU.Name = ptr.To(armcompute.DiskStorageAccountTypesPremiumLRS)
			case "disk-size":
				disk.Properties.DiskSizeGB = ptr.To(int32(256))
			case "shared-disk":
				disk.Properties.MaxShares = ptr.To(int32(2))
			}
			azure.vms[vmID], azure.disks[diskID] = vm, disk
			if _, err := configureKineControlPlaneDisks(t.Context(), azure, config, []corev1.Node{node}); err == nil {
				t.Fatal("unmatched or unowned resources must be rejected")
			}
			if len(azure.updates) != 0 {
				t.Fatal("invalid targets must be rejected before any Azure mutation")
			}
		})
	}
	config, node, azure := kineHardwareFixture()
	other := node.DeepCopy()
	other.Name = "unexpected"
	other.Spec.ProviderID = "azure:///subscriptions/other/resourceGroups/foreign/providers/Microsoft.Compute/virtualMachines/vm"
	if _, err := configureKineControlPlaneDisks(t.Context(), azure, config, []corev1.Node{node, *other}); err == nil || len(azure.updates) != 0 {
		t.Fatal("all control-plane targets must be validated before updating the first disk")
	}
}

func TestKubeadmEtcdImage(t *testing.T) {
	nodes := []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "cp-1"}}}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "etcd-cp-1", Namespace: "kube-system", Annotations: map[string]string{"kubernetes.io/config.mirror": "hash"}},
		Spec:       corev1.PodSpec{NodeName: "cp-1", Containers: []corev1.Container{{Name: "etcd", Image: "registry.k8s.io/etcd:3.6.6-0"}}},
	}
	image, err := kubeadmEtcdImage([]corev1.Pod{pod}, nodes)
	if err != nil || image != pod.Spec.Containers[0].Image {
		t.Fatalf("record the real kubeadm image: %s %v", image, err)
	}
	if _, err := kubeadmEtcdImage(nil, nodes); err == nil {
		t.Fatal("missing static Pods must not produce a success-shaped version")
	}
	pod.Annotations = nil
	if _, err := kubeadmEtcdImage([]corev1.Pod{pod}, nodes); err == nil {
		t.Fatal("non-static etcd Pods must be rejected")
	}
}
