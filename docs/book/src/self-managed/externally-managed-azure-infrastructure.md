# Externally managed Azure infrastructure

Normally, Cluster API will create infrastructure on Azure when standing up a new workload cluster. However, it is possible to have Cluster API reuse existing Azure infrastructure instead of creating its own infrastructure.

CAPZ supports [externally managed cluster infrastructure](https://github.com/kubernetes-sigs/cluster-api/blob/10d89ceca938e4d3d94a1d1c2b60515bcdf39829/docs/proposals/20210203-externally-managed-cluster-infrastructure.md).
If the `AzureCluster` resource includes a "cluster.x-k8s.io/managed-by" annotation then the [controller will skip any reconciliation](https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure.html#normal-resource).
This is useful for scenarios where a different persona is managing the cluster infrastructure out-of-band while still wanting to use CAPI for automated machine management.

You should only use this feature if your cluster infrastructure lifecycle management has constraints that the reference implementation does not support. See [user stories](https://github.com/kubernetes-sigs/cluster-api/blob/10d89ceca938e4d3d94a1d1c2b60515bcdf39829/docs/proposals/20210203-externally-managed-cluster-infrastructure.md#user-stories) for more details. 

## Creating AzureMachines in an existing VMSS Flex

MachineDeployment-backed `AzureMachine` resources can be created as members of an existing Azure Virtual Machine Scale Set with Flexible orchestration mode. Set `virtualMachineScaleSetID` on the `AzureMachineTemplate` to the resource ID of the existing VMSS Flex:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
spec:
  template:
    spec:
      vmSize: ${AZURE_NODE_MACHINE_TYPE}
      virtualMachineScaleSetID: /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}/providers/Microsoft.Compute/virtualMachineScaleSets/${VMSS_NAME}
```

The referenced VMSS must already exist, must use Flexible orchestration mode, and must be compatible with the VM configuration in the `AzureMachineTemplate`. Azure only allows a VM to be added to a VMSS at creation time, so `virtualMachineScaleSetID` is immutable. When this field is set, CAPZ does not attach the VM to an availability set because Azure does not allow `availabilitySet` and `virtualMachineScaleSet` on the same VM.

## Disabling Specific Component Reconciliation
Some controllers/webhooks may not be necessary to run in an externally managed cluster infrastructure scenario. These 
controllers/webhooks can be disabled through a flag on the manager called `disable-controllers-or-webhooks`. This flag 
accepts a comma separated list of values.

Currently, these are the only accepted values:
1. `DisableASOSecretController` - disables the ASOSecretController from being deployed
2. `DisableAzureJSONMachineController` - disables the AzureJSONMachineController from being deployed

