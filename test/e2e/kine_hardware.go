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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type kineHardwareConfig struct {
	SubscriptionID string `json:"subscriptionID"`
	ClusterName    string `json:"clusterName"`
	Size           string `json:"vmSize"`
	Location       string `json:"location"`
	Zone           string `json:"zone"`
	DiskSizeGB     int64  `json:"diskSizeGB"`
	IOPS           int64  `json:"iops"`
	MBps           int64  `json:"mbps"`
}

type kineControlPlaneDisk struct {
	Node     string `json:"node"`
	VMID     string `json:"vmID"`
	DiskID   string `json:"diskID"`
	IOPS     int64  `json:"iops"`
	MBps     int64  `json:"mbps"`
	Updated  bool   `json:"updated"`
	Verified bool   `json:"verified"`
}

type kineHardwareClient interface {
	virtualMachine(context.Context, *arm.ResourceID) (armcompute.VirtualMachine, error)
	disk(context.Context, *arm.ResourceID) (armcompute.Disk, error)
	updateDisk(context.Context, *arm.ResourceID, int64, int64) error
}

type kineAzureHardwareClient struct {
	vms   *armcompute.VirtualMachinesClient
	disks *armcompute.DisksClient
}

func (c kineAzureHardwareClient) virtualMachine(ctx context.Context, id *arm.ResourceID) (armcompute.VirtualMachine, error) {
	response, err := c.vms.Get(ctx, id.ResourceGroupName, id.Name, nil)
	return response.VirtualMachine, err
}

func (c kineAzureHardwareClient) disk(ctx context.Context, id *arm.ResourceID) (armcompute.Disk, error) {
	response, err := c.disks.Get(ctx, id.ResourceGroupName, id.Name, nil)
	return response.Disk, err
}

func (c kineAzureHardwareClient) updateDisk(ctx context.Context, id *arm.ResourceID, iops, mbps int64) error {
	poller, err := c.disks.BeginUpdate(ctx, id.ResourceGroupName, id.Name, armcompute.DiskUpdate{
		Properties: &armcompute.DiskUpdateProperties{
			DiskIOPSReadWrite: ptr.To(iops),
			DiskMBpsReadWrite: ptr.To(mbps),
		},
	}, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func prepareKineControlPlane(ctx context.Context, workload framework.ClusterProxy, clusterName string, controlPlanes int64, reportDir string) error {
	config := kineHardwareConfig{
		SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"), ClusterName: clusterName,
		Size: os.Getenv("KINE_VM_SIZE"), Location: os.Getenv("AZURE_LOCATION"), Zone: os.Getenv("KINE_ZONE"),
	}
	for name, target := range map[string]*int64{
		"KINE_DISK_SIZE_GB": &config.DiskSizeGB,
		"KINE_DISK_IOPS":    &config.IOPS,
		"KINE_DISK_MBPS":    &config.MBps,
	} {
		value, err := strconv.ParseInt(os.Getenv(name), 10, 64)
		if err != nil || value <= 0 {
			return fmt.Errorf("%s must be a positive integer for matched control-plane storage", name)
		}
		*target = value
	}
	if config.SubscriptionID == "" || config.ClusterName == "" || config.Size == "" || config.Location == "" || config.Zone == "" {
		return fmt.Errorf("missing matched control-plane hardware configuration")
	}
	ctx, cancel := context.WithTimeout(ctx, 70*time.Minute)
	defer cancel()
	nodes, err := workload.GetClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane"})
	if err != nil {
		return fmt.Errorf("list control-plane nodes for storage setup: %w", err)
	}
	if int64(len(nodes.Items)) != controlPlanes || controlPlanes < 1 {
		return fmt.Errorf("expected %d control-plane nodes for storage setup, found %d", controlPlanes, len(nodes.Items))
	}
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("create Azure credential for fixture disk setup: %w", err)
	}
	vms, err := armcompute.NewVirtualMachinesClient(config.SubscriptionID, credential, nil)
	if err != nil {
		return err
	}
	disks, err := armcompute.NewDisksClient(config.SubscriptionID, credential, nil)
	if err != nil {
		return err
	}
	records, err := configureKineControlPlaneDisks(ctx, kineAzureHardwareClient{vms: vms, disks: disks}, config, nodes.Items)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(reportDir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Requested kineHardwareConfig     `json:"requested"`
		Disks     []kineControlPlaneDisk `json:"disks"`
	}{config, records}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(reportDir, "control-plane-hardware.json"), data, 0o600); err != nil {
		return err
	}
	if os.Getenv("KINE_BACKEND") == "kubeadm-etcd" {
		pods, err := workload.GetClientSet().CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{LabelSelector: "component=etcd,tier=control-plane"})
		if err != nil {
			return fmt.Errorf("discover kubeadm etcd image: %w", err)
		}
		image, err := kubeadmEtcdImage(pods.Items, nodes.Items)
		if err != nil {
			return err
		}
		if err := os.Setenv("KINE_KUBEADM_ETCD_IMAGE", image); err != nil {
			return err
		}
		Logf("Kubeadm local etcd image: %s", image)
	}
	return nil
}

