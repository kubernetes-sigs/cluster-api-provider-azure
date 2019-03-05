# Getting started with cluster-api-provider-azure <!-- omit in toc -->

## Contents <!-- omit in toc -->

<!-- Below is generated using VSCode yzhang.markdown-all-in-one >

<!-- TOC depthFrom:2 -->

- [Requirements](#requirements)
  - [Optional](#optional)
  - [Install the project](#install-the-project)
    - [Release binaries](#release-binaries)
    - [Building from master](#building-from-master)
  - [Prepare your environment](#prepare-your-environment)
  - [Usage](#usage)
    - [Creating a Cluster](#creating-a-cluster)

<!-- /TOC -->

## Requirements

- Linux or MacOS (Windows isn't supported at the moment)
- A set of Azure credentials sufficient to bootstrap the cluster (an Azure service principal with Collaborator rights).
- [KIND]
- [kubectl]
- [kustomize]
- make
- gettext (with `envsubst` in your PATH)
- bazel

### Optional

- [Homebrew][brew] (MacOS)
- [jq]
- [Go]

[brew]: https://brew.sh/
[Go]: https://golang.org/dl/
[jq]: https://stedolan.github.io/jq/download/
[KIND]: https://sigs.k8s.io/kind
[kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[kustomize]: https://github.com/kubernetes-sigs/kustomize

### Install the project

#### Release binaries
TODO. Coming soon!

#### Building from master

If you're interested in developing cluster-api-provider-azure and getting the latest version from `master`, please follow the [development guide][development].

### Prepare your environment
An Azure Service Principal is needed for usage by the `clusterctl` tool and for populating the controller manifests. This utilizes [environment-based authentication](https://docs.microsoft.com/en-us/go/azure/azure-sdk-go-authorization#use-environment-based-authentication). The following environment variables should be set: `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET`.

An alternative is to install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) and have the project's script create the service principal automatically. _Note that the service principals created by the scripts will not be deleted automatically._

### Usage

#### Creating a Cluster
1. Generate the `cluster.yaml`, `machines.yaml`, and `addons.yaml` files, and create the service principal if needed.

    ```bash
    make clean # Clean up any previous manifests

    REGISTRY="<container-registry>" MANAGER_IMAGE_TAG="<image-tag>" RESOURCE_GROUP="<resource-group>" CLUSTER_NAME="<cluster-name>" make manifests # set CREATE_SP=TRUE if creating a new Service Principal is desired
   
    # If CREATE_SP=TRUE
    source cmd/clusterctl/examples/azure/out/credentials.sh
    ```
2. Ensure kind has been reset:
    ```bash
    make kind-reset
    ```
3. Create the cluster.

   **NOTE:** Kubernetes Version >= 1.11 is required to enable CRD subresources without needing a feature gate.

    ```bash
    make create-cluster
    ```

Once the cluster is created successfully, you can interact with the cluster using `kubectl` and the kubeconfig downloaded by the `clusterctl` tool.

```
export KUBECONFIG="$(kind get kubeconfig-path --name="clusterapi")"
kubectl get clusters
kubectl get machines
```

[development]: /docs/development.md
