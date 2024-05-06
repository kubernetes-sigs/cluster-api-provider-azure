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

***NOTE***: `${CLUSTER_NAME}` should adhere to the RFC 1123 standard. This means that it must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.

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

### AKS Fleet Integration

CAPZ supports joining your managed AKS clusters to a single AKS fleet. Azure Kubernetes Fleet Manager (Fleet) enables at-scale management of multiple Azure Kubernetes Service (AKS) clusters. For more documentation on Azure Kubernetes Fleet Manager, refer [AKS Docs](https://learn.microsoft.com/azure/kubernetes-fleet/overview)

To join a CAPZ cluster to an AKS fleet, you must first create an AKS fleet manager. For more information on how to create an AKS fleet manager, refer [AKS Docs](https://learn.microsoft.com/en-us/azure/kubernetes-fleet/quickstart-create-fleet-and-members). This fleet manager will be your point of reference for managing any CAPZ clusters that you join to the fleet.

Once you have created an AKS fleet manager, you can join your CAPZ cluster to the fleet by adding the `fleetsMember` field to your AzureManagedControlPlane resource spec:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  fleetsMember:
    group: fleet-update-group
    managerName: fleet-manager-name
    managerResourceGroup: fleet-manager-resource-group
```

The `managerName` and `managerResourceGroup` fields are the name and resource group of your AKS fleet manager. The `group` field is the name of the update group for the cluster, not to be confused with the resource group.

When the `fleetMember` field is included, CAPZ will create an AKS fleet member resource which will join the CAPZ cluster to the AKS fleet. The AKS fleet member resource will be created in the same resource group as the CAPZ cluster.

### AKS Extensions

CAPZ supports enabling AKS extensions on your managed AKS clusters. Cluster extensions provide an Azure Resource Manager driven experience for installation and lifecycle management of services like Azure Machine Learning or Kubernetes applications on an AKS cluster. For more documentation on AKS extensions, refer [AKS Docs](https://learn.microsoft.com/azure/aks/cluster-extensions).

You can either provision official AKS extensions or Kubernetes applications through Marketplace. Please refer to [AKS Docs](https://learn.microsoft.com/en-us/azure/aks/cluster-extensions#currently-available-extensions) for the list of currently available extensions.

To add an AKS extension to your managed cluster, simply add the `extensions` field to your AzureManagedControlPlane resource spec:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  extensions:
  - name: my-extension
    extensionType: "TraefikLabs.TraefikProxy"
    plan:
      name: "traefik-proxy"
      product: "traefik-proxy"
      publisher: "containous"
```

To list all of the available extensions for your cluster as well as its plan details, use the following az cli command:

```bash
az k8s-extension extension-types list-by-cluster --resource-group my-resource-group --cluster-name mycluster --cluster-type managedClusters
```

For more details, please refer to the [az k8s-extension cli reference](https://learn.microsoft.com/cli/azure/k8s-extension).


### Security Profile for AKS clusters.

Example for configuring AzureManagedControlPlane with a security profile:

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
  version: v1.26.6
  identity:
    type: UserAssigned
    userAssignedIdentityResourceID: /subscriptions/00000000-0000-0000-0000-00000000/resourcegroups/<your-resource-group>/providers/Microsoft.ManagedIdentity/userAssignedIdentities/<your-managed-identity>
  oidcIssuerProfile:
    enabled: true
  securityProfile:
    workloadIdentity:
      enabled: true
    imageCleaner:
      enabled: true
      intervalHours: 48
    azureKeyVaultKms:
      enabled: true
      keyID: https://key-vault.vault.azure.net/keys/secret-key/00000000000000000
    defender:
      logAnalyticsWorkspaceResourceID: /subscriptions/00000000-0000-0000-0000-00000000/resourcegroups/<your-resource-group>/providers/Microsoft.ManagedIdentity/userAssignedIdentities/<your-managed-identity>
      securityMonitoring:
        enabled: true
```

### Enabling Preview API Features for ManagedClusters

#### :warning: WARNING: This is meant to be used sparingly to enable features for development and testing that are not otherwise represented in the CAPZ API. Misconfiguration that conflicts with CAPZ's normal mode of operation is possible.

To enable preview features for managed clusters, you can use the `enablePreviewFeatures` field in the `AzureManagedControlPlane` resource spec. To use any of the new fields included in the preview API version, use the `asoManagedClusterPatches` field in the `AzureManagedControlPlane` resource spec and the `asoManagedClustersAgentPoolPatches` field in the `AzureManagedMachinePool` resource spec to patch in the new fields.

Please refer to the [ASO Docs](https://azure.github.io/azure-service-operator/reference/containerservice/) for the ContainerService API reference for the latest preview fields and their usage.

Example for enabling preview features for managed clusters:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlane
metadata:
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  enablePreviewFeatures: true
  asoManagedClusterPatches:
  - '{"spec": {"enableNamespaceResources": true}}'
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePool
metadata:
  ...
spec:
  asoManagedClustersAgentPoolPatches:
  - '{"spec": {"enableCustomCATrust": true}}'
```

#### OIDC Issuer on AKS

Setting `AzureManagedControlPlane.Spec.oidcIssuerProfile.enabled` to `true` will enable OIDC issuer profile for the `AzureManagedControlPlane`. Once enabled, you will see a configmap named `<cluster-name>-aso-oidc-issuer-profile` in the same namespace as the `AzureManagedControlPlane` resource. This configmap will contain the OIDC issuer profile url under the `oidc-issuer-profile-url` key.

Once OIDC issuer is enabled on the cluster, it's not supported to disable it.

To learn more about OIDC and AKS refer [AKS Docs on OIDC issuer](https://learn.microsoft.com/en-us/azure/aks/use-oidc-issuer).


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

## Adopting Existing AKS Clusters

### Option 1: Using the experimental ASO-based API

<!-- markdown-link-check-disable-next-line -->
The [experimental AzureASOManagedControlPlane and related APIs](/topics/aso.html#experimental-aso-api) support
adoption as a first-class use case. Going forward, this method is likely to be easier, more reliable, include
more features, and better supported for adopting AKS clusters than Option 2 below.

To adopt an AKS cluster into a full Cluster API Cluster, create an ASO ManagedCluster and associated
ManagedClustersAgentPool resources annotated with `sigs.k8s.io/cluster-api-provider-azure-adopt=true`. The
annotation may also be added to existing ASO resources to trigger adoption. CAPZ will automatically scaffold
the Cluster API resources like the Cluster, AzureASOManagedCluster, AzureASOManagedControlPlane, MachinePools,
and AzureASOManagedMachinePools. The [`asoctl import
azure-resource`](https://azure.github.io/azure-service-operator/tools/asoctl/#import-azure-resource) command
can help generate the required YAML.

Caveats:
- The `asoctl import azure-resource` command has at least [one known
  bug](https://github.com/Azure/azure-service-operator/issues/3805) requiring the YAML it generates to be
  edited before it can be applied to a cluster.
- CAPZ currently only records the ASO resources in the CAPZ resources' `spec.resources` that it needs to
  function, which include the ManagedCluster, its ResourceGroup, and associated ManagedClustersAgentPools.
  Other resources owned by the ManagedCluster like Kubernetes extensions or Fleet memberships are not
  currently imported to the CAPZ specs.
- Configuring the automatically generated Cluster API resources is not currently possible. If you need to
  change something like the `metadata.name` of a resource from what CAPZ generates, create the Cluster API
  resources manually referencing the pre-existing resources.
- Adopting existing clusters created with the GA AzureManagedControlPlane API to the experimental API with
  this method is theoretically possible, but untested. Care should be taken to prevent CAPZ from reconciling
  two different representations of the same underlying Azure resources.

### Option 2: Using the current AzureManagedControlPlane API

<aside class="note">

<h1> Warning </h1>

Note: This is a newly-supported feature in CAPZ that is less battle-tested than most other features. Potential
bugs or misuse can result in misconfigured or deleted Azure resources. Use with caution.

</aside>

CAPZ can adopt some AKS clusters created by other means under its management. This works by crafting CAPI and
CAPZ manifests which describe the existing cluster and creating those resources on the CAPI management
cluster. This approach is limited to clusters which can be described by the CAPZ API, which includes the
following constraints:

- the cluster operates within a single Virtual Network and Subnet
- the cluster's Virtual Network exists outside of the AKS-managed `MC_*` resource group
- the cluster's Virtual Network and Subnet are not shared with any other resources outside the context of this cluster

To ensure CAPZ does not introduce any unwarranted changes while adopting an existing cluster, carefully review
the [entire AzureManagedControlPlane spec](../reference/v1beta1-api#infrastructure.cluster.x-k8s.io/v1beta1.AzureManagedControlPlaneSpec)
and specify _every_ field in the CAPZ resource. CAPZ's webhooks apply defaults to many fields which may not
match the existing cluster.

Specific AKS features not represented in the CAPZ API, like those from a newer AKS API version than CAPZ uses,
do not need to be specified in the CAPZ resources to remain configured the way they are. CAPZ will still not
be able to manage that configuration, but it will not modify any settings beyond those for which it has
knowledge.

By default, CAPZ will not make any changes to or delete any pre-existing Resource Group, Virtual Network, or
Subnet resources. To opt-in to CAPZ management for those clusters, tag those resources with the following
before creating the CAPZ resources: `sigs.k8s.io_cluster-api-provider-azure_cluster_<CAPI Cluster name>: owned`.
Managed Cluster and Agent Pool resources do not need this tag in order to be adopted.

After applying the CAPI and CAPZ resources for the cluster, other means of managing the cluster should be
disabled to avoid ongoing conflicts with CAPZ's reconciliation process.

#### Pitfalls

The following describes some specific pieces of configuration that deserve particularly careful attention,
adapted from https://gist.github.com/mtougeron/1e5d7a30df396cd4728a26b2555e0ef0#file-capz-md.

- Make sure `AzureManagedControlPlane.metadata.name` matches the AKS cluster name
- Set the `AzureManagedControlPlane.spec.virtualNetwork` fields to match your existing VNET
- Make sure the `AzureManagedControlPlane.spec.sshPublicKey` matches what was set on the AKS cluster. (including any potential newlines included in the base64 encoding)
  - NOTE: This is a required field in CAPZ, if you don't know what public key was used, you can _change_ or _set_ it via the Azure CLI however before attempting to import the cluster.
- Make sure the `Cluster.spec.clusterNetwork` settings match properly to what you are using in AKS
- Make sure the `AzureManagedControlPlane.spec.dnsServiceIP` matches what is set in AKS
- Set the tag `sigs.k8s.io_cluster-api-provider-azure_cluster_<clusterName>` = `owned` on the AKS cluster
- Set the tag `sigs.k8s.io_cluster-api-provider-azure_role` = `common` on the AKS cluster

NOTE: Several fields, like `networkPlugin`, if not set on the AKS cluster at creation time, will mean that CAPZ will not be able to set that field. AKS doesn't allow such fields to be changed if not set at creation. However, if it was set at creation time, CAPZ will be able to successfully change/manage the field.
