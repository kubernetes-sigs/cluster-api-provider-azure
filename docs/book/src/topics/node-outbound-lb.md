# Node Outbound

This document describes how to configure your clusters' node outbound traffic.

## Node Outbound Load Balancer

### Public Clusters

For public clusters ie. clusters with api server load balancer type set to `Public`, CAPZ automatically configures a node outbound load balancer with the default settings.

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
    nodeOutboundLB:
      frontendIPsCount: 3
      idleTimeoutInMinutes: 4
```

<aside class="note warning">

<h1> Warning </h1>

Only `frontendIPsCount` and `idleTimeoutInMinutes` can be configured for any node outbound load balancer. Trying to modify any other value will result in a validation error.

</aside>

### Private Clusters

For private clusters ie. clusters with api server load balancer type set to `Internal`, CAPZ does not create a node outbound load balancer by default. 
To create a node outbound load balancer, include the `nodeOutboundLB` section with the desired settings. 

Here is an example of configuring a node outbound load balancer with 1 front end ip for a private cluster:

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
    nodeOutboundLB:
      frontendIPsCount: 1
```

## Node Outbound NAT gateway

You can configure a [NAT gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-gateway-resource) in a subnet to enable outbound traffic in the cluster nodes by setting the NAT gateway's name in the subnet configuration.
A Public IP will also be created for the NAT gateway.

Using this configuration, [a Load Balancer for the nodes outbound traffic](./node-outbound-lb.md) won't be created.

<aside class="note warning">

<h1> Warning </h1>

CAPZ will ignore the NAT gateway configuration in the control plane subnet because we always create a load balancer for the control plane, which we use for outbound traffic.

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
      - name: subnet-node
        role: node
        natGateway:
          name: node-natgw
          NatGatewayIP:
            name: pip-cluster-natgw-subnet-node-natgw
  resourceGroup: cluster-natgw
  ```

You can also define the Public IP name that should be used when creating the Public IP for the NAT gateway.
If you don't specify it, CAPZ will automatically generate a name for it.
