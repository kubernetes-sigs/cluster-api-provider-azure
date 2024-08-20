# MachinePools
- **Feature status:** Experimental
- **Feature gate:** MachinePool=true

> In Cluster API (CAPI) v1alpha2, users can create MachineDeployment, MachineSet or Machine custom
> resources. When you create a MachineDeployment or MachineSet, Cluster API components react and
> eventually Machine resources are created. Cluster API's current architecture mandates that a
> Machine maps to a single machine (virtual or bare metal) with the provider being responsible for
> the management of the underlying machine's infrastructure.

> Nearly all infrastructure providers have a way for their users to manage a group of machines
> (virtual or bare metal) as a single entity. Each infrastructure provider offers their own unique
> features, but nearly all are concerned with managing availability, health, and configuration updates.

> A MachinePool is similar to a MachineDeployment in that they both define
> configuration and policy for how a set of machines are managed. They Both define a common
> configuration, number of desired machine replicas, and policy for update. Both types also combine
> information from Kubernetes as well as the underlying provider infrastructure to give a view of
> the overall health of the machines in the set.

> MachinePool diverges from MachineDeployment in that the MachineDeployment controller uses
> MachineSets to achieve the aforementioned desired number of machines and to orchestrate updates
> to the Machines in the managed set, while MachinePool delegates the responsibility of these
> concerns to an infrastructure provider specific resource such as AWS Auto Scale Groups, GCP
> Managed Instance Groups, and Azure Virtual Machine Scale Sets.

> MachinePool is optional and doesn't replace the need for MachineSet/Machine since not every
> infrastructure provider will have an abstraction for managing multiple machines (i.e. bare metal).
> Users may always opt to choose MachineSet/Machine when they don't see additional value in
> MachinePool for their use case.

*Source: [MachinePool API Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/bf51a2502f9007b531f6a9a2c1a4eae1586fb8ca/docs/proposals/20190919-machinepool-api.md)*

