# Managed Clusters (AKS)
- **Feature status:** Experimental
- **Feature gate:** AKS=true,MachinePool=true

Cluster API Provider Azure (CAPZ) experimentally supports managing Azure
Kubernetes Service (AKS) clusters. CAPZ implements this with three
custom resources:
- AzureManagedControlPlane
- AzureManagedCluster
- AzureManagedMachinePool

The combination of AzureManagedControlPlane/AzureManagedCluster
corresponds to provisioning an AKS cluster. AzureManagedMachinePool
corresponds one-to-one with AKS node pools. This also means that
creating an AzureManagedControlPlane requires defining the default
machine pool, since AKS requires at least one system pool at creation
time.

## Deploy with clusterctl

A clusterctl flavor exists to deploy an AKS cluster with CAPZ. This
flavor requires the following environment variables to be set before
executing clusterctl.

```bash
# Kubernetes values
export CLUSTER_NAME="foobar"
export WORKER_MACHINE_COUNT=2
export KUBERNETES_VERSION="v1.17.4"

# Azure values
export AZURE_LOCATION="southcentralus"
export AZURE_RESOURCE_GROUP="${CLUSTER_NAME}"
# set AZURE_SUBSCRIPTION_ID to the GUID of your subscription
# this example uses an sdk authentication file and parses the subscriptionId with jq
# this file may be created using
#
# `az ad sp create-for-rbac --sdk-auth [roles] > sp.json`
#
# when logged in with a service principal, it's also available using
#
# `az account show --sdk-auth`
#
# Otherwise, you can set this value manually.
#
export AZURE_SUBSCRIPTION_ID="$(cat ~/sp.json | jq -r .subscriptionId | tr -d '\n')"
export AZURE_NODE_MACHINE_TYPE="Standard_D2s_v3"
```

Managed clusters also require the following feature flags set as environment variables:

```bash
export EXP_MACHINE_POOL=true
export EXP_AKS=true
```

Execute clusterctl to template the resources, then apply to a management cluster:

```bash
# replace v0.4.4 with the appropriate version you're using
clusterctl config cluster my-cluster --kubernetes-version v1.17.4 --flavor aks -i azure:v0.4.4 > cluster.yaml

# assumes an existing management cluster
kubectl apply -f cluster.yaml

# check status of created resources
kubectl get cluster-api -o wide
```

## Specification

We'll walk through an example to view available options.

```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: Cluster
metadata:
  name: my-cluster
spec:
  clusterNetwork:
    services:
      cidrBlocks:
      - 192.168.0.0/16
  controlPlaneRef:
    apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
    kind: AzureManagedControlPlane
    name: my-cluster-control-plane
  infrastructureRef:
    apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
    kind: AzureManagedCluster
    name: my-cluster
---
apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  defaultPoolRef:
    name: agentpool0
  location: southcentralus
  resourceGroup: foo-bar
  sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
  subscriptionID: fae7cc14-bfba-4471-9435-f945b42a16dd # fake uuid
  version: v1.17.4
  networkPolicy: azure # or calico
  networkPlugin: azure # or kubenet
---
apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureManagedCluster
metadata:
  name: my-cluster
spec:
  subscriptionID: fae7cc14-bfba-4471-9435-f945b42a16dd # fake uuid
---
apiVersion: exp.cluster.x-k8s.io/v1alpha3
kind: MachinePool
metadata:
  name: agentpool0
spec:
  clusterName: my-cluster
  replicas: 2
  template:
    spec:
      clusterName: my-cluster
      infrastructureRef:
        apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureManagedMachinePool
        name: agentpool0
        namespace: default
      version: v1.17.4
---
apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  osDiskSizeGB: 512
  sku: Standard_D8s_v3
```

The main features for configuration today are
[networkPolicy](https://docs.microsoft.com/en-us/azure/aks/concepts-network#network-policies)
and
[networkPlugin](https://docs.microsoft.com/en-us/azure/aks/concepts-network#azure-virtual-networks).
Other configuration values like subscriptionId and node machine type
should be fairly clear from context.

| option        | available values |
|---------------|------------------|
| networkPlugin | azure, kubenet   |
| networkPolicy | azure, calico    |

## Features

AKS clusters deployed from CAPZ currently only support a limited,
"blessed" configuration. This was primarily to keep the initial
implementation simple. If you'd like to run managed AKS cluster with CAPZ
and need an additional feature, please open a pull request or issue with
details. We're happy to help!

Current limitations
- DNS IP is hardcoded to the x.x.x.10 inside the service CIDR.
  - primarily due to lack of validation, see
    https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/612
- Only supports system managed identities.
  - We would like to support user managed identities where appropriate.
- Only supports Standard load balancer (SLB).
  - We will not support Basic load balancer in CAPZ. SLB is generally
    the path forward in Azure.
