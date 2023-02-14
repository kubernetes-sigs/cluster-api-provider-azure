# Managed Clusters (AKS)

- **Feature status:** GA
- **Feature gate:** MachinePool=true

Cluster API Provider Azure (CAPZ) supports managing Azure
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
export KUBERNETES_VERSION="v1.24.6"

# Azure values
export AZURE_LOCATION="southcentralus"
export AZURE_RESOURCE_GROUP="${CLUSTER_NAME}"
# set AZURE_SUBSCRIPTION_ID to the GUID of your subscription
# this example uses an sdk authentication file and parses the subscriptionId with jq
# this file may be created using
```

Create a new service principal and save to a local file:

```bash
az ad sp create-for-rbac --role Contributor --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}" --sdk-auth > sp.json
```

export the following variables in your current shell.

```bash
export AZURE_SUBSCRIPTION_ID="$(cat sp.json | jq -r .subscriptionId | tr -d '\n')"
export AZURE_CLIENT_SECRET="$(cat sp.json | jq -r .clientSecret | tr -d '\n')"
export AZURE_CLIENT_ID="$(cat sp.json | jq -r .clientId | tr -d '\n')"
export AZURE_NODE_MACHINE_TYPE="Standard_D2s_v3"
export AZURE_CLUSTER_IDENTITY_SECRET_NAME="cluster-identity-secret"
export AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE="default"
export CLUSTER_IDENTITY_NAME="cluster-identity"
```

Managed clusters require the Cluster API "MachinePool" feature flag enabled. You can do that via an environment variable thusly:

```bash
export EXP_MACHINE_POOL=true
```

Optionally, the you can enable the CAPZ "AKSResourceHealth" feature flag as well:

```bash
export EXP_AKS_RESOURCE_HEALTH=true
```

Create a local kind cluster to run the management cluster components:

```bash
kind create cluster
```

Create an identity secret on the management cluster:

```bash
kubectl create secret generic "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" --from-literal=clientSecret="${AZURE_CLIENT_SECRET}"
```

Execute clusterctl to template the resources, then apply to your kind management cluster:

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
  addonProfiles:
  - name: azureKeyvaultSecretsProvider
    enabled: true
  - name: azurepolicy
    enabled: true
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

The main features for configuration are:

- [networkPolicy](https://docs.microsoft.com/en-us/azure/aks/concepts-network#network-policies)
- [networkPlugin](https://docs.microsoft.com/en-us/azure/aks/concepts-network#azure-virtual-networks)
- [addonProfiles](https://learn.microsoft.com/cli/azure/aks/addon?view=azure-cli-latest#az-aks-addon-list-available) - for additional addons not listed below, look for the `*ADDON_NAME` values in [this code](https://github.com/Azure/azure-cli/blob/main/src/azure-cli/azure/cli/command_modules/acs/_consts.py).

Other configuration values like subscriptionId and node machine type
should be fairly clear from context.

| option                    | available values              |
|---------------------------|-------------------------------|
| networkPlugin             | azure, kubenet                |
| networkPolicy             | azure, calico                 |

| addon name                | YAML value                |
|---------------------------|---------------------------|
| http_application_routing  | httpApplicationRouting    |
| monitoring                | omsagent                  |
| virtual-node              | aciConnector              |
| kube-dashboard            | kubeDashboard             |
| azure-policy              | azurepolicy               |
| ingress-appgw             | ingressApplicationGateway |
| confcom                   | ACCSGXDevicePlugin        |
| open-service-mesh         | openServiceMesh           |
| azure-keyvault-secrets-provider |  azureKeyvaultSecretsProvider |
| gitops                    | Unsupported?              |
| web_application_routing   | Unsupported?              |

### Use an existing Virtual Network to provision an AKS cluster

If you'd like to deploy your AKS cluster in an existing Virtual Network, but create the cluster itself in a different resource group, you can configure the AzureManagedControlPlane resource with a reference to the existing Virtual Network and subnet. For example:

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
  virtualNetwork:
    cidrBlock: 10.0.0.0/8
    name: test-vnet
    resourceGroup: test-rg
    subnet:
      cidrBlock: 10.0.2.0/24
      name: test-subnet
```

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

