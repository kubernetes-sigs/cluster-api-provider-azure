# Custom images

This document will help you get a CAPZ Kubernetes cluster up and running with your custom image.

## Reference images

An *image* defines the operating system and Kubernetes components that will populate the disk of each node in your cluster.

By default, images published by the Cluster API for Azure team are used. These images live in an Azure [community gallery](https://learn.microsoft.com/azure/virtual-machines/share-gallery-community).

You can list these *reference images* with these commands:

```bash
# List the image definitions (distro and version)
az sig image-definition list-community \
  --public-gallery-name ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019 \
  --location northcentralus
# List the versions for an image definition (Ubuntu 24.04 for example)
# Version names are Kubernetes releases, such as "1.28.15" or "1.31.2".
az sig image-version list-community \
  --public-gallery-name ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019 \
  --gallery-image-definition capi-ubun2-2404 \
  --location northcentralus
```

The reference images are replicated to the set of regions used in CAPZ e2e tests. To see if a reference image is available in the location where you intend to provision a cluster, change the value of the `--location` argument in the previous command.

It is recommended to use the latest patch release of Kubernetes for a [supported minor release][supported-k8s].

<aside class="note warning">

<h1> Availability </h1>

The Cluster API for Azure team publishes *reference images* for each Kubernetes release, for both Linux and Windows.

Reference images for versions of Kubernetes which have known security issues or which are no longer [supported by Cluster API][supported-capi] will be removed from the Azure Marketplace and the CAPZ community gallery.

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

See [Upgrading clusters][upgrading-clusters] for more details.

## Creating a cluster from a custom image

To use a custom image, it needs to be referenced in an `image:` section of your `AzureMachineTemplate`. See below for more specific examples.

### Using Azure Compute Gallery (Recommended)

To use an image from the [Azure Compute Gallery][azure-compute-gallery], previously known as Shared Image Gallery (SIG), fill in the `resourceGroup`, `name`, `subscriptionID`, `gallery`, and `version` fields:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-compute-gallery-example
spec:
  template:
    spec:
      image:
        computeGallery:
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

Please also see the [replication recommendations][replication-recommendations] for the Azure Compute Gallery.

If the image you want to use is based on an image released by a third party publisher such as for example
`Flatcar Linux` by `Kinvolk`, then you need to specify the `publisher`, `offer`, and `sku` fields as well:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-compute-gallery-example
spec:
  template:
    spec:
      image:
        computeGallery:
          resourceGroup: "cluster-api-images"
          name: "capi-1234567890"
          subscriptionID: "01234567-89ab-cdef-0123-4567890abcde"
          gallery: "ClusterAPI"
          version: "0.3.1234567890"
          plan:
            publisher: "kinvolk"
            offer: "flatcar-container-linux-free"
            sku: "stable"
```

This will make API calls to create Virtual Machines or Virtual Machine Scale Sets to have the `Plan` correctly set.

### Using a community gallery

A "community gallery" is an Azure Compute Gallery with "community" permissions, but it has a globally unique name, is available to all Azure users, and is accessed differently.

The CAPZ project publishes reference images to a community gallery. But as mentioned at the top of this document, they are the default. You don't need to specify a custom image section in your template to use the reference images.

To use an image from a community gallery, set the `name`, `gallery`, and `version` fields:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-community-gallery-example
spec:
  template:
    spec:
      image:
        computeGallery:
          name: "ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019"
          gallery: "capi-ubun2-2404"
          version: "1.31.2"
```

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

Managed images support only 20 simultaneous deployments, so for most use cases Azure Compute Gallery is recommended.

### Using Azure Marketplace

To use an image from [Azure Marketplace][azure-marketplace], populate the `publisher`, `offer`, `sku`, and `version` fields and, if this image is published by a third party publisher, set the `thirdPartyImage` flag to `true` so an image Plan can be generated for it. In the case of a third party image, you must accept the license terms with the [Azure CLI](https://learn.microsoft.com/cli/azure/vm/image/terms?view=azure-cli-latest) before consuming it.

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

### Using Azure Community Gallery

To use an image from [Azure Community Gallery][azure-community-gallery], set `name` field to gallery's public name and don't set `subscriptionID` and `resourceGroup` fields:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-community-gallery-example
spec:
  template:
    spec:
      image:
        computeGallery:
          gallery: testGallery-3282f15c-906a-4c4b-b206-eb3c51adb5be
          name: capi-flatcar-stable-3139.2.0
          version: 0.3.1651499183
```

If the image you want to use is based on an image released by a third party publisher such as for example
`Flatcar Linux` by `Kinvolk`, then you need to specify the `publisher`, `offer`, and `sku` fields as well:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: capz-community-gallery-example
spec:
  template:
    spec:
      image:
        computeGallery:
          gallery: testGallery-3282f15c-906a-4c4b-b206-eb3c51adb5be
          name: capi-flatcar-stable-3139.2.0
          version: 0.3.1651499183
          plan:
            publisher: kinvolk
            offer: flatcar-container-linux-free
            sku: stable
```

This will make API calls to create Virtual Machines or Virtual Machine Scale Sets to have the `Plan` correctly set.

In the case of a third party image, you must accept the license terms with the [Azure CLI][azure-cli] before consuming it.

## Example: CAPZ with Mariner Linux

To clarify how to use a custom image, let's look at an example of using [Mariner Linux][mariner] with CAPZ.

Mariner is a minimal, open source Linux distribution, optimized for Azure. The [image-builder][image-builder] project has support for building Mariner images.

## Build Mariner with image-builder

Populate an `az-creds.env` file with your Azure credentials:

```
AZURE_SUBSCRIPTION_ID=xxxxxxx
AZURE_TENANT_ID=xxxxxxx
AZURE_CLIENT_ID=xxxxxxxx
AZURE_CLIENT_SECRET=xxxxxx
```

Then run image-builder, referencing those credentials as an environment file:

```shell
docker run -it --rm --env-file azure-creds.env registry.k8s.io/scl-image-builder/cluster-node-image-builder-amd64:v0.1.17 build-azure-sig-mariner-2
```

The entrypoint to this docker image is `make`. (You can clone the image-builder repository and run `make -C images/capi build-azure-sig-mariner-2` locally if you prefer.)

This makefile target creates an Azure resource group called "cluster-api-images" in `southcentralus` by default. When it finishes, it will contain an Azure Compute Gallery with a Mariner image.

```shell
# skipping output to show just the end of the build...
==> azure-arm.sig-mariner-2: Resource group has been deleted.
==> azure-arm.sig-mariner-2: Running post-processor: manifest
Build 'azure-arm.sig-mariner-2' finished after 18 minutes 2 seconds.

==> Wait completed after 18 minutes 2 seconds

==> Builds finished. The artifacts of successful builds are:
--> azure-arm.sig-mariner-2: Azure.ResourceManagement.VMImage:

OSType: Linux
ManagedImageResourceGroupName: cluster-api-images
ManagedImageName: capi-mariner-2-1689801407
ManagedImageId: /subscriptions/xxxxxxx-xxxx-xxx-xxx/resourceGroups/cluster-api-images/providers/Microsoft.Compute/images/capi-mariner-2-1689801407
ManagedImageLocation: southcentralus
ManagedImageSharedImageGalleryId: /subscriptions/xxxxxxx-xxxx-xxx-xxx/resourceGroups/cluster-api-images/providers/Microsoft.Compute/galleries/ClusterAPI1689801353abcd/images/capi-mariner-2/versions/0.3.1689801407
SharedImageGalleryResourceGroup: cluster-api-images
SharedImageGalleryName: ClusterAPI1689801353abcd
SharedImageGalleryImageName: capi-mariner-2
SharedImageGalleryImageVersion: 0.3.1689801407
SharedImageGalleryReplicatedRegions: southcentralus
```

## Add the Mariner image to a CAPZ cluster template

Edit your cluster template to add `image` fields to any AzureMachineTemplates:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-control-plane
  namespace: default
spec:
  template:
    spec:
      image:
        computeGallery:
          resourceGroup: cluster-api-images
          name: capi-mariner-2
          subscriptionID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
          gallery: ClusterAPI1689801353abcd
          version: "0.3.1689801407"
```

The last four fields are the `SharedImageGalleryImageName`, your Azure subscription ID, the `SharedImageGalleryName`, and the `SharedImageGalleryImageVersion` from the final output of the image-builder command above. Make sure to add this `image` section to both the control plane and worker node AzureMachineTemplates.

## Deploy a Mariner cluster

Since our Compute Gallery image lives in `southcentralus`, our cluster should too. Set `AZURE_LOCATION=southcentralus` in your environment or in your template.

Now you can deploy your CAPZ Mariner cluster as usual with `kubectl apply -f` or other means.

Mariner stores CA certificates in an uncommon location, so we need to tell cloud-provider-azure's Helm chart where. Add this argument to the `helm` command you use to install cloud-provider-azure:

```shell
--set-string cloudControllerManager.caCertDir=/etc/pki/tls
```

That's it! You should now have a CAPZ cluster running Mariner Linux.


[azure-cli]: https://learn.microsoft.com/cli/azure/vm/image/terms?view=azure-cli-latest
[azure-community-gallery]: https://learn.microsoft.com/azure/virtual-machines/azure-compute-gallery#community
[azure-marketplace]: https://learn.microsoft.com/azure/marketplace/marketplace-publishers-guide
[azure-capi-images]: https://image-builder.sigs.k8s.io/capi/providers/azure.html
[azure-compute-gallery]: https://learn.microsoft.com/azure/virtual-machines/linux/shared-image-galleries
[capi-images]: https://image-builder.sigs.k8s.io/capi/capi.html
[creating-managed-image]: https://learn.microsoft.com/azure/virtual-machines/linux/capture-image
[creating-vm-offer]: https://docs.azure.cn/en-us/articles/azure-marketplace/imagepublishguide#5-azure-
[image-builder]: https://github.com/kubernetes-sigs/image-builder
[image-builder-azure]: https://github.com/kubernetes-sigs/image-builder/tree/master/images/capi/packer/azure
[kubeadm-preflight-checks]: https://github.com/kubernetes/kubeadm/blob/master/docs/design/design_v1.10.md#preflight-checks
[mariner]: https://microsoft.github.io/azurelinux/docs/
[replication-recommendations]: https://learn.microsoft.com/azure/virtual-machines/linux/shared-image-galleries#scaling
[supported-capi]: https://cluster-api.sigs.k8s.io/reference/versions.html#supported-kubernetes-versions
[supported-k8s]: https://kubernetes.io/releases/version-skew-policy/#supported-versions
[upgrading-clusters]: https://cluster-api.sigs.k8s.io/tasks/upgrading-clusters.html
