# Custom Network Interfaces for AzureMachines

## Pre-existing Network Interfaces
To deploy an AzureMachine using a pre-existing network interface, set the `AzureMachine` or
`AzureMachineTemplate` spec to include the name and optionally the resource group of the existing network
interface(s) as follows:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: template-byo-nic
spec:
  template:
    metadata: {}
    spec:
      networkInterfaces:
      - name: byo-nic
        resourceGroup: group-byo-nic
        acceleratedNetworking: true
        privateIPConfigs: 1
        subnetName: byo-nic-node-subnet
      osDisk:
        cachingType: None
        diskSizeGB: 30
        osType: Linux
      sshPublicKey: mykey
      vmSize: Standard_B2s
```

The pre-existing network interface can be in the same resource group or a different resource group in the same
subscription as the target cluster. When deleting the `AzureMachine`, the network interface will only be
deleted if they are "managed" by capz, ie. they were created during `AzureMachine` deployment.  Pre-existing
network interfaces will *not* be deleted. If a resource group is specified, it must already exist. CAPZ will
*not* create or delete a resource group that is specific to a network interface. If the resource group is
omitted, it will default to the resourceGroup of the `AzureCluster`.

## Custom Network Interface
Alternatively, if you specify a network interface name and optionally a resource group, but the network
interface does not exist, CAPZ will create it and manage its lifecycle. In this case, CAPZ *will* delete the
network interface upon `AzureMachine` deletion. If a resource group is specified, it must already exist. CAPZ
will *not* create or delete a resource group that is specific to a network interface. If the resource group is
omitted, it will default to the resourceGroup of the `AzureCluster`.

<aside class="note">

<h1> Important </h1>

Network interfaces created by CAPZ prior to version v1.5.0 do not have the required tag to ensure the network
interface will be deleted when the associated `AzureMachine` is deleted. If you have network interfaces in
your cluster created by CAPZ v1.5.0 and earlier, prior to upgrading to CAPZ v1.10.0 or
higher, you must tag the CAPZ-managed network interfaces with the CAPZ resource ownership tag.

Example tag:
`sigs.k8s.io_cluster-api-provider-azure_cluster_<clustername>: owned`

</aside>
