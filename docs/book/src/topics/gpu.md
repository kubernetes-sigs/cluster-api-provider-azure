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
Once the nodes are `Ready`, run the following commands against the workload cluster to install the `gpu-operator` components:

We're going to use helm to install the official nvidia chart. If you don't have helm, [install it now](https://helm.sh/docs/intro/install/):

 - `brew install helm` on MacOS
 - `choco install kubernetes-helm` on Windows
 - [Installation instructions for Linux](https://helm.sh/docs/intro/install/#from-source-linux-macos)

Now that we have helm, we can install gpu-operator. First we make sure our KUBECONFIG is pointing to our target cluster:

```bash
$ clusterctl get kubeconfig azure-gpu > azure-gpu-cluster.conf
$ export KUBECONFIG=azure-gpu-cluster.conf
```

Now we can run `helm install`:

```bash
$ helm install --repo https://nvidia.github.io/gpu-operator gpu-operator --create-namespace --namespace gpu-operator-resources --generate-name
NAME: gpu-operator-1647645444
LAST DEPLOYED: Fri Mar 18 16:17:28 2022
NAMESPACE: gpu-operator-resources
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

It will take a few mins, but eventually you should see a bunch of components running, and successful (with a pod status of `Completed`) validator pods:

```bash
$ kubectl get pods -n gpu-operator-resources
NAME                                                              READY   STATUS      RESTARTS   AGE
gpu-feature-discovery-2gf4s                                       1/1     Running     0          6m7s
gpu-feature-discovery-76x2j                                       1/1     Running     0          6m8s
gpu-operator-1647645444-node-feature-discovery-master-8477dndrv   1/1     Running     0          6m50s
gpu-operator-1647645444-node-feature-discovery-worker-2jsxv       1/1     Running     0          6m50s
gpu-operator-1647645444-node-feature-discovery-worker-pkl9n       1/1     Running     0          6m50s
gpu-operator-1647645444-node-feature-discovery-worker-rb5rh       1/1     Running     0          6m50s
gpu-operator-84b88fc49c-m2bs2                                     1/1     Running     0          6m50s
nvidia-container-toolkit-daemonset-2hv8w                          1/1     Running     0          6m8s
nvidia-container-toolkit-daemonset-l52k4                          1/1     Running     0          6m7s
nvidia-cuda-validator-kzz84                                       0/1     Completed   0          2m14s
nvidia-cuda-validator-z725g                                       0/1     Completed   0          2m11s
nvidia-dcgm-exporter-7sbb9                                        1/1     Running     0          6m7s
nvidia-dcgm-exporter-f2bh4                                        1/1     Running     0          6m8s
nvidia-device-plugin-daemonset-58dx5                              1/1     Running     0          6m7s
nvidia-device-plugin-daemonset-kvtd2                              1/1     Running     0          6m8s
nvidia-device-plugin-validator-gf2k5                              0/1     Completed   0          77s
nvidia-device-plugin-validator-hsk7g                              0/1     Completed   0          102s
nvidia-driver-daemonset-kqz6q                                     1/1     Running     0          6m7s
nvidia-driver-daemonset-l2w96                                     1/1     Running     0          6m8s
nvidia-operator-validator-69wqw                                   1/1     Running     0          6m7s
nvidia-operator-validator-zl4zd                                   1/1     Running     0          6m8s
```

Then run the following commands against the workload cluster to verify that the
[NVIDIA device plugin](https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/master/nvidia-device-plugin.yml)
has initialized and the `nvidia.com/gpu` resource is available:

```bash
$ kubectl get nodes
NAME                                  STATUS   ROLES                  AGE   VERSION
nvidia-gpu-4538-control-plane-s4d4f   Ready    control-plane,master   16m   v1.22.6
nvidia-gpu-4538-md-0-mswnp            Ready    <none>                 14m   v1.22.6
nvidia-gpu-4538-md-0-qjbg6            Ready    <none>                 14m   v1.22.6
$ kubectl get node nvidia-gpu-4538-md-0-mswnp -o jsonpath={.status.allocatable} | jq
{
  "attachable-volumes-azure-disk": "24",
  "cpu": "6",
  "ephemeral-storage": "119716326407",
  "hugepages-1Gi": "0",
  "hugepages-2Mi": "0",
  "memory": "57475348Ki",
  "nvidia.com/gpu": "1",
  "pods": "110"
}
```

The important bit is the `"nvidia.com/gpu": "1",` line above.

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
