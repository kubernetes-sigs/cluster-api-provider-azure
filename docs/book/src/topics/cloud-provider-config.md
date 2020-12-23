# Configuring the Kubernetes Cloud Provider for Azure

The [Azure cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure) has a number of configuration options driven by a file on cluster nodes. This file canonically lives on a node at /etc/kubernetes/azure.json. The Azure cloud provider documentation details the [configuration options exposed](https://github.com/kubernetes-sigs/cloud-provider-azure/blob/master/docs/cloud-provider-config.md#cloud-provider-config) by this file.

CAPZ automatically generates this file based on user-provided values in AzureMachineTemplate and AzureMachine. All AzureMachines in the same MachineDeployment or control plane will all share a single cloud provider secret, while AzureMachines created inidividually will have their own secret.

For AzureMachineTemplate and standalone AzureMachines, the generated secret will have the name "${RESOURCE}-azure-json", where "${RESOURCE}" is the name of either the AzureMachineTemplate or AzureMachine. The secret will have one data field, `azure.json`, with the raw content for that file. When the secret `${RESOURCE}-azure-json` already exists in the same namespace as an AzureCluster and does not have the label `"${CLUSTER_NAME}": "owned"`, CAPZ will not generate the default described above. Instead it will directly use whatever the user provides in that secret.
