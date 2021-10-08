# Flannel

This document describes how to use [Flannel](https://github.com/flannel-io/flannel) as your CNI solution.  By default, the CNI plugin is not installed for self-managed clusters, so you have to [install your own](https://cluster-api.sigs.k8s.io/user/quick-start.html#deploy-a-cni-solution) (e.g. Calico with VXLAN).

## Modify the Cluster resources

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

### Modify Flannel config

_NOTE_: This is based off of the instructions at: https://github.com/flannel-io/flannel#deploying-flannel-manually

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

