# Cluster API Provider Azure Helm Chart

## Prerequisites

- [Docker Desktop](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/)
- [ClusterCTL](https://cluster-api.sigs.k8s.io/clusterctl/overview.html) Version v1.6.1 or older
- [Helm](https://helm.sh) version v3.14.0 or later

## Prerequisites Installations

- Docker Desktop
 <https://www.docker.com/products/docker-desktop>

- Install Kind
  <https://kind.sigs.k8s.io/>

- Install Clusterctl
  <https://cluster-api.sigs.k8s.io/clusterctl/overview.html>

- Install Helm3
- <https://helm.sh/docs/intro/install/>

## Usage

To install the Helm chart, run the following command:

```bash

helm repo add capz https://kubernetes-sigs.github.io/azure-managed-cluster-capz-helm

```

### Create an Azure Service Principal or user-assigned Managed Identity

- [How to create a service principal](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli?view=azure-cli-latest)
- [How to create a User-assigned Managed Identity](https://learn.microsoft.com/en-gb/entra/identity/managed-identities-azure-resources/how-manage-user-assigned-managed-identities?pivots=identity-mi-methods-azp#create-a-user-assigned-managed-identity)

Set the following environment variables that are needed based on [The Cluster API Provider Azure documentation](https://capz.sigs.k8s.io/topics/managedcluster)

### Create a KIND cluster (Kind clusters only work service principal)

```bash
kind create cluster --name capi-helm
```

### Initialize Cluster API and install Azure CAPZ provider

```bash
clusterctl init --infrastructure azure
```

### Deploy a cluster with Helm (please customize parameters as required)

The `values.yaml` file contains the default values for the helm chart. You can override these values by creating a new values file and passing it to the helm install command.

```bash

**Using Service Principal:**

```bash

```bash
helm install capz1 capz/azure-managed-cluster  \
--namespace default \
--set controlplane.sshPublicKey="$(cat ~/.ssh/id_rsa.pub)" \
--set subscriptionID="${AZURE_SUBSCRIPTION_ID}" \
--set identity.clientID="${AZURE_CLIENT_ID}" \
--set identity.tenantID="${AZURE_TENANT_ID}" \
--set identity.clientSecret="${AZURE_CLIENT_SECRET}" \
--set identity.type=ServicePrincipal 
```

**Using Managed Identity**

NB: Ensure the AZURE_USER_ASSIGNED_IDENTITY_RESOURCE_ID is set by getting the resource id of the managed identity created in Azure and it has the required permissions.

```bash

helm install capz1 capz/azure-managed-cluster  \
--namespace default \
--set subscriptionID="${AZURE_SUBSCRIPTION_ID}" \
--set identity.clientID="${AZURE_CLIENT_ID}" \
--set identity.tenantID="${AZURE_TENANT_ID}" \
--set identity.type=UserAssignedMSI \
--set identity.resourceID="${AZURE_USER_ASSIGNED_IDENTITY_RESOURCE_ID}" 
```

Check the status with:

```bash
kubectl get cluster-api
kubectl  logs -n capz-system -l control-plane=capz-controller-manager -c manager -f
```

Get the credentials

```bash
kubectl get secret capi-helm-kubeconfig -o yaml -o jsonpath={.data.value} | base64 --decode > aks1.kubeconfig
kubectl get secret aks-cluster-api-kubeconfig -o yaml -o jsonpath={.data.value} | base64 --decode > aks1.kubeconfig
```

Test the cluster!

```bash
kubectl --kubeconfig=aks1.kubeconfig cluster-info
```

Clean up:

```bash
helm delete capz1 -n default
kubectl delete namespace default
```