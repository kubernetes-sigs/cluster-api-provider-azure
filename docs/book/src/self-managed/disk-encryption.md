# Disk Encryption
This document describes how to configure different encryption options for disks allocated to VMs provisioned in Azure. 

## Azure Disk Storage Server-Side Encryption
Azure Disk Storage Server-Side Encryption (SSE) is also referred to as encryption-at-rest. This encryption option does not encrypt temporary disks or disk caches.

When enabled, Azure Disk Storage SSE encrypts data stored on Azure managed disks, i.e. OS and data disks. This option can be enabled using customer-managed keys.

Customer-managed keys must be configured through a Disk Encryption Set (DES) resource. For more information on Azure Disk Storage SSE, please see this [link](https://learn.microsoft.com/azure/virtual-machines/disk-encryption).

### Example with OS Disk using DES
When using customer-managed keys, you only need to provide the DES ID within the managedDisk spec. 
> **Note**: The DES must be within the same subscription.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: <machine-template-name>
  namespace: <namespace>
spec:
  template:
    spec:
      [...]
      osDisk:
        managedDisk:
          diskEncryptionSet:
            id: <disk_encryption_set_id>
      [...]
```

## Encryption at Host
This encryption option is a VM option enhancing Azure Disk Storage SSE to ensure any temp disk or disk cache is encrypted at rest.

For more information on encryption at host, please see this [link](https://learn.microsoft.com/azure/virtual-machines/disk-encryption#encryption-at-host---end-to-end-encryption-for-your-vm-data).

### Example with OS Disk and DES
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: <machine-template-name>
  namespace: <namespace>
spec:
  template:
    spec:
      [...]
      osDisk:
        managedDisk:
          diskEncryptionSet:
            id: <disk_encryption_set_id>
      securityProfile:
        encryptionAtHost: true
      [...]
```