## AzureMachinePool
Cluster API Provider Azure (CAPZ) has experimental support for `MachinePool` through the infrastructure
types `AzureMachinePool` and `AzureMachinePoolMachine`. An `AzureMachinePool` corresponds to a
[Virtual Machine Scale Set](https://learn.microsoft.com/azure/virtual-machine-scale-sets/overview) (VMSS),
which provides the cloud provider-specific resource for orchestrating a group of Virtual Machines. The
`AzureMachinePoolMachine` corresponds to a virtual machine instance within the VMSS.

### Orchestration Modes

Azure Virtual Machine Scale Sets support two orchestration modes: `Uniform` and `Flexible`. CAPZ defaults to `Uniform` mode. See [VMSS Orchestration modes in Azure](https://learn.microsoft.com/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-orchestration-modes) for more information.

To use `Flexible` mode requires Kubernetes v1.26.0 or later. Ensure that `orchestrationMode` on the `AzureMachinePool` spec is set:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: capz-mp-0
spec:
  orchestrationMode: Flexible
```

Then, after applying the template to start provisioning, install the [cloud-provider-azure Helm chart](https://github.com/kubernetes-sigs/cloud-provider-azure/tree/master/helm/cloud-provider-azure#readme) to the workload cluster.

### Safe Rolling Upgrades and Delete Policy
`AzureMachinePools` provides the ability to safely deploy new versions of Kubernetes, or more generally, changes to the
Virtual Machine Scale Set model, e.g., updating the OS image run by the virtual machines in the scale set. For example,
if a cluster operator wanted to change the Kubernetes version of the `MachinePool`, they would update the `Version`
field on the `MachinePool`, then `AzureMachinePool` would respond by rolling out the new OS image for the specified
Kubernetes version to each of the virtual machines in the scale set progressively cordon, draining, then replacing the
machine. This enables `AzureMachinePools` to upgrade the underlying pool of virtual machines with minimal interruption 
to the workloads running on them.

`AzureMachinePools` also provides the ability to specify the order of virtual machine deletion.

#### Describing the Deployment Strategy
Below we see a partially described `AzureMachinePool`. The `strategy` field describes the 
`AzureMachinePoolDeploymentStrategy`. At the time of writing this, there is only one strategy type, `RollingUpdate`, 
which provides the ability to specify delete policy, max surge, and max unavailable.

- **deletePolicy:** provides three options for order of deletion `Oldest`, `Newest`, and `Random`
- **maxSurge:** provides the ability to specify how many machines can be added in addition to the current replica count
  during an upgrade operation. This can be a percentage, or a fixed number.
- **maxUnavailable:** provides the ability to specify how many machines can be unavailable at any time. This can be a 
  percentage, or a fixed number.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: capz-mp-0
spec:
  strategy:
    rollingUpdate:
      deletePolicy: Oldest
      maxSurge: 25%
      maxUnavailable: 1
    type: RollingUpdate
```

### AzureMachinePoolMachines
`AzureMachinePoolMachine` represents a virtual machine in the scale set. `AzureMachinePoolMachines` are created by the
`AzureMachinePool` controller and are used to track the life cycle of a virtual machine in the scale set. When a 
`AzureMachinePool` is created, each virtual machine instance will be represented as a `AzureMachinePoolMachine`
resource. A cluster operator can delete the `AzureMachinePoolMachine` resource if they would like to delete a specific
virtual machine from the scale set. This is useful if one would like to manually control upgrades and rollouts through
CAPZ.

### Using `clusterctl` to deploy
To deploy a MachinePool / AzureMachinePool via `clusterctl generate` there's a [flavor](https://cluster-api.sigs.k8s.io/clusterctl/commands/generate-cluster.html#flavors)
for that.

Make sure to set up your Azure environment as described [here](https://cluster-api.sigs.k8s.io/user/quick-start.html).

```shell
clusterctl generate cluster my-cluster --kubernetes-version v1.22.0 --flavor machinepool > my-cluster.yaml
```

The template used for this [flavor](https://cluster-api.sigs.k8s.io/clusterctl/commands/generate-cluster.html#flavors)
is located [here](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-machinepool.yaml).

### Example MachinePool, AzureMachinePool and KubeadmConfig Resources
Below is an example of the resources needed to create a pool of Virtual Machines orchestrated with
a Virtual Machine Scale Set.
```yaml
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  name: capz-mp-0
spec:
  clusterName: capz
  replicas: 2
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfig
          name: capz-mp-0
      clusterName: capz
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachinePool
        name: capz-mp-0
      version: v1.22.0
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: capz-mp-0
spec:
  location: westus2
  strategy:
    rollingUpdate:
      deletePolicy: Oldest
      maxSurge: 25%
      maxUnavailable: 1
    type: RollingUpdate
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: Premium_LRS
      osType: Linux
    sshPublicKey: ${YOUR_SSH_PUB_KEY}
    vmSize: Standard_D2s_v3
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfig
metadata:
  name: capz-mp-0
spec:
  files:
  - content: |
      {
        "cloud": "AzurePublicCloud",
        "tenantId": "tenantID",
        "subscriptionId": "subscriptionID",
        "aadClientId": "clientID",
        "aadClientSecret": "secret",
        "resourceGroup": "capz",
        "securityGroupName": "capz-node-nsg",
        "location": "westus2",
        "vmType": "vmss",
        "vnetName": "capz-vnet",
        "vnetResourceGroup": "capz",
        "subnetName": "capz-node-subnet",
        "routeTableName": "capz-node-routetable",
        "loadBalancerSku": "Standard",
        "maximumLoadBalancerRuleCount": 250,
        "useManagedIdentityExtension": false,
        "useInstanceMetadata": true
      }
    owner: root:root
    path: /etc/kubernetes/azure.json
    permissions: "0644"
  joinConfiguration:
    nodeRegistration:
      name: '{{ ds.meta_data["local_hostname"] }}'
```
