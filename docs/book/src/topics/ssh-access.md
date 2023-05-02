# SSH access to nodes

This document describes how to get SSH access to virtual machines that are part of a CAPZ cluster.

In order to get SSH access to a Virtual Machine on Azure, two requirements have to be met:

- get network-level access to the SSH service
- get authentication sorted

This documents describe some possible strategies to fulfill both requirements.

## Network Access

### Default behavior

By default, `control plane` VMs have SSH access allowed from any source in their `Network Security Group`s. Also by default,
VMs don't have a public IP address assigned. 

To get SSH access to one of the `control plane` VMs you can use the `API Load Balancer`'s IP, because by default an `Inbound NAT Rule`
is created to route traffic coming to the load balancer on TCP port 22 (the SSH port) to one of the nodes with role `master` in the workload cluster.

This of course works only for clusters that are using a `Public` Load Balancer.

In order to reach all other VMs, you can use the NATted control plane VM as a bastion host and use the private IP
address for the other nodes.

For example, let's consider this CAPZ cluster (using a Public Load Balancer) with two nodes:

```shell
NAME                        STATUS   ROLES                  AGE    VERSION    INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION     CONTAINER-RUNTIME
test1-control-plane-cn9lm   Ready    control-plane,master   111m   v1.18.16   10.0.0.4      <none>        Ubuntu 18.04.5 LTS   5.4.0-1039-azure   containerd://1.4.3
test1-md-0-scctm            Ready    <none>                 109m   v1.18.16   10.1.0.4      <none>        Ubuntu 18.04.5 LTS   5.4.0-1039-azure   containerd://1.4.3
```

You can SSH to the control plane node using the load balancer's public DNS name:

```shell
$ kubectl get azurecluster test1 -o json | jq '.spec.networkSpec.apiServerLB.frontendIPs[0].publicIP.dnsName'
test1-21192f78.eastus.cloudapp.azure.com

$ ssh username@test1-21192f78.eastus.cloudapp.azure.com hostname
test1-control-plane-cn9lm
```

As you can see, the Load Balancer routed the request to node `test1-control-plane-cn9lm` that is the only node with role `control-plane` in this workload cluster.

In order to SSH to node 'test1-md-0-scctm', you can use the other node as a bastion:

```shell
$ ssh -J username@test1-21192f78.eastus.cloudapp.azure.com username@10.1.0.4 hostname
test1-md-0-scctm
```

Clusters using an `Internal` Load Balancer (private clusters) can't use this approach. Network-level SSH access to those clusters has to be made on the private IP address of VMs
by first getting access to the Virtual Network. How to do that is out of the scope of this document.
A possible alternative that works for private clusters as well is described in the next paragraph.

### Azure Bastion

A possible alternative to the process described above is to use the [`Azure Bastion`](https://azure.microsoft.com/en-us/services/azure-bastion/) feature.
This approach works the same way for workload clusters using either type of `Load Balancers`.

In order to enable `Azure Bastion` on a CAPZ workload cluster, edit the `AzureCluster` CR and set the `spec/bastionSpec/azureBastion` field.
It is enough to set the field's value to the empty object `{}` and the default configuration settings will be used while deploying the `Azure Bastion`.

For example this is an `AzureCluster` CR with the `Azure Bastion` feature enabled:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: test1
  namespace: default
spec:
  bastionSpec:
    azureBastion: {}
  ...
```

Once the `Azure Bastion` is deployed, it will be possible to SSH to any of the cluster VMs through the
`Azure Portal`. Please follow the [official documentation](https://docs.microsoft.com/en-us/azure/bastion/bastion-overview)
for a deeper explanation on how to do that.

#### Advanced settings

When the `AzureBastion` feature is enabled in a CAPZ cluster, 3 new resources will be deployed in the resource group:

- The `Azure Bastion` resource;
- A subnet named `AzureBastionSubnet` (the name is mandatory and can't be changed);
- A public `IP address`.

The default values for the new resources should work for most use cases, but if you need to customize them you can 
provide your own values. Here is a detailed example:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: test1
  namespace: default
spec:
  bastionSpec:
    azureBastion:
      name: "..." // The name of the Azure Bastion, defaults to '<cluster name>-azure-bastion'
      subnet:
        name: "..." // The name of the Subnet. The only supported name is `AzureBastionSubnet` (this is an Azure limitation).
        securityGroup: {} // No security group is assigned by default. You can choose to have one created and assigned by defining it. 
      publicIP:
        "name": "..." // The name of the Public IP, defaults to '<cluster name>-azure-bastion-pip'.
      sku: "..." // The SKU/tier of the Azure Bastion resource. The options are `Standard` and `Basic`. The default value is `Basic`.
      enableTunneling: "..." // Whether or not to enable tunneling/native client support. The default value is `false`.
```

If you specify a security group to be associated with the Azure Bastion subnet, it needs to have some networking rules defined or
the `Azure Bastion` resource creation will fail. Please refer to [the documentation](https://docs.microsoft.com/en-us/azure/bastion/bastion-nsg) for more details.

## Authentication

With the networking part sorted, we still have to work out a way of authenticating to the VMs via SSH.

### Provisioning SSH keys using Machine Templates

In order to add an SSH authorized key for user `username` and provide `sudo` access to the `control plane` VMs, you can adjust the `KubeadmControlPlane` CR
as in the following example:

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KubeadmControlPlane
...
spec:
  ...
  kubeadmConfigSpec:
    ...
    users:
    - name: username
      sshAuthorizedKeys:
      - "ssh-rsa AAAA..."
    files:
    - content: "username ALL = (ALL) NOPASSWD: ALL"
      owner: root:root
      path: /etc/sudoers.d/username
      permissions: "0440"
    ...
```

Similarly, you can achieve the same result for `Machine Deployments` by customizing the `KubeadmConfigTemplate` CR: 

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: test1-md-0
  namespace: default
spec:
  template:
    spec:
      files:
      ...
      - content: "username ALL = (ALL) NOPASSWD: ALL"
        owner: root:root
        path: /etc/sudoers.d/username
        permissions: "0440"
      ...
      users:
      - name: username
        sshAuthorizedKeys:
        - "ssh-rsa AAAA..."
```

### Setting SSH keys or passwords using the Azure Portal

An alternative way of gaining SSH access to VMs on Azure is to set the `password` or `authorized key` via the `Azure Portal`.
In the Portal, navigate to the `Virtual Machine` details page and find the `Reset password` function in the left pane.
