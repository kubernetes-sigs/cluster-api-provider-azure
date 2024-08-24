## Joining self-managed VMSS nodes to an AKS control plane

<aside class="note warning">

<h1> Warning </h1>

This is not an officially supported AKS scenario. It is meant to facilitate development and testing of alpha/beta Kubernetes features. Please use at your own risk.

</aside>

### Installing Addons

In order for the nodes to become ready, you'll need to install Cloud Provider Azure and a CNI.

AKS will install Cloud Provider Azure on the self-managed nodes as long as they have the appropriate labels. You can add the required label on the nodes by running the following command on the AKS cluster:

```bash
kubectl label node <node name> kubernetes.azure.com/cluster=<nodeResourceGroupName>
```

Repeat this for each node in the MachinePool.

<aside class="note">

<h1> Warning </h1>

Note: CAPI does not currently support propagating labels from the MachinePool to the nodes, in the future this could be part of the MachinePool definition.

</aside>

For the CNI, you can install the CNI of your choice. For example, to install Azure CNI, run the following command on the AKS cluster:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/addons/azure-cni-v1.yaml
```

### Notes

Some notes about how this works under the hood:

- CAPZ will fetch the kubeconfig for the AKS cluster and store it in a secret named `${CLUSTER_NAME}-kubeconfig` in the management cluster. That secret is then used for discovery by the `KubeadmConfig` resource.
- You can customize the `MachinePool`, `AzureMachinePool`, and `KubeadmConfig` resources to your liking. The example above is just a starting point. Note that the key configurations to keep are in the `KubeadmConfig` resource, namely the `files`, `joinConfiguration`, and `preKubeadmCommands` sections.
- The `KubeadmConfig` resource will be used to generate a `kubeadm join` command that will be executed on each node in the VMSS. It uses the cluster kubeconfig for discovery. The `kubeadm init phase upload-config all` is run as a preKubeadmCommand to ensure that the kubeadm and kubelet configurations are uploaded to a ConfigMap. This step would normally be done by the `kubeadm init` command, but since we're not running `kubeadm init` we need to do it manually.

### Creating the MachinePool

You can add a self-managed VMSS node pool to any CAPZ-managed AKS cluster by applying the following resources to the management cluster:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  name: ${CLUSTER_NAME}-vmss
  namespace: default
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: ${WORKER_MACHINE_COUNT}
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfig
          name: ${CLUSTER_NAME}-vmss
      clusterName: ${CLUSTER_NAME}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: AzureMachinePool
        name: ${CLUSTER_NAME}-vmss
      version: ${KUBERNETES_VERSION}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: ${CLUSTER_NAME}-vmss
  namespace: default
spec:
  location: ${AZURE_LOCATION}
  strategy:
    rollingUpdate:
      deletePolicy: Oldest
      maxSurge: 25%
      maxUnavailable: 1
    type: RollingUpdate
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: Premium_LRS
      osType: Linux
    sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
    vmSize: ${AZURE_NODE_MACHINE_TYPE}
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfig
metadata:
  name: ${CLUSTER_NAME}-vmss
  namespace: default
spec:
  files:
  - contentFrom:
      secret:
        key: worker-node-azure.json
        name: ${CLUSTER_NAME}-vmss-azure-json
    owner: root:root
    path: /etc/kubernetes/azure.json
    permissions: "0644"
  - contentFrom:
      secret:
        key: value
        name: ${CLUSTER_NAME}-kubeconfig
    owner: root:root
    path: /etc/kubernetes/admin.conf
    permissions: "0644"
  joinConfiguration:
    discovery:
      file:
        kubeConfigPath: /etc/kubernetes/admin.conf
    nodeRegistration:
      kubeletExtraArgs:
        cloud-provider: external
      name: '{{ ds.meta_data["local_hostname"] }}'
  preKubeadmCommands:
  - kubeadm init phase upload-config all
  ```
