# Azure Service Operator

## Overview

CAPZ interfaces with Azure to create and manage some types of resources using [Azure Service Operator
(ASO)](https://azure.github.io/azure-service-operator/).

More context around the decision for CAPZ to pivot towards using ASO can be found in the
[proposal](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/docs/proposals/20230123-azure-service-operator.md).

## Primary changes

For most users, the introduction of ASO is expected to be fully transparent and backwards compatible. Changes
that may affect specific use cases are described below.

### Installation

Beginning with CAPZ v1.11.0, ASO's control plane will be installed automatically by `clusterctl` in the
`capz-system` namespace alongside CAPZ's control plane components. When ASO is already installed on a cluster,
installing ASO again with CAPZ is expected to fail and `clusterctl` cannot install CAPZ without ASO. The
suggested workaround for users facing this issue is to uninstall the existing ASO control plane (but **keep**
the ASO CRDs) and then to install CAPZ.

### Bring-your-own (BYO) resource

CAPZ had already allowed users to pre-create some resources like resource groups and virtual networks and
reference those resources in CAPZ resources. CAPZ will then use those existing resources without creating new
ones and assume the user is responsible for managing them, so will not actively reconcile changes to or delete
those resources.

This use case is still supported with ASO installed. The main difference is that an ASO resource will be
created for CAPZ's own bookkeeping, but configured not to be actively reconciled by ASO. When the Cluster API
Cluster owning the resource is deleted, the ASO resource will also be deleted from the management cluster but
the resource will not be deleted in Azure.

Additionally, BYO resources may include ASO resources managed by the user. CAPZ will not modify or delete such
resources. Note that `clusterctl move` will not move user-managed ASO resources.

## Using ASO for non-CAPZ resources

CAPZ's installation of ASO can be used directly to manage Azure resources outside the domain of
Cluster API.

### Installing more CRDs

CAPZ's installation of ASO configures only the ASO CRDs that are required by CAPZ. To make more resource types
available, install their corresponding CRDs. ASO publishes a manifest containing all CRDs for each
[release](https://github.com/Azure/azure-service-operator/releases). Extract only the ones you need using tool
like [`yq`](https://mikefarah.gitbook.io/yq/), then make the following modifications to each CRD to account
for CAPZ installing ASO in the `capz-system` namespace:

- Change `metadata.annotations."cert-manager.io/inject-ca-from"` to `capz-system/azureserviceoperator-serving-cert`
- Change `spec.conversion.webhook.clientConfig.service.namespace` to `capz-system`

More details about how ASO manages CRDs can be found [here](https://azure.github.io/azure-service-operator/guide/crd-management/).
