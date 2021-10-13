# Control Plane Outbound Load Balancer

This document describes how to configure your clusters' control plane outbound load balancer.

### Public Clusters

For public clusters ie. clusters with api server load balancer type set to `Public`, CAPZ automatically does not support adding a control plane outbound load balancer.
This is because the api server load balancer already allows for outbound traffic in public clusters.

### Private Clusters

For private clusters ie. clusters with api server load balancer type set to `Internal`, CAPZ does not create a control plane outbound load balancer by default. 
To create a control plane outbound load balancer, include the `controlPlaneOutboundLB` section with the desired settings. 

Here is an example of configuring a control plane outbound load balancer with 1 front end ip for a private cluster:

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
    controlPlaneOutboundLB:
      frontendIPsCount: 1
```

<aside class="note warning">

<h1> Warning </h1>

The field `controlPlaneOutboundLB` cannot be modified after cluster creation. Trying to do so will result in a validation error.

</aside>
