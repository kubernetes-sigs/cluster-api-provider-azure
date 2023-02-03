# Custom images

This document will help you get a CAPZ Kubernetes cluster up and running with your custom image.

## Reference images

An *image* defines the operating system and Kubernetes components that will populate the disk of each node in your cluster.

By default, images offered by "capi" in the [Azure Marketplace][azure-marketplace] are used.

You can list these *reference images* with this command:

```bash
az vm image list --publisher cncf-upstream --offer capi --all -o table
```

It is recommended to use the latest patch release of Kubernetes for a [supported minor release][supported-k8s].

<aside class="note warning">

<h1> Availability </h1>

The Cluster API for Azure team publishes *reference images* for each Kubernetes release, for both Linux and Windows.

Reference images for versions of Kubernetes which have known security issues or which are no longer [supported by Cluster API][supported-capi] will be removed from the Azure Marketplace.

</aside>

<aside class="note warning">

<h1> Security </h1>

The reference images are not updated with security fixes. They are intended only to facilitate testing and to help users try out Cluster API for Azure.

The reference images should not be used in a production environment. It is highly recommended to [maintain your own custom image](#building-a-custom-image) instead.

</aside>

## Building a custom image

Cluster API uses the Kubernetes [Image Builder][image-builder] tools. You should use the [Azure images][image-builder-azure] from that project as a starting point for your custom image.

[The Image Builder Book][capi-images] explains how to build the images defined in that repository, with instructions for [Azure CAPI Images][azure-capi-images] in particular.

### Operating system requirements

For your custom image to work with Cluster API, it must meet the operating system requirements of the bootstrap provider. For example, the default `kubeadm` bootstrap provider has a set of [`preflight checks`][kubeadm-preflight-checks] that a VM is expected to pass before it can join the cluster.

### Kubernetes version requirements

The reference images are each built to support a specific version of Kubernetes. When using your custom images based on them, take care to match the image to the `version:` field of the `KubeadmControlPlane` and `MachineDeployment` in the YAML template for your workload cluster.

To upgrade to a new Kubernetes release with custom images requires this preparation:

- create a new custom image which supports the Kubernetes release version
- copy the existing `AzureMachineTemplate` and change its `image:` section to reference the new custom image
- create the new `AzureMachineTemplate` on the management cluster
- modify the existing `KubeadmControlPlane` and `MachineDeployment` to reference the new `AzureMachineTemplate` and update the `version:` field to match

See [Upgrading workload clusters][upgrading-workload-clusters] for more details.

## Creating a cluster from a custom image

To use a custom image, it needs to be referenced in an `image:` section of your `AzureMachineTemplate`. See below for more specific examples.

### Using Shared Image Gallery (Recommended)

To use an image from the [Shared Image Gallery][shared-image-gallery], fill in the `resourceGroup`, `name`, `subscriptionID`, `gallery`, and `version` fields:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-shared-gallery-example
spec:
  template:
    spec:
      image:
        sharedGallery:
          resourceGroup: "cluster-api-images"
          name: "capi-1234567890"
          subscriptionID: "01234567-89ab-cdef-0123-4567890abcde"
          gallery: "ClusterAPI"
          version: "0.3.1234567890"
```

If you build Azure CAPI images with the `make` targets in Image Builder, these required values are printed after a successful build. For example:

```bash
$ make -C images/capi/ build-azure-sig-ubuntu-1804
# many minutes later...
==> sig-ubuntu-1804:
Build 'sig-ubuntu-1804' finished.

==> Builds finished. The artifacts of successful builds are:
--> sig-ubuntu-1804: Azure.ResourceManagement.VMImage:

OSType: Linux
ManagedImageResourceGroupName: cluster-api-images
ManagedImageName: capi-1234567890
ManagedImageId: /subscriptions/01234567-89ab-cdef-0123-4567890abcde/resourceGroups/cluster-api-images/providers/Microsoft.Compute/images/capi-1234567890
ManagedImageLocation: southcentralus
ManagedImageSharedImageGalleryId: /subscriptions/01234567-89ab-cdef-0123-4567890abcde/resourceGroups/cluster-api-images/providers/Microsoft.Compute/galleries/ClusterAPI/images/capi-ubuntu-1804/versions/0.3.1234567890
```

Please also see the [replication recommendations][replication-recommendations] for the Shared Image Gallery.

If the image you want to use is based on an image released by a third party publisher such as for example 
`Flatcar Linux` by `Kinvolk`, then you need to specify the `publisher`, `offer`, and `sku` fields as well:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-shared-gallery-example
spec:
  template:
    spec:
      image:
        sharedGallery:
          resourceGroup: "cluster-api-images"
          name: "capi-1234567890"
          subscriptionID: "01234567-89ab-cdef-0123-4567890abcde"
          gallery: "ClusterAPI"
          version: "0.3.1234567890"
          publisher: "kinvolk"
          offer: "flatcar-container-linux-free"
          sku: "stable"
```

This will make API calls to create Virtual Machines or Virtual Machine Scale Sets to have the `Plan` correctly set.

### Using image ID

To use a managed image resource by ID, only the `id` field must be set:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-image-id-example
spec:
  template:
    spec:
      image:
        id: "/subscriptions/01234567-89ab-cdef-0123-4567890abcde/resourceGroups/myResourceGroup/providers/Microsoft.Compute/images/myImage"
```

A managed image resource can be created from a Virtual Machine. Please refer to Azure documentation on [creating a managed image][creating-managed-image] for more detail.

Managed images support only 20 simultaneous deployments, so for most use cases Shared Image Gallery is recommended.

### Using Azure Marketplace

To use an image from [Azure Marketplace][azure-marketplace], populate the `publisher`, `offer`, `sku`, and `version` fields and, if this image is published by a third party publisher, set the `thirdPartyImage` flag to `true` so an image Plan can be generated for it. In the case of a third party image, you must accept the license terms with the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/vm/image/terms?view=azure-cli-latest) before consuming it.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-marketplace-example
spec:
  template:
    spec:
      image:
        marketplace:
          publisher: "example-publisher"
          offer: "example-offer"
          sku: "k8s-1dot18dot8-ubuntu-1804"
          version: "2020-07-25"
          thirdPartyImage: true
```

[azure-marketplace]: https://docs.microsoft.com/azure/marketplace/marketplace-publishers-guide
[azure-capi-images]: https://image-builder.sigs.k8s.io/capi/providers/azure.html
[capi-images]: https://image-builder.sigs.k8s.io/capi/capi.html
[creating-managed-image]: https://docs.microsoft.com/azure/virtual-machines/linux/capture-image
[creating-vm-offer]: https://docs.azure.cn/en-us/articles/azure-marketplace/imagepublishguide#5-azure-
[image-builder]: https://github.com/kubernetes-sigs/image-builder
[image-builder-azure]: https://github.com/kubernetes-sigs/image-builder/tree/master/images/capi/packer/azure
[kubeadm-preflight-checks]: https://github.com/kubernetes/kubeadm/blob/master/docs/design/design_v1.10.md#preflight-checks
[replication-recommendations]: https://docs.microsoft.com/azure/virtual-machines/linux/shared-image-galleries#scaling
[shared-image-gallery]: https://docs.microsoft.com/azure/virtual-machines/linux/shared-image-galleries
[supported-capi]: https://cluster-api.sigs.k8s.io/reference/versions.html#supported-kubernetes-versions
[supported-k8s]: https://kubernetes.io/releases/version-skew-policy/#supported-versions
[upgrading-workload-clusters]: https://cluster-api.sigs.k8s.io/tasks/upgrading-clusters.html
