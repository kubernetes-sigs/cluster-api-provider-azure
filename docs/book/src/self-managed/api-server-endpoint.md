# API Server Endpoint

This document describes how to configure your clusters' api server load balancer and IP.

### Load Balancer Type

CAPZ supports two load balancer types, `Public` and `Internal`.

`Public`, which is also the default, means that your API Server Load Balancer will have a publicly accessible IP address. This Load Balancer type supports a "public cluster" configuration, which load balances internet source traffic to the apiserver across the cluster's control plane nodes.

`Internal` means that the API Server endpoint will only be accessible from within the cluster's virtual network (or peered VNets). This configuration supports a "private cluster" configuration, which load balances internal VNET source traffic to the apiserver across the cluster's control plane nodes.

For a more complete "private cluster" template example, you may refer to [this reference template](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-private.yaml) that the capz project maintains.

For more information on Azure load balancing, see [Load Balancer documentation](https://learn.microsoft.com/azure/load-balancer/load-balancer-overview).

<aside class="note warning">

<h1> Warning </h1>

When creating a workload cluster with `apiServerLB` type `Internal`, the management cluster needs to be in the same VNet, or a peered VNet, as the workload cluster. Otherwise, it will not be able to access the target cluster's api server and the cluster creation will fail.

</aside>

Here is an example of configuring the API Server LB type:

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
```

### Private IP

When using an api server load balancer of type `Internal`, the default private IP address associated with that load balancer will be `10.0.0.100`.
If also specifying a [custom virtual network](./custom-vnet.md), make sure you provide a private IP address that is in the range of your control plane subnet and not in use.

For example:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-private-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    vnet:
      name: my-vnet
      cidrBlocks:
        - 172.16.0.0/16
    subnets:
      - name: my-subnet-cp
        role: control-plane
        cidrBlocks:
          - 172.16.0.0/24
      - name: my-subnet-node
        role: node
        cidrBlocks:
          - 172.16.2.0/24
    apiServerLB:
      type: Internal
      frontendIPs:
        - name: lb-private-ip-frontend
          privateIP: 172.16.0.100
```

### Public IP

When using an api server load balancer of type `Public`, a dynamic public IP address will be created, along with a unique FQDN.

You can also choose to provide your own public api server IP. To do so, specify the existing public IP as follows:

````yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    apiServerLB:
      type: Public
      frontendIPs:
        - name: lb-public-ip-frontend
          publicIP:
            name: my-public-ip
            dnsName: my-cluster-986b4408.eastus.cloudapp.azure.com
````

Note that `dns` is the FQDN associated to your public IP address (look for "DNS name" in the Azure Portal).

When you BYO api server IP, CAPZ does not manage its lifecycle, ie. the IP will not get deleted as part of cluster deletion.

### Load Balancer SKU

At this time, CAPZ only supports Azure Standard Load Balancers. See [SKU comparison](https://learn.microsoft.com/azure/load-balancer/skus#skus) for more information on Azure Load Balancers SKUs.
