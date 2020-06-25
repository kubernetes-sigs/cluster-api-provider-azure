# Spot Virtual Machines

[Azure Spot Virtual Machines](https://azure.microsoft.com/en-gb/pricing/spot/) allow users to reduce the costs of their
compute resources by utilising Azure's spare capacity for a lower price.

With this lower cost, comes the risk of preemption.
When capacity within a particular Availability Zone is increased,
Azure may need to reclaim Spot Virtual Machines to satisfy the demand on their data centres.

## When should I use Spot Virtual Machines?

Spot Virtual Machines are ideal for workloads that can be interrupted.
For example, short jobs or stateless services that can be rescheduled quickly,
without data loss, and resume operation with limited degradation to a service.

## How do I use Spot Virtual Machines?

**Note**: This feature is only available on Machines at present and not on the
experimental MachinePools.

To enable a Machine to be backed by a Spot Virtual Machine, add `spotMarketOptions`
to your `AzureMachineTemplate`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureMachineTemplate
metadata:
  name: capz-md-0
spec:
  location: westus2
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: Premium_LRS
      osType: Linux
    sshPublicKey: ${YOUR_SSH_PUB_KEY}
    vmSize: Standard_D2s_v3
    spotVMOptions: {}
```

You may also add a `maxPrice` to the options to limit the maximum spend for the
instance. It is however, recommended **not** to set a `maxPrice` as Azure will
cap your spending at the on-demand price if this field is left empty and you will
experience fewer interruptions.

```yaml
spec:
  template:
    spotVMOptions:
      maxPrice: 0.04 # Price in USD per hour (up to 5 decimal places)
```
