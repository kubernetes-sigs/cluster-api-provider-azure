# GPU-enabled clusters

## Overview

With CAPZ you can create GPU-enabled Kubernetes clusters on Microsoft Azure.

Before you begin, be aware that:

- [Scheduling GPUs](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/) is a Kubernetes beta feature
- [NVIDIA GPUs](https://learn.microsoft.com/azure/virtual-machines/sizes-gpu) are supported on Azure NC-series, NV-series, and NVv3-series VMs
- [NVIDIA GPU Operator](https://github.com/NVIDIA/gpu-operator) allows administrators of Kubernetes clusters to manage GPU nodes just like CPU nodes in the cluster.

To deploy a cluster with support for GPU nodes, use the [nvidia-gpu flavor](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-nvidia-gpu.yaml).

## An example GPU cluster

Let's create a CAPZ cluster with an N-series node and run a GPU-powered vector calculation.

### Generate an nvidia-gpu cluster template

Use the `clusterctl generate cluster` command to generate a manifest that defines your GPU-enabled
workload cluster.

Remember to use the `nvidia-gpu` flavor with N-series nodes.

```bash
AZURE_CONTROL_PLANE_MACHINE_TYPE=Standard_B2s \
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
$ kubectl apply -f azure-gpu-cluster.yaml
cluster.cluster.x-k8s.io/azure-gpu serverside-applied
azurecluster.infrastructure.cluster.x-k8s.io/azure-gpu serverside-applied
kubeadmcontrolplane.controlplane.cluster.x-k8s.io/azure-gpu-control-plane serverside-applied
azuremachinetemplate.infrastructure.cluster.x-k8s.io/azure-gpu-control-plane serverside-applied
machinedeployment.cluster.x-k8s.io/azure-gpu-md-0 serverside-applied
azuremachinetemplate.infrastructure.cluster.x-k8s.io/azure-gpu-md-0 serverside-applied
kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/azure-gpu-md-0 serverside-applied
```

Wait until the cluster and nodes are finished provisioning...

```bash
$ kubectl get cluster azure-gpu
NAME        PHASE
azure-gpu   Provisioned
$ kubectl get machines
NAME                             PROVIDERID                                                                                                                                     PHASE     VERSION
azure-gpu-control-plane-t94nm    azure:////subscriptions/<subscription_id>/resourceGroups/azure-gpu/providers/Microsoft.Compute/virtualMachines/azure-gpu-control-plane-nnb57   Running   v1.22.1
azure-gpu-md-0-f6b88dd78-vmkph   azure:////subscriptions/<subscription_id>/resourceGroups/azure-gpu/providers/Microsoft.Compute/virtualMachines/azure-gpu-md-0-gcc8v            Running   v1.22.1
```

... and then you can install a [CNI](https://cluster-api.sigs.k8s.io/user/quick-start.html#deploy-a-cni-solution) of your choice.

Once all nodes are `Ready`, install the official NVIDIA gpu-operator via Helm.

### Install nvidia gpu-operator Helm chart

If you don't have `helm`, installation instructions for your environment can be found [here](https://helm.sh).

First, grab the kubeconfig from your newly created cluster and save it to a file:

```bash
$ clusterctl get kubeconfig azure-gpu > ./azure-gpu-cluster.conf
```

Now we can use Helm to install the official chart:

```bash
$ helm install --kubeconfig ./azure-gpu-cluster.conf --repo https://helm.ngc.nvidia.com/nvidia gpu-operator --generate-name
```

The installation of GPU drivers via gpu-operator will take several minutes. Coffee or tea may be appropriate at this time.

After a time, you may run the following command against the workload cluster to check if all the `gpu-operator` resources are installed:

```bash
$ kubectl --kubeconfig ./azure-gpu-cluster.conf get pods -o wide | grep 'gpu\|nvidia'
NAMESPACE          NAME                                                              READY   STATUS      RESTARTS   AGE     IP               NODE                                      NOMINATED NODE   READINESS GATES
default            gpu-feature-discovery-r6zgh                                       1/1     Running     0          7m21s   192.168.132.75   azure-gpu-md-0-gcc8v            <none>           <none>
default            gpu-operator-1674686292-node-feature-discovery-master-79d8pbcg6   1/1     Running     0          8m15s   192.168.96.7     azure-gpu-control-plane-nnb57   <none>           <none>
default            gpu-operator-1674686292-node-feature-discovery-worker-g9dj2       1/1     Running     0          8m15s   192.168.132.66   gpu-md-0-gcc8v            <none>           <none>
default            gpu-operator-95b545d6f-rmlf2                                      1/1     Running     0          8m15s   192.168.132.67   gpu-md-0-gcc8v            <none>           <none>
default            nvidia-container-toolkit-daemonset-hstgw                          1/1     Running     0          7m21s   192.168.132.70   gpu-md-0-gcc8v            <none>           <none>
default            nvidia-cuda-validator-pdmkl                                       0/1     Completed   0          3m47s   192.168.132.74   azure-gpu-md-0-gcc8v            <none>           <none>
default            nvidia-dcgm-exporter-wjm7p                                        1/1     Running     0          7m21s   192.168.132.71   azure-gpu-md-0-gcc8v            <none>           <none>
default            nvidia-device-plugin-daemonset-csv6k                              1/1     Running     0          7m21s   192.168.132.73   azure-gpu-md-0-gcc8v            <none>           <none>
default            nvidia-device-plugin-validator-gxzt2                              0/1     Completed   0          2m49s   192.168.132.76   azure-gpu-md-0-gcc8v            <none>           <none>
default            nvidia-driver-daemonset-zww52                                     1/1     Running     0          7m46s   192.168.132.68   azure-gpu-md-0-gcc8v            <none>           <none>
default            nvidia-operator-validator-kjr6m                                   1/1     Running     0          7m21s   192.168.132.72   azure-gpu-md-0-gcc8v            <none>           <none>
```

You should see all pods in either a state of `Running` or `Completed`. If that is the case, then you know the driver installation and GPU node configuration is successful.

Then run the following commands against the workload cluster to verify that the
[NVIDIA device plugin](https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/main/deployments/static/nvidia-device-plugin.yml)
has initialized and the `nvidia.com/gpu` resource is available:

```bash
$ kubectl --kubeconfig ./azure-gpu-cluster.conf get nodes
NAME                            STATUS   ROLES    AGE   VERSION
azure-gpu-control-plane-nnb57   Ready    master   42m   v1.22.1
azure-gpu-md-0-gcc8v            Ready    <none>   38m   v1.22.1
$ kubectl --kubeconfig ./azure-gpu-cluster.conf get node azure-gpu-md-0-gcc8v -o jsonpath={.status.allocatable} | jq
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
$ kubectl --kubeconfig ./azure-gpu-cluster.conf apply -f cuda-vector-add.yaml
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
