# Flatcar Clusters

## Overview

CAPZ enables you to create Kubernetes clusters using Flatcar Container Linux on Microsoft Azure. Flatcar Container Linux comes in two flavors:

### The `flatcar-sysext` flavor (**recommended**)

This variant relies on a vanilla Flatcar Community Gallery image which leverages the [systemd-sysext](https://www.flatcar.org/docs/latest/provisioning/sysext/) feature to install and update Kubernetes components. The Kubernetes version is not bound to the Flatcar version (i.e. Flatcar can be upgraded independently from Kubernetes and vice versa).

The template comes with a [systemd-sysupdate](https://www.freedesktop.org/software/systemd/man/latest/sysupdate.d.html) configuration file that will download each new patch version of Kubernetes (i.e. if you start with Kubernetes 1.x.y, systemd-sysupdate will automatically pull 1.x.y+1 but not 1.x+1.y). Please note that this behavior is disabled by default. To enable the Kubernetes auto-update you can:
  * Update the template to enable the `systemd-sysupdate.timer`
  * Or run the following command on the nodes: `sudo systemctl enable --now systemd-sysupdate.timer`

When the Kubernetes release reaches end-of-life it will not receive updates anymore. To switch to a new major version, do a `sudo rm /etc/sysupdate.kubernetes.d/kubernetes-*.conf` and download the new update config into the folder with `cd /etc/sysupdate.kubernetes.d && sudo wget https://github.com/flatcar/sysext-bakery/releases/download/latest/kubernetes-${KUBERNETES_VERSION%.*}.conf`.

To coordinate the node reboot, we recommend using [Kured](https://github.com/kubereboot/kured). Note that running `kubeadm upgrade apply` on the first controller and `kubeadm upgrade node` on all other nodes is not automated (yet): see the [docs](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-upgrade/).

Find the latest published images:
```console
az sig image-version list --gallery-image-definition flatcar-stable-amd64 --gallery-name flatcar --resource-group flatcar-image-gallery-publishing -o table
Location    Name      ProvisioningState    ResourceGroup
----------  --------  -------------------  --------------------------------
westeurope  3374.2.0  Succeeded            flatcar-image-gallery-publishing
westeurope  3374.2.1  Succeeded            flatcar-image-gallery-publishing
westeurope  3374.2.3  Succeeded            flatcar-image-gallery-publishing
....
```

### The `flatcar` flavor

This variant relies on a Flatcar image built using the image-builder project. The Kubernetes version is bound to the Flatcar version and a rebuild of the image is required for each Kubernetes or Flatcar upgrade.

#### Image creation

The testing reference images are built using [image-builder](https://github.com/kubernetes-sigs/image-builder) by Flatcar maintainers and published to the Flatcar CAPI Community Gallery on Azure with community gallery name `flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0`.

<aside class="note warning">

<h1> Security </h1>

The reference images are not updated with security fixes. They are intended only to facilitate testing and to help users try out Cluster API for Azure.

The reference images should not be used in a production environment. It is highly recommended to [maintain your own custom image](./custom-images.md#building-a-custom-image) instead.

</aside>

Find the latest published images:

```console
$ az sig image-definition list-community --location westeurope --public-gallery-name flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0 --only-show-errors -o table
HyperVGeneration    Location    Name                                OsState      OsType    UniqueId
------------------  ----------  ----------------------------------  -----------  --------  ---------------------------------------------------------------------------------------------------------------
V2                  westeurope  flatcar-stable-amd64-capi-v1.23.13  Generalized  Linux     /CommunityGalleries/flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0/Images/flatcar-stable-amd64-capi-v1.23.13
V2                  westeurope  flatcar-stable-amd64-capi-v1.25.4   Generalized  Linux     /CommunityGalleries/flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0/Images/flatcar-stable-amd64-capi-v1.25.4
V2                  westeurope  flatcar-stable-amd64-capi-v1.26.0   Generalized  Linux     /CommunityGalleries/flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0/Images/flatcar-stable-amd64-capi-v1.26.0
$
$ az sig image-version list-community --location westeurope --public-gallery-name flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0 --only-show-errors --gallery-image-definition flatcar-stable-amd64-capi-v1.26.0
ExcludeFromLatest    Location    Name      PublishedDate                     UniqueId
-------------------  ----------  --------  --------------------------------  --------------------------------------------------------------------------------------------------------------------------------
False                westeurope  3227.2.3  2022-12-09T18:05:58.830464+00:00  /CommunityGalleries/flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0/Images/flatcar-stable-amd64-capi-v1.26.0/Versions/3227.2.3
```

If you would like customize your images please refer to the documentation on building your own [custom images](custom-images.md).


## Trying it out

To create a cluster using Flatcar Container Linux, use `flatcar` or `flatcar-sysext` cluster flavor.

- Note: When working with **Flatcar machines**, append `--set-string cloudControllerManager.caCertDir=/usr/share/ca-certificates` to the `cloud-provider-azure` _helm_ command. Refer ["External Cloud Provider's Note for flatcar-flavored machine"](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/764aa1e8bd02d150dff90ff6bc7f8daa2b38810f/docs/book/src/topics/addons.md#external-cloud-provider)
  - However, no changes are needed when using tilt to bring up Flatcar workload clusters.



