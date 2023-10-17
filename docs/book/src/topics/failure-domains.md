# Failure Domains

## Failure domains in Azure

A failure domain in the Azure provider maps to an **availability zone** within an Azure region. In Azure an availability zone is a separate data center within a region that offers redundancy and separation from the other availability zones within a region.

To ensure a cluster (or any application) is resilient to failure it is best to spread instances across all the availability zones within a region. If a zone goes down, your cluster will continue to run as the other 2 zones are physically separated and can continue to run.

Full details of availability zones, regions can be found in the [Azure docs](https://learn.microsoft.com/azure/reliability/availability-zones-overview).

## How to use failure domains

### Default Behaviour

By default, only control plane machines get automatically spread to all cluster zones. A workaround for spreading worker machines is to create N `MachineDeployments` for your N failure domains, scaling them independently. Resiliency to failures comes through having multiple `MachineDeployments` (see below).

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
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
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-0
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-0
      version: ${KUBERNETES_VERSION}
      failureDomain: "1"
---
apiVersion: cluster.x-k8s.io/v1beta1
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
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-1
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-1
      version: ${KUBERNETES_VERSION}
      failureDomain: "2"
---
apiVersion: cluster.x-k8s.io/v1beta1
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
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-2
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-2
      version: ${KUBERNETES_VERSION}
      failureDomain: "3"
```

The Cluster API controller will look for the **FailureDomains** status field and will set the **FailureDomain** field in a `Machine` if a value hasn't already been explicitly set. It will try to ensure that the machines are spread across all the failure domains.

The `AzureMachine` controller looks for a failure domain (i.e. availability zone) to use from the `Machine` first before failure back to the `AzureMachine`. This failure domain is then used when provisioning the virtual machine.

### Explicit Placement

If you would rather control the placement of virtual machines into a failure domain (i.e. availability zones) then you can explicitly state the failure domain. The best way is to specify this using the **FailureDomain** field within the `Machine` (or `MachineDeployment`) spec.

> **DEPRECATION NOTE**: Failure domains were introduced in v1alpha3. Prior to this you might have used the **AvailabilityZone** on the `AzureMachine`. This has been deprecated in v1alpha3, and now removed in v1beta1. Please update your definitions and use **FailureDomain** instead.

For example:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: my-cluster
    cluster.x-k8s.io/control-plane: "true"
  name: controlplane-0
  namespace: default
spec:
  version: "v1.22.1"
  clusterName: my-cluster
  failureDomain: "1"
  bootstrap:
    configRef:
        apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
        kind: KubeadmConfigTemplate
        name: my-cluster-md-0
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureMachineTemplate
    name: my-cluster-md-0

```

If you can't use `Machine` (or `MachineDeployment`) to explicitly place your VMs (for example, `KubeadmControlPlane` does not accept those as an object reference but rather uses `AzureMachineTemplate` directly), then you can opt to restrict the announcement of discovered failure domains from the cluster's status itself.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  location: eastus
  failureDomains:
    1:
      controlPlane: true
```

### Using Virtual Machine Scale Sets

You can use an `AzureMachinePool` object to deploy a Virtual Machine Scale Set which automatically distributes VM instances across the configured availability zones.
Set the **FailureDomains** field to the list of availability zones that you want to use. Be aware that not all regions have the same availability zones. You can use `az vm list-skus -l <location> --zone -o table` to list all the available zones per vm size in that location/region.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: my-cluster
  name: ${CLUSTER_NAME}-vmss-0
  namespace: default
spec:
  clusterName: my-cluster
  failureDomains:
    - "1"
    - "3"
  replicas: 3
  template:
    spec:
      clusterName: my-cluster
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-vmss-0
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachinePool
        name: ${CLUSTER_NAME}-vmss-0
      version: ${KUBERNETES_VERSION}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: my-cluster
  name: ${CLUSTER_NAME}-vmss-0
  namespace: default
spec:
  location: westeurope
  template:
    osDisk:
      diskSizeGB: 30
      osType: Linux
    vmSize: Standard_B2s
```

## Availability sets when there are no failure domains

Although failure domains provide protection against datacenter failures, not all azure regions support availability zones. In such cases, azure [availability sets](https://learn.microsoft.com/azure/virtual-machines/manage-availability#configure-multiple-virtual-machines-in-an-availability-set-for-redundancy) can be used to provide redundancy and high availability.

When cluster api detects that the region has no failure domains, it creates availability sets for different groups of virtual machines. The virtual machines, when created, are assigned an availability set based on the group they belong to.

The availability sets created are as follows:

1. For control plane vms, an availability set will be created and suffixed with the string "control-plane".
2. For worker node vms, an availability set will be created for each machine deployment or machine set, and suffixed with the name of the machine deployment or machine set. Important note: make sure that the machine deployment's `Spec.Template.Labels` field includes the `"cluster.x-k8s.io/deployment-name"` label. It will not have this label by default if the machine deployment was created with a custom `Spec.Selector.MatchLabels` field. A machine set should have a `Spec.Template.Labels` field which includes `"cluster.x-k8s.io/set-name"`.

Consider the following cluster configuration:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  labels:
    cni: calico
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 192.168.0.0/16
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane
    name: ${CLUSTER_NAME}-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureCluster
    name: ${CLUSTER_NAME}
---
apiVersion: cluster.x-k8s.io/v1beta1
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
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-0
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-0
      version: ${KUBERNETES_VERSION}
---
apiVersion: cluster.x-k8s.io/v1beta1
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
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-1
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-1
      version: ${KUBERNETES_VERSION}
---
apiVersion: cluster.x-k8s.io/v1beta1
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
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-md-2
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachineTemplate
        name: ${CLUSTER_NAME}-md-2
      version: ${KUBERNETES_VERSION}
```

In the example above, there will be *4* availability sets created, *1* for the control plane, and *1* for each of the *3* machine deployments.
