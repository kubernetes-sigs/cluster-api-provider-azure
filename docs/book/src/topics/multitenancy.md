# Multi-tenancy

To enable single controller multi-tenancy, a different Identity can be added to the Azure Cluster that will be used as the Azure Identity when creating Azure resources related to that cluster.

This is achieved using the [aad-pod-identity](https://azure.github.io/aad-pod-identity) library.

## Identity Types

### Service Principal With Client Password

Once a new SP Identity is created in Azure, the corresponding values should be used to create an `AzureClusterIdentity` resource:

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

### Service Principal With Certificate

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

### User-Assigned Managed Identity

<aside class="note">

<h1> Note </h1>

This option is only available when the cluster is managed from a Kubernetes cluster running on Azure.

</aside>

#### Prerequisites

1. [Create](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-manage-user-assigned-managed-identities?pivots=identity-mi-methods-azp#create-a-user-assigned-managed-identity) a user-assigned managed identity in Azure.
2. [Create a role assignment](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/howto-assign-access-portal#use-azure-rbac-to-assign-a-managed-identity-access-to-another-resource) to give the identity Contributor access to the Azure subscription where the workload cluster will be created.
3. [Configure] the identity on the management cluster nodes by adding it to each worker node VM. If using AKS as the management cluster see [these instructions](https://docs.microsoft.com/en-us/azure/aks/use-managed-identity).

#### Creating the AzureClusterIdentity

After a user-assigned managed identity is created in Azure and assigned to the management cluster, the corresponding values should be used to create an `AzureClusterIdentity` resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: example-identity
  namespace: default
spec:
  type: ServicePrincipal
  tenantID: <azure-tenant-id>
  clientID: <client-id-of-user-assigned-identity>
  resourceID: <resource-id-of-user-assigned-identity>
  allowedNamespaces: 
    list:
    - <cluster-namespace>
```

#### Assigning VM identities for cloud-provider authentication

When using a user-assigned managed identity to create the workload cluster, a VM identity should also be assigned to each control-plane machine in the workload cluster for Cloud Provider to use. See [here](../topics/vm-identity.md#managed-identities) for more information.

### Manual Service Principal Identity

Manual Service Principal Identity is similar to [Service Principal Identity](https://capz.sigs.k8s.io/topics/multitenancy.html#service-principal-identity) except that the service principal's `clientSecret` is directly fetched from the secret containing it.
To use this type of identity, set the identity type as `ManualServicePrincipal` in `AzureClusterIdentity`. For example,

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

The rest of the configuration is the same as that of service principal identity. This useful in scenarios where you don't want to have a dependency on [aad-pod-identity](https://azure.github.io/aad-pod-identity).

## allowedNamespaces

AllowedNamespaces is used to identify the namespaces the clusters are allowed to use the identity from. Namespaces can be selected either using an array of namespaces or with label selector.
An empty allowedNamespaces object indicates that AzureClusters can use this identity from any namespace.
If this object is nil, no namespaces will be allowed (default behaviour, if this field is not provided)
A namespace should be either in the NamespaceList or match with Selector to use the identity.
Please note NamespaceList will take precedence over Selector if both are set.

## IdentityRef in AzureCluster

The Identity can be added to an `AzureCluster` by using `IdentityRef` field:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: example-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    vnet:
      name: example-cluster-vnet
  resourceGroup: example-cluster
  subscriptionID: <AZURE_SUBSCRIPTION_ID>
  identityRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureClusterIdentity
    name: <name-of-identity>
    namespace: <namespace-of-identity>
```

For more details on how aad-pod-identity works, please check the guide [here](https://azure.github.io/aad-pod-identity/docs/).
