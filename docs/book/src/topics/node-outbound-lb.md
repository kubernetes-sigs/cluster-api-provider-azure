# Node Outbound Load Balancer

This document describes how to configure your clusters' node outbound load balancer.

### Public Clusters

For public clusters ie. clusters with api server load balancer type set to `Public`, CAPZ automatically configures a node outbound load balancer with the default settings.

To provide custom settings for the node outbound load balacer, use the `nodeOutboundLB` section in cluster configuration.

Here is an example of a node outbound load balancer with `frontendIPsCount` set to 3. CAPZ will read this value and create 3 front end ips for this load balancer.

<aside class="note">

<h1>Note</h1>

You may want more than one outbound IP address if you are running a large cluster that is processing lots of connections.
See [here](https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections#multifesnat) for more documentation about how adding more outbound IP addresses can increase the number of SNAT ports available for use by the Standard Load Balancer in your cluster.

</aside>

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
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
```

<aside class="note warning">

<h1> Warning </h1>

Only `frontendIPsCount` is allowed to be configured for any node outbound load balancer. Trying to modify any other value will result in a validation error.

</aside>

### Private Clusters

For private clusters ie. clusters with api server load balancer type set to `Internal`, CAPZ does not create a node outbound load balancer by default. 
To create a node outbound load balancer, include the `nodeOutboundLB` section with the desired settings. 

Here is an example of configuring a node outbound load balancer with 1 front end ip for a private cluster:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: AzureCluster
metadata:
  name: my-public-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    apiServerLB:
      type: Internal
    nodeOutboundLB:
      frontendIPsCount: 1
```
