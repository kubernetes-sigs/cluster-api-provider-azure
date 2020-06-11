# Failure Domains

The Azure provider includes the support for [failure domains](https://cluster-api.sigs.k8s.io/developer/providers/v1alpha2-to-v1alpha3.html#optional-support-failure-domains) introduced as part of v1alpha3.

## Failure domains in Azure

A failure domain in the Azure provider maps to an **availability zone** within an Azure region. In Azure an availability zone is a separate data center within a region that offers redundancy and separation from the other availability zones within a region.

To ensure a cluster (or any application) is resilient to failure its best to spread instances across all the availability zones within a region. If a zone is lost your cluster will continue to run as the other 2 zones are physically separated and can continue to run.

Full details of availability zones, regions can be found in the [Azure docs](https://docs.microsoft.com/en-us/azure/availability-zones/az-overview).

## How to use failure domains

### Default Behaviour

The default behaviour of Cluster API is to try and spread machines out across all the failure domains. The controller for the `AzureCluster` queries the Resource Manager API for the availability zones for the **Location** of the cluster. The availability zones are reported back to Cluster API via the **FailureDomains** field in the status of `AzureCluster`.

The Cluster API controller will look for the **FailureDomains** status field and will set the **FailureDomain** field in a `Machine` if a value hasn't already been explicitly set. It will try to ensure that the machines are spread across all the failure domains.

The `AzureMachine` controller looks for a failure domain (i.e. availability zone) to use from the `Machine` first before failure back to the `AzureMachine`. This failure domain is then used when provisioning the virtual machine.

### Explicit Placement

If you would rather control the placement of virtual machines into a failure domain (i.e. availability zones) then you can explicitly state the failure domain. The best way is to specify this using the **FailureDomain** field within the `Machine` (or `MachineDeployment`) spec.

> **DEPRECATION NOTE**: Failure domains where introduced in v1alpha3. Prior to this you might have used the **AvailabilityZone** on the `AzureMachine` and this is now deprecated. Please update your definitions and use **FailureDomain** instead.

For example:

```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: Machine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: my-cluster
    cluster.x-k8s.io/control-plane: "true"
  name: controlplane-0
  namespace: default
spec:
  version: "v1.18.3"
  clusterName: my-cluster
  failureDomain: "1"
  bootstrap:
    configRef:
        apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
        kind: KubeadmConfigTemplate
        name: my-cluster-md-0
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: AzureMachineTemplate
    name: my-cluster-md-0

```
