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

source ./scripts/ci-build-kubernetes.sh
```

[A template is provided](../../../../templates/test/dev/cluster-template-custom-builds.yaml) that enables building clusters from custom built Kubernetes components:

```bash
export CLUSTER_TEMPLATE="test/dev/cluster-template-custom-builds.yaml"
./hack/create-dev-cluster.sh
```

## Testing the out-of-tree cloud provider

To test changes made to the [Azure cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure), first build and push images for cloud-controller-manager and/or cloud-node-manager from the branch of the cloud-provider-azure repo that the desired changes are in. Based on the repository, image name, and image tag you produce from your custom image build and push, set the appropriate environment variables below:

```bash
$ export IMAGE_REGISTRY=docker.io/myusername
$ export CCM_IMAGE_NAME=azure-cloud-controller-manager
$ export CNM_IMAGE_NAME=azure-node-controller-manager
$ export IMAGE_TAG=canary
```

Then, create a cluster:

```bash
$ export CLUSTER_NAME=my-cluster
$ make create-workload-cluster
```

Once your cluster deploys, you should receive the kubeconfig to the workload cluster. Set your `KUBECONFIG` environment variable to point to the kubeconfig file, then use the official cloud-provider-azure Helm chart to deploy the cloud-provider-azure components using your custom built images:

```bash
$ helm install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name --set infra.clusterName=${CLUSTER_NAME} \
--set cloudControllerManager.imageRepository="${IMAGE_REGISTRY}" \
--set cloudNodeManager.imageRepository="${IMAGE_REGISTRY}" \
--set cloudControllerManager.imageName="${CCM_IMAGE_NAME}" \
--set cloudNodeManager.imageName="${CNM_IMAGE_NAME}" \
--set cloudControllerManager.imageTag="${IMAGE_TAG}" \
--set cloudNodeManager.imageTag="${IMAGE_TAG}"
```

The helm command above assumes that you want to test custom images of both cloud-controller-manager and cloud-node-manager. If you only wish to test one component, you may omit the other one referenced in the example above to produce the desired `helm install` command (for example, if you wish to only test a custom cloud-controller-manager image, omit the three `--set cloudNodeManager...` arguments above).

Once you have installed the components via Helm, you should see the relevant pods running in your test cluster under the `kube-system` namespace. To iteratively develop on this test cluster, you may manually edit the `cloud-controller-manager` Deployment resource, and/or the `cloud-node-manager` Daemonset resource delivered via `helm install`. Or you may issue follow-up `helm` commands with each test iteration. For example:

```bash
$ export IMAGE_TAG=canaryv2
$ helm upgrade --install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name --set infra.clusterName=${CLUSTER_NAME} \
--set cloudControllerManager.imageRepository="${IMAGE_REGISTRY}" \
--set cloudNodeManager.imageRepository="${IMAGE_REGISTRY}" \
--set cloudControllerManager.imageName="${CCM_IMAGE_NAME}" \
--set cloudNodeManager.imageName="${CNM_IMAGE_NAME}" \
--set cloudControllerManager.imageTag="${IMAGE_TAG}" \
--set cloudNodeManager.imageTag="${IMAGE_TAG}"
$ export IMAGE_TAG=canaryv3
$ helm upgrade --install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name --set infra.clusterName=${CLUSTER_NAME} \
--set cloudControllerManager.imageRepository="${IMAGE_REGISTRY}" \
--set cloudNodeManager.imageRepository="${IMAGE_REGISTRY}" \
--set cloudControllerManager.imageName="${CCM_IMAGE_NAME}" \
--set cloudNodeManager.imageName="${CNM_IMAGE_NAME}" \
--set cloudControllerManager.imageTag="${IMAGE_TAG}" \
--set cloudNodeManager.imageTag="${IMAGE_TAG}"
```

Each successive `helm upgrade --install` command will release a new version of the chart, which will have the effect of replacing the Deployment and/or Daemonset image configurations (and thus replace the pods running in the cluster) with the new image version built and pushed for each test iteration.
