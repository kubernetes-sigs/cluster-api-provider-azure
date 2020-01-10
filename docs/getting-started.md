# Getting started with cluster-api-provider-azure <!-- omit in toc -->

## Contents <!-- omit in toc -->

<!-- Below is generated using VSCode yzhang.markdown-all-in-one >

<!-- TOC depthFrom:2 -->

- [Requirements](#requirements)
  - [Optional](#optional)
- [Prerequisites](#prerequisites)
  - [Install release binaries](#install-release-binaries)
    - [Building from master](#building-from-master)
- [Deploying a cluster](#deploying-a-cluster)
  - [Setting up the environment](#setting-up-the-environment)
  - [Generating cluster manifests and example cluster](#generating-cluster-manifests-and-example-cluster)
    - [Customizing the cluster deployment](#customizing-the-cluster-deployment)
    - [Running the manifest generation script](#running-the-manifest-generation-script)
  - [Creating a cluster](#creating-a-cluster)
- [Using the cluster](#using-the-cluster)
- [Troubleshooting](#troubleshooting)
  - [Bootstrap running, but resources aren't being created](#bootstrap-running-but-resources-arent-being-created)
  - [Resources are created but control plane is taking a long time to become ready](#resources-are-created-but-control-plane-is-taking-a-long-time-to-become-ready)

<!-- /TOC -->

## Requirements

- Linux or MacOS (Windows isn't supported at the moment)
- A set of Azure credentials sufficient to bootstrap the cluster (an Azure service principal with Collaborator rights).
- [KIND]
- [kubectl]
- [kustomize]
- make
- [gettext](https://www.gnu.org/software/gettext/) (with `envsubst` in your PATH)
- md5sum
- [bazel](https://docs.bazel.build/versions/1.2.0/getting-started.html)

### Optional

- [Homebrew][brew] (MacOS)
- [jq]
- [Go]

[brew]: https://brew.sh/
[go]: https://golang.org/dl/
[jq]: https://stedolan.github.io/jq/download/
[kind]: https://sigs.k8s.io/kind
[kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[kustomize]: https://github.com/kubernetes-sigs/kustomize

## Prerequisites

### Install release binaries

Get the latest [release of `clusterctl`](https://github.com/kubernetes-sigs/cluster-api-provider-azure/releases) and place them in your PATH.

#### Building from master

If you're interested in developing cluster-api-provider-azure and getting the latest version from `master`, please follow the [development guide][development].

## Deploying a cluster

### Setting up the environment

An Azure Service Principal is needed for usage by the `clusterctl` tool and for populating the controller manifests. This utilizes [environment-based authentication](https://docs.microsoft.com/en-us/go/azure/azure-sdk-go-authorization#use-environment-based-authentication).

The following environment variables should be set:

- `AZURE_SUBSCRIPTION_ID`
- `AZURE_TENANT_ID`
- `AZURE_CLIENT_ID`
- `AZURE_CLIENT_SECRET`

An alternative is to install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) and have the project's script create the service principal automatically. _Note that the service principals created by the scripts will not be deleted automatically._

### Generating cluster manifests and example cluster

Download the cluster-api-provider-azure-examples.tar file and unpack it.

```bash
tar -xvf cluster-api-provider-azure-examples.tar
```

#### Customizing the cluster deployment

A set of sane defaults are utilized when generating manifests via `./azure/generate-yaml.sh`, but these can be overridden by exporting environment variables.

Here is a list of commonly overriden configuration parameters (the full list is available in `./azure/generate-yaml.sh`):

```bash
# Azure settings.
export AZURE_LOCATION="eastus"
export AZURE_RESOURCE_GROUP="kubecuddles"

# Cluster settings.
export CLUSTER_NAME="pony-unicorns"

# Machine settings.
export CONTROL_PLANE_MACHINE_TYPE="Standard_B2ms"
export NODE_MACHINE_TYPE="Standard_B2ms"
```

#### Using images

By default, the code will use the Azure Marketplace "capi" offer.

You can also [build your own image](https://github.com/kubernetes-sigs/image-builder/tree/master/images/capi/packer/azure) and specify the image ID in the manifests generated in the next step.

#### Running the manifest generation script

Now that the deployment has been customized, the next step is to generate the required manifests:

```bash
./azure/generate-yaml.sh
```

**Please review `manifests.md` to understand which manifests to use for various cluster scenarios.**

Manually editing the generated manifests should not be required, but this is the stage where final customizations can be done.

Verify that the manifests reflect the expected settings before continuing.

### Creating a cluster

You can now start the Cluster API controllers and deploy a new cluster in Azure:

```bash
clusterctl create cluster -v 4 \
  --bootstrap-type kind \
  --provider azure \
  -m ./azure/out/<machine-manifest> \
  -c ./azure/out/<cluster-manifest> \
  -p ./azure/out/provider-components.yaml \
  -a ./azure/out/addons.yaml
```

Here is some example output from `clusterctl`:

<details>

```
I0324 23:19:37.110948   27739 decoder.go:224] decoding stream as YAML
I0324 23:19:37.111615   27739 decoder.go:224] decoding stream as YAML
I0324 23:19:37.115835   27739 createbootstrapcluster.go:27] Creating bootstrap cluster
I0324 23:19:37.115883   27739 kind.go:69] Running: kind [create cluster --name=clusterapi]

I0324 23:23:58.081879   27739 kind.go:72] Ran: kind [create cluster --name=clusterapi] Output: Creating cluster "clusterapi" ...
 • Ensuring node image (kindest/node:v1.13.3) 🖼  ...
 ✓ Ensuring node image (kindest/node:v1.13.3) 🖼
 • [control-plane] Creating node container 📦  ...
 ✓ [control-plane] Creating node container 📦
 • [control-plane] Fixing mounts 🗻  ...
 ✓ [control-plane] Fixing mounts 🗻
 • [control-plane] Starting systemd 🖥  ...
 ✓ [control-plane] Starting systemd 🖥
 • [control-plane] Waiting for docker to be ready 🐋  ...
 ✓ [control-plane] Waiting for docker to be ready 🐋
 • [control-plane] Pre-loading images 🐋  ...
 ✓ [control-plane] Pre-loading images 🐋
 • [control-plane] Creating the kubeadm config file ⛵  ...
 ✓ [control-plane] Creating the kubeadm config file ⛵
 • [control-plane] Starting Kubernetes (this may take a minute) ☸  ...
 ✓ [control-plane] Starting Kubernetes (this may take a minute) ☸
Cluster creation complete. You can now use the cluster with:

kubectl cluster-info
I0324 23:23:58.081925   27739 kind.go:69] Running: kind [get kubeconfig-path --name=clusterapi]
I0324 23:23:58.149609   27739 kind.go:72] Ran: kind [get kubeconfig-path --name=clusterapi] Output: /home/fakeuser/.kube/kind-config-clusterapi
I0324 23:23:58.150729   27739 clusterdeployer.go:78] Applying Cluster API stack to bootstrap cluster
I0324 23:23:58.150752   27739 applyclusterapicomponents.go:26] Applying Cluster API Provider Components
I0324 23:23:58.150774   27739 clusterclient.go:919] Waiting for kubectl apply...
I0324 23:23:58.756964   27739 clusterclient.go:948] Waiting for Cluster v1alpha resources to become available...
I0324 23:23:58.763134   27739 clusterclient.go:961] Waiting for Cluster v1alpha resources to be listable...
I0324 23:23:58.771783   27739 clusterdeployer.go:83] Provisioning target cluster via bootstrap cluster
I0324 23:23:58.777256   27739 applycluster.go:36] Creating cluster object test-1 in namespace "default"
I0324 23:23:58.783757   27739 clusterdeployer.go:92] Creating control plane test-1-controlplane-0 in namespace "default"
I0324 23:23:58.788974   27739 applymachines.go:36] Creating machines in namespace "default"
I0324 23:23:58.801805   27739 clusterclient.go:972] Waiting for Machine test-1-controlplane-0 to become ready...
<output snipped>
I0324 23:44:38.804516   27739 clusterclient.go:972] Waiting for Machine test-1-controlplane-0 to become ready...
I0324 23:44:38.811944   27739 clusterdeployer.go:97] Updating bootstrap cluster object for cluster test-1 in namespace "default" with control plane endpoint running on test-1-controlplane-0
I0324 23:44:38.835236   27739 clusterdeployer.go:102] Creating target cluster
I0324 23:44:38.835270   27739 getkubeconfig.go:38] Getting target cluster kubeconfig.
I0324 23:44:38.840932   27739 getkubeconfig.go:59] Waiting for kubeconfig on test-1-controlplane-0 to become ready...
I0324 23:44:38.846004   27739 applyaddons.go:25] Applying Addons
I0324 23:44:38.846038   27739 clusterclient.go:919] Waiting for kubectl apply...
I0324 23:44:40.794352   27739 clusterdeployer.go:120] Pivoting Cluster API stack to target cluster
I0324 23:44:40.794449   27739 pivot.go:67] Applying Cluster API Provider Components to Target Cluster
I0324 23:44:40.794495   27739 clusterclient.go:919] Waiting for kubectl apply...
I0324 23:44:43.012086   27739 pivot.go:72] Pivoting Cluster API objects from bootstrap to target cluster.
I0324 23:44:43.012155   27739 pivot.go:83] Ensuring cluster v1alpha1 resources are available on the source cluster
I0324 23:44:43.012225   27739 clusterclient.go:948] Waiting for Cluster v1alpha resources to become available...
I0324 23:44:43.015046   27739 clusterclient.go:961] Waiting for Cluster v1alpha resources to be listable...
I0324 23:44:43.023466   27739 pivot.go:88] Ensuring cluster v1alpha1 resources are available on the target cluster
I0324 23:44:43.023547   27739 clusterclient.go:948] Waiting for Cluster v1alpha resources to become available...
I0324 23:44:43.190476   27739 clusterclient.go:961] Waiting for Cluster v1alpha resources to be listable...
I0324 23:44:43.251215   27739 pivot.go:93] Parsing list of cluster-api controllers from provider components
I0324 23:44:43.251262   27739 decoder.go:224] decoding stream as YAML
I0324 23:44:43.269534   27739 pivot.go:101] Scaling down controller azure-provider-system/azure-provider-controller-manager
I0324 23:44:43.402303   27739 pivot.go:101] Scaling down controller cluster-api-system/cluster-api-controller-manager
I0324 23:44:43.409192   27739 pivot.go:107] Retrieving list of MachineClasses to move
I0324 23:44:43.411218   27739 pivot.go:212] Preparing to copy MachineClasses: []
I0324 23:44:43.411252   27739 pivot.go:117] Retrieving list of Clusters to move
I0324 23:44:43.415212   27739 pivot.go:171] Preparing to move Clusters: [test-1]
I0324 23:44:43.415246   27739 pivot.go:234] Moving Cluster default/test-1
I0324 23:44:43.415269   27739 pivot.go:236] Ensuring namespace "default" exists on target cluster
I0324 23:44:43.929506   27739 pivot.go:247] Retrieving list of MachineDeployments to move for Cluster default/test-1
I0324 23:44:43.932485   27739 pivot.go:287] Preparing to move MachineDeployments: []
I0324 23:44:43.932519   27739 pivot.go:256] Retrieving list of MachineSets not associated with a MachineDeployment to move for Cluster default/test-1
I0324 23:44:43.935291   27739 pivot.go:331] Preparing to move MachineSets: []
I0324 23:44:43.935325   27739 pivot.go:265] Retrieving list of Machines not associated with a MachineSet to move for Cluster default/test-1
I0324 23:44:43.937558   27739 pivot.go:374] Preparing to move Machines: [test-1-controlplane-0]
I0324 23:44:43.937592   27739 pivot.go:385] Moving Machine default/test-1-controlplane-0
I0324 23:44:44.353159   27739 clusterclient.go:972] Waiting for Machine test-1-controlplane-0 to become ready...
I0324 23:44:44.408896   27739 pivot.go:399] Successfully moved Machine default/test-1-controlplane-0
I0324 23:44:44.434373   27739 pivot.go:278] Successfully moved Cluster default/test-1
I0324 23:44:44.434407   27739 pivot.go:127] Retrieving list of MachineDeployments not associated with a Cluster to move
I0324 23:44:44.436304   27739 pivot.go:287] Preparing to move MachineDeployments: []
I0324 23:44:44.436327   27739 pivot.go:136] Retrieving list of MachineSets not associated with a MachineDeployment or a Cluster to move
I0324 23:44:44.437964   27739 pivot.go:331] Preparing to move MachineSets: []
I0324 23:44:44.437987   27739 pivot.go:145] Retrieving list of Machines not associated with a MachineSet or a Cluster to move
I0324 23:44:44.439735   27739 pivot.go:374] Preparing to move Machines: []
I0324 23:44:44.439780   27739 pivot.go:186] Preparing to delete MachineClasses: []
I0324 23:44:44.439803   27739 pivot.go:158] Deleting provider components from source cluster
I0324 23:44:57.831009   27739 clusterdeployer.go:125] Saving provider components to the target cluster
I0324 23:44:58.696579   27739 clusterdeployer.go:133] Updating target cluster object with control plane endpoint running on test-1-controlplane-0
I0324 23:44:58.846821   27739 clusterdeployer.go:138] Creating node machines in target cluster.
I0324 23:44:58.886538   27739 applymachines.go:36] Creating machines in namespace "default"
I0324 23:44:58.923186   27739 clusterclient.go:972] Waiting for Machine test-1-node-c426q to become ready...
<output snipped>
I0324 23:53:58.955885   27739 clusterclient.go:972] Waiting for Machine test-1-node-c426q to become ready...
I0324 23:53:58.997577   27739 clusterdeployer.go:143] Done provisioning cluster. You can now access your cluster with kubectl --kubeconfig kubeconfig
I0324 23:53:58.997892   27739 createbootstrapcluster.go:36] Cleaning up bootstrap cluster.
I0324 23:53:58.997937   27739 kind.go:69] Running: kind [delete cluster --name=clusterapi]
I0324 23:54:00.260254   27739 kind.go:72] Ran: kind [delete cluster --name=clusterapi] Output: Deleting cluster "clusterapi" ...
```

</details>

The created KIND cluster is ephemeral and is cleaned up automatically when done. During the cluster creation, the kubectl context is set to "kind-clusterapi" and can be retrieved using `kubectl cluster-info --context kind-clusterapi`.

For a more in-depth look into what `clusterctl` is doing during this create step, please see the [clusterctl document](/docs/clusterctl.md).

## Using the cluster

The kubeconfig for the new cluster is created in the directory from where the above `clusterctl create` was run.

Run the following command to point `kubectl` to the kubeconfig of the new cluster:

```bash
export KUBECONFIG=$(pwd)/kubeconfig
```

Alternatively, move the kubeconfig file to a desired location and set the `KUBECONFIG` environment variable accordingly.

## Troubleshooting

### Bootstrap running, but resources aren't being created

Logs can be tailed using [`kubectl`][kubectl]:

```bash
kubectl logs azure-provider-controller-manager-0 -n azure-provider-system -f
```

### Resources are created but control plane is taking a long time to become ready

You can check the custom script logs by SSHing into the VM created and reading `/var/lib/waagent/custom-script/download/0/{stdout,stderr}`.

[development]: /docs/development.md
