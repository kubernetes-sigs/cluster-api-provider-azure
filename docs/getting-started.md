# Getting started with cluster-api-provider-azure

## Prerequisites

### Requirements

- A [Microsoft Azure account](https://azure.microsoft.com/en-us/)
- Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest)

### Setting up your Azure environment

An Azure Service Principal is needed for deploying Azure resources. The below instructions utilize [environment-based authentication](https://docs.microsoft.com/en-us/go/azure/azure-sdk-go-authorization#use-environment-based-authentication).

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
  NOTE: the "owner" role is required to be able to create role assignments for [system-assigned managed identity](book/src/topics/identity.md).

  ```bash
  az ad sp create-for-rbac --role contributor
  ```

  6. Save the output from the above command in environment variables.

  ```bash
  export AZURE_TENANT_ID="<Tenant>"
  export AZURE_CLIENT_ID="<AppId>"
  export AZURE_CLIENT_SECRET='<Password>'
  export AZURE_LOCATION="eastus" # this should be an Azure region that your subscription has quota for.
  ```

:warning: NOTE: If your password contains single quotes (`'`), make sure to escape them. To escape a single quote, close the quoting before it, insert the single quote, and re-open the quoting.
For example, if your password is `foo'blah$`, you should do `export AZURE_CLIENT_SECRET='foo'\''blah$'`.

  7. (Optional) Set the name of the AzureCloud to be used. The default value that would be used by most users is "AzurePublicCloud", other values are:

 - ChinaCloud: "AzureChinaCloud"
 - GermanCloud: "AzureGermanCloud"
 - PublicCloud: "AzurePublicCloud"
 - USGovernmentCloud: "AzureUSGovernmentCloud"

```bash
export AZURE_ENVIRONMENT="AzurePublicCloud"
```

### Building your first cluster

Check out the [Cluster API Quick Start](https://cluster-api.sigs.k8s.io/user/quick-start.html) to create your first Kubernetes cluster on Azure using Cluster API. Make sure to select the "Azure" tabs.

### Documentation

Please see the [CAPZ book](https://capz.sigs.k8s.io) for in-depth user documentation.
