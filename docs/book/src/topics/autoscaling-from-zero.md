# Autoscaling from Zero

- **Feature status:** Experimental

## Overview

> To enable Cluster API providers to dynamically scale node groups from zero to one and from one to
> zero, the Cluster API project has proposed a mechanism for providers to annotate MachineSet and
> MachinePool infrastructure templates with capacity and node information. This enables the cluster
> autoscaler to make informed scheduling decisions even when no nodes exist in a node group.

> The autoscaling-from-zero feature works by having the infrastructure provider populate status fields
> on infrastructure templates (e.g., AzureMachineTemplate) with capacity (CPU, memory) and node
> information (architecture, operating system). The cluster-autoscaler then reads these status fields
> to simulate node capacity for scale-from-zero decisions.

*Source: [Opt-in Autoscaling from Zero Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md)*

CAPZ implements this proposal by automatically populating AzureMachineTemplate status fields based on Azure VM SKU information. This enables cluster-autoscaler to scale MachineDeployments to zero replicas and back up based on workload demand.

**Key benefits:**
- Cost optimization by scaling unused node groups to zero
- Efficient resource utilization for dev/test environments
- Support for batch workloads that scale between job runs

<aside class="note">

<h1> Note </h1>

This feature requires cluster-autoscaler with the Cluster API cloud provider. For managed AKS clusters, use the native AKS autoscaler instead.

</aside>

## How It Works

CAPZ's AzureMachineTemplate controller automatically populates status fields when a template is created or reconciled:

1. The controller queries the Azure Resource SKUs API for VM size specifications
2. It extracts capacity information (CPU cores, memory) from the SKU
3. It determines node architecture (amd64/arm64) from SKU capabilities
4. It derives the operating system (linux/windows) from the template's `osDisk.osType` field
5. This information is written to `status.capacity` and `status.nodeInfo` fields

The cluster-autoscaler reads these status fields to simulate node capacity for pending pods, enabling scale-from-zero decisions without requiring actual nodes to exist.

The controller respects cluster pause annotations and requires the template to have an owner reference to a Cluster resource.

## Configuration

### Example MachineDeployment with Autoscaling from Zero

Below is an example of the resources needed to enable autoscaling from zero for a worker node pool.

#### AzureMachineTemplate

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-worker
  namespace: default
spec:
  template:
    spec:
      vmSize: Standard_D2s_v3
      osDisk:
        diskSizeGB: 128
        osType: Linux
      sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64}
# Status is automatically populated by CAPZ controller:
# status:
#   capacity:
#     cpu: "2"
#     memory: "8Gi"
#   nodeInfo:
#     architecture: amd64
#     operatingSystem: linux
```

#### MachineDeployment

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: ${CLUSTER_NAME}-worker
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: 0  # Can start at zero
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: ${CLUSTER_NAME}
  template:
    spec:
      clusterName: ${CLUSTER_NAME}
      version: ${KUBERNETES_VERSION}
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-worker
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-worker
```

## Status Fields

The CAPZ controller populates the following fields in AzureMachineTemplate status:

| Field | Description | Example | Source |
|-------|-------------|---------|--------|
| `status.capacity.cpu` | Number of vCPUs | `"2"`, `"4"`, `"8"` | Azure SKU API |
| `status.capacity.memory` | Memory size | `"8Gi"`, `"16Gi"` | Azure SKU API |
| `status.nodeInfo.architecture` | CPU architecture | `amd64`, `arm64` | Azure SKU API |
| `status.nodeInfo.operatingSystem` | OS type | `linux`, `windows` | Template `osDisk.osType` |

Inspect the status of an AzureMachineTemplate:

```bash
kubectl get azuremachinetemplate ${CLUSTER_NAME}-worker -o jsonpath='{.status}' | jq
```

Example output:
```json
{
  "capacity": {
    "cpu": "2",
    "memory": "8Gi"
  },
  "nodeInfo": {
    "architecture": "amd64",
    "operatingSystem": "linux"
  }
}
```

## Related Resources

- [ClusterClass](./clusterclass.md) - Using autoscaling-from-zero with ClusterClass
- [Machine Pools (VMSS)](../self-managed/machinepools.md) - Alternative scaling approach
- [Cluster API Autoscaling](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/autoscaling)
- [Autoscaling from Zero Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md)
- [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler)
- [Azure VM Sizes](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes)
