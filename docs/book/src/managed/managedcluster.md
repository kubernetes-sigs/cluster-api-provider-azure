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

Sections in this document:
- [Deploy with clusterctl](#deploy-with-clusterctl)
- [Specification walkthrough](#specification)
  - [Use an existing Virtual Network to provision an AKS cluster](#use-an-existing-virtual-network-to-provision-an-aks-cluster)
  - [Disable Local Accounts in AKS when using Azure Active Directory](#disable-local-accounts-in-aks-when-using-azure-active-directory)
  - [AKS Fleet Integration](#aks-fleet-integration)
  - [AKS Extensions](#aks-extensions)
  - [Security Profile for AKS clusters](#security-profile-for-aks-clusters)
  - [Enabling Preview API Features for ManagedClusters](#enabling-preview-api-features-for-managedclusters)
  - [OIDC Issuer on AKS](#oidc-issuer-on-aks)
  - [Enable AKS features with custom headers](#enable-aks-features-with-custom-headers---aks-custom-headers)

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
kubectl get cluster my-cluster -o wide
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


### Security Profile for AKS clusters

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
  version: v1.29.4
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

### OIDC Issuer on AKS

Setting `AzureManagedControlPlane.Spec.oidcIssuerProfile.enabled` to `true` will enable OIDC issuer profile for the `AzureManagedControlPlane`. Once enabled, you will see a configmap named `<cluster-name>-aso-oidc-issuer-profile` in the same namespace as the `AzureManagedControlPlane` resource. This configmap will contain the OIDC issuer profile url under the `oidc-issuer-profile-url` key.

Once OIDC issuer is enabled on the cluster, it's not supported to disable it.

To learn more about OIDC and AKS refer [AKS Docs on OIDC issuer](https://learn.microsoft.com/en-us/azure/aks/use-oidc-issuer).

### Enable AKS features with custom headers (--aks-custom-headers)

CAPZ no longer supports passing custom headers to AKS APIs with `infrastructure.cluster.x-k8s.io/custom-header-` annotations.
Custom headers are deprecated in AKS in favor of new features first landing in preview API versions:

https://github.com/Azure/azure-rest-api-specs/pull/18232
