# Identity

Managed identities for Azure resources is a feature of Azure Active Directory. Each of the [Azure services that support managed identities for Azure resources](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/services-support-msi) are subject to their own timeline. Make sure you review the [availability](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/services-support-msi) status of managed identities for your resource and [known issues](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/known-issues) before you begin. 

This feature is used to create nodes which have an identity provisioned onto the node by the Azure control plane, rather than providing credentials in the azure.json file. This is a preferred way to manage identities and roles for a given resource in Azure as the lifespan of the identity is linked to the lifespan of the resource.

### Flavors of Identities in Azure
All identities used in Azure are owned by Azure Active Directory (AAD). An identity, or principal, in AAD will provide the basis for each of the flavors of identities we will describe.

### Service Principal
A service principal is an identity in AAD which is described by a TenantID, ClientID, and ClientSecret. The set of these three values will enable the holder to exchange the values for a JWT token to communicate with Azure. The values are normally stored in a file or environment variables. The user generally creates a service principal, saves the credentials, and then uses the credentials in applications. 


### System-assigned managed identity
A system assigned identity is a managed identity which is tied to the lifespan of a resource in Azure. The identity is created by Azure in AAD for the resource it is applied upon and reaped when the resource is deleted. Unlike a service principal, a system assigned identity is available on the local resource through a local port service via the instance metadata service.

To use the System assigned identity, you should use the template for the `system-assigned-identity` flavor, `{flavor}` is the name the user can pass to the `clusterctl config cluster --flavor` flag to identify the specific template to use.

⚠️  **When a Node is created with a System Assigned Identity, A role of Subscription contributor is added to this generated Identity** 

### User-assigned managed identity

A standalone Azure resource that is created by the user outside of the scope of this provider. The identity can be assigned to one or more Azure Machines. The lifecycle of a user-assigned identity is managed separately from the lifecycle of the Azure Machines to which it's assigned

To use the System assigned identity, you should use the template for the `user-assigned-identity` flavor, `{flavor}` is the name the user can pass to the `clusterctl config cluster --flavor` flag to identify the specific template to use.