func configureKineControlPlaneDisks(ctx context.Context, azure kineHardwareClient, config kineHardwareConfig, nodes []corev1.Node) ([]kineControlPlaneDisk, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no control-plane nodes available for matched disk setup")
	}
	records := make([]kineControlPlaneDisk, 0, len(nodes))
	seen := map[string]bool{}
	// Validate every target before making any updates.
	for _, node := range nodes {
		vmID, err := kineFixtureResourceID(node.Spec.ProviderID, config, "virtualMachines")
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", node.Name, err)
		}
		vm, err := azure.virtualMachine(ctx, vmID)
		if err != nil {
			return nil, fmt.Errorf("get control-plane VM %s: %w", vmID, err)
		}
		if !strings.EqualFold(ptr.Deref(vm.ID, ""), vmID.String()) ||
			ptr.Deref(vm.Tags[infrav1.ClusterTagKey(config.ClusterName)], "") != string(infrav1.ResourceLifecycleOwned) ||
			ptr.Deref(vm.Tags[infrav1.NameAzureClusterAPIRole], "") != infrav1.ControlPlane {
			return nil, fmt.Errorf("VM %s is not an owned control-plane VM of this fixture", vmID)
		}
		p := vm.Properties
		if p == nil || p.HardwareProfile == nil || p.HardwareProfile.VMSize == nil ||
			string(*p.HardwareProfile.VMSize) != config.Size || !strings.EqualFold(ptr.Deref(vm.Location, ""), config.Location) ||
			len(vm.Zones) != 1 || ptr.Deref(vm.Zones[0], "") != config.Zone {
			return nil, fmt.Errorf("VM %s does not match the shared size/location/zone", vmID)
		}
		if p.StorageProfile == nil || p.StorageProfile.DiskControllerType == nil ||
			string(*p.StorageProfile.DiskControllerType) != "SCSI" || len(p.StorageProfile.DataDisks) != 1 {
			return nil, fmt.Errorf("VM %s must have a SCSI controller and exactly one data disk", vmID)
		}
		osDisk := p.StorageProfile.OSDisk
		if osDisk == nil || ptr.Deref(osDisk.DiskSizeGB, 0) != 128 ||
			osDisk.Caching == nil || *osDisk.Caching != armcompute.CachingTypesReadWrite ||
			osDisk.ManagedDisk == nil || osDisk.ManagedDisk.StorageAccountType == nil ||
			*osDisk.ManagedDisk.StorageAccountType != armcompute.StorageAccountTypesPremiumLRS {
			return nil, fmt.Errorf("VM %s must use the shared 128-GiB Premium_LRS ReadWrite-cached OS disk", vmID)
		}
		attachment := p.StorageProfile.DataDisks[0]
		if attachment == nil || attachment.Lun == nil || *attachment.Lun != 0 ||
			attachment.Caching == nil || *attachment.Caching != armcompute.CachingTypesNone ||
			attachment.ManagedDisk == nil || attachment.ManagedDisk.ID == nil {
			return nil, fmt.Errorf("VM %s must have one uncached managed disk at LUN 0", vmID)
		}
		diskID, err := kineFixtureResourceID(*attachment.ManagedDisk.ID, config, "disks")
		if err != nil {
			return nil, err
		}
		if seen[strings.ToLower(diskID.String())] {
			return nil, fmt.Errorf("disk %s is shared by multiple control-plane nodes", diskID)
		}
		seen[strings.ToLower(diskID.String())] = true
		disk, err := azure.disk(ctx, diskID)
		if err != nil {
			return nil, fmt.Errorf("get control-plane disk %s: %w", diskID, err)
		}
		if err := validateKineControlPlaneDisk(disk, diskID, vmID, config); err != nil {
			return nil, err
		}
		records = append(records, kineControlPlaneDisk{
			Node: node.Name, VMID: vmID.String(), DiskID: diskID.String(),
			IOPS: ptr.Deref(disk.Properties.DiskIOPSReadWrite, 0), MBps: ptr.Deref(disk.Properties.DiskMBpsReadWrite, 0),
		})
	}
	for i := range records {
		record := &records[i]
		diskID, err := kineFixtureResourceID(record.DiskID, config, "disks")
		if err != nil {
			return nil, err
		}
		vmID, err := kineFixtureResourceID(record.VMID, config, "virtualMachines")
		if err != nil {
			return nil, err
		}
		if record.IOPS != config.IOPS || record.MBps != config.MBps {
			Logf("Setting owned control-plane disk %s to %d IOPS and %d MB/s before workload", diskID, config.IOPS, config.MBps)
			if err := azure.updateDisk(ctx, diskID, config.IOPS, config.MBps); err != nil {
				return nil, fmt.Errorf("update owned control-plane disk %s: %w", diskID, err)
			}
			record.Updated = true
		}
		state := ""
		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			disk, err := azure.disk(ctx, diskID)
			if err != nil {
				return false, err
			}
			if err := validateKineControlPlaneDisk(disk, diskID, vmID, config); err != nil {
				return false, err
			}
			state = ptr.Deref(disk.Properties.ProvisioningState, "")
			record.IOPS = ptr.Deref(disk.Properties.DiskIOPSReadWrite, 0)
			record.MBps = ptr.Deref(disk.Properties.DiskMBpsReadWrite, 0)
			if state == "Failed" || state == "Canceled" {
				return false, fmt.Errorf("disk update entered state %s", state)
			}
			return state == "Succeeded" && record.IOPS == config.IOPS && record.MBps == config.MBps, nil
		})
		if err != nil {
			return nil, fmt.Errorf("verify disk %s before workload: expected %d IOPS/%d MBps, observed state=%q %d IOPS/%d MBps: %w",
				diskID, config.IOPS, config.MBps, state, record.IOPS, record.MBps, err)
		}
		record.IOPS, record.MBps, record.Verified = config.IOPS, config.MBps, true
	}
	return records, nil
}

