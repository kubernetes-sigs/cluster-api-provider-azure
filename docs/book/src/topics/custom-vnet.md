# Custom Virtual Networks

## Pre-existing vnet and subnets

To deploy a cluster using a pre-existing vnet, modify the `AzureCluster` spec to include the name and resource group of the existing vnet as follows, as well as the control plane and node subnets as follows:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: cluster-byo-vnet
  namespace: default
spec:
  location: southcentralus
  networkSpec:
    vnet:
      resourceGroup: custom-vnet
      name: my-vnet
    subnets:
      - name: control-plane-subnet
        role: control-plane
      - name: node-subnet
        role: node
  resourceGroup: cluster-byo-vnet
  ```

When providing a vnet, it is required to also provide the two subnets that should be used for control planes and nodes.

If providing an existing vnet and subnets with existing network security groups, make sure that the control plane security group allows inbound to port 6443, as port 6443 is used by kubeadm to bootstrap the control planes. Alternatively, you can [provide a custom control plane endpoint](https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm#kubeadmconfig-objects) in the `KubeadmConfig` spec.

The pre-existing vnet can be in the same resource group or a different resource group in the same subscription as the target cluster. When deleting the `AzureCluster`, the vnet and resource group will only be deleted if they are "managed" by capz, ie. they were created during cluster deployment. Pre-existing vnets and resource groups will *not* be deleted.

## Custom Network Spec

It is also possible to customize the vnet to be created without providing an already existing vnet. To do so, simply modify the `AzureCluster` `NetworkSpec` as desired. Here is an illustrative example of a cluster with a customized vnet address space (CIDR) and customized subnets:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: cluster-example
  namespace: default
spec:
  location: southcentralus
  networkSpec:
    vnet:
      name: my-vnet
      cidrBlocks: 
        - 10.0.0.0/16
    subnets:
      - name: my-subnet-cp
        role: control-plane
        cidrBlocks: 
          - 10.0.1.0/24
      - name: my-subnet-node
        role: node
        cidrBlocks: 
          - 10.0.2.0/24
  resourceGroup: cluster-example
  ```

If no CIDR block is provided, `10.0.0.0/8` will be used by default, with default internal LB private IP `10.0.0.100`.

Whenever using custom vnet and subnet names and/or a different vnet resource group, please make sure to update the `azure.json` content part of both the nodes and control planes' `kubeadmConfigSpec` accordingly before creating the cluster.

### Custom Ingress Rules

Ingress rules can also be customized as part of the subnet specification in a custom network spec.
Note that ingress rules for the Kubernetes API Server port (default 6443) and SSH (22) are automatically added to the controlplane subnet only if Ingress Rules aren't specified.
It is the responsibility of the user to supply those rules themselves if using custom ingresses.

Here is an illustrative example of customizing ingresses that builds on the one above by adding an ingress rule to the control plane nodes:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: cluster-example
  namespace: default
spec:
  location: southcentralus
  networkSpec:
    vnet:
      name: my-vnet
      cidrBlocks: 
        - 10.0.0.0/16
    subnets:
      - name: my-subnet-cp
        role: control-plane
        cidrBlocks: 
          - 10.0.1.0/24
        securityGroup:
          name: my-subnet-cp-nsg
          ingressRule:
            - name: "allow_ssh"
              description: "allow SSH"
              priority: 2200
              protocol: "*"
              destination: "*"
              destinationPorts: "22"
              source: "*"
              sourcePorts: "*"
            - name: "allow_apiserver"
              description: "Allow K8s API Server"
              priority: 2201
              protocol: "*"
              destination: "*"
              destinationPorts: "6443"
              source: "*"
              sourcePorts: "*"
            - name: "allow_port_50000"
              description: "allow port 50000"
              priority: 2202
              protocol: "*"
              destination: "*"
              destinationPorts: "50000"
              source: "*"
              sourcePorts: "*"
      - name: my-subnet-node
        role: node
        cidrBlocks: 
          - 10.0.2.0/24
  resourceGroup: cluster-example
```
