# VM Identity

This document describes the available identities that be configured on the Azure host. For example, this is what grants permissions to the Azure Cloud Provider to provision LB services in Azure on the control plane nodes.

## Flavors of Identities in Azure

All identities used in Azure are owned by Azure Active Directory (AAD). An identity, or principal, in AAD will provide the basis for each of the flavors of identities we will describe.

### Managed Identities

Managed identity is a feature of Azure Active Directory (AAD) and Azure Resource Manager (ARM), which assigns ARM Role Base Access Control (RBAC) rights to AAD identities for use in Azure resources, like Virtual Machines. Each of the [Azure services that support managed identities for Azure resources](https://learn.microsoft.com/azure/active-directory/managed-identities-azure-resources/services-support-msi) are subject to their own timeline. Make sure you review the [availability](https://learn.microsoft.com/azure/active-directory/managed-identities-azure-resources/services-support-msi) status of managed identities for your resource and [known issues](https://learn.microsoft.com/azure/active-directory/managed-identities-azure-resources/known-issues) before you begin.

Managed identity is used to create nodes which have an AAD identity provisioned onto the node by Azure Resource Manager (the Azure control plane) rather than providing credentials in the azure.json file. Managed identities are the preferred way to provide RBAC rights for a given resource in Azure as the lifespan of the identity is linked to the lifespan of the resource.

### User-assigned managed identity (recommended)

A standalone Azure resource that is created by the user outside of the scope of this provider. The identity can be assigned to one or more Azure Machines. The lifecycle of a user-assigned identity is managed separately from the lifecycle of the Azure Machines to which it's assigned.

This lifecycle allows you to separate your resource creation and identity administration responsibilities. User-assigned identities and their role assignments can be configured in advance of the resources that require them. Users who create the resources only require the access to assign a user-assigned identity, without the need to create new identities or role assignments.

Full details on how to create and manage user assigned identities using [Azure CLI](https://github.com/Azure/azure-cli) can be found in the [Azure docs](https://learn.microsoft.com/azure/active-directory/managed-identities-azure-resources/how-to-manage-ua-identity-cli).

### System-assigned managed identity
A system-assigned identity is a managed identity which is tied to the lifespan of a resource in Azure. The identity is created by Azure in AAD for the resource it is applied upon and reaped when the resource is deleted. Unlike a service principal, a system assigned identity is available on the local resource through a local port service via the [instance metadata service](https://learn.microsoft.com/azure/virtual-machines/linux/instance-metadata-service?tabs=linux).

⚠️  **When a Node is created with a System Assigned Identity, A role of Subscription contributor is added to this generated Identity**

<aside class="note warning">

<h1> Warning </h1>

To create an Azure VM with the system-assigned managed identity enabled, your AzureClusterIdentity needs the [Virtual Machine Contributor](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles#virtual-machine-contributor) role assignment. In order to be able to grant the subscription contributor role to the identity, it also needs `Microsoft.Authorization/roleAssignments/write` permissions, such as [User Access Administrator](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles#user-access-administrator) or [Owner](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles#owner).

</aside>

### How to use managed identity

#### User-assigned

<aside class="note">

<h1> Note </h1>

While CAPZ allows you to specify multiple user-assigned identities, only the first one will be used for Cloud Provider authentication. The other identities are left at the user's discretion for other use cases.

The first user assigned identity should have the `Contributor` role on the resource group or, if resources are spread across multiple resource groups (e.g. custom vnet in a separate RG), `Contributor` role on the subscription. You may also want to grant `acrpull` permissions to allow your nodes to access [Azure Container Registries](https://learn.microsoft.com/azure/container-registry/container-registry-authentication-managed-identity).

</aside>

* In Machines

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  template:
    spec:
      identity: UserAssigned
      userAssignedIdentities:
      - providerID: ${USER_ASSIGNED_IDENTITY_PROVIDER_ID}
      ...
```

The CAPZ controller will look for `UserAssigned` value in `identity` field under `AzureMachineTemplate`, and assign the user identities listed in `userAssignedIdentities` to the virtual machine.

* In Machine Pool

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: ${CLUSTER_NAME}-mp-0
  namespace: default
spec:
  identity: UserAssigned
  userAssignedIdentities:
  - providerID: ${USER_ASSIGNED_IDENTITY_PROVIDER_ID}
  ...
```

The CAPZ controller will look for `UserAssigned` value in `identity` field under `AzureMachinePool`, and assign the user identities listed in `userAssignedIdentities` to the virtual machine scale set.

Alternatively, you can also use the `user-assigned-identity` flavor to build a simple machine deployment-enabled cluster by using `clusterctl generate cluster --flavor user-assigned-identity` to generate a cluster template.

#### System-assigned

* In Machines

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  template:
    spec:
      identity: SystemAssigned
      ...
```

The CAPZ controller will look for `SystemAssigned` value in `identity` field under `AzureMachineTemplate`, and enable system-assigned managed identity in the virtual machine.

For more granularity regarding permissions, you can specify the scope and the role assignment of the system-assigned managed identity by setting the `scope` and `definitionID` fields inside the `systemAssignedIdentityRole` struct. In the following example, we assign the `Owner` role to the system-assigned managed identity on the resource group. IDs for the role assignments can be found in the [Azure docs](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles).

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  template:
    spec:
      identity: SystemAssigned
      systemAssignedIdentityRole:
        scope: /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}
        definitionID: $/subscriptions/${AZURE_SUBSCRIPTION_ID}/providers/Microsoft.Authorization/roleDefinitions/8e3af657-a8ff-443c-a75c-2fe8c4bcb635
      ...
```

* In Machine Pool

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: ${CLUSTER_NAME}-mp-0
  namespace: default
spec:
  identity: SystemAssigned
  ...
```

The CAPZ controller will look for `SystemAssigned` value in `identity` field under `AzureMachinePool`, and enable system-assigned managed identity in the virtual machine scale set.

Alternatively, you can also use the `system-assigned-identity` flavor to build a simple machine deployment-enabled cluster by using `clusterctl generate cluster --flavor system-assigned-identity` to generate a cluster template.

### Service Principal (not recommended)

A service principal is an identity in AAD which is described by a tenant ID and client (or "app") ID. It can have one or more associated secrets or certificates. The set of these values will enable the holder to exchange the values for a JWT token to communicate with Azure. The user generally creates a service principal, saves the credentials, and then uses the credentials in applications. To read more about Service Principals and AD Applications see ["Application and service principal objects in Azure Active Directory"](https://learn.microsoft.com/azure/active-directory/develop/app-objects-and-service-principals).

<aside class="note warning">

<h1> Warning </h1>

Using Service Principal authentication for Cloud Provider Azure is less secure than Managed Identity. Your Service Principal credentials will be written to a file on the disk of each VM in order to be accessible by Cloud Provider.

</aside>

To use a client id/secret for authentication for Cloud Provider, simply leave the `identity` empty, or set it to `None`. The autogenerated [cloud provider config secret](cloud-provider-config.md) will contain the client id and secret used in your AzureClusterIdentity for AzureCluster creation as `aadClientID` and `aadClientSecret`.

To use a certificate/password for authentication, you will need to write the certificate file on the VM (for example using the files option if using CABPK/cloud-init) and mount it to the cloud-controller-manager, then refer to it as `aadClientCertPath`, along with `aadClientCertPassword`, in your cloud provider config. Please consider using a user-assigned identity instead before going down that route as they are more secure and flexible, as described above.

#### Creating a Service Principal

* **With the [Azure CLI](https://github.com/Azure/azure-cli)**

  * Subscription level Scope

     ```shell
     az login
     az account set --subscription="${AZURE_SUBSCRIPTION_ID}"
     az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}"
     ```

  * Resource group level scope

     ```shell
     az login
     az account set --subscription="${AZURE_SUBSCRIPTION_ID}"
     az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}"
     ```

   This will output your `appId`, `password`, `name`, and `tenant`.  The `name` or `appId` is used for the `AZURE_CLIENT_ID` and the `password` is used for `AZURE_CLIENT_SECRET`.

   Confirm your service principal by opening a new shell and run the following commands substituting in `name`, `password`, and `tenant`:

   ```shell
   az login --service-principal -u NAME -p PASSWORD --tenant TENANT
   az vm list-sizes --location eastus
   ```
