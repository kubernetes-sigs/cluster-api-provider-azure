# Multi-tenancy

To enable single controller multi-tenancy, a different Identity can be added to the Azure Cluster that will be used as the Azure Identity when creating Azure resources related to that cluster.

This is achieved using the [aad-pod-identity](https://azure.github.io/aad-pod-identity) library. 

## Service Principal Identity

Once a new SP Identity is created in Azure, the corresponding values should be used to create an `AzureClusterIdentity` resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
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

## allowedNamespaces
AllowedNamespaces is an array of namespaces that AzureClusters can use this Identity from. CAPZ will not support AzureClusters in namespaces  outside this list. 
An empty list (default) indicates that AzureCluster can use this AzureClusterIdentity from any namespace. 

## IdentityRef in AzureCluster

The Identity can be added to an `AzureCluster` by using `IdentityRef` field:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
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
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: AzureClusterIdentity
    name: <name-of-identity>
    namespace: <namespace-of-identity>
```

For more details on how aad-pod-identity works, please check the guide [here](https://azure.github.io/aad-pod-identity/docs/).

## User Assigned Identity

_will be supported in a future release_