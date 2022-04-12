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
creating an AzureManagedControlPlane requires at least one AzureManagedMachinePool
with `spec.mode` `System`, since AKS expects at least one system pool at creation 
time. For more documentation on system node pool refer [AKS Docs](https://docs.microsoft.com/en-us/azure/aks/use-system-pools) 

## Deploy with clusterctl

A clusterctl flavor exists to deploy an AKS cluster with CAPZ. This
flavor requires the following environment variables to be set before
executing clusterctl.

```bash
# Kubernetes values
export CLUSTER_NAME="my-cluster"
export WORKER_MACHINE_COUNT=2
export KUBERNETES_VERSION="v1.19.6"

# Azure values
export AZURE_LOCATION="southcentralus"
export AZURE_RESOURCE_GROUP="${CLUSTER_NAME}"
# set AZURE_SUBSCRIPTION_ID to the GUID of your subscription
# this example uses an sdk authentication file and parses the subscriptionId with jq
# this file may be created using
#
# `az ad sp create-for-rbac --role Contributor --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}" --sdk-auth > sp.json`
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
clusterctl init --infrastructure azure
clusterctl generate cluster ${CLUSTER_NAME} --kubernetes-version ${KUBERNETES_VERSION} --flavor aks > cluster.yaml

# assumes an existing management cluster
kubectl apply -f cluster.yaml

# check status of created resources
kubectl get cluster-api -o wide
```

## Specification

We'll walk through an example to view available options.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
spec:
  clusterNetwork:
    services:
      cidrBlocks:
      - 192.168.0.0/16
  controlPlaneRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureManagedControlPlane
    name: my-cluster-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureManagedCluster
    name: my-cluster
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  location: southcentralus
  resourceGroupName: foo-bar
  sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
  subscriptionID: 00000000-0000-0000-0000-000000000000 # fake uuid
  version: v1.21.2
  networkPolicy: azure # or calico
  networkPlugin: azure # or kubenet
  sku:
    tier: Free # or Paid
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedCluster
metadata:
  name: my-cluster
---
apiVersion: cluster.x-k8s.io/v1beta1
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
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureManagedMachinePool
        name: agentpool0
        namespace: default
      version: v1.21.2
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: System
  osDiskSizeGB: 30
  sku: Standard_D2s_v3
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  name: agentpool1
spec:
  clusterName: my-cluster
  replicas: 2
  template:
    spec:
      clusterName: my-cluster
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureManagedMachinePool
        name: agentpool1
        namespace: default
      version: v1.21.2
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool1
spec:
  mode: User
  osDiskSizeGB: 40
  sku: Standard_D2s_v4
```

