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
- The [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli?view=azure-cli-latest)
- A [supported version](https://github.com/kubernetes-sigs/cluster-api-provider-azure#compatibility) of `clusterctl`

### Setting up your Azure environment

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
## Creating an AKS Management Cluster with Workload Identity

  1. Create an AKS Cluster with Workload Identity and OIDC Endpoint Enabled.
  ```bash
  az aks create \
  --resource-group <resource-group-name> \
  --name <aks-cluster-name> \
  --enable-oidc-issuer \
  --enable-workload-identity \
  --node-count 2 \
  --node-vm-size Standard_B2s \
  --generate-ssh-keys \
  --location <region>
  ```
  
  2. Retrieve Credentials for the AKS Cluster to interact with it using kubectl:
  ```bash
  az aks get-credentials --resource-group <resource-group-name> --name <aks-cluster-name>
  ```

  3. Retrieve the OIDC Issuer URL.
  ```bash
  az aks show \
  --resource-group <resource-group-name> \
  --name <aks-cluster-name> \
  --query "oidcIssuerProfile.issuerUrl" -o tsv
  ```
  Hold onto the OIDC issuer URL for creating federated credentials.

  4. Create a User-Assigned Managed Identity (UAMI) to use for Workload Identity.
  ```bash
  az identity create \
  --name <uami-name> \
  --resource-group <resource-group-name> \
  --location <region>
  ```
  Hold onto the UAMI `clientID` and `principalID` for the next steps.

  5. Assign the Contributor role to the UAMI so it can manage Azure resources.
  ```bash
  az role assignment create \
  --assignee <uami-principal-id> \
  --role Contributor \
  --scope /subscriptions/<subscription-id>
  ```

  6. Add a Federated Credential to the UAMI

To configure the federated credential for the UAMI, follow the detailed instructions in [Azure Workload Identity: Federated identity credential for an Azure AD application](https://azure.github.io/azure-workload-identity/docs/topics/federated-identity-credential.html#federated-identity-credential-for-a-user-assigned-managed-identity).
For CAPZ, the federated credential should be configured for the capz-manager service account in the capz-system namespace, like the below:
```bash
az identity federated-credential create \
  --name <federated-credential-name> \
  --identity-name <uami-name> \
  --resource-group <resource-group-name> \
  --issuer <oidc-issuer-url> \
  --subject "system:serviceaccount:capz-system:capz-manager" \
```

7. Initialize the management cluster

Run the following command to initialize the management cluster with Cluster API Provider Azure (CAPZ):

 `clusterctl init --infrastructure azure`
 
 This command sets up the necessary components, including Cluster API Core, CAPZ, and Azure Service Operator (ASO).
 View the [Cluster API Quick Start: Initialize the management cluster](https://cluster-api.sigs.k8s.io/user/quick-start.html) documentation for more detailed instructions. Ensure you select the "Azure" tabs for Azure-specific guidance. 

  7. Annotate the capz-manager service account in the capz-system namespace with the UAMI's clientId:
  ```bash
  kubectl annotate serviceaccount capz-manager \
  -n capz-system \
  azure.workload.identity/client-id=<uami-client-id>
  ```
## Building your First Cluster

To create a workload cluster, follow the [Cluster API Quick Start: Create your first workload cluster](https://cluster-api.sigs.k8s.io/user/quick-start.html)  for detailed instructions. Ensure you select the "Azure" tabs for Azure-specific guidance.