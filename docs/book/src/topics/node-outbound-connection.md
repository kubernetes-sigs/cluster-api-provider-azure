# Node Outbound

This document describes how to configure your clusters' node outbound traffic.

## IPv4 Clusters

For IPv4 clusters ie. clusters with CIDR type is `IPv4`, CAPZ automatically configures a [NAT gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-gateway-resource) for node outbound traffic with the default settings. Default, the cluster is IPv4 type unless you specify the CIDR to be an IPv6 address.

To provide custom settings for a node NAT gateway, you can configure the NAT gateway in the node `subnets` section of cluster configuration by setting the NAT gateway's name. A Public IP will also be created for the NAT gateway once the NAT gateway name is provided.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: cluster-natgw
  namespace: default
spec:
  location: southcentralus
  networkSpec:
    vnet:
      name: my-vnet
    subnets:
      - name: subnet-cp
        role: control-plane
      - name: subnet-node
        role: node
        natGateway:
          name: node-natgw
          NatGatewayIP:
            name: pip-cluster-natgw-subnet-node-natgw
  resourceGroup: cluster-natgw
  ```

You can also specify the Public IP name that should be used when creating the Public IP for the NAT gateway.
If you don't specify it, CAPZ will automatically generate a name for it.

<aside class="note">

<h1>Note</h1>

You may want to more than one gateways within the same virtual network. You attach more NAT gateways to different node subnets.
Multiple gateways can't be attached to a single subnet.

</aside>

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: cluster-natgw
  namespace: default
spec:
  location: southcentralus
  networkSpec:
    vnet:
      name: my-vnet
    subnets:
      - name: subnet-cp
        role: control-plane
      - name: subnet-node-1
        role: node
        natGateway:
          name: node-natgw-1
          NatGatewayIP:
            name: pip-cluster-natgw-subnet-node-natgw-1
      - name: subnet-node-2
        role: node
        natGateway:
          name: node-natgw-2
          NatGatewayIP:
            name: pip-cluster-natgw-subnet-node-natgw-2
  resourceGroup: cluster-natgw
```

<aside class="note warning">

<h1> Warning </h1>

CAPZ will ignore the NAT gateway configuration in the control plane subnet because we always create a load balancer for the control plane, which we use for outbound traffic.

</aside>


## IPv6 Clusters

For IPv6 clusters ie. clusters with CIDR type is `IPv6`, NAT gateway is not supported for IPv6 cluster. IPv6 cluster uses load balancer for outbound connections.

### Public IPv6 Clusters

For public IPv6 clusters ie. clusters with api server load balancer type set to `Public` and CIDR type set to `IPv6`, CAPZ automatically configures a node outbound load balancer with the default settings.

To provide custom settings for the node outbound load balancer, use the `nodeOutboundLB` section in cluster configuration.

The `idleTimeoutInMinutes` specifies the number of minutes to keep a TCP connection open for the outbound rule (defaults to 4). See [here](https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-tcp-reset#configurable-tcp-idle-timeout) for more details.

Here is an example of a node outbound load balancer with `frontendIPsCount` set to 3. CAPZ will read this value and create 3 front end ips for this load balancer.

<aside class="note">

<h1>Note</h1>

You may want more than one outbound IP address if you are running a large cluster that is processing lots of connections.
See [here](https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections#multifesnat) for more documentation about how adding more outbound IP addresses can increase the number of SNAT ports available for use by the Standard Load Balancer in your cluster.

</aside>

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-public-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    apiServerLB:
      type: Public
    subnets:
    - cidrBlocks:
      - 2001:0DB8:0000:1/64
      name: subnet-node
      role: node
    nodeOutboundLB:
      frontendIPsCount: 3
      idleTimeoutInMinutes: 4
```

<aside class="note warning">

<h1> Warning </h1>

Only `frontendIPsCount` and `idleTimeoutInMinutes` can be configured for any node outbound load balancer. Trying to modify any other value will result in a validation error.

</aside>

### Private IPv6 Clusters

For private IPv6 clusters ie. clusters with api server load balancer type set to `Internal` and CIDR type set to `IPv6`, CAPZ does not create a node outbound load balancer by default. 
To create a node outbound load balancer, include the `nodeOutboundLB` section with the desired settings. 

Here is an example of configuring a node outbound load balancer with 1 front end ip for a private IPv6 cluster:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-private-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    apiServerLB:
      type: Internal
    subnets:
    - cidrBlocks:
      - 2001:0DB8:0000:1/64
      name: subnet-node
      role: node
    nodeOutboundLB:
      frontendIPsCount: 1
```
