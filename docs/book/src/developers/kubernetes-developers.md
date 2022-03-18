# For Kubernetes Developers

If you are working on Kubernetes upstream, you can use the Cluster API Azure Provider to test your build of Kubernetes in an Azure environment.

## Kubernetes 1.17+

Kubernetes has removed `make WHAT=cmd/hyperkube` command you will have to build individual Kubernetes components and deploy them separately. That includes:

- [kube-apiserver](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/)
- [kube-controller-manager](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/),
- [kube-scheduler](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-scheduler/)
- [kube-proxy](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/)
- [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/)
- [kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/)
- [kubectl](https://kubernetes.io/docs/reference/kubectl/overview/).

* Run the following commands to build Kubernetes and upload artifacts to a registry and Azure blob storage:

```bash
export AZURE_STORAGE_ACCOUNT=<AzureStorageAccount>
export AZURE_STORAGE_KEY=<AzureStorageKey>
export REGISTRY=<Registry>
export TEST_K8S="true"
export JOB_NAME="test" # an enviroment variable used by CI, can be any non-empty string

source ./scripts/ci-build-kubernetes.sh
```

[A template is provided](../../../../templates/test/dev/cluster-template-custom-builds.yaml) that enables building clusters from custom built Kubernetes components:

```bash
export CLUSTER_TEMPLATE="test/dev/cluster-template-custom-builds.yaml"
./hack/create-dev-cluster.sh
```

## Testing the out-of-tree cloud provider

To test changes made to the [Azure cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure), first build and push images for cloud-controller-manager and/or cloud-node-manager from the root of the cloud-provider-azure repo.

Then, use the `external-cloud-provider` flavor to create a cluster:

// TODO the below `make create-workload-cluster` command doesn't actually work

```bash
$ export CLUSTER_NAME=my-cluster-out-of-tree
$ export CLUSTER_TEMPLATE=cluster-template-external-cloud-provider.yaml
$ make create-workload-cluster
```
After the cluster has provisioned, install the cloud-provider-azure components using the capz-tested helm chart:

```bash
$ helm install cloud-provider-azure templates/helm/cloud-provider-azure \
--set capz.clusterName=$CLUSTER_NAME --set cloudProviderAzure.cloudControllerManager.image=myrepo/my-ccm:v0.0.1 \
--set capz.clusterName=$CLUSTER_NAME --set cloudProviderAzure.cloudNodeManager.image=myrepo/my-cnm:v0.0.1 \
```
