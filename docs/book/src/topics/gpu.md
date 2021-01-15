# GPU-enabled clusters

## Overview

With CAPZ you can create GPU-enabled Kubernetes clusters on Microsoft Azure.

Before you begin, be aware that:

- [Scheduling GPUs](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/) is a Kubernetes beta feature
- [NVIDIA GPUs](https://docs.microsoft.com/en-us/azure/virtual-machines/sizes-gpu) are supported on Azure NC-series, NV-series, and NVv3-series VMs

To deploy a cluster with support for GPU nodes, use the [nvidia-gpu flavor](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/cluster-template-nvidia-gpu.yaml).

## An example GPU cluster

Let's create a CAPZ cluster with an N-series node and run a GPU-powered vector calculation.

### Generate an nvidia-gpu cluster template

Use the `clusterctl config cluster` command to generate a manifest that defines your GPU-enabled
workload cluster.

Remember to use the `nvidia-gpu` flavor with N-series nodes.

```bash
AZURE_CONTROL_PLANE_MACHINE_TYPE=Standard_D2s_v3 \
AZURE_NODE_MACHINE_TYPE=Standard_NC6s_v3 \
AZURE_LOCATION=southcentralus \
clusterctl config cluster azure-gpu \
  --kubernetes-version=v1.19.7 \
  --worker-machine-count=1 \
  --flavor=nvidia-gpu > azure-gpu-cluster.yaml
```

### Create the cluster

Apply the manifest from the previous step to your management cluster to have CAPZ create a
workload cluster:

```bash
$ kubectl apply -f azure-gpu-cluster.yaml
cluster.cluster.x-k8s.io/azure-gpu created
azurecluster.infrastructure.cluster.x-k8s.io/azure-gpu created
kubeadmcontrolplane.controlplane.cluster.x-k8s.io/azure-gpu-control-plane created
azuremachinetemplate.infrastructure.cluster.x-k8s.io/azure-gpu-control-plane created
machinedeployment.cluster.x-k8s.io/azure-gpu-md-0 created
azuremachinetemplate.infrastructure.cluster.x-k8s.io/azure-gpu-md-0 created
kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/azure-gpu-md-0 created
```

Wait until the cluster and nodes are finished provisioning. The GPU nodes may take several minutes
to provision, since each one must install drivers and supporting software.

```bash
$ kubectl get cluster azure-gpu
NAME        PHASE
azure-gpu   Provisioned
$ kubectl get machines
NAME                             PROVIDERID                                                                                                                                     PHASE     VERSION
azure-gpu-control-plane-t94nm    azure:////subscriptions/<subscription_id>/resourceGroups/azure-gpu/providers/Microsoft.Compute/virtualMachines/azure-gpu-control-plane-nnb57   Running   v1.19.7
azure-gpu-md-0-f6b88dd78-vmkph   azure:////subscriptions/<subscription_id>/resourceGroups/azure-gpu/providers/Microsoft.Compute/virtualMachines/azure-gpu-md-0-gcc8v            Running   v1.19.7
```

You can run these commands against the workload cluster to verify that the
[NVIDIA device plugin](https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/master/nvidia-device-plugin.yml)
has initialized and the `nvidia.com/gpu` resource is available:

```bash
$ clusterctl get kubeconfig azure-gpu > azure-gpu-cluster.conf
$ export KUBECONFIG=azure-gpu-cluster.conf
$ kubectl -n kube-system get po | grep nvidia
kube-system   nvidia-device-plugin-daemonset-d5dn6                    1/1     Running   0          16m
$ kubectl get nodes
NAME                            STATUS   ROLES    AGE   VERSION
azure-gpu-control-plane-nnb57   Ready    master   42m   v1.19.7
azure-gpu-md-0-gcc8v            Ready    <none>   38m   v1.19.7
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
      image: "k8s.gcr.io/cuda-vector-add:v0.1"
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
