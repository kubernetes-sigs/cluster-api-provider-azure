# Multi-tenancy

To enable single controller multi-tenancy, a different Identity can be added to the Azure Cluster that will be used as the Azure Identity when creating Azure resources related to that cluster.

This is achieved using the [aad-pod-identity](https://azure.github.io/aad-pod-identity) library. 

## Service Principal Identity

Once a new SP Identity is created in Azure, a new resource of [AzureIdentity](https://azure.github.io/aad-pod-identity/docs/concepts/azureidentity/) should be created in the managment cluster, for example

```yaml
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentity
metadata:
  name: <name-of-identity>
spec:
  type: 1
  tenantID: <tenant-id-from-azure-subscription>
  clientID: <client-id-of-SP-identity>
  clientPassword: {"name":"<secret-name-for-client-password>","namespace":"default"}
```

The password will need to be added in a secret similar to the following example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name-for-client-password>
type: Opaque
data:
  clientSecret: <client-secret-of-SP-identity>
```

OR the password can also as a Certificate 

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

and then this identity name will be added to the Azure Cluster 

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
  identityName: <name-of-identity>
```  

for more details on how aad-pod-identity works, please check the guide [here](https://azure.github.io/aad-pod-identity/docs/)

## User Assiged Identity

_will be supported in a future release_