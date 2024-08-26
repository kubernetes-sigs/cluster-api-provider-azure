# Azure Service Operator

## Overview

CAPZ interfaces with Azure to create and manage some types of resources using [Azure Service Operator
(ASO)](https://azure.github.io/azure-service-operator/).

More context around the decision for CAPZ to pivot towards using ASO can be found in the
[proposal](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/docs/proposals/20230123-azure-service-operator.md).

[Visit this page](../managed/asomanagedcluster.md) to learn more about the AzureASOManaged cluster API which provisions an AKS cluster.

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

## Configuration with Environment Variables

These environment variables are passed through to the `aso-controller-settings` Secret to configure ASO when
CAPZ is installed and are consumed by `clusterctl init`. They may also be modified directly in the Secret
after installing ASO with CAPZ:

- `AZURE_AUTHORITY_HOST`
- `AZURE_RESOURCE_MANAGER_AUDIENCE`
- `AZURE_RESOURCE_MANAGER_ENDPOINT`
- `AZURE_SYNC_PERIOD`

More details on each can be found in [ASO's documentation](https://azure.github.io/azure-service-operator/guide/aso-controller-settings-options/).

## Using ASO for non-CAPZ resources

CAPZ's installation of ASO can be used directly to manage Azure resources outside the domain of
Cluster API.

### Installing more CRDs

#### For a fresh installation
Before performing a `clusterctl init`, users can specify additional ASO CRDs to be installed in the management cluster by exporting `ADDITIONAL_ASO_CRDS` variable.
For example, to install all the CRDs of `cache.azure.com` and `MongodbDatabase.documentdb.azure.com`:
- `export ADDITIONAL_ASO_CRDS="cache.azure.com/*;documentdb.azure.com/MongodbDatabase"`
- continue with the installation of CAPZ as specified here [Cluster API Quick Start](https://cluster-api.sigs.k8s.io/user/quick-start.html).

#### For an existing CAPZ installation being upgraded to v1.14.0(or beyond)
CAPZ's installation of ASO configures only the ASO CRDs that are required by CAPZ. To make more resource types available, export `ADDITIONAL_ASO_CRDS` and then upgrade CAPZ.
For example, to install the all CRDs of `cache.azure.com` and `MongodbDatabase.documentdb.azure.com`, follow these steps:
- `export ADDITIONAL_ASO_CRDS="cache.azure.com/*;documentdb.azure.com/MongodbDatabase"`
- continue with the upgrade of CAPZ as specified [here](https://cluster-api.sigs.k8s.io/tasks/upgrading-cluster-api-versions.html?highlight=upgrade#when-to-upgrade]

You will see that the `--crd-pattern` in Azure Service Operator's Deployment (in the `capz-system` namespace) looks like below:
   ```
   .
   - --crd-names=cache.azure.com/*;documentdb.azure.com/MongodbDatabase
   .
   ```

More details about how ASO manages CRDs can be found [here](https://azure.github.io/azure-service-operator/guide/crd-management/).

**Note:** To install the resource for the newly installed CRDs, make sure that the ASO operator has the authentication to install the resources. Refer [authentication in ASO](https://azure.github.io/azure-service-operator/guide/authentication/) for more details.
An example configuration file and demo for `Azure Cache for Redis` can be found [here](https://github.com/Azure-Samples/azure-service-operator-samples/tree/master/azure-votes-redis).
