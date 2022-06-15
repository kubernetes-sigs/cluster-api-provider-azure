# GPU-enabled clusters

## Overview

With CAPZ you can create GPU-enabled Kubernetes clusters on Microsoft Azure.

Before you begin, be aware that:

- [Scheduling GPUs](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/) is a Kubernetes beta feature
- [NVIDIA GPUs](https://docs.microsoft.com/en-us/azure/virtual-machines/sizes-gpu) are supported on Azure NC-series, NV-series, and NVv3-series VMs
- [NVIDIA GPU Operator](https://github.com/NVIDIA/gpu-operator) allows administrators of Kubernetes clusters to manage GPU nodes just like CPU nodes in the cluster.

To deploy a cluster with support for GPU nodes, use the [nvidia-gpu flavor](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-nvidia-gpu.yaml).

## An example GPU cluster

Let's create a CAPZ cluster with an N-series node and run a GPU-powered vector calculation.

### Generate an nvidia-gpu cluster template

Use the `clusterctl generate cluster` command to generate a manifest that defines your GPU-enabled
workload cluster.

Remember to use the `nvidia-gpu` flavor with N-series nodes.

```bash
AZURE_CONTROL_PLANE_MACHINE_TYPE=Standard_D2s_v3 \
AZURE_NODE_MACHINE_TYPE=Standard_NC6s_v3 \
AZURE_LOCATION=southcentralus \
clusterctl generate cluster azure-gpu \
  --kubernetes-version=v1.22.1 \
  --worker-machine-count=1 \
  --flavor=nvidia-gpu > azure-gpu-cluster.yaml
```

### Create the cluster

Apply the manifest from the previous step to your management cluster to have CAPZ create a
workload cluster:

```bash
$ kubectl apply -f azure-gpu-cluster.yaml --server-side
cluster.cluster.x-k8s.io/azure-gpu serverside-applied
azurecluster.infrastructure.cluster.x-k8s.io/azure-gpu serverside-applied
kubeadmcontrolplane.controlplane.cluster.x-k8s.io/azure-gpu-control-plane serverside-applied
azuremachinetemplate.infrastructure.cluster.x-k8s.io/azure-gpu-control-plane serverside-applied
machinedeployment.cluster.x-k8s.io/azure-gpu-md-0 serverside-applied
azuremachinetemplate.infrastructure.cluster.x-k8s.io/azure-gpu-md-0 serverside-applied
kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/azure-gpu-md-0 serverside-applied
clusterresourceset.addons.cluster.x-k8s.io/crs-gpu-operator serverside-applied
configmap/nvidia-clusterpolicy-crd serverside-applied
configmap/nvidia-gpu-operator-components serverside-applied
clusterresourceset.addons.cluster.x-k8s.io/azure-gpu-crs-0 serverside-applied
```

<aside class="note">

<h1> Note </h1>

`--server-side` is used in `kubectl apply` because a config map created as part of this cluster exceeds the annotations size limit.
More on server side apply can be found [here](https://kubernetes.io/docs/reference/using-api/server-side-apply/)

</aside>

Wait until the cluster and nodes are finished provisioning. The GPU nodes may take several minutes
to provision, since each one must install drivers and supporting software.

```bash
$ kubectl get cluster azure-gpu
NAME        PHASE
azure-gpu   Provisioned
$ kubectl get machines
NAME                             PROVIDERID                                                                                                                                     PHASE     VERSION
azure-gpu-control-plane-t94nm    azure:////subscriptions/<subscription_id>/resourceGroups/azure-gpu/providers/Microsoft.Compute/virtualMachines/azure-gpu-control-plane-nnb57   Running   v1.22.1
azure-gpu-md-0-f6b88dd78-vmkph   azure:////subscriptions/<subscription_id>/resourceGroups/azure-gpu/providers/Microsoft.Compute/virtualMachines/azure-gpu-md-0-gcc8v            Running   v1.22.1
```

Install a [CNI](https://cluster-api.sigs.k8s.io/user/quick-start.html#deploy-a-cni-solution) of your choice.
Once the nodes are `Ready`, run the following commands against the workload cluster to check if all the `gpu-operator` resources are installed:

```bash
$ clusterctl get kubeconfig azure-gpu > azure-gpu-cluster.conf
$ export KUBECONFIG=azure-gpu-cluster.conf
$ kubectl get pods | grep gpu-operator
default                  gpu-operator-1612821988-node-feature-discovery-master-664dnsmww   1/1     Running                 0          107m
default                  gpu-operator-1612821988-node-feature-discovery-worker-64mcz       1/1     Running                 0          107m
default                  gpu-operator-1612821988-node-feature-discovery-worker-h5rws       1/1     Running                 0          107m
$ kubectl get pods -n gpu-operator-resources
NAME                                       READY   STATUS      RESTARTS   AGE
gpu-feature-discovery-66d4f                1/1     Running     0          2s
nvidia-container-toolkit-daemonset-lxpkx   1/1     Running     0          3m11s
nvidia-dcgm-exporter-wwnsw                 1/1     Running     0          5s
nvidia-device-plugin-daemonset-lpdwz       1/1     Running     0          13s
nvidia-device-plugin-validation            0/1     Completed   0          10s
nvidia-driver-daemonset-w6lpb              1/1     Running     0          3m16s
```

Then run the following commands against the workload cluster to verify that the
[NVIDIA device plugin](https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/master/nvidia-device-plugin.yml)
has initialized and the `nvidia.com/gpu` resource is available:

```bash
$ kubectl -n kube-system get po | grep nvidia
kube-system   nvidia-device-plugin-daemonset-d5dn6                    1/1     Running   0          16m
$ kubectl get nodes
NAME                            STATUS   ROLES    AGE   VERSION
azure-gpu-control-plane-nnb57   Ready    master   42m   v1.22.1
azure-gpu-md-0-gcc8v            Ready    <none>   38m   v1.22.1
$ kubectl get node azure-gpu-md-0-gcc8v -o jsonpath={.status.allocatable} | jq
{
  "attachable-volumes-azure-disk": "12",
  "cpu": "6",
  "ephemeral-storage": "119716326407",
  "hugepages-1Gi": "0",
  "hugepages-2Mi": "0",
  "memory": "115312060Ki",
  "nvidia.com/gpu": "1",
  "pods": "110"
}
```

### Run a test app

Let's create a pod manifest for the `cuda-vector-add` example from the Kubernetes documentation and
deploy it:

```shell
$ cat > cuda-vector-add.yaml << EOF
apiVersion: v1
kind: Pod
metadata:
  name: cuda-vector-add
spec:
  restartPolicy: OnFailure
  containers:
    - name: cuda-vector-add
      # https://github.com/kubernetes/kubernetes/blob/v1.7.11/test/images/nvidia-cuda/Dockerfile
      image: "registry.k8s.io/cuda-vector-add:v0.1"
      resources:
        limits:
          nvidia.com/gpu: 1 # requesting 1 GPU
EOF
$ kubectl apply -f cuda-vector-add.yaml
```

The container will download, run, and perform a [CUDA](https://developer.nvidia.com/cuda-zone)
calculation with the GPU.

```bash
$ kubectl get po cuda-vector-add
cuda-vector-add   0/1     Completed   0          91s
$ kubectl logs cuda-vector-add
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

If you see output like the above, your GPU cluster is working!