Azure Kubernetes Service can have the cluster autoscaler enabled by specifying `scaling` spec in any of the `AzureManagedMachinePool` defined.

```yaml
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

The cluster autoscaler behavior settings can be set in the `AzureManagedControlPlane`. Not setting a property will default to the value used by AKS. All values are expected to be strings.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  autoscalerProfile:
    balanceSimilarNodeGroups: "false"
    expander: "random"
    maxEmptyBulkDelete: "10"
    maxGracefulTerminationSec: "600"
    maxNodeProvisionTime: "15m"
    maxTotalUnreadyPercentage: "45"
    newPodScaleUpDelay: "0s"
    okTotalUnreadyCount: "3"
    scanInterval: "10s"
    scaleDownDelayAfterAdd: "10m"
    scaleDownDelayAfterDelete: "10s"
    scaleDownDelayAfterFailure: "3m"
    scaleDownUnneededTime: "10m"
    scaleDownUnreadyTime: "20m"
    scaleDownUtilizationThreshold: "0.5"
    skipNodesWithLocalStorage: "false"
    skipNodesWithSystemPods: "true"
```

### AKS Node Labels to an Agent Pool

You can configure the `NodeLabels` value for each AKS node pool (`AzureManagedMachinePool`) that you define in your spec.

Below an example `nodeLabels` configuration is assigned to `agentpool0`, specifying that each node in the pool will add a label `dedicated : kafka`

```yaml
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

```yaml
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

```yaml
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

## AKS Node Pool KubeletDiskType configuration

You can configure the `KubeletDiskType` value for each AKS node pool (`AzureManagedMachinePool`) that you define in your spec (see [here](https://learn.microsoft.com/en-us/rest/api/aks/agent-pools/create-or-update?tabs=HTTP#kubeletdisktype) for the official AKS documentation). There are two options to choose from: `"OS"` or `"Temporary"`.

Before this feature can be used, you must register the `KubeletDisk` feature on your Azure subscription with the following az cli command.

```bash
az feature register --namespace Microsoft.ContainerService --name KubeletDisk
```

Below an example `kubeletDiskType` configuration is assigned to `agentpool0`, specifying that the emptyDir volumes, container runtime data root, and Kubelet ephemeral storage will be stored on the temporary disk:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: System
  osDiskSizeGB: 30
  sku: Standard_D2s_v3
  kubeletDiskType: "Temporary"
```

### AKS Node Pool Taints

You can configure the `Taints` value for each AKS node pool (`AzureManagedMachinePool`) that you define in your spec.

Below is an example of `taints` configuration for the `agentpool0`:

```yaml
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

### AKS Node Pool OS Type

If your cluster uses the Azure network plugin (`AzureManagedControlPlane.networkPlugin`) you can set the operating system
for your User nodepools. The `osType` field is immutable and only can be set at creation time, it defaults to `Linux` and
can be either `Linux` or `Windows`.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: agentpool0
spec:
  mode: User
  osDiskSizeGB: 30
  sku: Standard_D2s_v3
  osDiskType: "Ephemeral"
  osType: Windows
```

### AKS Node Pool Kubelet Custom Configuration

Reference:

- https://learn.microsoft.com/en-us/azure/aks/custom-node-configuration

When you create your node pool (`AzureManagedMachinePool`), you may specify various kubelet configuration which tunes the kubelet runtime on all nodes in that pool. For example:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: pool1
spec:
  mode: User
  kubeletConfig:
    cpuManagerPolicy: "static"
    cpuCfsQuota: true
    cpuCfsQuotaPeriod: "110ms"
    imageGcHighThreshold: 70
    imageGcLowThreshold: 50
    topologyManagerPolicy: "best-effort"
    allowedUnsafeSysctls:
      - "net.*"
      - "kernel.msg*"
    failSwapOn: false
    containerLogMaxSizeMB: 500
    containerLogMaxFiles: 50
    podMaxPids: 2048
