# Load Balancer Zone Redundancy

## Zone Redundancy for Load Balancers in Azure

Azure Load Balancers can be configured as zone-redundant to ensure high availability across multiple availability zones within a region. A zone-redundant load balancer distributes traffic across all zones, providing resilience against zone failures.

**Key concepts:**
- Zone redundancy for load balancers is configured through the **frontend IP configuration**
- For **internal load balancers**, zones are set directly on the frontend IP configuration
- For **public load balancers**, zones are inherited from the zone configuration of the public IP address
- **Zones are immutable** - once created, they cannot be changed, added, or removed

Full details can be found in the [Azure Load Balancer reliability documentation](https://learn.microsoft.com/azure/reliability/reliability-load-balancer).

## Configuring Zone-Redundant Load Balancers

CAPZ exposes the `availabilityZones` field on load balancer specifications to enable zone redundancy.

### Internal Load Balancers

For internal load balancers (such as a private API server), you can configure availability zones directly on the load balancer spec:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    apiServerLB:
      type: Internal
      availabilityZones:
        - "1"
        - "2"
        - "3"
```

This configuration creates a zone-redundant internal load balancer with frontend IPs distributed across zones 1, 2, and 3.

### Public Load Balancers

For public load balancers, zone redundancy is primarily controlled by the public IP addresses. However, you can still set `availabilityZones` on the load balancer for consistency:

```yaml
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
      availabilityZones:
        - "1"
        - "2"
        - "3"
```

> **Note**: For public load balancers, ensure that the associated public IP addresses are also zone-redundant for complete zone redundancy.

### Node Outbound Load Balancer

You can also configure zone redundancy for node outbound load balancers:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  location: westus2
  networkSpec:
    nodeOutboundLB:
      type: Public
      availabilityZones:
        - "1"
        - "2"
        - "3"
      frontendIPs:
        - name: node-outbound-ip
          publicIP:
            name: node-outbound-publicip
```

### Control Plane Outbound Load Balancer

For clusters with private API servers, you can configure the control plane outbound load balancer:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  location: eastus
  networkSpec:
    apiServerLB:
      type: Internal
      availabilityZones:
        - "1"
        - "2"
        - "3"
    controlPlaneOutboundLB:
      availabilityZones:
        - "1"
        - "2"
        - "3"
      frontendIPs:
        - name: controlplane-outbound-ip
          publicIP:
            name: controlplane-outbound-publicip
```

## Complete Example: Highly Available Cluster

Here's a complete example of a highly available cluster with zone-redundant load balancers:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: ha-cluster
  namespace: default
spec:
  location: eastus
  resourceGroup: ha-cluster-rg
  networkSpec:
    # Zone-redundant internal API server load balancer
    apiServerLB:
      type: Internal
      name: ha-cluster-internal-lb
      availabilityZones:
        - "1"
        - "2"
        - "3"
      frontendIPs:
        - name: api-server-internal-ip
          privateIPAddress: "10.0.0.100"

    # Zone-redundant control plane outbound load balancer
    controlPlaneOutboundLB:
      name: ha-cluster-cp-outbound-lb
      availabilityZones:
        - "1"
        - "2"
        - "3"
      frontendIPs:
        - name: cp-outbound-ip
          publicIP:
            name: cp-outbound-publicip

    # Zone-redundant node outbound load balancer
    nodeOutboundLB:
      name: ha-cluster-node-outbound-lb
      availabilityZones:
        - "1"
        - "2"
        - "3"
      frontendIPs:
        - name: node-outbound-ip
          publicIP:
            name: node-outbound-publicip

    # Custom VNet configuration
    vnet:
      name: ha-cluster-vnet
      cidrBlocks:
        - "10.0.0.0/16"

    subnets:
      - name: control-plane-subnet
        role: control-plane
        cidrBlocks:
          - "10.0.0.0/24"
      - name: node-subnet
        role: node
        cidrBlocks:
          - "10.0.1.0/24"
```

## Important Considerations

### Immutability

Once a load balancer is created with availability zones, the zone configuration **cannot be changed**. This is an Azure platform limitation. To change zones, you must:

1. Delete the load balancer
2. Recreate it with the new zone configuration

> **Warning**: Changing load balancer zones requires recreating the cluster's load balancers, which will cause service interruption.

### Region Support

Not all Azure regions support availability zones. Before configuring zone-redundant load balancers, verify that your target region supports zones:

```bash
az vm list-skus -l <location> --zone -o table
```

### Standard SKU Requirement

Zone-redundant load balancers require the **Standard SKU**. CAPZ uses Standard SKU by default, so no additional configuration is needed.

### Backend Pool Placement

For optimal high availability:
- Spread your control plane nodes across all availability zones
- Spread your worker nodes across all availability zones
- Ensure backend pool members exist in the same zones as the load balancer

See the [Failure Domains](failure-domains.md) documentation for details on distributing VMs across zones.

## Migration from Non-Zone-Redundant Load Balancers

If you have an existing cluster without zone-redundant load balancers, migration requires careful planning:

### For New Clusters

When creating a new cluster, simply include the `availabilityZones` field in your `AzureCluster` specification from the start.

### For Existing Clusters

**Migration is not straightforward** because:
1. Azure does not allow modifying zones on existing load balancers
2. CAPZ's webhook validation prevents zone changes to enforce this immutability
3. Load balancer recreation requires cluster downtime

**Recommended approach for existing clusters:**
1. Create a new cluster with zone-redundant configuration
2. Migrate workloads to the new cluster
3. Decommission the old cluster

**Alternative for development/test clusters:**
1. Delete the `AzureCluster` resource (this will delete the infrastructure)
2. Recreate the `AzureCluster` with `availabilityZones` configured
3. Reconcile the cluster

> **Important**: The alternative approach causes significant downtime and should only be used in non-production environments.

## Troubleshooting

### Load Balancer Not Zone-Redundant

If your load balancer is not zone-redundant despite configuration:

1. **Verify the zones are set in spec:**
   ```bash
   kubectl get azurecluster <cluster-name> -o jsonpath='{.spec.networkSpec.apiServerLB.availabilityZones}'
   ```

2. **Check the Azure load balancer frontend configuration:**
   ```bash
   az network lb frontend-ip show \
     --lb-name <lb-name> \
     --name <frontend-name> \
     --resource-group <rg-name> \
     --query zones
   ```

3. **Verify the region supports zones:**
   ```bash
   az vm list-skus -l <location> --zone -o table | grep -i standardsku
   ```

### Validation Errors

If you encounter validation errors when updating `availabilityZones`:

```
field is immutable
```

This is expected behavior. Zones cannot be modified after creation. You must recreate the load balancer with the desired configuration.

## Best Practices

1. **Enable zone redundancy from the start** when creating new clusters in zone-capable regions
2. **Use all available zones** in the region (typically 3 zones) for maximum resilience
3. **Spread backend pools** across all zones configured on the load balancer
4. **Monitor zone health** and be prepared to handle zone failures
5. **Test failover scenarios** to ensure your cluster can survive zone outages
6. **Document your zone configuration** for disaster recovery procedures

## Related Documentation

- [Failure Domains](failure-domains.md) - Configure VMs across availability zones
- [API Server Endpoint](api-server-endpoint.md) - API server load balancer configuration
- [Azure Load Balancer Reliability](https://learn.microsoft.com/azure/reliability/reliability-load-balancer) - Azure official documentation
