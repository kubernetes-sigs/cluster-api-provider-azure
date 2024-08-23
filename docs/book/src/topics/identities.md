# Supported Identity methods

Identities are used on the management cluster and the VMs/clusters/workloads which get provisioned by the management cluster.
Also see relevant [identities use cases](identities-use-cases.md), [Azure Active Directory integration](aad-integration.md), and [Multi-tenancy](multitenancy.md) pages.

## Deprecated Identity Types

<aside class="note warning">
<h1> Warning </h1>
The ability to set credentials using environment variables has been removed. Instead, use <code class="hjls">AzureClusterIdentity</code> as described below.
</aside>

<aside class="note warning">
<h1> Warning </h1>
The identity type <code class="hjls">ManualServicePrincipal</code> has been deprecated because it is now identical to <code class="hjls">ServicePrincipal</code> and therefore redundant. None of the identity types use AAD Pod Identity any longer.
</aside>

For details on the deprecated identity types, [see this page](multitenancy.md#deprecated-identity-types).

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

## Service Principal

Service Principal identity uses the service principal's `clientSecret` in a Kubernetes Secret. To use this type of identity, set the identity type as `ServicePrincipal` in `AzureClusterIdentity`. For example,

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: example-identity
  namespace: default
spec:
  type: ServicePrincipal
  tenantID: <azure-tenant-id>
  clientID: <client-id-of-SP-identity>
  clientSecret: {"name":"<secret-name-for-client-password>","namespace":"default"}
  allowedNamespaces:
    list:
    - <cluster-namespace>
```

Deploy this resource to your cluster:
```bash
kubectl apply -f azure-cluster-identity.yaml
```

A Kubernetes Secret should also be created to store the client password:

```bash
kubectl create secret generic "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" --from-literal=clientSecret="${AZURE_CLIENT_SECRET}"
```

The resulting Secret should look similar to the following example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name-for-client-password>
type: Opaque
data:
  clientSecret: <client-secret-of-SP-identity>
```

## Service Principal With Certificate

Once a new SP Identity is created in Azure, the corresponding values should be used to create an `AzureClusterIdentity` resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: example-identity
  namespace: default
spec:
  type: ServicePrincipalCertificate
  tenantID: <azure-tenant-id>
  clientID: <client-id-of-SP-identity>
  clientSecret: {"name":"<secret-name-for-client-password>","namespace":"default"}
  allowedNamespaces:
    list:
    - <cluster-namespace>
```

If needed, convert the PEM file to PKCS12 and set a password:

```bash
openssl pkcs12 -export -in fileWithCertAndPrivateKey.pem -out ad-sp-cert.pfx -passout pass:<password>
```

Create a k8s secret with the certificate and password:

```bash
kubectl create secret generic "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" --from-file=certificate=ad-sp-cert.pfx --from-literal=password=<password>
```

The resulting Secret should look similar to the following example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name-for-client-password>
type: Opaque
data:
  certificate: CERTIFICATE
  password: PASSWORD
```

## User-Assigned Managed Identity

<aside class="note">

<h1> Note </h1>

This option is only available when the cluster is managed from a Kubernetes cluster running on Azure.

</aside>

#### Prerequisites

1. [Create](https://learn.microsoft.com/azure/active-directory/managed-identities-azure-resources/how-manage-user-assigned-managed-identities?pivots=identity-mi-methods-azp#create-a-user-assigned-managed-identity) a user-assigned managed identity in Azure.
2. [Create a role assignment](https://learn.microsoft.com/en-us/entra/identity/managed-identities-azure-resources/how-to-assign-access-azure-resource?pivots=identity-mi-access-portal#use-azure-rbac-to-assign-a-managed-identity-access-to-another-resource-using-the-azure-portal) to give the identity Contributor access to the Azure subscription where the workload cluster will be created.
3. Configure the identity on the management cluster nodes by adding it to each worker node VM. If using AKS as the management cluster see [these instructions](https://learn.microsoft.com/azure/aks/use-managed-identity).

#### Creating the AzureClusterIdentity

After a user-assigned managed identity is created in Azure and assigned to the management cluster, the corresponding values should be used to create an `AzureClusterIdentity` resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: example-identity
  namespace: default
spec:
  type: UserAssignedMSI
  tenantID: <azure-tenant-id>
  clientID: <client-id-of-user-assigned-identity>
  allowedNamespaces:
    list:
    - <cluster-namespace>
```

### Assigning VM identities for cloud provider authentication (self-managed)

When using a user-assigned managed identity to create the workload cluster, a VM identity should also be assigned to each control plane machine in the workload cluster for Azure Cloud Provider to use. See [here](../self-managed/vm-identity.md#managed-identities) for more information.


## Azure Host Identity

The identity assigned to the Azure host which in the control plane provides the identity to Azure Cloud Provider, and can be used on all nodes to provide access to Azure services during cloud-init, etc.

- User-assigned Managed Identity
- System-assigned Managed Identity
- Service Principal
- See details about each type in the [VM identity](../self-managed/vm-identity.md) page

More details in [Azure built-in roles documentation](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles).
