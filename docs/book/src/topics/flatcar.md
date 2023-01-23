# Flatcar Clusters

## Overview

CAPZ enables you to create Kubernetes clusters using Flatcar Container Linux on Microsoft Azure.

### Image creation

The testing reference images are built using [image-builder](https://github.com/kubernetes-sigs/image-builder) by Flatcar maintainers and published to the Flatcar CAPI Community Gallery on Azure with community gallery name `flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0`.

<aside class="note warning">

<h1> Security </h1>

The reference images are not updated with security fixes. They are intended only to facilitate testing and to help users try out Cluster API for Azure.

The reference images should not be used in a production environment. It is highly recommended to [maintain your own custom image](#building-a-custom-image) instead.

</aside>

Find the latest published images:

```console
$ az sig image-definition list-community --location westeurope --public-gallery-name flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0 --only-show-errors
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

To create a cluster using Flatcar Container Linux, use `flatcar` cluster flavor.
