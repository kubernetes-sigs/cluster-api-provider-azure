# Multi-tenancy

To enable single controller multi-tenancy, a different Identity can be added to the Azure Cluster that will be used as the Azure Identity when creating Azure resources related to that cluster.

This is achieved using [workload identity](workload-identity.md).

## Supported Identity Types

Please read the [identities](identities.md) page for more information on the supported identity types.

### allowedNamespaces

AllowedNamespaces is used to identify the namespaces the clusters are allowed to use the identity from. Namespaces can be selected either using an array of namespaces or with label selector.
An empty allowedNamespaces object indicates that AzureClusters can use this identity from any namespace.
If this object is nil, no namespaces will be allowed (default behavior, if this field is not provided)
A namespace should be either in the NamespaceList or match with Selector to use the identity.
Please note NamespaceList will take precedence over Selector if both are set.

## Deprecated Identity Types

<aside class="note warning">
<h1> Warning </h1>
The ability to set credentials using environment variables has been removed. Instead, use <code class="hjls">AzureClusterIdentity</code> as described in the identities documentation.
</aside>

<aside class="note warning">
<h1> Warning </h1>
The identity type <code class="hjls">ManualServicePrincipal</code> has been deprecated because it is now identical to <code class="hjls">ServicePrincipal</code> and therefore redundant. None of the identity types use AAD Pod Identity any longer.
</aside>

## Manual Service Principal (deprecated)

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
