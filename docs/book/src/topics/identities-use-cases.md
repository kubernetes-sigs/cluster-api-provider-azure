# Identity User Stories

This describes some common user stories for identities being utilized with CAPZ.
Please see the core [identities page](identities.md) first.
Also see related [identities use cases](identities-use-cases.md) and [Multi-tenancy](multitenancy.md) pages.

## Story 1 - Locked down with Service Principal Per Subscription

Alex is an engineer in a large organization which has a strict Azure account architecture. This architecture dictates that Kubernetes clusters must be hosted in dedicated Subscriptions with AAD identity having RBAC rights to provision the infrastructure only in the Subscription. The workload clusters must run with a System Assigned machine identity. The organization has adopted Cluster API in order to manage Kubernetes infrastructure, and expects 'management' clusters running the Cluster API controllers to manage 'workload' clusters in dedicated Azure Subscriptions with an AAD account which only has access to that Subscription.

The current configuration exists:

- Subscription for each cluster
- AAD Service Principals with Subscription Owner rights for each Subscription
- A management Kubernetes cluster running Cluster API Provider Azure controllers

Alex can provision a new workload cluster in the specified Subscription with the corresponding AAD Service Principal by creating new Cluster API resources in the management cluster. Each of the workload cluster machines would run as the System Assigned identity described in the Cluster API resources. The CAPZ controller in the management cluster uses the Service Principal credentials when reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.

## Story 2 - Locked down by Namespace and Subscription

Alex is an engineer in a large organization which has a strict Azure account architecture. This architecture dictates that Kubernetes clusters must be hosted in dedicated Subscriptions with AAD identity having RBAC rights to provision the infrastructure only in the Subscription. The workload clusters must run with a System Assigned machine identity.

Erin is a security engineer in the same company as Alex. Erin is responsible for provisioning identities. Erin will create a Service Principal for use by Alex to provision the infrastructure in Alex's cluster. The identity Erin creates should only be able to be used in a predetermined Kubernetes namespace where Alex will define the workload cluster. The identity should be able to be used by CAPZ to provision workload clusters in other namespaces.

The organization has adopted Cluster API in order to manage Kubernetes infrastructure, and expects 'management' clusters running the Cluster
API controllers to manage 'workload' clusters in dedicated Azure Subscriptions with an AAD account which only has access to that Subscription.

The current configuration exists:

- Subscription for each cluster
- AAD Service Principals with Subscription Owner rights for each Subscription
- A management Kubernetes cluster running Cluster API Provider Azure controllers

Alex can provision a new workload cluster in the specified Subscription with the corresponding AAD Service Principal by creating new Cluster API resources in the management cluster in the predetermined namespace. Each of the workload cluster machines would run as the System Assigned identity described in the Cluster API resources. The CAPZ controller in the management cluster uses the Service Principal credentials when reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.

Erin can provision an identity in a namespace of limited access and define the allowed namespaces, which will include the predetermined namespace for the workload cluster.

## Story 3 - Using an Azure User Assigned Identity

Erin is an engineer working in a large organization. Erin does not want to be responsible for ensuring Service Principal secrets are rotated on a regular basis. Erin would like to use an Azure User Assigned Identity to provision workload cluster infrastructure. The User Assigned Identity will have the RBAC rights needed to provision the infrastructure in Erin's subscription.

The current configuration exists:

- Subscription for the workload cluster
- A User Assigned Identity with RBAC with Subscription Owner rights for the Subscription
- A management Kubernetes cluster running Cluster API Provider Azure controllers

Erin can provision a new workload cluster in the specified Subscription with the Azure User Assigned Identity by creating new Cluster API resources in the management cluster. The CAPZ controller in the management cluster uses the User Assigned Identity credentials when reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.
