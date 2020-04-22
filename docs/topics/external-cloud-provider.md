# External Cloud Provider

To deploy a cluster using [external cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure), create a cluster configuration with the [external cloud provider template](../../templates/cluster-template-external-cloud-provider.yaml).

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