func kineFixtureResourceID(value string, config kineHardwareConfig, resourceType string) (*arm.ResourceID, error) {
	id, err := arm.ParseResourceID(strings.TrimPrefix(value, "azure://"))
	if err != nil {
		return nil, fmt.Errorf("invalid Azure resource ID: %w", err)
	}
	if !strings.EqualFold(id.SubscriptionID, config.SubscriptionID) || !strings.EqualFold(id.ResourceGroupName, config.ClusterName) ||
		!strings.EqualFold(id.ResourceType.String(), "Microsoft.Compute/"+resourceType) {
		return nil, fmt.Errorf("resource %s is outside this fixture's subscription, resource group, or expected resource type", id)
	}
	return id, nil
}

func validateKineControlPlaneDisk(disk armcompute.Disk, diskID, vmID *arm.ResourceID, config kineHardwareConfig) error {
	p := disk.Properties
	if !strings.EqualFold(ptr.Deref(disk.ID, ""), diskID.String()) || !strings.EqualFold(ptr.Deref(disk.ManagedBy, ""), vmID.String()) ||
		disk.SKU == nil || disk.SKU.Name == nil || string(*disk.SKU.Name) != "UltraSSD_LRS" ||
		p == nil || p.DiskSizeGB == nil || int64(*p.DiskSizeGB) != config.DiskSizeGB ||
		(p.MaxShares != nil && *p.MaxShares > 1) ||
		!strings.EqualFold(ptr.Deref(disk.Location, ""), config.Location) ||
		len(disk.Zones) != 1 || ptr.Deref(disk.Zones[0], "") != config.Zone {
		return fmt.Errorf("disk %s is not the expected dedicated Ultra Disk attached to VM %s", diskID, vmID)
	}
	return nil
}

func kubeadmEtcdImage(pods []corev1.Pod, nodes []corev1.Node) (string, error) {
	remaining := map[string]bool{}
	for _, node := range nodes {
		remaining[node.Name] = true
	}
	images := map[string]bool{}
	for _, pod := range pods {
		if !remaining[pod.Spec.NodeName] || pod.Annotations["kubernetes.io/config.mirror"] == "" {
			return "", fmt.Errorf("unexpected non-static or duplicate etcd Pod %s/%s", pod.Namespace, pod.Name)
		}
		image := ""
		for _, container := range pod.Spec.Containers {
			if container.Name == "etcd" {
				image = container.Image
			}
		}
		if image == "" {
			return "", fmt.Errorf("etcd Pod %s/%s has no etcd image", pod.Namespace, pod.Name)
		}
		delete(remaining, pod.Spec.NodeName)
		images[image] = true
	}
	if len(remaining) != 0 || len(images) == 0 {
		return "", fmt.Errorf("missing kubeadm etcd Pods for control-plane nodes: %v", remaining)
	}
	result := make([]string, 0, len(images))
	for image := range images {
		result = append(result, image)
	}
	sort.Strings(result)
	return strings.Join(result, ","), nil
}
