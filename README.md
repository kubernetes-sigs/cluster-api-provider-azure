# Kubernetes Cluster API Azure Provider  [![Build Status](https://dev.azure.com/Cluster-API-Provider-Azure/Cluster-API-Provider-Azure%20Project/_apis/build/status/platform9.azure-provider)](https://dev.azure.com/Cluster-API-Provider-Azure/Cluster-API-Provider-Azure%20Project/_build/latest?definitionId=1)[![Go Report Card](https://goreportcard.com/badge/sigs.k8s.io/cluster-api-provider-azure)](https://goreportcard.com/report/kubernetes-sigs/cluster-api-provider-azure)

## Getting Started
### Prerequisites
1. Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
2. Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) and a [minikube driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md). It is recommended to use KVM2 driver for Linux and VirtualBox for MacOS.
3. Install [kustomize](https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md).
4. Clone the Project
    ```bash
    git clone https://github.com/kubernetes-sigs/cluster-api-provider-azure $(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-azure
    ```
5. Build the `clusterctl` tool

   ```
   cd $(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-azure
   make clusterctl
   ```

### Prepare your environment
An Azure Service Principal is needed for usage by the `clusterctl` tool and for populating the controller manifests. This utilizes [environment-based authentication](https://docs.microsoft.com/en-us/go/azure/azure-sdk-go-authorization#use-environment-based-authentication). The following environment variables should be set: `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET`.

An alternative is to install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) and have the project's script create the service principal automatically. _Not that the service prinicpals created by the scripts will not be deleted automatically._

### Usage

#### Creating a Cluster
1. Generate the `cluster.yaml`, `machines.yaml`, and `addons.yaml` files, and create the service principal if needed.

   ```
   cd cmd/clusterctl/examples/azure
   CREATE_SP=FALSE ./generate-yaml.sh # set to TRUE if creating a new Service Principal is desired
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
   Kubernetes Version >= 1.11 is required to enable CRD subresources without needing a feature gate.


    **Linux**
    ```bash
    ./bin/clusterctl create cluster --provider azure \
    -m cmd/clusterctl/examples/azure/out/machines.yaml \
    -c cmd/clusterctl/examples/azure/out/cluster.yaml \
    -p cmd/clusterctl/examples/azure/out/provider-components.yaml \
    --vm-driver kvm2 --minikube kubernetes-version=v1.12.2
    ```

    **macOS** 
    ```bash
   ./bin/clusterctl create cluster --provider azure \
   -m cmd/clusterctl/examples/azure/out/machines.yaml \
   -c cmd/clusterctl/examples/azure/out/cluster.yaml \
   -p cmd/clusterctl/examples/azure/out/provider-components.yaml \
   --vm-driver virtualbox --minikube kubernetes-version=v1.12.2
   ```
Once the cluster is created succesfully, you can interact with the cluster using `kubectl` and the kubeconfig downloaded by the `clusterctl` tool.

```
kubectl --kubeconfig=kubeconfig get clusters
kubectl --kubeconfig=kubeconfig get machines
```

### Creating and using controller images
TODO

### Testing
TODO
