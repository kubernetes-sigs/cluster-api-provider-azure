# Configuring the Kubernetes Cloud Provider for Azure

The [Azure cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure) has a number of configuration options driven by a file on cluster nodes. This file canonically lives on a node at /etc/kubernetes/azure.json. The Azure cloud provider documentation details the [configuration options exposed](https://github.com/kubernetes-sigs/cloud-provider-azure/blob/master/docs/cloud-provider-config.md#cloud-provider-config) by this file.

CAPZ automatically generates this file based on user-provided values in AzureMachineTemplate and AzureMachine. All AzureMachines in the same MachineDeployment or control plane will all share a single cloud provider secret, while AzureMachines created inidividually will have their own secret.

For AzureMachineTemplate and standalone AzureMachines, the generated secret will have the name "${RESOURCE}-azure-json", where "${RESOURCE}" is the name of either the AzureMachineTemplate or AzureMachine. The secret will have two data fields: `control-plane-azure.json` and `worker-node-azure.json`, with the raw content for that file containing the control plane and worker node data respectively. When the secret `${RESOURCE}-azure-json` already exists in the same namespace as an AzureCluster and does not have the label `"${CLUSTER_NAME}": "owned"`, CAPZ will not generate the default described above. Instead it will directly use whatever the user provides in that secret.

<aside class="note warning">

<h1> Warning </h1>

For backwards compatibility, the generated secret will also have the `azure.json` field with the control plane data.
But, this is deprecated and will be removed in capz `v0.6.x`. It is recommended to use the `control-plane-azure.json` and `worker-node-azure.json` fields instead.

</aside>

### Overriding Cloud Provider Config

While many of the cloud provider config values are inferred from the capz infrastructure spec, there are other configuration parameters that cannot be inferred, and hence default to the values set by the azure cloud provider. In order to provider custom values to such configuration options through capz, you must use the `spec.cloudProviderConfigOverrides` in `AzureCluster`. The following example overrides the load balancer rate limit configuration:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  location: eastus
  networkSpec:
    vnet:
      name: ${CLUSTER_NAME}-vnet
  resourceGroup: cherry
  subscriptionID: ${AZURE_SUBSCRIPTION_ID}
  cloudProviderConfigOverrides:
    rateLimits:
      - name: "defaultRateLimit"
        config:
          cloudProviderRateLimit: true
          cloudProviderRateLimitBucket: 1
          cloudProviderRateLimitBucketWrite: 1
          cloudProviderRateLimitQPS: 1,
          cloudProviderRateLimitQPSWrite: 1,
      - name: "loadBalancerRateLimit"
        config:
          cloudProviderRateLimit: true
          cloudProviderRateLimitBucket: 2,
          CloudProviderRateLimitBucketWrite: 2,
          cloudProviderRateLimitQPS: 0,
          CloudProviderRateLimitQPSWrite: 0
```

<aside class="note warning">

<h1> Warning </h1>

Presently, only rate limit configuration is supported for overrides, and this works only on clusters running Kubernetes versions above `v1.18.0`.
See [per client rate limiting](https://cloud-provider-azure.sigs.k8s.io/install/configs/#per-client-rate-limiting) for more info.

</aside>

<aside class="note warning">

<h1> Warning </h1>

All cloud provider config values can be customized by creating the `${RESOURCE}-azure-json` secret beforehand. `cloudProviderConfigOverrides` is only applicable when the secret is managed by the Azure Provider.

</aside>