```

Below are the full set of AKS-supported kubeletConfig configurations. All properties are children of the `spec.kubeletConfig` configuration in an `AzureManagedMachinePool` resource:

| Configuration               | Property Type     | Allowed Value(s)                                                                         |
|-----------------------------|-------------------|------------------------------------------------------------------------------------------|
| `cpuManagerPolicy`          | string            | `"none"`, `"static"`                                                                     |
| `cpuCfsQuota`               | boolean           | `true`, `false`                                                                          |
| `cpuCfsQuotaPeriod`         | string            | value in milliseconds, must end in `"ms"`, e.g., `"100ms"`                               |
| `failSwapOn`                | boolean           | `true`, `false`                                                                          |
| `imageGcHighThreshold`      | integer           | integer values in the range 0-100 (inclusive)                                            |
| `imageGcLowThreshold`       | integer           | integer values in the range 0-100 (inclusive), must be lower than `imageGcHighThreshold` |
| `topologyManagerPolicy`     | string            | `"none"`, `"best-effort"`, `"restricted"`, `"single-numa-node"`                          |
| `allowedUnsafeSysctls`      | string            | `"kernel.shm*"`, `"kernel.msg*"`, `"kernel.sem"`, `"fs.mqueue.*"`, `"net.*"`             |
| `containerLogMaxSizeMB`     | integer           | any integer                                                                              |
| `containerLogMaxFiles`      | integer           | any integer >= `2`                                                                       |
| `podMaxPids`                | integer           | any integer >= `-1`, note that this must not be higher than kernel PID limit             |

For more detailed information on the behaviors of the above configurations, see [the official Kubernetes documentation](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/). Note that not all possible Kubernetes Kubelet Configuration options are available to use on your AKS node pool, only those specified above.

CAPZ will not assign any default values for any excluded configuration properties. It is also not required to include the `spec.kubeletConfig` configuration in an `AzureManagedMachinePool` resource spec. In cases where no CAPZ configuration is declared, AKS will apply its own opinionated default configurations when the node pool is created.

Note: these configurations can not be updated after a node pool is created.

### Enable AKS features with custom headers (--aks-custom-headers)

To enable some AKS cluster / node pool features you need to pass special headers to the cluster / node pool create request.
For example, to [add a node pool for GPU nodes](https://docs.microsoft.com/en-us/azure/aks/gpu-cluster#add-a-node-pool-for-gpu-nodes),
you need to pass a custom header `UseGPUDedicatedVHD=true` (with `--aks-custom-headers UseGPUDedicatedVHD=true` argument).
To do this with CAPZ, you need to add special annotations to AzureManagedCluster (for cluster
features) or AzureManagedMachinePool (for node pool features). These annotations should have a prefix `infrastructure.cluster.x-k8s.io/custom-header-` followed
by the name of the AKS feature. For example, to create a node pool with GPU support, you would add the following
annotation to AzureManagedMachinePool:

```yaml
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

### OS configurations of Linux agent nodes (AKS)

Reference:

