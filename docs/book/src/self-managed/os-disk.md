# OS Disk

This document describes how to configure the OS disk for VMs provisioned in Azure. 

### Managed Disk Options

### Storage Account Type

By default, Azure will pick the supported storage account type for your AzureMachine based on the specified VM size. If you'd like to specify a specific storage type, you can do so by specifying a `storageAccountType`:

```yaml
        managedDisk:
          storageAccountType: Premium_LRS
```

Supported values are `Premium_LRS`, `Standard_LRS`, and `StandardSSDLRS`. Note that `UltraSSD_LRS` can only be used with data disks, it cannot be used with OS Disk.

Also, note that not all Azure VM sizes support Premium storage. To learn more about which sizes are premium storage-compatible, see [Sizes for virtual machines in Azure](https://learn.microsoft.com/azure/virtual-machines/sizes). 

See [Azure documentation on disk types](https://learn.microsoft.com/azure/virtual-machines/disks-types) to learn more about the different storage types.

See [Introduction to Azure managed disks](https://learn.microsoft.com/azure/virtual-machines/managed-disks-overview) for more information on managed disks.

If the optional field `diskSizeGB` is not provided, it will default to 30GB.

## Ephemeral OS

Ephemeral OS uses local VM storage for changes to the OS disk.
Storage devices local to the VM host will not be bound by normal managed
disk SKU limits. Instead they will always be capable of saturating the
VM level limits. This can significantly improve performance on the OS
disk. Ephemeral storage used for the OS will not persist between
maintenance events and VM redeployments. This is ideal for stateless
base OS disks, where any stateful data is kept elsewhere.

There are a few kinds of local storage devices available on Azure VMs.
Each VM size will have a different combination. For example, some sizes
support premium storage caching, some sizes have a temp disk while
others do not, and some sizes have local nvme devices with direct
access. Ephemeral OS uses the cache for the VM size, if one exists.
Otherwise it will try to use the temp disk if the VM has one. These are
the only supported options, and we do not expose the ability to manually
choose between these two disks (the default behavior is typically most
desirable). This corresponds to the `placement` property in the Azure
Compute REST API.

See [the Azure documentation](https://learn.microsoft.com/azure/virtual-machines/linux/ephemeral-os-disks) for full details.

## Azure Machine DiffDiskSettings

Azure Machines support optionally specifying a field called `diffDiskSettings`. This mirrors the Azure Compute REST API.

When `diffDiskSettings.option` is set to `Local`, ephemeral OS will be enabled. We use the API shape provided by compute directly as they expose other options, although this is the main one relevant at this time.

## Known Limitations

Not all SKU sizes support ephemeral OS. CAPZ will query Azure's resource
SKUs API to check if the requested VM size supports ephemeral OS. If
not, the azuremachine controller will log an event with the
corresponding error on the AzureMachine object.

## Example

The below example shows how to enable ephemeral OS for a machine template. For control plane nodes, we strongly recommend using [etcd data disks](data-disks.md) to avoid data loss.

````yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  template:
    spec:
      location: ${AZURE_LOCATION}
      osDisk:
        diffDiskSettings:
          option: Local
        diskSizeGB: 30
        managedDisk:
          storageAccountType: Standard_LRS
        osType: Linux
      sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
      vmSize: ${AZURE_NODE_MACHINE_TYPE}
````
