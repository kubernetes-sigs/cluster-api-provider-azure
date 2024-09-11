# Deploy cluster on Public MEC

- **Feature status:** Experimental
- **Feature gate:** EdgeZone=true

## Overview

<!-- markdown-link-check-disable-next-line -->
Cluster API Provider Azure (CAPZ) has experimental support for deploying clusters on [Azure Public MEC](https://azure.microsoft.com/solutions/public-multi-access-edge-compute-mec). Before you begin, you need an Azure subscription which has access to Public MEC.

To deploy a cluster on Public MEC, provide extended location info through environment variables and use the "edgezone" flavor.

## Example: Deploy cluster on Public MEC by `clusterctl`

The clusterctl "edgezone" flavor exists to deploy clusters on Public MEC. This flavor requires the following environment variables to be set before executing `clusterctl`.

```bash
# Kubernetes values
export CLUSTER_NAME="my-cluster"
export WORKER_MACHINE_COUNT=2
export CONTROL_PLANE_MACHINE_COUNT=1
export KUBERNETES_VERSION="v1.25.0"

# Azure values
export AZURE_LOCATION="eastus2euap"
export AZURE_EXTENDEDLOCATION_TYPE="EdgeZone"
export AZURE_EXTENDEDLOCATION_NAME="microsoftrrdclab3"
export AZURE_RESOURCE_GROUP="${CLUSTER_NAME}"
```

Create a new service principal and save to local file:
```bash
az ad sp create-for-rbac --role Contributor --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}" --sdk-auth > sp.json
```
Export the following variables to your current shell:
```bash
export AZURE_SUBSCRIPTION_ID="$(cat sp.json | jq -r .subscriptionId | tr -d '\n')"
export AZURE_CLIENT_SECRET="$(cat sp.json | jq -r .clientSecret | tr -d '\n')"
export AZURE_CLIENT_ID="$(cat sp.json | jq -r .clientId | tr -d '\n')"
export AZURE_CONTROL_PLANE_MACHINE_TYPE="Standard_D2s_v3"
export AZURE_NODE_MACHINE_TYPE="Standard_D2s_v3"
export AZURE_CLUSTER_IDENTITY_SECRET_NAME="cluster-identity-secret"
export AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE="default"
export CLUSTER_IDENTITY_NAME="cluster-identity"
```

Public MEC-enabled clusters also require the following feature flags set as environment variables:

```bash
export EXP_EDGEZONE=true
```

Create a local kind cluster to run the management cluster components:

```bash
kind create cluster
```

Create an identity secret on the management cluster:

```bash
kubectl create secret generic "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" --from-literal=clientSecret="${AZURE_CLIENT_SECRET}"
```

Execute clusterctl to template the resources:

```bash
clusterctl init --infrastructure azure
clusterctl generate cluster ${CLUSTER_NAME} --kubernetes-version ${KUBERNETES_VERSION} --flavor edgezone > edgezone-cluster.yaml
```
Public MEC doesn't have access to CAPI images in Azure Marketplace, therefore, users need to prepare CAPI image by themselves. You can follow doc [Custom Images](./custom-images.md) to setup custom image.

Apply the modified template to your kind management cluster:
```bash
kubectl apply -f edgezone-cluster.yaml
```

Once target cluster's control plane is up, install [Azure cloud provider components](https://github.com/kubernetes-sigs/cloud-provider-azure/tree/master/helm/cloud-provider-azure) by helm. The minimum version for "out-of-tree" Azure cloud provider is v1.0.3,  "in-tree" Azure cloud provider is not supported. (Reference: ./addons.md#external-cloud-provider)

```bash
# get the kubeconfig of the cluster
kubectl get secrets ${CLUSTER_NAME}-kubeconfig -o json | jq -r .data.value | base64 --decode > ./kubeconfig

helm install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name --set infra.clusterName=${CLUSTER_NAME} --kubeconfig=./kubeconfig
```
