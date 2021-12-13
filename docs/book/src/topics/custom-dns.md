# Custom Private DNS Zone Name

It is possible to set the DNS zone name to a custom value by setting `PrivateDNSZoneName` in the `NetworkSpec`. By default the DNS zone name is `${CLUSTER_NAME}.capz.io`.

*This feature is enabled only if the `apiServerLB.type` is `Internal`*

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
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
# Manage DNS Via CAPZ Tool

Private DNS when created by CAPZ can be managed by CAPZ tool itself automatically. To give the flexibility to have BYO 
as well as managed DNS zone, an enhancement is made that causes all the managed zones created in the CAPZ version before 
the enhancement changes to be treated as unmanaged. The enhancement is captured in PR
[1791](https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/1791) 

To manage the private DNS via CAPZ please tag it manually from azure portal.

Steps to tag:

- Go to azure portal and search for `Private DNS zones`.
- Select the DNS zone that you want to be managed.
- Go to `Tags` section and add key as `sigs.k8s.io_cluster-api-provider-azure_cluster_<clustername>` and value as
`owned`. (Note: clustername is the name of the cluster that you created)