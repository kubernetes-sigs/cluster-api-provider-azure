# Confidential VMs

This document describes how to deploy a cluster with Azure [Confidential VM](https://learn.microsoft.com/azure/confidential-computing/confidential-vm-overview) nodes.

## Limitations

Before you begin, be aware of the following:

- [VM Size Support](https://learn.microsoft.com/en-us/azure/confidential-computing/confidential-vm-overview#size-support)
- [OS Support](https://learn.microsoft.com/en-us/azure/confidential-computing/confidential-vm-overview#os-support)
- [Limitations](https://learn.microsoft.com/en-us/azure/confidential-computing/confidential-vm-overview#limitations)

## Confidential VM Images

One of the limitations of Confidential VMs is that they support specific OS images, as they need to get [successfully attested](https://learn.microsoft.com/en-us/azure/confidential-computing/confidential-vm-overview#attestation-and-tpm) during boot.

Confidential VM images are not included in the list of `capi` reference images. Before creating a cluster hosted on Azure Confidential VMs, you can create a [custom image](custom-images.md) based on a Confidential VM supported OS image using [image-builder](https://github.com/kubernetes-sigs/image-builder). For example, you can run the following to create such an image based on Ubuntu Server 22.04 LTS for CVMs:

```bash
$ make -C images/capi build-azure-sig-ubuntu-2204-cvm
# many minutes later...
==> sig-ubuntu-2204-cvm:
Build 'sig-ubuntu-2204-cvm' finished.

==> Builds finished. The artifacts of successful builds are:
--> sig-ubuntu-2204-cvm: Azure.ResourceManagement.VMImage:

OSType: Linux
ManagedImageResourceGroupName: cluster-api-images
ManagedImageName: capi-ubuntu-2204-cvm-1684153817
ManagedImageId: /subscriptions/01234567-89ab-cdef-0123-4567890abcde/resourceGroups/cluster-api-images/providers/Microsoft.Compute/images/capi-ubuntu-2204-cvm-1684153817
ManagedImageLocation: southcentralus
ManagedImageSharedImageGalleryId: /subscriptions/01234567-89ab-cdef-0123-4567890abcde/resourceGroups/cluster-api-images/providers/Microsoft.Compute/galleries/ClusterAPI/images/capi-ubuntu-2204-cvm/versions/0.3.1684153817
```

## Example

The below example shows how to deploy a cluster with the control-plane nodes as Confidential VMs. SecurityEncryptionType is set to VMGuestStateOnly (i.e. only the VMGuestState blob will be encrypted), while VTpmEnabled and SecureBootEnabled are both set to true. Make sure to choose a supported VM size (e.g. `Standard_DC4as_v5`) and OS (e.g. Ubuntu Server 22.04 LTS for Confidential VMs).
NOTE: the same can be applied to worker nodes

```yaml
kind: AzureMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: capz-confidential-vms-example
spec:
  template:
    spec:
      image:
        computeGallery:
          subscriptionID: "01234567-89ab-cdef-0123-4567890abcde"
          resourceGroup: "cluster-api-images"
          gallery: "ClusterAPI"
          name: "capi-ubuntu-2204-cvm-1684153817"
          version: "0.3.1684153817"
      securityProfile:
        securityType: "ConfidentialVM"
        uefiSettings:
          vTpmEnabled: true
          secureBootEnabled: true
      osDisk:
        diskSizeGB: 128
        osType: "Linux"
        managedDisk:
          storageAccountType: "Premium_LRS"
          securityProfile:
            securityEncryptionType: "VMGuestStateOnly"
      vmSize: "Standard_DC4as_v5"
````
