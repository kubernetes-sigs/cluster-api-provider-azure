# Identity

Managed identities for Azure resources is a feature of Azure Active Directory. Each of the [Azure services that support managed identities for Azure resources](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/services-support-msi) are subject to their own timeline. Make sure you review the [availability](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/services-support-msi) status of managed identities for your resource and [known issues](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/known-issues) before you begin. 

This feature is used to create nodes which have an identity provisioned onto the node by the Azure control plane, rather than providing credentials in the azure.json file. This is a preferred way to manage identities and roles for a given resource in Azure as the lifespan of the identity is linked to the lifespan of the resource.

### Flavors of Identities in Azure
All identities used in Azure are owned by Azure Active Directory (AAD). An identity, or principal, in AAD will provide the basis for each of the flavors of identities we will describe.

### Service Principal
A service principal is an identity in AAD which is described by a TenantID, ClientID, and ClientSecret. The set of these three values will enable the holder to exchange the values for a JWT token to communicate with Azure. The values are normally stored in a file or environment variables. The user generally creates a service principal, saves the credentials, and then uses the credentials in applications. You can read more about
Service Principals and AD Applications: ["Application and service principal objects in Azure Active Directory"](https://azure.microsoft.com/en-us/documentation/articles/active-directory-application-objects/).

#### Creating a Service Principal

* **With the [Azure CLI](https://github.com/Azure/azure-cli)**

  * Subscription level scope
     ```shell
     az login
     az account set --subscription="${AZURE_SUBSCRIPTION_ID}"
     az ad sp create-for-rbac --role="Owner" --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}"
     ```
  * Resource group level scope
     ```shell
     az login
     az account set --subscription="${AZURE_SUBSCRIPTION_ID}"
     az ad sp create-for-rbac --role="Owner" --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}"
     ```

   This will output your `appId`, `password`, `name`, and `tenant`.  The `name` or `appId` is used for the `AZURE_CLIENT_ID` and the `password` is used for `AZURE_CLIENT_SECRET`.

   Confirm your service principal by opening a new shell and run the following commands substituting in `name`, `password`, and `tenant`:

   ```shell
   az login --service-principal -u NAME -p PASSWORD --tenant TENANT
   az vm list-sizes --location eastus
   ```


### System-assigned managed identity
A system assigned identity is a managed identity which is tied to the lifespan of a resource in Azure. The identity is created by Azure in AAD for the resource it is applied upon and reaped when the resource is deleted. Unlike a service principal, a system assigned identity is available on the local resource through a local port service via the instance metadata service.

⚠️  **When a Node is created with a System Assigned Identity, A role of Subscription contributor is added to this generated Identity** 

### User-assigned managed identity

A standalone Azure resource that is created by the user outside of the scope of this provider. The identity can be assigned to one or more Azure Machines. The lifecycle of a user-assigned identity is managed separately from the lifecycle of the Azure Machines to which it's assigned.

Full details on how to create and manage user assigned identities using [Azure CLI](https://github.com/Azure/azure-cli) can be found in the [Azure docs](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-manage-ua-identity-cli).

### How to use managed identity

#### System-assigned managed identity

* In Machine Deployment
```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: MachineDeployment
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  selector:
    matchLabels: null
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-0
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-0
      version: ${KUBERNETES_VERSION}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  template:
    spec:
      identity: SystemAssigned
      location: ${AZURE_LOCATION}
      osDisk:
        diskSizeGB: 128
        managedDisk:
          storageAccountType: Premium_LRS
        osType: Linux
      sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
      vmSize: ${AZURE_NODE_MACHINE_TYPE}
---
```

The CAPZ controller will look for `SystemAssigned` value in `identity` field under `AzureMachineTemplate`, and enable system-assigned managed identity in the virtual machine.

* In Machine Pool
```yaml
apiVersion: exp.cluster.x-k8s.io/v1alpha3
kind: MachinePool
metadata:
  name: ${CLUSTER_NAME}-mp-0
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfig
          name: ${CLUSTER_NAME}-mp-0
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureMachinePool
        name: ${CLUSTER_NAME}-mp-0
      version: ${KUBERNETES_VERSION}
---
apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureMachinePool
metadata:
  name: ${CLUSTER_NAME}-mp-0
  namespace: default
spec:
  identity: SystemAssigned
  location: ${AZURE_LOCATION}
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: Premium_LRS
      osType: Linux
    sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
    vmSize: ${AZURE_NODE_MACHINE_TYPE}
---
```

The CAPZ controller will look for `SystemAssigned` value in `identity` field under `AzureMachinePool`, and enable system-assigned managed identity in the virtual machine scale set.

Alternatively, you can also use the `system-assigned-identity`, and `machinepool-system-assigned-identity` flavors by setting the `{flavor}` in `clusterctl config cluster --flavor {flavor}` to use system-assigned managed identity in machine deployment, and machine pool respectively.

#### User-assigned managed identity

* In Machine Deployment

```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: MachineDeployment
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  selector:
    matchLabels: null
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-0
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-0
      version: ${KUBERNETES_VERSION}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  template:
    spec:
      identity: UserAssigned
      location: ${AZURE_LOCATION}
      osDisk:
        diskSizeGB: 128
        managedDisk:
          storageAccountType: Premium_LRS
        osType: Linux
      sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
      userAssignedIdentities:
      - providerID: ${USER_ASSIGNED_IDENTITY_PROVIDER_ID}
      vmSize: ${AZURE_NODE_MACHINE_TYPE}
---
```

The CAPZ controller will look for `UserAssigned` value in `identity` field under `AzureMachineTemplate`, and assign the user identities listed in `userAssignedIdentities` to the virtual machine.

* In Machine Pool

```yaml
apiVersion: exp.cluster.x-k8s.io/v1alpha3
kind: MachinePool
metadata:
  name: ${CLUSTER_NAME}-mp-0
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfig
          name: ${CLUSTER_NAME}-mp-0
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureMachinePool
        name: ${CLUSTER_NAME}-mp-0
      version: ${KUBERNETES_VERSION}
---
apiVersion: exp.infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureMachinePool
metadata:
  name: ${CLUSTER_NAME}-mp-0
  namespace: default
spec:
  identity: UserAssigned
  location: ${AZURE_LOCATION}
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: Premium_LRS
      osType: Linux
    sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
    vmSize: ${AZURE_NODE_MACHINE_TYPE}
  userAssignedIdentities:
  - providerID: ${USER_ASSIGNED_IDENTITY_PROVIDER_ID}
---
```

The CAPZ controller will look for `UserAssigned` value in `identity` field under `AzureMachinePool`, and assign the user identities listed in `userAssignedIdentities` to the virtual machine scale set.

Similar to system assigned identity, you can use the `user-assigned-identity`, and `machinepool-user-assigned-identity` flavors by setting the `{flavor}` in `clusterctl config cluster --flavor {flavor}` to use user-assigned managed identity in machine deployment, and machine pool respectively.
