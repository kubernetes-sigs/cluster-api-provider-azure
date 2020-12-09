# External Cloud Provider

To deploy a cluster using [external cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure), create a cluster configuration with the [external cloud provider template](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/cluster-template-external-cloud-provider.yaml).

After control plane is up and running, deploy external cloud provider components (`cloud-controller-manager` and `cloud-node-manager`) using:

```bash
kubectl --kubeconfig=./${CLUSTER_NAME}.kubeconfig \
  apply -f templates/addons/cloud-controller-manager.yaml
```

```bash
kubectl --kubeconfig=./${CLUSTER_NAME}.kubeconfig \
  apply -f templates/addons/cloud-node-manager.yaml
```

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
