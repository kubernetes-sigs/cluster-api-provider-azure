# Trusted launch for VMs

This document describes how to deploy a cluster with nodes that support [trusted launch](https://learn.microsoft.com/azure/virtual-machines/trusted-launch).

## Limitations

Before you begin, be aware of the following:

- [Limitations](https://learn.microsoft.com/en-us/azure/virtual-machines/trusted-launch#limitations)
- [SecureBoot](https://learn.microsoft.com/en-us/azure/virtual-machines/trusted-launch#secure-boot)
- [vTPM](https://learn.microsoft.com/en-us/azure/virtual-machines/trusted-launch#vtpm)

## Trusted Launch Images

One of the limitations of trusted launch for VMs is that they require [generation 2](https://learn.microsoft.com/en-us/azure/virtual-machines/generation-2) VMs.

Trusted launch supported OS images are not included in the list of `capi` reference images. Before creating a cluster hosted on VMs with trusted launch features enabled, you can create a [custom image](custom-images.md) based on a one of the trusted launch supported OS images using [image-builder](https://github.com/kubernetes-sigs/image-builder). For example, you can run the following to create such an image based on Ubuntu Server 22.04 LTS:

```bash
$ make -C images/capi build-azure-sig-ubuntu-2204-gen2
# many minutes later...
==> sig-ubuntu-2204-gen2:
Build 'sig-ubuntu-2204-gen2' finished.

==> Builds finished. The artifacts of successful builds are:
--> sig-ubuntu-2204-gen2: Azure.ResourceManagement.VMImage:

OSType: Linux
ManagedImageResourceGroupName: cluster-api-images
ManagedImageName: capi-ubuntu-2204-gen2-1684153817
ManagedImageId: /subscriptions/01234567-89ab-cdef-0123-4567890abcde/resourceGroups/cluster-api-images/providers/Microsoft.Compute/images/capi-ubuntu-2204-gen2-1684153817
ManagedImageLocation: southcentralus
ManagedImageSharedImageGalleryId: /subscriptions/01234567-89ab-cdef-0123-4567890abcde/resourceGroups/cluster-api-images/providers/Microsoft.Compute/galleries/ClusterAPI/images/capi-ubuntu-2204-gen2/versions/0.3.1684153817
```

## Example

The below example shows how to deploy a cluster with control-plane nodes that have SecureBoot and vTPM enabled. Make sure to choose a supported generation 2 VM size (e.g. `Standard_B2s`) and OS (e.g. Ubuntu Server 22.04 LTS).
NOTE: the same can be applied to worker nodes

```yaml
kind: AzureMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: capz-trusted-launch-example
spec:
  template:
    spec:
      image:
        computeGallery:
          subscriptionID: "01234567-89ab-cdef-0123-4567890abcde"
          resourceGroup: "cluster-api-images"
          gallery: "ClusterAPI"
          name: "capi-ubuntu-2204-gen2-1684153817"
          version: "0.3.1684153817"
      securityProfile:
        securityType: "TrustedLaunch"
        uefiSettings:
          vTpmEnabled: true
          secureBootEnabled: true
      osDisk:
        diskSizeGB: 128
        osType: "Linux"
      vmSize: "Standard_B2s"
```
