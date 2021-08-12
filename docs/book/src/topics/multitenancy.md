# Multi-tenancy

To enable single controller multi-tenancy, a different Identity can be added to the Azure Cluster that will be used as the Azure Identity when creating Azure resources related to that cluster.

This is achieved using the [aad-pod-identity](https://azure.github.io/aad-pod-identity) library. 

## Service Principal Identity

Once a new SP Identity is created in Azure, the corresponding values should be used to create an `AzureClusterIdentity` resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
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
The password will need to be added in a secret similar to the following example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name-for-client-password>
type: Opaque
data:
  clientSecret: <client-secret-of-SP-identity>
```

OR the password can also be added as a Certificate:

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

## Manual Service Principal Identity

Manual Service Principal Identity is similar to [Service Principal Identity](https://capz.sigs.k8s.io/topics/multitenancy.html#service-principal-identity) except that the service principal's `clientSecret` is directly fetched from the secret containing it.
To use this type of identity, set the identity type as `ManualServicePrincipal` in `AzureClusterIdentity`. For example,
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
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
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
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
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
    kind: AzureClusterIdentity
    name: <name-of-identity>
    namespace: <namespace-of-identity>
```

For more details on how aad-pod-identity works, please check the guide [here](https://azure.github.io/aad-pod-identity/docs/).

## User Assigned Identity

_will be supported in a future release_