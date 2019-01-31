# Kubernetes Cluster API Azure Provider  [![Go Report Card](https://goreportcard.com/badge/kubernetes-sigs/cluster-api-provider-azure)](https://goreportcard.com/report/kubernetes-sigs/cluster-api-provider-azure)

## Getting Started

### Requirements

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

Currently, you'll need to build the latest version from `master`:

```bash
# Get the latest version of cluster-api-provider-azure
go get sigs.k8s.io/cluster-api-provider-azure

# Ensure that you have the project root as your current working directory.
cd $(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-azure

# Build the `clusterctl` tool.
make clusterctl
```

### Prepare your environment
An Azure Service Principal is needed for usage by the `clusterctl` tool and for populating the controller manifests. This utilizes [environment-based authentication](https://docs.microsoft.com/en-us/go/azure/azure-sdk-go-authorization#use-environment-based-authentication). The following environment variables should be set: `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET`.

An alternative is to install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) and have the project's script create the service principal automatically. _Note that the service principals created by the scripts will not be deleted automatically._

### Usage

#### Creating a Cluster
1. Generate the `cluster.yaml`, `machines.yaml`, and `addons.yaml` files, and create the service principal if needed.

   ```
   cd cmd/clusterctl/examples/azure
   RESOURCE_GROUP=capz-test CLUSTER_NAME="capz-test-0" ./generate-yaml.sh # set CREATE_SP=TRUE if creating a new Service Principal is desired
   cd ../../../..
   # If CREATE_SP=TRUE
   source cmd/clusterctl/examples/azure/out/credentials.sh
   ```
2. Generate the `provider-components.yaml` file.

   ```
   kustomize build config/default/ > cmd/clusterctl/examples/azure/out/provider-components.yaml
   echo "---" >> cmd/clusterctl/examples/azure/out/provider-components.yaml
   kustomize build vendor/sigs.k8s.io/cluster-api/config/default/ >> cmd/clusterctl/examples/azure/out/provider-components.yaml
   ```
3. Create the cluster.

   **NOTE:** Kubernetes Version >= 1.11 is required to enable CRD subresources without needing a feature gate.

    ```bash
    ./bin/clusterctl create cluster -v 3 \
    --provider azure \
    --bootstrap-type kind \
    -m ./cmd/clusterctl/examples/azure/out/machines.yaml \
    -c ./cmd/clusterctl/examples/azure/out/cluster.yaml \
    -p ./cmd/clusterctl/examples/azure/out/provider-components.yaml \
    -a ./cmd/clusterctl/examples/azure/out/addons.yaml
   ```

Once the cluster is created successfully, you can interact with the cluster using `kubectl` and the kubeconfig downloaded by the `clusterctl` tool.

```
export KUBECONFIG="$(kind get kubeconfig-path --name="clusterapi")"
kubectl get clusters
kubectl get machines
```

### Creating and using controller images

Here's an example of how to build controller images, if you're interested in testing changes in the image yourself:

```bash
# Build the image.
PREFIX=quay.io/k8s \
NAME=cluster-api-azure-controller \
TAG=0.2.0-alpha.5 \
make docker-build

# Push the image.
PREFIX=quay.io/k8s \
NAME=cluster-api-azure-controller \
TAG=0.2.0-alpha.5 \
make docker-push
```

**NOTE:** In order for the created images to be used for testing, you must push them to a public container registry.

### Submitting PRs and testing

Pull requests and issues are highly encouraged!
If you're interested in submitting PRs to the project, please be sure to run some initial checks prior to submission:

```bash
make check # Runs a suite of quick scripts to check code structure
make test # Runs tests on the Go code
```
