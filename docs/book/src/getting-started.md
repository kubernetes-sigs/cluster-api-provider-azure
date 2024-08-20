# Getting started with cluster-api-provider-azure

## Prerequisites

### Requirements

<!-- markdown-link-check-disable-next-line -->
- A [Microsoft Azure account](https://azure.microsoft.com/)
  - Note: If using a new subscription, make sure to [register](https://learn.microsoft.com/azure/azure-resource-manager/management/resource-providers-and-types) the following resource providers:
    - `Microsoft.Compute`
    - `Microsoft.Network`
    - `Microsoft.ContainerService`
    - `Microsoft.ManagedIdentity`
    - `Microsoft.Authorization`
    - `Microsoft.ResourceHealth` (if the `EXP_AKS_RESOURCE_HEALTH` feature flag is enabled)
- Install the [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli?view=azure-cli-latest)
- A [supported version](https://github.com/kubernetes-sigs/cluster-api-provider-azure#compatibility) of `clusterctl`

### Setting up your Azure environment

An Azure Service Principal is needed for deploying Azure resources. The below instructions utilize [environment-based authentication](https://learn.microsoft.com/go/azure/azure-sdk-go-authorization#use-environment-based-authentication).

  1. Login with the Azure CLI.

   ```bash
  az login
   ```

  2. List your Azure subscriptions.

   ```bash
  az account list -o table
   ```

  3. If more than one account is present, select the account that you want to use.

   ```bash
  az account set -s <SubscriptionId>
   ```

  4. Save your Subscription ID in an environment variable.

  ```bash
  export AZURE_SUBSCRIPTION_ID="<SubscriptionId>"
  ```

  5. Create an Azure Service Principal by running the following command or skip this step and use a previously created Azure Service Principal.
  NOTE: the "owner" role is required to be able to create role assignments for system-assigned managed identity.

  ```bash
  az ad sp create-for-rbac --role contributor --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}"
  ```

  6. Save the output from the above command somewhere easily accessible and secure. You will need to save the `tenantID`, `clientID`, and `client secret`. When creating a Cluster, you will need to provide these values as a part of the `AzureClusterIdentity` object. Note that authentication via environment variables is now removed and an `AzureClusterIdentity` is required to be created. An example `AzureClusterIdentity` object is shown below:

  ```yaml
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: AzureClusterIdentity
  metadata:
    labels:
      clusterctl.cluster.x-k8s.io/move-hierarchy: "true"
    name: <cluster-identity-name>
    namespace: default
  spec:
    allowedNamespaces: {}
    clientID: <clientID>
    clientSecret:
      name: <client-secret-name>
      namespace: <client-secret-namespace>
    tenantID: <tenantID>
    type: ServicePrincipal
  ```

<aside class="note warning">

<h1> Warning </h1>

NOTE: If your password contains single quotes (`'`), make sure to escape them. To escape a single quote, close the quoting before it, insert the single quote, and re-open the quoting.
For example, if your password is `foo'blah$`, you should do `export AZURE_CLIENT_SECRET='foo'\''blah$'`.

</aside>

<aside class="note warning">

<h1> Warning </h1>

The capability to set credentials using environment variables is now deprecated and will be removed in future releases, the recommended approach is to use `AzureClusterIdentity` as explained [here](./topics/multitenancy.md)

</aside>


### Building your first cluster
Check out the [Cluster API Quick Start](https://cluster-api.sigs.k8s.io/user/quick-start.html) to create your first Kubernetes cluster on Azure using Cluster API. Make sure to select the "Azure" tabs.

If you are looking to install additional ASO CRDs, set `ADDITIONAL_ASO_CRDS` to the list of CRDs you want to install. Refer to adding additional CRDs for Azure Service Operator [here](./topics/aso.md#Using-aso-for-non-capz-resources).

<h1> Warning </h1>

Not all versions of clusterctl are supported.  Please see which versions are [currently supported](https://github.com/kubernetes-sigs/cluster-api-provider-azure#compatibility)

### Documentation

Please see the [CAPZ book](https://capz.sigs.k8s.io) for in-depth user documentation.
