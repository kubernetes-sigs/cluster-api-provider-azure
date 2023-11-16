# Supported Identity methods

Identities are used on the management cluster and the VMs/clusters/workloads which get provisioned by the management cluster.
Also see relevant [identities use cases](identities-use-cases.md), [Azure Active Directory integration](aad-integration.md), and [Multi-tenancy](multitenancy.md) pages.

## Deprecated Identity Types

<aside class="note warning">

<h1> Warning </h1>
The capability to set credentials using environment variables has been removed, the required approach is to use `AzureClusterIdentity` as shown with the below supported identity examples.
</aside>

<aside class="note warning">
<h1> Warning </h1>
All of the methods which utilize AAD Pod Identity will no longer function starting in the 1.13 release of CAPZ.
</aside>

For more details on the deprecated Pod identity types, [see this page](multitenancy.md#deprecated-identity-types)

## Workload Identity (Recommended)

Follow this [link](./workload-identity.md) for a quick start guide on setting up workload identity.

Once you've set up the management cluster with the workload identity (see link above), the corresponding values should be used to create an `AzureClusterIdentity` resource. Create an `azure-cluster-identity.yaml` file with the following content:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: cluster-identity
spec:
  type: WorkloadIdentity
  tenantID: <your-tenant-id>
  clientID: <your-client-id>
  allowedNamespaces:
    list:
    - <cluster-namespace>
```

## Manual Service Principal Identity

Manual Service Principal Identity uses the service principal's `clientSecret` directly fetched from the secret containing it.  To use this type of identity, set the identity type as `ManualServicePrincipal` in `AzureClusterIdentity`. For example,

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: example-identity
  namespace: default
spec:
  type: ManualServicePrincipal
  tenantID: <azure-tenant-id>
  clientID: <client-id-of-SP-identity>
  clientSecret: {"name":"<secret-name-for-client-password>","namespace":"default"}
  allowedNamespaces:
    list:
    - <cluster-namespace>
```

## Azure Host Identity

The identity assigned to the Azure host which in the control plane provides the identity to Azure Cloud Provider, and can be used on all nodes to provide access to Azure services during cloud-init, etc.

- User-assigned Managed Identity
- System-assigned Managed Identity
- Service Principal
- See details about each type in the [VM identity](vm-identity.md) page

More details in [Azure built-in roles documentation](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles).
