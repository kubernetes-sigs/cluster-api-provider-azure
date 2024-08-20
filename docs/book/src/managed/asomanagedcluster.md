## ASO Managed Clusters (AKS)

- **Feature status:** alpha, not experimental, fully supported
- **Feature gate:** MachinePool=true

New in CAPZ v1.15.0 is a new flavor of APIs that addresses the following limitations of
the existing CAPZ APIs for advanced use cases for provisioning AKS clusters:

- A limited set of Azure resource types can be represented.
- A limited set of Azure resource topologies can be expressed. e.g. Only a single Virtual Network resource can
  be reconciled for each CAPZ-managed AKS cluster.
- For each Azure resource type supported by CAPZ, CAPZ generally only uses a single Azure API version to
  define resources of that type.
- For each Azure API version known by CAPZ, only a subset of fields defined in that version by the Azure API
  spec are exposed by the CAPZ API.

This new API defines new AzureASOManagedCluster, AzureASOManagedControlPlane, and
AzureASOManagedMachinePool resources. An AzureASOManagedCluster might look like this:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: AzureASOManagedCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  resources:
  - apiVersion: resources.azure.com/v1api20200601
    kind: ResourceGroup
    metadata:
      name: my-resource-group
    spec:
      location: eastus
```

See [here](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/templates/cluster-template-aks-aso.yaml) for a full AKS example using all the new resources.

The main element of the new API is `spec.resources` in each new resource, which defines arbitrary, literal ASO
resources inline to be managed by CAPZ. These inline ASO resource definitions take the place of almost all
other configuration currently defined by CAPZ. e.g. Instead of a CAPZ-specific `spec.location` field on the
existing AzureManagedControlPlane, the same value would be expected to be set on an ASO ManagedCluster
resource defined in an AzureASOManagedControlPlane's `spec.resources`. This pattern allows users to define, in
full, any ASO-supported version of a resource type in any of these new CAPZ resources.

The obvious tradeoff with this new style of API is that CAPZ resource definitions can become more verbose for
basic use cases. To address this, CAPZ still offers flavor templates that use this API with all of the
boilerplate predefined to serve as a starting point for customization.

The overall theme of this API is to leverage ASO as much as possible for representing Azure resources in the
Kubernetes API, thereby making CAPZ the thinnest possible translation layer between ASO and Cluster API.

This experiment will help inform CAPZ whether this pattern may be a candidate for a potential v2 API. This
functionality is enabled by default and can be disabled with the `ASOAPI` feature flag (set by the `EXP_ASO_API` environment variable).
Please try it out and offer any feedback!
