# Overview

This section provides examples for addons for self-managed clusters. For managed cluster addons, please go to the [managed cluster specifications](https://capz.sigs.k8s.io/topics/managedcluster.html#specification).

Self managed cluster addon options covered here:

- CNI - including Calico for IPv4, IPv6, dual stack, and Flannel
- [External Cloud provider](#external-cloud-provider) - including Azure File, Azure Disk CSI storage drivers

# CNI

By default, the CNI plugin is not installed for self-managed clusters, so you have to [install your own](https://cluster-api.sigs.k8s.io/user/quick-start.html#deploy-a-cni-solution).

Some of the instructions below use [Helm](https://helm.sh) to install the addons. If you're not familiar with using Helm to manage Kubernetes applications as packages, there's lots of good [Helm documentation on the official website](https://helm.sh/docs/). You can install Helm by following the [official instructions](https://helm.sh/docs/intro/install/).

## Calico

To install [Calico](https://www.tigera.io/project-calico/) on a self-managed cluster using the office Calico Helm chart, run the commands corresponding to the cluster network configuration.

### For IPv4 Clusters

Grab the IPv4 CIDR from your cluster by running this kubectl statement against the management cluster:

```bash
export IPV4_CIDR_BLOCK=$(kubectl get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[0]}')
```

Then install the Helm chart on the workload cluster:

```bash
helm repo add projectcalico https://docs.tigera.io/calico/charts && \
helm install calico projectcalico/tigera-operator --version v3.26.1 -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/addons/calico/values.yaml --set-string "installation.calicoNetwork.ipPools[0].cidr=${IPV4_CIDR_BLOCK}" --namespace tigera-operator --create-namespace
```

### For IPv6 Clusters

Grab the IPv6 CIDR from your cluster by running this kubectl statement against the management cluster:

```bash
export IPV6_CIDR_BLOCK=$(kubectl get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[0]}')
```

Then install the Helm chart on the workload cluster:

```bash
helm repo add projectcalico https://docs.tigera.io/calico/charts && \
helm install calico projectcalico/tigera-operator --version v3.26.1 -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/addons/calico-ipv6/values.yaml  --set-string "installation.calicoNetwork.ipPools[0].cidr=${IPV6_CIDR_BLOCK}" --namespace tigera-operator --create-namespace
```

### For Dual-Stack Clusters

Grab the IPv4 and IPv6 CIDRs from your cluster by running this kubectl statement against the management cluster:

```bash
export IPV4_CIDR_BLOCK=$(kubectl get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[0]}')
export IPV6_CIDR_BLOCK=$(kubectl get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[1]}')
```

Then install the Helm chart on the workload cluster:

```bash
helm repo add projectcalico https://docs.tigera.io/calico/charts && \
helm install calico projectcalico/tigera-operator --version v3.26.1 -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/addons/calico-dual-stack/values.yaml --set-string "installation.calicoNetwork.ipPools[0].cidr=${IPV4_CIDR_BLOCK}","installation.calicoNetwork.ipPools[1].cidr=${IPV6_CIDR_BLOCK}" --namespace tigera-operator --create-namespace
```

<aside class="note">

<h1> Note </h1>

For Windows nodes, you also need to copy the kubeadm-config configmap to the calico-system namespace so the calico-node-windows Daemonset can find it:

```bash
kubectl create ns calico-system
kubectl get configmap kubeadm-config --namespace=kube-system -o yaml \
| sed 's/namespace: kube-system/namespace: calico-system/' \
| kubectl create -f -
```

</aside>

For more information, see the [official Calico documentation](https://projectcalico.docs.tigera.io/getting-started/kubernetes/helm).

## Flannel

This section describes how to use [Flannel](https://github.com/flannel-io/flannel) as your CNI solution.

### Modify the Cluster resources

Before deploying the cluster, change the `KubeadmControlPlane` value at `spec.kubeadmConfigSpec.clusterConfiguration.controllerManager.extraArgs.allocate-node-cidrs` to `"true"`

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KubeadmControlPlane
spec:
  kubeadmConfigSpec:
    clusterConfiguration:
      controllerManager:
        extraArgs:
          allocate-node-cidrs: "true"
```

#### Modify Flannel config

_NOTE_: This is based off of the instructions at: <https://github.com/flannel-io/flannel#deploying-flannel-manually>

You need to make an adjustment to the default flannel configuration so that the CIDR inside your CAPZ cluster matches the Flannel Network CIDR.

View your capi-cluster.yaml and make note of the Cluster Network CIDR Block.  For example:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 192.168.0.0/16
```

Download the file at `https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml` and modify the `kube-flannel-cfg` ConfigMap.
Set the value at `data.net-conf.json.Network` value to match your Cluster Network CIDR Block.

```bash
wget https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
```

Edit kube-flannel.yml and change this section so that the Network section matches your Cluster CIDR

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: kube-flannel-cfg
data:
  net-conf.json: |
    {
      "Network": "192.168.0.0/16",
      "Backend": {
        "Type": "vxlan"
      }
    }
```

Apply kube-flannel.yml

```bash
kubectl apply -f kube-flannel.yml
```

## Using Azure CNI V1

While following the [quick start steps in Cluster API book](https://cluster-api.sigs.k8s.io/user/quick-start.html#quick-start), Azure CNI v1 can be used in place of Calico as a [container networking interface solution](https://cluster-api.sigs.k8s.io/user/quick-start.html#deploy-a-cni-solution) for your workload cluster.

Artifacts required for Azure CNI:

- [azure-cni.yaml](https://raw.githubusercontent.com/Azure/azure-container-networking/v1.5.3/hack/manifests/cni-installer-v1.yaml)

### Limitations

- Azure CNI v1 is only supported for Linux nodes. Refer to: [CAPZ#3650](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3650)

- We can only configure one subnet per control-plane node. Refer to: [CAPZ#3506](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3506)

- We can only configure one Network Interface per worker node. Refer to: [Azure-container-networking#3611](https://github.com/Azure/azure-container-networking/issues/1945)

### Update Cluster Configuration

The following resources need to be updated when using `capi-quickstart.yaml` (the default cluster manifest generated while following the Cluster API quick start).

- `kind: AzureCluster`
  - update `spec.networkSpecs.subnets` with the name and role of the subnets you want to use in your workload cluster.

  - ```yaml
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureCluster
    metadata:
      name: ${CLUSTER_NAME}
      namespace: default
    spec:
      .
      .
      networkSpec:
        subnets:
        - name: control-plane-subnet # update this as per your nomenclature
          role: control-plane
        - name: node-subnet # update this as per your nomenclature
          role: node
      .
      .
    ```

- `kind: KubeadmControlPlane` of control plane nodes
  - add `max-pods: "30"` to `spec.kubeadmConfigSpec.initConfiguration.nodeRegistration.kubeletExtraArgs`.
  - add `max-pods: "30"` to `spec.kubeadmConfigSpec.joinConfiguration.nodeRegistration.kubeletExtraArgs`.

  - ```yaml
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane
    metadata:
      name: ${CLUSTER_NAME}-control-plane
      namespace: default
    spec:
      kubeadmConfigSpec:
        .
        .
        initConfiguration:
          nodeRegistration:
            kubeletExtraArgs:
              max-pods: "30"
              .
              .
        joinConfiguration:
          nodeRegistration:
            kubeletExtraArgs:
              max-pods: "30"
              .
              .
    ```

- `kind: AzureMachineTemplate` of control-plane
  - Add `networkInterfaces` to controlplane's `AzureMachineTemplate`

  - ```yaml
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureMachineTemplate
    metadata:
      name: ${CLUSTER_NAME}-control-plane
      namespace: default
    spec:
      template:
        spec:
          .
          .
          networkInterfaces:
          - privateIPConfigs: 30
            subnetName: control-plane-subnet
          .
          .
    ```

- `kind: AzureMachineTemplate` of worker node
  - Add `networkInterfaces` to worker node's `AzureMachineTemplate`

  - ```yaml
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: AzureMachineTemplate
    metadata:
      name: ${CLUSTER_NAME}-md-0
      namespace: default
    spec:
      template:
        spec:
          networkInterfaces:
          - privateIPConfigs: 30
            subnetName: node-subnet
          .
          .
    ```

- `kind: KubeadmControlPlane` of worker nodes
  - add `max-pods: "30"` to `spec.template.spec.joinConfiguration.nodeRegistration.kubeletExtraArgs`.

  - ```yaml
    apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
    kind: KubeadmConfigTemplate
    metadata:
      name: ${CLUSTER_NAME}-md-0
      namespace: default
    spec:
      template:
        spec:
          .
          .
          joinConfiguration:
            nodeRegistration:
              kubeletExtraArgs:
                max-pods: "30"
                .
                .
    ```

# External Cloud Provider

The "external" or "out-of-tree" cloud provider for Azure is the recommended  cloud provider for CAPZ clusters. The "in-tree" cloud provider has been deprecated since v1.20 and only bug fixes are allowed in its Kubernetes repository directory.

Below are instructions to install [external cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure) components on a self-managed cluster using the official helm chart. For more information see the official [`cloud-provider-azure` helm chart documentation](https://github.com/kubernetes-sigs/cloud-provider-azure/tree/master/helm/cloud-provider-azure).

Grab the CIDR ranges from your cluster by running this kubectl statement against the management cluster:

```bash
export CCM_CIDR_BLOCK=$(kubectl get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[0]}')
if DUAL_CIDR=$(kubectl get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[1]}' 2> /dev/null); then
  export CCM_CLUSTER_CIDR="${CCM_CLUSTER_CIDR}\,${DUAL_CIDR}"
fi
```

Then install the Helm chart on the workload cluster:

```bash
helm install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name --set infra.clusterName=${CLUSTER_NAME} --set "cloudControllerManager.clusterCIDR=${CCM_CIDR_BLOCK}"
```

- **Note**: 
  When working with **Flatcar machines**, append `--set-string cloudControllerManager.caCertDir=/usr/share/ca-certificates` to the `cloud-provider-azure` _helm_ command. The helm command to install cloud provider azure for Flatcar-flavored workload cluster will be:

    ```bash
    helm install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name --set infra.clusterName=${CLUSTER_NAME} --set "cloudControllerManager.clusterCIDR=${CCM_CIDR_BLOCK}" --set-string "cloudControllerManager.caCertDir=/usr/share/ca-certificates"
    ```

The Helm chart will pick the right version of `cloud-controller-manager` and `cloud-node-manager` to work with the version of Kubernetes your cluster is running.

After running `helm install`, you should eventually see a set of pods like these in a `Running` state:

```bash
kube-system   cloud-controller-manager                                            1/1     Running   0          41s
kube-system   cloud-node-manager-5pklx                                            1/1     Running   0          26s
kube-system   cloud-node-manager-hbbqt                                            1/1     Running   0          30s
kube-system   cloud-node-manager-mfsdg                                            1/1     Running   0          39s
kube-system   cloud-node-manager-qrz74                                            1/1     Running   0          24s
```

To know more about configuring cloud-provider-azure, see [Configuring the Kubernetes Cloud Provider for Azure](./cloud-provider-config.md).

## Storage Drivers

### Azure File CSI Driver

To install the Azure File CSI driver please refer to the [installation guide](https://github.com/kubernetes-sigs/azurefile-csi-driver/blob/master/docs/install-azurefile-csi-driver.md)

Repository: <https://github.com/kubernetes-sigs/azurefile-csi-driver>

### Azure Disk CSI Driver

To install the Azure Disk CSI driver please refer to the [installation guide](https://github.com/kubernetes-sigs/azuredisk-csi-driver/blob/master/docs/install-azuredisk-csi-driver.md)

Repository: <https://github.com/kubernetes-sigs/azuredisk-csi-driver>
