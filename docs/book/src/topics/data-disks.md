# Data Disks

This document describes how to specify data disks to be provisioned and attached to VMs provisioned in Azure. 

## Azure Machine Data Disks

Azure Machines support optionally specifying a list of data disks to be attached to the virtual machine. Each data disk must have:
 - `nameSuffix` - the name suffix of the disk to be created. Each disk will be named `<machineName>_<nameSuffix>` to ensure uniqueness. 
 - `diskSizeGB` - the disk size in GB.
 - `managedDisk` - (optional) the managed disk for a VM (see below)
 - `lun` - the logical unit number (see below)

### Managed Disk Options

See [Introduction to Azure managed disks](https://docs.microsoft.com/en-us/azure/virtual-machines/managed-disks-overview) for more information.
 
### Disk LUN
 
 The LUN specifies the logical unit number of the data disk, between 0 and 63. Its value is used to identify data disks within the VM and therefore must be unique for each data disk attached to a VM.
 
 When adding data disks to a Linux VM, you may encounter errors if a disk does not exist at LUN 0. It is therefore recommended to ensure that the first data disk specified is always added at LUN 0.
 
 See [Attaching a disk to a Linux VM on Azure](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/add-disk) for more information.
 
 > IMPORTANT! The `lun` specified in the AzureMachine Spec must match the LUN used to refer to the device in Kubeadm diskSetup. See below for an example.

### Ultra disk support for data disks
If we use StorageAccountType as `UltraSSD_LRS` in Managed Disks, the ultra disk support will be enabled for the region and zone which supports the `UltraSSDAvailable` capability.

To check all available vm-sizes in a given region which supports availability zone that has the `UltraSSDAvailable` capability supported, execute following using Azure CLI:
```bash
az vm list-skus -l <location> -z -s <VM-size>
```

Provided that the chosen region and zone support Ultra disks, Azure Machine objects having Ultra disks specified as Data disks will have their virtual machines created with the `AdditionalCapabilities.UltraSSDEnabled` additional capability set to `true`. This capability can also be manually set on the Azure Machine spec and will override the automatically chosen value (if any).

When the chosen StorageAccountType is `UltraSSD_LRS`, caching is not supported for the disk and the corresponding `cachingType` field must be set to `None`. In this configuration, if no value is set, `cachingType` will be defaulted to `None`.

See [Ultra disk](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#ultra-disk) for ultra disk performance and GA scope.

### Ultra disk support for Persistent Volumes
First, to check all available vm-sizes in a given region which supports availability zone that has the `UltraSSDAvailable` capability supported, execute following using Azure CLI:
```bash
az vm list-skus -l <location> -z -s <VM-size>
```

Provided that the chosen region and zone support Ultra disks, Ultra disk based Persistent Volumes can be attached to Pods scheduled on specific Azure Machines, provided that the spec field `.spec.additionalCapabilities.ultraSSDEnabled` on those Machines has been set to `true`.
NOTE: A misconfiguration or lack this field on the targeted Node's Machine will result in the Pod using the PV be unable to reach the Running Phase.

See [Use ultra disks dynamically with a storage class](https://docs.microsoft.com/en-us/azure/aks/use-ultra-disks#use-ultra-disks-dynamically-with-a-storage-class) for more information on how to configure an Ultra disk based StorageClass and PersistentVolumeClaim.

See [Ultra disk](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#ultra-disk) for ultra disk performance and GA scope.

## Configuring partitions, file systems and mounts 

`KubeadmConfig` makes it easy to partition, format, and mount your data disk so your Linux VM can use it. Use the `diskSetup` and `mounts` options to describe partitions, file systems and mounts.

You may refer to your device as `/dev/disk/azure/scsi1/lun<i>` where `<i>` is the LUN.

See [cloud-init documentation](https://cloudinit.readthedocs.io/en/latest/reference/modules.html#disk-setup) for more information about cloud-init disk setup.


## Example

The below example shows how to create and attach a custom data disk "my_disk" at LUN 1 for every control plane machine, in addition to the etcd data disk. 
NOTE: the same can be applied to worker machines.

````yaml
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
    [...]
    diskSetup:
      partitions:
        - device: /dev/disk/azure/scsi1/lun0
          tableType: gpt
          layout: true
          overwrite: false
        - device: /dev/disk/azure/scsi1/lun1
          tableType: gpt
          layout: true
          overwrite: false
      filesystems:
        - label: etcd_disk
          filesystem: ext4
          device: /dev/disk/azure/scsi1/lun0
          extraOpts:
            - "-E"
            - "lazy_itable_init=1,lazy_journal_init=1"
        - label: ephemeral0
          filesystem: ext4
          device: ephemeral0.1
          replaceFS: ntfs
        - label: my_disk
          filesystem: ext4
          device: /dev/disk/azure/scsi1/lun1
    mounts:
      - - LABEL=etcd_disk
        - /var/lib/etcddisk
      - - LABEL=my_disk
        - /var/lib/mydir
---
kind: AzureMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      [...]
      dataDisks:
        - nameSuffix: etcddisk
          diskSizeGB: 256
          managedDisk:
            storageAccountType: Standard_LRS
          lun: 0
        - nameSuffix: mydisk
          diskSizeGB: 128
          lun: 1
````
