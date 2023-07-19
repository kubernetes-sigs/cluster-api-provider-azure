## How to use Identities with CAPZ

## CAPZ controller: 
This is the identity used by the management cluster to provision infrastructure in Azure
 - Multi-tenant config via AzureClusterIdentity
   - [AAD Pod Identity](https://azure.github.io/aad-pod-identity/) using Service Principals and Managed Identities: by default, the identity used by the workload cluster running on Azure is the same Service Principal assigned to the management cluster. If an identity is specified on the Azure Cluster Resource, that identity will be used when creating Azure resources related to that cluster. See [Multi-tenancy](multitenancy.md) page for details.

 - Env config (deprecated)
   - Service Principal: A [service principal](https://learn.microsoft.com/azure/active-directory/develop/app-objects-and-service-principals) is an identity in AAD which is described by a TenantID, ClientID, and ClientSecret. The set of these three values will enable the holder to exchange the values for a JWT token to communicate with Azure. The values are normally stored in a file or environment variables.
   - Configuration:
      - Scope: Subscription
      - Role: `Contributor` since the controller is responsible for creating resource groups and cluster resources within the group. To create a resource group within a subscription, one must have subscription contributor rights. Note, this role's scope can be reduced to Resource Group Contributor if all resource groups are created prior to cluster creation.
      - If the workload clusters are going to use system-assigned managed identities, then the role here should be `Owner` to be able to create role assignments for system-assigned managed identity.
More details in [Azure built-in roles documentation](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles).

<aside class="note warning"> 

<h1> Warning </h1> 

The capability to set credentials using environment variables is now deprecated and will be removed in future releases, the recommended approach is to use `AzureClusterIdentity` as explained [here](multitenancy.md) 

</aside>

## Azure Host Identity:
The identity assigned to the Azure host which in the control plane provides the identity to Azure Cloud Provider, and can be used on all nodes to provide access to Azure services during cloud-init, etc.

- User-assigned Managed Identity
- System-assigned Managed Identity
- Service Principal
- See details about each type in the [VM identity](vm-identity.md) page

## Pod Identity:
The identity used by pods running within the workload cluster to provide access to Azure services during runtime. For example, to access blobs stored in Azure Storage or to access Azure database services.
 - [AAD Pod Identity](https://azure.github.io/aad-pod-identity/): The workload cluster requires an identity to communicate with Azure. This identity can be either a managed identity (in the form of system-assigned identity or user-assigned identity) or a service principal. The AAD Pod Identity pod allows the cluster to use the Identity referenced by the Azure CLuster to access cloud resources securely with Azure Active Directory.

## User Stories

#### Story 1 - Locked down with Service Principal Per Subscription

Alex is an engineer in a large organization which has a strict Azure account architecture. This architecture dictates that Kubernetes clusters must be hosted in dedicated Subscriptions with AAD identity having RBAC rights to provision the infrastructure only in the Subscription. The workload clusters must run with a System Assigned machine identity. The organization has adopted Cluster API in order to manage Kubernetes infrastructure, and expects 'management' clusters running the Cluster API controllers to manage 'workload' clusters in dedicated Azure Subscriptions with an AAD account which only has access to that Subscription.

The current configuration exists:
* Subscription for each cluster
* AAD Service Principals with Subscription Owner rights for each Subscription
* A management Kubernetes cluster running Cluster API Provider Azure controllers

Alex can provision a new workload cluster in the specified Subscription with the corresponding AAD Service Principal by creating new Cluster API resources in the management cluster. Each of the workload cluster machines would run as the System Assigned identity described in the Cluster API resources. The CAPZ controller in the management cluster uses the Service Principal credentials when reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.

#### Story 2 - Locked down by Namespace and Subscription

Alex is an engineer in a large organization which has a strict Azure account architecture. This architecture dictates that Kubernetes clusters must be hosted in dedicated Subscriptions with AAD identity having RBAC rights to provision the infrastructure only in the Subscription. The workload clusters must run with a System Assigned machine identity.

Erin is a security engineer in the same company as Alex. Erin is responsible for provisioning identities. Erin will create a Service Principal for use by Alex to provision the infrastructure in Alex's cluster. The identity Erin creates should only be able to be used in a predetermined Kubernetes namespace where Alex will define the workload cluster. The identity should be able to be used by CAPZ to provision workload clusters in other namespaces.

The organization has adopted Cluster API in order to manage Kubernetes infrastructure, and expects 'management' clusters running the Cluster
API controllers to manage 'workload' clusters in dedicated Azure Subscriptions with an AAD account which only has access to that Subscription.

The current configuration exists:
* Subscription for each cluster
* AAD Service Principals with Subscription Owner rights for each Subscription
* A management Kubernetes cluster running Cluster API Provider Azure controllers

Alex can provision a new workload cluster in the specified Subscription with the corresponding AAD Service Principal by creating new Cluster API resources in the management cluster in the predetermined namespace. Each of the workload cluster machines would run as the System Assigned identity described in the Cluster API resources. The CAPZ controller in the management cluster uses the Service Principal credentials when reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.

Erin can provision an identity in a namespace of limited access and define the allowed namespaces, which will include the predetermined namespace for the workload cluster.

#### Story 3 - Using an Azure User Assigned Identity

Erin is an engineer working in a large organization. Erin does not want to be responsible for ensuring Service Principal secrets are rotated on a regular basis. Erin would like to use an Azure User Assigned Identity to provision workload cluster infrastructure. The User Assigned Identity will have the RBAC rights needed to provision the infrastructure in Erin's subscription.

The current configuration exists:

* Subscription for the workload cluster
* A User Assigned Identity with RBAC with Subscription Owner rights for the Subscription
* A management Kubernetes cluster running Cluster API Provider Azure controllers

Erin can provision a new workload cluster in the specified Subscription with the Azure User Assigned Identity by creating new Cluster API resources in the management cluster. The CAPZ controller in the management cluster uses the User Assigned Identity credentials when reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.

#### Story 4 - Legacy Behavior Preserved

Dascha is an engineer in a smaller, less strict organization with a few Azure accounts intended to build all infrastructure. There is a single Azure Subscription named 'dev', and Dascha wants to provision a new cluster in this Subscription. An existing Kubernetes cluster is already running the Cluster API operators and managing resources in the dev Subscription. Dascha can provision a new cluster by creating Cluster API resources in the existing cluster, omitting the ProvisionerIdentity field in the AzureCluster spec. The CAPZ operator will use the Azure credentials provided in its deployment template.
