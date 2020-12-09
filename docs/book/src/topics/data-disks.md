# Data Disks

This document describes how to specify data disks to be provisioned and attached to VMs provisioned in Azure. 

## Azure Machine Data Disks

Azure Machines support optionally specifying a list of data disks to be attached to the virtual machine. Each data disk must have:
 - `nameSuffix` - the name suffix of the disk to be created. Each disk will be named `<machineName>_<nameSuffix>` to ensure uniqueness. 
 - `diskSizeGB` - the disk size in GB.
 - `lun` - the logical unit number (see below)
 
### Disk LUN
 
 The LUN specifies the logical unit number of the data disk, between 0 and 63. Its value is used to identify data disks within the VM and therefore must be unique for each data disk attached to a VM.
 
 When adding data disks to a Linux VM, you may encounter errors if a disk does not exist at LUN 0. It is therefore recommended to ensure that the first data disk specified is always added at LUN 0.
 
 See [Attaching a disk to a Linux VM on Azure](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/add-disk) for more information.
 
 > IMPORTANT! The `lun` specified in the AzureMachine Spec must match the LUN used to refer to the device in Kubeadm diskSetup. See below for an example.

## Configuring partitions, file systems and mounts 

`KubeadmConfig` makes it easy to partition, format, and mount your data disk so your Linux VM can use it. Use the `diskSetup` and `mounts` options to describe partitions, file systems and mounts.

You may refer to your device as `/dev/disk/azure/scsi1/lun<i>` where `<i>` is the LUN.

See [cloud-init documentation](https://cloudinit.readthedocs.io/en/latest/topics/modules.html#disk-setup) for more information about cloud-init disk setup.


## Example

The below example shows how to create and attach a custom data disk "my_disk" at LUN 1 for every control plane machine, in addition to the etcd data disk. 
NOTE: the same can be applied to worker machines.

````yaml
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
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
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      [...]
      dataDisks:
        - nameSuffix: etcddisk
          diskSizeGB: 256
          lun: 0
        - nameSuffix: mydisk
          diskSizeGB: 128
          lun: 1
````