The main features for configuration today are
[networkPolicy](https://docs.microsoft.com/en-us/azure/aks/concepts-network#network-policies) and
[networkPlugin](https://docs.microsoft.com/en-us/azure/aks/concepts-network#azure-virtual-networks).
Other configuration values like subscriptionId and node machine type
should be fairly clear from context.

| option                    | available values              |
|---------------------------|-------------------------------|
| networkPlugin             | azure, kubenet                |
| networkPolicy             | azure, calico                 |


### Multitenancy

Multitenancy for managed clusters can be configured by using `aks-multi-tenancy` flavor. The steps for creating an azure managed identity and mapping it to an `AzureClusterIdentity` are similar to the ones described [here](https://capz.sigs.k8s.io/topics/multitenancy.html).
The `AzureClusterIdentity` object is then mapped to a managed cluster through the `identityRef` field in `AzureManagedControlPlane.spec`.
Following is an example configuration:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  clusterNetwork:
    services:
      cidrBlocks:
      - 192.168.0.0/16
  controlPlaneRef:
    apiVersion: exp.infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureManagedControlPlane
    name: ${CLUSTER_NAME}
  infrastructureRef:
    apiVersion: exp.infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureManagedCluster
    name: ${CLUSTER_NAME}
---
apiVersion: exp.infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  identityRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureClusterIdentity
    name: ${CLUSTER_IDENTITY_NAME}
    namespace: ${CLUSTER_IDENTITY_NAMESPACE}
  location: ${AZURE_LOCATION}
  resourceGroupName: ${AZURE_RESOURCE_GROUP:=${CLUSTER_NAME}}
  sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
  subscriptionID: ${AZURE_SUBSCRIPTION_ID}
  version: ${KUBERNETES_VERSION}
---
```

### AKS Managed Azure Active Directory Integration

Azure Kubernetes Service can be configured to use Azure Active Directory for user authentication.
AAD for managed clusters can be configured by enabling the `managed` spec in `AzureManagedControlPlane` to `true` 
and by providing Azure AD GroupObjectId in `AdminGroupObjectIDs` array. The group is needed as admin group for
the cluster to grant cluster admin permissions. You can use an existing Azure AD group, or create a new one. For more documentation about AAD refer [AKS AAD Docs](https://docs.microsoft.com/en-us/azure/aks/managed-aad)

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  location: southcentralus
  resourceGroupName: foo-bar
  sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
  subscriptionID: fae7cc14-bfba-4471-9435-f945b42a16dd # fake uuid
  version: v1.21.2
  aadProfile:
    managed: true
    adminGroupObjectIDs:
    - 917056a9-8eb5-439c-g679-b34901ade75h # fake admin groupId
```

### AKS Cluster Autoscaler

Azure Kubernetes Service can be configured to use cluster autoscaler by specifying `scaling` spec in the `AzureManagedMachinePool`

```
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: System
  osDiskSizeGB: 30
  sku: Standard_D2s_v3
  scaling:
    minSize: 2
    maxSize: 10
```

### AKS Node Labels to an Agent Pool

You can configure the `NodeLabels` value for each AKS node pool (`AzureManagedMachinePool`) that you define in your spec.

Below an example `nodeLabels` configuration is assigned to `agentpool0`, specifying that each node in the pool will add a label `dedicated : kafka`

```
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: System
  osDiskSizeGB: 512
  sku: Standard_D2s_v3
  nodeLabels:
    dedicated: kafka 
```

### AKS Node Pool MaxPods configuration

You can configure the `MaxPods` value for each AKS node pool (`AzureManagedMachinePool`) that you define in your spec (see [here](https://docs.microsoft.com/en-us/azure/aks/configure-azure-cni#configure-maximum---new-clusters) for the official AKS documentation). This corresponds to the kubelet `--max-pods` configuration (official kubelet configuration documentation can be found [here](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/)).

Below an example `maxPods` configuration is assigned to `agentpool0`, specifying that each node in the pool will enforce a maximum of 24 scheduled pods:

```
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: System
  osDiskSizeGB: 30
  sku: Standard_D2s_v3
  maxPods: 32
```

### AKS Node Pool OsDiskType configuration

You can configure the `OsDiskType` value for each AKS node pool (`AzureManagedMachinePool`) that you define in your spec (see [here](https://docs.microsoft.com/en-us/azure/aks/cluster-configuration#ephemeral-os) for the official AKS documentation). There are two options to choose from: `"Managed"` (the default) or `"Ephemeral"`.

Below an example `osDiskType` configuration is assigned to `agentpool0`, specifying that each node in the pool will use a local, ephemeral OS disk for faster disk I/O at the expense of possible data loss:

```
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: System
  osDiskSizeGB: 30
  sku: Standard_D2s_v3
  osDiskType: "Ephemeral"
```

### AKS Node Pool Taints

You can configure the `Taints` value for each AKS node pool (`AzureManagedMachinePool`) that you define in your spec.

Below is an example of `taints` configuration for the `agentpool0`:

```
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: System
  osDiskSizeGB: 512
  sku: Standard_D2s_v3
  taints:
    - effect: no-schedule
      key: dedicated
      value: kafka
```

### Enable AKS features with custom headers (--aks-custom-headers)
To enable some AKS cluster / node pool features you need to pass special headers to the cluster / node pool create request. 
For example, to [add a node pool for GPU nodes](https://docs.microsoft.com/en-us/azure/aks/gpu-cluster#add-a-node-pool-for-gpu-nodes),
you need to pass a custom header `UseGPUDedicatedVHD=true` (with `--aks-custom-headers UseGPUDedicatedVHD=true` argument). 
To do this with CAPZ, you need to add special annotations to AzureManagedCluster (for cluster 
features) or AzureManagedMachinePool (for node pool features). These annotations should have a prefix `infrastructure.cluster.x-k8s.io/custom-header-` followed 
by the name of the AKS feature. For example, to create a node pool with GPU support, you would add the following 
annotation to AzureManagedMachinePool:
```
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  ...
  annotations:
    "infrastructure.cluster.x-k8s.io/custom-header-UseGPUDedicatedVHD": "true"
  ...
spec:
  ...
```

### Use a public Standard Load Balancer

A public Load Balancer when integrated with AKS serves two purposes:
- To provide outbound connections to the cluster nodes inside the AKS virtual network. It achieves this objective by translating the nodes private IP address to a public IP address that is part of its Outbound Pool.
- To provide access to applications via Kubernetes services of type LoadBalancer. With it, you can easily scale your applications and create highly available services.

For more documentation about public Standard Load Balancer refer [AKS Doc](https://docs.microsoft.com/en-us/azure/aks/load-balancer-standard) and [AKS REST API Doc](https://docs.microsoft.com/en-us/rest/api/aks/managed-clusters/create-or-update)

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  location: southcentralus
  resourceGroupName: foo-bar
  sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
  subscriptionID: 00000000-0000-0000-0000-000000000000 # fake uuid
  version: v1.21.2
  loadBalancerProfile: # Load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs
    managedOutboundIPs: 2 # 1-100
    outboundIPPrefixes:
    - /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/foo-bar/providers/Microsoft.Network/publicIPPrefixes/my-public-ip-prefix # fake public ip prefix
    outboundIPs:
    - /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/foo-bar/providers/Microsoft.Network/publicIPAddresses/my-public-ip # fake public ip
    allocatedOutboundPorts: 100 # 0-64000
    idleTimeoutInMinutes: 10 # 4-120
```

### Secure access to the API server using authorized IP address ranges

In Kubernetes, the API server receives requests to perform actions in the cluster such as to create resources or scale the number of nodes. The API server is the central way to interact with and manage a cluster. To improve cluster security and minimize attacks, the API server should only be accessible from a limited set of IP address ranges.

For more documentation about authorized IP address ranges refer [AKS Doc](https://docs.microsoft.com/en-us/azure/aks/api-server-authorized-ip-ranges) and [AKS REST API Doc](https://docs.microsoft.com/en-us/rest/api/aks/managed-clusters/create-or-update)

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: AzureManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  location: southcentralus
  resourceGroupName: foo-bar
  sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
  subscriptionID: 00000000-0000-0000-0000-000000000000 # fake uuid
  version: v1.21.2
  apiServerAccessProfile:
    authorizedIPRanges:
    - 12.34.56.78/32
    enablePrivateCluster: false
    privateDNSZone: None # System, None. Allowed only when enablePrivateCluster is true
    enablePrivateClusterPublicFQDN: false # Allowed only when enablePrivateCluster is true
```

## Immutable fields for Managed Clusters (AKS)

Some fields from the family of Managed Clusters CRD are immutable. Which means 
those can only be set during the creation time. 

Following is the list of immutable fields for managed clusters:

| CRD                      | jsonPath                           | Comment                   |
|--------------------------|------------------------------------|---------------------------|
| AzureManagedControlPlane | .spec.subscriptionID               |                           |
| AzureManagedControlPlane | .spec.resourceGroupName            |                           |
| AzureManagedControlPlane | .spec.nodeResourceGroupName        |                           |
| AzureManagedControlPlane | .spec.location                     |                           |
| AzureManagedControlPlane | .spec.sshPublicKey                 |                           |
| AzureManagedControlPlane | .spec.dnsServiceIP                 |                           |
| AzureManagedControlPlane | .spec.networkPlugin                |                           |
| AzureManagedControlPlane | .spec.networkPolicy                |                           |
| AzureManagedControlPlane | .spec.loadBalancerSKU              |                           |
| AzureManagedControlPlane | .spec.apiServerAccessProfile       | except AuthorizedIPRanges |
| AzureManagedMachinePool  | .spec.sku                          |                           |
| AzureManagedMachinePool  | .spec.osDiskSizeGB                 |                           |
| AzureManagedMachinePool  | .spec.osDiskType                   |                           |
| AzureManagedMachinePool  | .spec.taints                       |                           |
| AzureManagedMachinePool  | .spec.availabilityZones            |                           |
| AzureManagedMachinePool  | .spec.maxPods                      |                           |

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
- Only supports Azure Active Directory Managed by Azure.
  - We will not support Legacy Azure Active Directory

## Troubleshooting

If a user tries to delete the MachinePool which refers to the last system node pool AzureManagedMachinePool webhook will reject deletion, so time stamp never gets set on the AzureManagedMachinePool. However the timestamp would be set on the MachinePool and would be in deletion state. To recover from this state create a new MachinePool manually referencing the AzureManagedMachinePool, edit the required references and finalizers to link the MachinePool to the AzureManagedMachinePool. In the AzureManagedMachinePool remove the owner reference to the old MachinePool, and set it to the new MachinePool. Once the new MachinePool is pointing to the AzureManagedMachinePool you can delete the old MachinePool. To delete the old MachinePool remove the finalizers in that object.

Here is an Example:

```yaml
# MachinePool deleted 
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  finalizers:             # remove finalizers once new object is pointing to the AzureManagedMachinePool
  - machinepool.cluster.x-k8s.io
  labels:
    cluster.x-k8s.io/cluster-name: capz-managed-aks
  name: agentpool0
  namespace: default
  ownerReferences:
  - apiVersion: cluster.x-k8s.io/v1beta1
    kind: Cluster
    name: capz-managed-aks
    uid: 152ecf45-0a02-4635-987c-1ebb89055fa2
  uid: ae4a235a-f0fa-4252-928a-0e3b4c61dbea
spec:
  clusterName: capz-managed-aks
  minReadySeconds: 0
  providerIDList:
  - azure:///subscriptions/9107f2fb-e486-a434-a948-52e2929b6f18/resourceGroups/MC_rg_capz-managed-aks_eastus/providers/Microsoft.Compute/virtualMachineScaleSets/aks-agentpool0-10226072-vmss/virtualMachines/0
  replicas: 1
  template:
    metadata: {}
    spec:
      bootstrap:
        dataSecretName: ""
      clusterName: capz-managed-aks
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureManagedMachinePool
        name: agentpool0
        namespace: default
      version: v1.21.2

---
# New Machinepool
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  finalizers:
  - machinepool.cluster.x-k8s.io
  generation: 2
  labels:
    cluster.x-k8s.io/cluster-name: capz-managed-aks
  name: agentpool2    # change the name of the machinepool
  namespace: default 
  ownerReferences:
  - apiVersion: cluster.x-k8s.io/v1beta1
    kind: Cluster
    name: capz-managed-aks
    uid: 152ecf45-0a02-4635-987c-1ebb89055fa2   
  # uid: ae4a235a-f0fa-4252-928a-0e3b4c61dbea     # remove the uid set for machinepool
spec:
  clusterName: capz-managed-aks
  minReadySeconds: 0
  providerIDList:
  - azure:///subscriptions/9107f2fb-e486-a434-a948-52e2929b6f18/resourceGroups/MC_rg_capz-managed-aks_eastus/providers/Microsoft.Compute/virtualMachineScaleSets/aks-agentpool0-10226072-vmss/virtualMachines/0  
  replicas: 1
  template:
    metadata: {}
    spec:
      bootstrap:
        dataSecretName: ""
      clusterName: capz-managed-aks
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureManagedMachinePool
        name: agentpool0
        namespace: default
      version: v1.21.2
```
