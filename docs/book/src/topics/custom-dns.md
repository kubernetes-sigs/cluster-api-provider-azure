# Custom Private DNS Zone Name

It is possible to set the DNS zone name to a custom value by setting `PrivateDNSZoneName` in the `NetworkSpec`. By default the DNS zone name is `${CLUSTER_NAME}.capz.io`.

*This feature is enabled only if the `apiServerLB.type` is `Internal`*

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: AzureCluster
metadata:
  name: cluster-example
  namespace: default
spec:
  location: southcentralus
  networkSpec:
    privateDNSZoneName: "kubernetes.myzone.com"
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
    apiServerLB:
      type: Internal
      frontendIPs:
        - name: lb-private-ip-frontend
          privateIP: 172.16.0.100
  resourceGroup: cluster-example

```
