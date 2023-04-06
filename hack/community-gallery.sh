#!/bin/bash

set -euo pipefail

location=${LOCATION:-"westus2"}
galleryName=${GALLERY_NAME:-"CAPZGallery"}
resourceGroup=${RESOURCE_GROUP:-"CAPZGallery"}
publisherUri=${PUBLISHER_URI:-"https://github.com/kubernetes-sigs/cluster-api-provider-azure"}
publisherEmail=${PUBLISHER_EMAIL:-"CAPZGallery@microsoft.com"}
eulaLink=${EULA_LINK:-"https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/LICENSE"}
prefix=${GALLERY_PREFIX:-"CAPZGallery"}
tags=${TAGS:-"DO-NOT-DELETE=CAPZGallery"}

az group create --name $resourceGroup --location $location --tags $tags

az sig create \
   --gallery-name $galleryName \
   --permissions community \
   --resource-group $resourceGroup \
   --publisher-uri $publisherUri \
   --publisher-email $publisherEmail \
   --eula $eulaLink \
   --public-name-prefix $prefix \
   --tags $tags
