# Failure Domains

The Azure provider includes the support for [failure domains](https://cluster-api.sigs.k8s.io/developer/providers/v1alpha2-to-v1alpha3.html#optional-support-failure-domains) introduced as part of v1alpha3.

## Failure domains in Azure

A failure domain in the Azure provider maps to an **availability zone** within an Azure region. In Azure an availability zone is a separate data center within a region that offers redundancy and separation from the other availability zones within a region.

To ensure a cluster (or any application) is resilient to failure it is best to spread instances across all the availability zones within a region. If a zone goes down, your cluster will continue to run as the other 2 zones are physically separated and can continue to run.

Full details of availability zones, regions can be found in the [Azure docs](https://docs.microsoft.com/en-us/azure/availability-zones/az-overview).

## How to use failure domains

### Default Behaviour

By default, only control plane machines get automatically spread to all cluster zones. A workaround for spreading worker machines is to create N `MachineDelpoyments` for your N failure domains, scaling them independently. Resiliency to failures comes through having multiple `MachineDeployments` (see below).

```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: MachineDeployment
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  selector:
    matchLabels: null
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-0
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-0
      version: ${KUBERNETES_VERSION}
      failureDomain: "1"
---
apiVersion: cluster.x-k8s.io/v1alpha3
kind: MachineDeployment
metadata:
  name: ${CLUSTER_NAME}-md-1
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  selector:
    matchLabels: null
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-1
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-1
      version: ${KUBERNETES_VERSION}
      failureDomain: "2"
---
apiVersion: cluster.x-k8s.io/v1alpha3
kind: MachineDeployment
metadata:
  name: ${CLUSTER_NAME}-md-2
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  selector:
    matchLabels: null
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-2
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-2
      version: ${KUBERNETES_VERSION}
      failureDomain: "3"
```

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
  version: "v1.18.8"
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
