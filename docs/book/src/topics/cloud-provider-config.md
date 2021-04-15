# Configuring the Kubernetes Cloud Provider for Azure

The [Azure cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure) has a number of configuration options driven by a file on cluster nodes. This file canonically lives on a node at /etc/kubernetes/azure.json. The Azure cloud provider documentation details the [configuration options exposed](https://github.com/kubernetes-sigs/cloud-provider-azure/blob/master/docs/cloud-provider-config.md#cloud-provider-config) by this file.

CAPZ automatically generates this file based on user-provided values in AzureMachineTemplate and AzureMachine. All AzureMachines in the same MachineDeployment or control plane will all share a single cloud provider secret, while AzureMachines created inidividually will have their own secret.

For AzureMachineTemplate and standalone AzureMachines, the generated secret will have the name "${RESOURCE}-azure-json", where "${RESOURCE}" is the name of either the AzureMachineTemplate or AzureMachine. The secret will have one data field, `azure.json`, with the raw content for that file. When the secret `${RESOURCE}-azure-json` already exists in the same namespace as an AzureCluster and does not have the label `"${CLUSTER_NAME}": "owned"`, CAPZ will not generate the default described above. Instead it will directly use whatever the user provides in that secret.

# External Cloud Provider

To deploy a cluster using [external cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure), create a cluster configuration with the [external cloud provider template](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/cluster-template-external-cloud-provider.yaml).

After components are deployed, you should see following pods in `Running` state:

```bash
kube-system   cloud-controller-manager                                            1/1     Running   0          41s
kube-system   cloud-node-manager-5pklx                                            1/1     Running   0          26s
kube-system   cloud-node-manager-hbbqt                                            1/1     Running   0          30s
kube-system   cloud-node-manager-mfsdg                                            1/1     Running   0          39s
kube-system   cloud-node-manager-qrz74                                            1/1     Running   0          24s
```

## Storage Drivers

### Azure File CSI Driver

To install the Azure File CSI driver please refer to the [installation guide](https://github.com/kubernetes-sigs/azurefile-csi-driver/blob/master/docs/install-azurefile-csi-driver.md)

Repository: https://github.com/kubernetes-sigs/azurefile-csi-driver

### Azure Disk CSI Driver

To install the Azure Disk CSI driver please refer to the [installation guide](https://github.com/kubernetes-sigs/azuredisk-csi-driver/blob/master/docs/install-azuredisk-csi-driver.md)

Repository: https://github.com/kubernetes-sigs/azuredisk-csi-driver