- [How-to-guide Linux OS Custom Configuration](https://learn.microsoft.com/en-us/azure/aks/custom-node-configuration#linux-os-custom-configuration)
- [AKS API definition Linux OS Config](https://learn.microsoft.com/en-us/rest/api/aks/agent-pools/create-or-update?tabs=HTTP#linuxosconfig)

When you create your node pool (`AzureManagedMachinePool`), you can specify configuration which tunes the linux OS configuration on all nodes in that pool. For example:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  name: "${CLUSTER_NAME}-pool1"
spec:
  linuxOSConfig:
    swapFileSizeMB: 1500
    sysctls:
      fsAioMaxNr: 65536
      fsFileMax: 8192
      fsInotifyMaxUserWatches: 781250
      fsNrOpen: 8192
      kernelThreadsMax: 20
      netCoreNetdevMaxBacklog: 1000
      netCoreOptmemMax: 20480
      netCoreRmemDefault: 212992
      netCoreRmemMax: 212992
      netCoreSomaxconn: 163849
      netCoreWmemDefault: 212992
      netCoreWmemMax: 212992
      netIpv4IPLocalPortRange: "32000 60000"
      netIpv4NeighDefaultGcThresh1: 128
      netIpv4NeighDefaultGcThresh2: 512
      netIpv4NeighDefaultGcThresh3: 1024
      netIpv4TCPFinTimeout: 5
      netIpv4TCPKeepaliveProbes: 1
      netIpv4TCPKeepaliveTime: 30
      netIpv4TCPMaxSynBacklog: 128
      netIpv4TCPMaxTwBuckets: 8000
      netIpv4TCPTwReuse: true
      netIpv4TCPkeepaliveIntvl: 10
      netNetfilterNfConntrackBuckets: 65536
      netNetfilterNfConntrackMax: 131072
      vmMaxMapCount: 65530
      vmSwappiness: 10
      vmVfsCachePressure: 15
    transparentHugePageDefrag: "defer+madvise"
    transparentHugePageEnabled: "madvise"
```

Below are the full set of AKS-supported `linuxOSConfig` configurations. All properties are children of the `spec.linuxOSConfig` configuration in an `AzureManagedMachinePool` resource:

| Configuration               | Property Type     | Allowed Value(s)                                                                         |
|-----------------------------|-------------------|------------------------------------------------------------------------------------------|
| `swapFileSizeMB`            | integer           | minimum value `1`.                                                                       |
| `sysctls`                   | SysctlConfig      |                                                                                          |
| `transparentHugePageDefrag` | string            | `"always"`, `"defer"`, `"defer+madvise"`, `"madvise"` or `"never"`                       |
| `transparentHugePageEnabled`| string            | `"always"`, `"madvise"` or `"never"`                                                     |

**Note**: To enable swap file on nodes, i.e.`swapFileSizeMB` to be applied, `Kubeletconfig.failSwapOn` must be set to `false`

#### SysctlsConfig

Below are the full set of supported `SysctlConfig` configurations. All properties are children of the `spec.linuxOSConfig.sysctls` configuration in an `AzureManagedMachinePool` resource:

| Configuration                   | Property Type     | Allowed Value(s)                                                                         |
|---------------------------------|-------------------|------------------------------------------------------------------------------------------|
| `fsAioMaxNr`                    | integer           | allowed value in the range [`65536` - `6553500`] (inclusive)                             |
| `fsFileMax`                     | integer           | allowed value in the range [`8192` - `12000500`] (inclusive)                             |
| `fsInotifyMaxUserWatches`       | integer           | allowed value in the range [`781250` - `2097152`] (inclusive)                            |
| `fsNrOpen`                      | integer           | allowed value in the range [`8192` - `20000500`] (inclusive)                             |
| `kernelThreadsMax`              | integer           | allowed value in the range [`20` - `513785`] (inclusive)                                 |
| `netCoreNetdevMaxBacklog`       | integer           | allowed value in the range [`1000` - `3240000`] (inclusive)                              |
| `netCoreOptmemMax`              | integer           | allowed value in the range [`20480` - `4194304`] (inclusive)                             |
| `netCoreRmemDefault`            | integer           | allowed value in the range [`212992` - `134217728`] (inclusive)                          |
| `netCoreRmemMax`                | integer           | allowed value in the range [`212992` - `134217728`] (inclusive)                          |
| `netCoreSomaxconn`              | integer           | allowed value in the range [`4096` - `3240000`] (inclusive)                              |
| `netCoreWmemDefault`            | integer           | allowed value in the range [`212992` - `134217728`] (inclusive)                          |
| `netCoreWmemMax`                | integer           | allowed value in the range [`212992`- `134217728`] (inclusive)                           |
| `netIpv4IPLocalPortRange`       | string            | Must be specified as `"first last"`. Ex: `1024 33000`. First must be in `[1024 - 60999]` and last must be in `[32768 - 65000]`|
| `netIpv4NeighDefaultGcThresh1`  | integer           | allowed value in the range [`128` - `80000`] (inclusive)                                 |
| `netIpv4NeighDefaultGcThresh2`  | integer           | allowed value in the range [`512` - `90000`] (inclusive)                                 |
| `netIpv4NeighDefaultGcThresh3`  | integer           | allowed value in the range [`1024` - `100000`] (inclusive)                               |
| `netIpv4TCPFinTimeout`          | integer           | allowed value in the range [`5` - `120`] (inclusive)                                     |
| `netIpv4TCPKeepaliveProbes`     | integer           | allowed value in the range [`1` - `15`] (inclusive)                                      |
| `netIpv4TCPKeepaliveTime`       | integer           | allowed value in the range [`30` - `432000`] (inclusive)                                 |
| `netIpv4TCPMaxSynBacklog`       | integer           | allowed value in the range [`128` - `3240000`] (inclusive)                               |
| `netIpv4TCPMaxTwBuckets`        | integer           | allowed value in the range [`8000` - `1440000`] (inclusive)                              |
| `netIpv4TCPTwReuse`             | bool              | allowed values `true` or `false`                                                         |
| `netIpv4TCPkeepaliveIntvl`      | integer           | allowed value in the range [`1` - `75`] (inclusive)                                      |
| `netNetfilterNfConntrackBuckets`| integer           | allowed value in the range [`65536` - `147456`] (inclusive)                              |
| `netNetfilterNfConntrackMax`    | integer           | allowed value in the range [`131072` - `1048576`] (inclusive)                            |
| `vmMaxMapCount`                 | integer           | allowed value in the range [`65530` - `262144`] (inclusive)                              |
| `vmSwappiness`                  | integer           | allowed value in the range [`0` - `100`] (inclusive)                                     |
| `vmVfsCachePressure`            | integer           | allowed value in the range [`1` - `500`] (inclusive)                                     |

**Note**: Both of the values must be specified to enforce `NetIpv4IPLocalPortRange`.

## Immutable fields for Managed Clusters (AKS)

Some fields from the family of Managed Clusters CRD are immutable. Which means
those can only be set during the creation time.

Following is the list of immutable fields for managed clusters:

| CRD                       | jsonPath                     | Comment                   |
|---------------------------|------------------------------|---------------------------|
| AzureManagedControlPlane  | .name                        |                           |
| AzureManagedControlPlane  | .spec.subscriptionID         |                           |
| AzureManagedControlPlane  | .spec.resourceGroupName      |                           |
| AzureManagedControlPlane  | .spec.nodeResourceGroupName  |                           |
| AzureManagedControlPlane  | .spec.location               |                           |
| AzureManagedControlPlane  | .spec.sshPublicKey           |                           |
| AzureManagedControlPlane  | .spec.dnsServiceIP           |                           |
| AzureManagedControlPlane  | .spec.networkPlugin          |                           |
| AzureManagedControlPlane  | .spec.networkPolicy          |                           |
| AzureManagedControlPlane  | .spec.loadBalancerSKU        |                           |
| AzureManagedControlPlane  | .spec.apiServerAccessProfile | except AuthorizedIPRanges |
| AzureManagedControlPlane  | .spec.virtualNetwork         |                           |
| AzureManagedControlPlane  | .spec.virtualNetwork.subnet  | except serviceEndpoints   |
| AzureManagedMachinePool   | .spec.name                   |                           |
| AzureManagedMachinePool   | .spec.sku                    |                           |
| AzureManagedMachinePool   | .spec.osDiskSizeGB           |                           |
| AzureManagedMachinePool   | .spec.osDiskType             |                           |
| AzureManagedMachinePool   | .spec.availabilityZones      |                           |
| AzureManagedMachinePool   | .spec.maxPods                |                           |
| AzureManagedMachinePool   | .spec.osType                 |                           |
| AzureManagedMachinePool   | .spec.enableNodePublicIP     |                           |
| AzureManagedMachinePool   | .spec.nodePublicIPPrefixID   |                           |
| AzureManagedMachinePool   | .spec.kubeletConfig          |                           |
| AzureManagedMachinePool   | .spec.linuxOSConfig          |                           |

## Features

AKS clusters deployed from CAPZ currently only support a limited,
"blessed" configuration. This was primarily to keep the initial
implementation simple. If you'd like to run managed AKS cluster with CAPZ
and need an additional feature, please open a pull request or issue with
details. We're happy to help!

Current limitations

- DNS IP is hardcoded to the x.x.x.10 inside the service CIDR.
  - primarily due to lack of validation, see [#612](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/612)
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
