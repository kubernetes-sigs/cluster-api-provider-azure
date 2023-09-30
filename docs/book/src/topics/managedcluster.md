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
time. For more documentation on system node pool refer [AKS Docs](https://learn.microsoft.com/azure/aks/use-system-pools)

## Deploy with clusterctl

A clusterctl flavor exists to deploy an AKS cluster with CAPZ. This
flavor requires the following environment variables to be set before
executing clusterctl.

```bash
# Kubernetes values
export CLUSTER_NAME="my-cluster"
export WORKER_MACHINE_COUNT=2
export KUBERNETES_VERSION="v1.27.3"

# Azure values
export AZURE_LOCATION="southcentralus"
export AZURE_RESOURCE_GROUP="${CLUSTER_NAME}"
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
export AZURE_TENANT_ID="$(cat sp.json | jq -r .tenantId | tr -d '\n')"
export AZURE_NODE_MACHINE_TYPE="Standard_D2s_v3"
export AZURE_CLUSTER_IDENTITY_SECRET_NAME="cluster-identity-secret"
export AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE="default"
export CLUSTER_IDENTITY_NAME="cluster-identity"
```

Managed clusters require the Cluster API "MachinePool" feature flag enabled. You can do that via an environment variable thusly:

```bash
export EXP_MACHINE_POOL=true
```

Optionally, you can enable the CAPZ "AKSResourceHealth" feature flag as well:

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
    tier: Free # or Standard
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

Please note that we don't declare a configuration for the apiserver endpoint. This configuration data will be populated automatically based on the data returned from AKS API during cluster create as `.spec.controlPlaneEndpoint.Host` and `.spec.controlPlaneEndpoint.Port` in both the `AzureManagedCluster` and `AzureManagedControlPlane` resources. Any user-provided data will be ignored and overwritten by data returned from the AKS API.

The [CAPZ API reference documentation](../reference/v1beta1-api.html) describes all of the available options. See also the AKS API documentation for [Agent Pools](https://learn.microsoft.com/rest/api/aks/agent-pools/create-or-update?tabs=HTTP) and [Managed Clusters](https://learn.microsoft.com/rest/api/aks/managed-clusters/create-or-update?tabs=HTTP).

The main features for configuration are:

- [networkPolicy](https://learn.microsoft.com/azure/aks/concepts-network#network-policies)
- [networkPlugin](https://learn.microsoft.com/azure/aks/concepts-network#azure-virtual-networks)
- [addonProfiles](https://learn.microsoft.com/cli/azure/aks/addon?view=azure-cli-latest#az-aks-addon-list-available) - for additional addons not listed below, look for the `*ADDON_NAME` values in [this code](https://github.com/Azure/azure-cli/blob/main/src/azure-cli/azure/cli/command_modules/acs/_consts.py).

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

### Enable AKS features with custom headers (--aks-custom-headers)

CAPZ no longer supports passing custom headers to AKS APIs with `infrastructure.cluster.x-k8s.io/custom-header-` annotations.
Custom headers are deprecated in AKS in favor of new features first landing in preview API versions:

https://github.com/Azure/azure-rest-api-specs/pull/18232

### Disable Local Accounts in AKS when using Azure Active Directory

When deploying an AKS cluster, local accounts are enabled by default. 
Even when you enable RBAC or Azure AD integration,
--admin access still exists as a non-auditable backdoor option.
Disabling local accounts closes the backdoor access to the cluster
Example to disable local accounts for AAD enabled cluster.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  ...
spec:
  aadProfile:
    managed: true
    adminGroupObjectIDs:
    -  00000000-0000-0000-0000-000000000000 # group object id created in azure.
  disableLocalAccounts: true  
  ...
```

Note: CAPZ and CAPI requires access to the target cluster to maintain and manage the cluster. 
Disabling local accounts will cut off direct access to the target cluster. 
CAPZ and CAPI can access target cluster only via the Service Principal, 
hence the user has to provide appropriate access to the Service Principal to access the target cluster. 
User can do that by adding the Service Principal to the appropriate group defined in Azure and 
add the corresponding group ID in `spec.aadProfile.adminGroupObjectIDs`. 
CAPI and CAPZ will be able to authenticate via AAD while accessing the target cluster.

## Features

AKS clusters deployed from CAPZ currently only support a limited,
"blessed" configuration. This was primarily to keep the initial
implementation simple. If you'd like to run managed AKS cluster with CAPZ
and need an additional feature, please open a pull request or issue with
details. We're happy to help!

## Best Practices

A set of best practices for managing AKS clusters is documented here: https://learn.microsoft.com/azure/aks/best-practices

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

## Joining self-managed VMSS nodes to an AKS control plane

<aside class="note warning">

<h1> Warning </h1>

This is not an officially supported AKS scenario. It is meant to facilitate development and testing of alpha/beta Kubernetes features. Please use at your own risk.

</aside>

### Creating the MachinePool

You can add a self-managed VMSS node pool to any CAPZ-managed AKS cluster by applying the following resources to the management cluster:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  name: ${CLUSTER_NAME}-vmss
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfig
          name: ${CLUSTER_NAME}-vmss
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachinePool
        name: ${CLUSTER_NAME}-vmss
      version: ${KUBERNETES_VERSION}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: ${CLUSTER_NAME}-vmss
  namespace: default
spec:
  location: ${AZURE_LOCATION}
  strategy:
    rollingUpdate:
      deletePolicy: Oldest
      maxSurge: 25%
      maxUnavailable: 1
    type: RollingUpdate
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: Premium_LRS
      osType: Linux
    sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
    vmSize: ${AZURE_NODE_MACHINE_TYPE}
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfig
metadata:
  name: ${CLUSTER_NAME}-vmss
  namespace: default
spec:
  files:
  - contentFrom:
      secret:
        key: worker-node-azure.json
        name: ${CLUSTER_NAME}-vmss-azure-json
    owner: root:root
    path: /etc/kubernetes/azure.json
    permissions: "0644"
  - contentFrom:
      secret:
        key: value
        name: ${CLUSTER_NAME}-kubeconfig
    owner: root:root
    path: /etc/kubernetes/admin.conf
    permissions: "0644"  
  joinConfiguration:
    discovery:
      file:
        kubeConfigPath: /etc/kubernetes/admin.conf
    nodeRegistration:
      kubeletExtraArgs:
        azure-container-registry-config: /etc/kubernetes/azure.json
        cloud-provider: external
      name: '{{ ds.meta_data["local_hostname"] }}'
  preKubeadmCommands:
  - kubeadm init phase upload-config all
  ```

### Installing Addons

In order for the nodes to become ready, you'll need to install Cloud Provider Azure and a CNI.

AKS will install Cloud Provider Azure on the self-managed nodes as long as they have the appropriate labels. You can add the required label on the nodes by running the following command on the AKS cluster:
  
```bash
kubectl label node <node name> kubernetes.azure.com/cluster=<nodeResourceGroupName>
```

Repeat this for each node in the MachinePool.

<aside class="note">

<h1> Warning </h1>

Note: CAPI does not currently support propagating labels from the MachinePool to the nodes, in the future this could be part of the MachinePool definition.

</aside>

For the CNI, you can install the CNI of your choice. For example, to install Azure CNI, run the following command on the AKS cluster:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/addons/azure-cni-v1.yaml
```

### Notes

Some notes about how this works under the hood:

- CAPZ will fetch the kubeconfig for the AKS cluster and store it in a secret named `${CLUSTER_NAME}-kubeconfig` in the management cluster. That secret is then used for discovery by the `KubeadmConfig` resource.
- You can customize the `MachinePool`, `AzureMachinePool`, and `KubeadmConfig` resources to your liking. The example above is just a starting point. Note that the key configurations to keep are in the `KubeadmConfig` resource, namely the `files`, `joinConfiguration`, and `preKubeadmCommands` sections.
- The `KubeadmConfig` resource will be used to generate a `kubeadm join` command that will be executed on each node in the VMSS. It uses the cluster kubeconfig for discovery. The `kubeadm init phase upload-config all` is run as a preKubeadmCommand to ensure that the kubeadm and kubelet configurations are uploaded to a ConfigMap. This step would normally be done by the `kubeadm init` command, but since we're not running `kubeadm init` we need to do it manually.
