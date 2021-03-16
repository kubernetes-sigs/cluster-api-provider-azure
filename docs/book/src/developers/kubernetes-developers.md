# For Kubernetes Developers

If you are working on Kubernetes upstream, you can use the Cluster API Azure Provider to test your build of Kubernetes in an Azure environment.

## Building Kubernetes From Source

The details for building kubernetes is outside the scope of this article.  Please refer to the [kubernetes documentation](https://github.com/kubernetes/community/tree/master/contributors/devel).  Below we will provide the basic build instructions which need to be run in the root of the kubernetes repository.

## Kubernetes 1.17+

Kubernetes has removed `make WHAT=cmd/hyperkube` command you will have to build individual Kubernetes components and deploy them separately. That includes:

- [kube-apiserver](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/)
- [kube-controller-manager](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/),
- [kube-scheduler](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-scheduler/)
- [kube-proxy](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/)
- [kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/)
- [kubectl](https://kubernetes.io/docs/reference/kubectl/overview/).

* Run the following commands to build Kubernetes and push images of individual components to a registry.  The image tag `KUBE_DOCKER_IMAGE_TAG` will need to be in a SemVer format such as `1.18.1-dirty` to work with the [Cluster API Kubeadm Bootstrap Provider](https://github.com/kubernetes-sigs/cluster-api/tree/master/bootstrap/kubeadm).  You will also need a few other images (coredns, pause, etcd) avalaible in your custom registry so we will tag them here.

```bash
export KUBE_DOCKER_REGISTRY=<your-docker-registry>
export KUBE_DOCKER_IMAGE_TAG=<your-custom-tag>
make
make quick-release-images

# Kubeadm expects then in the following format (could also use multi arch manifest if needed)
docker tag $KUBE_DOCKER_REGISTRY/kube-apiserver-amd64:$KUBE_DOCKER_IMAGE_TAG $KUBE_DOCKER_REGISTRY/kube-apiserver:$KUBE_DOCKER_IMAGE_TAG
docker tag $KUBE_DOCKER_REGISTRY/kube-controller-manager-amd64:$KUBE_DOCKER_IMAGE_TAG $KUBE_DOCKER_REGISTRY/kube-controller-manager:$KUBE_DOCKER_IMAGE_TAG
docker tag $KUBE_DOCKER_REGISTRY/kube-proxy-amd64:$KUBE_DOCKER_IMAGE_TAG $KUBE_DOCKER_REGISTRY/kube-proxy:$KUBE_DOCKER_IMAGE_TAG
docker tag $KUBE_DOCKER_REGISTRY/kube-scheduler-amd64:$KUBE_DOCKER_IMAGE_TAG $KUBE_DOCKER_REGISTRY/kube-scheduler:$KUBE_DOCKER_IMAGE_TAG

docker push $KUBE_DOCKER_REGISTRY/kube-apiserver-amd64:$KUBE_DOCKER_IMAGE_TAG
docker push $KUBE_DOCKER_REGISTRY/kube-controller-manager-amd64:$KUBE_DOCKER_IMAGE_TAG
docker push $KUBE_DOCKER_REGISTRY/kube-proxy-amd64:$KUBE_DOCKER_IMAGE_TAG
docker push $KUBE_DOCKER_REGISTRY/kube-scheduler-amd64:$KUBE_DOCKER_IMAGE_TAG

# kubeadm expects coredns, pause and etcd to be in the custom registry
docker pull k8s.gcr.io/pause:3.2
docker tag k8s.gcr.io/pause:3.2 $KUBE_DOCKER_REGISTRY/pause:3.2
docker push $KUBE_DOCKER_REGISTRY/pause:3.2

docker pull k8s.gcr.io/etcd:3.4.3-0
docker tag k8s.gcr.io/etcd:3.4.3-0 $KUBE_DOCKER_REGISTRY/etcd:3.4.3-0
docker push $KUBE_DOCKER_REGISTRY/etcd:3.4.3-0

docker pull k8s.gcr.io/coredns:1.6.7
docker tag k8s.gcr.io/coredns:1.6.7 $KUBE_DOCKER_REGISTRY/coredns:1.6.7
docker push $KUBE_DOCKER_REGISTRY/coredns:1.6.7
```

Next update your Kubeadm control plane configuration.  The details are truncated for brevity:

```yaml
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1alpha4
metadata:
  name: "your-control-plane"
spec:
  kubeadmConfigSpec:
    clusterConfiguration:
      imageRepository: "<your-registry>"
      kubernetesVersion: "v1.18.8-dirty"
```

# Using a custom kubelet

Kubeadm expects the kubelet binary to exist on the VM before it will join the node. For Cluster API you can work around this using the `preKubeadmCommands` in the `KubeadmConfigTemplate` and `KubeadmControlPlane` and providing a script to pull the component from a storage location.

In your `KubeadmControlPlane` and `KubeadmConfigTemplate` add a script reference:

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1alpha4
kind: KubeadmControlPlane
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  kubeadmConfigSpec:
    preKubeadmCommands:
    - bash -c /tmp/kubeadm-bootstrap.sh
    files:
    - path: /tmp/kubeadm-bootstrap.sh
      owner: "root:root"
      permissions: "0744"
      content: |
        #!/bin/bash

        curl -Lo $KUBELET_PACAKGE_URL/$KUBELET_PACKAGE_NAME
        mv "$KUBELET_PACKAGE_NAME" "/usr/bin/$KUBELET_PACKAGE_NAME"
        chmod +x "/usr/bin/$KUBELET_PACKAGE_NAME"
        systemctl restart kubelet
---
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha4
kind: KubeadmConfigTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
spec:
  template:
    spec:
      preKubeadmCommands:
        - bash -c /tmp/kubeadm-bootstrap.sh
      files:
        - path: /tmp/kubeadm-bootstrap.sh
          owner: "root:root"
          permissions: "0744"
          content: |
          # same as above
```


Finally, deploy your manifests and check that your images where deployed by connecting to the workload cluster and running `kubectl describe -n kube-system <kube-controller-manager-pod-id>`.

## Testing the out-of-tree cloud provider

To test changes made to the [Azure cloud provider](https://github.com/kubernetes-sigs/cloud-provider-azure), first build and push images for cloud-controller-manager and/or cloud-node-manager from the root of the cloud-provider-azure repo.

Then, use the `external-cloud-provider` flavor to create a cluster:

```bash
AZURE_CLOUD_CONTROLLER_MANAGER_IMG=myrepo/my-ccm:v0.0.1 \
AZURE_CLOUD_NODE_MANAGER_IMG=myrepo/my-cnm:v0.0.1 \
CLUSTER_TEMPLATE=cluster-template-external-cloud-provider.yaml \
make create-workload-cluster
```

## Testing clusters built from Kubernetes source

[A template is provided](../../templates/test/dev/cluster-template-custom-builds.yaml) that enables building clusters from custom built Kubernetes components. To quickly build a cluster using this template, export the following environment variables with values that declare the relevant custom-built Kubernetes component references:

- `$KUBE_BINARY_URL`
  - URL to .tar.gz file containing Kubernetes source-built artifacts
- `$KUBE_APISERVER_IMAGE_URL`
  - URL to source-built Kubernetes apiserver container image
- `$KUBE_CONTROLLER_MANAGER_IMAGE_URL`
  - URL to source-built Kubernetes controller-manager container image
- `$KUBE_SCHEDULER_IMAGE_URL`
  - URL to source-built Kubernetes scheduler container image
- `$KUBE_PROXY_IMAGE_URL`
  - URL to source-built Kubernetes kube-proxy container image

In addition, ensure that [these environment variables described in the capz documentation](https://capz.sigs.k8s.io/developers/development.html#customizing-the-cluster-deployment) are also declared and exported.

After that, you can run `make create-workload-cluster` from the git root of the `cluster-api-provider-azure` repository to create a new cluster running your specified custom Kubernetes build components. For example:

```sh
$ make create-workload-cluster
# Create workload Cluster.
/Users/jackfrancis/work/src/sigs.k8s.io/cluster-api-provider-azure/hack/tools/bin/envsubst-drone < /Users/jackfrancis/work/src/sigs.k8s.io/cluster-api-provider-azure/templates/test/dev/cluster-template-custom-builds.yaml | kubectl apply -f -
cluster.cluster.x-k8s.io/capzcustom created
azurecluster.infrastructure.cluster.x-k8s.io/capzcustom created
kubeadmcontrolplane.controlplane.cluster.x-k8s.io/capzcustom-control-plane created
azuremachinetemplate.infrastructure.cluster.x-k8s.io/capzcustom-control-plane created
machinedeployment.cluster.x-k8s.io/capzcustom-md-0 created
azuremachinetemplate.infrastructure.cluster.x-k8s.io/capzcustom-md-0 created
kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/capzcustom-md-0 created
# Wait for the kubeconfig to become available.
timeout --foreground 300 bash -c "while ! kubectl get secrets | grep capzcustom-kubeconfig; do sleep 1; done"
capzcustom-kubeconfig                 cluster.x-k8s.io/secret               1      1s
# Get kubeconfig and store it locally.
kubectl get secrets capzcustom-kubeconfig -o json | jq -r .data.value | base64 --decode > ./kubeconfig
timeout --foreground 600 bash -c "while ! kubectl --kubeconfig=./kubeconfig get nodes | grep master; do sleep 1; done"
Unable to connect to the server: dial tcp 20.190.10.231:6443: i/o timeout
capzcustom-control-plane-gmvnc   NotReady   control-plane,master   8s    v1.20.5
run "kubectl --kubeconfig=./kubeconfig ..." to work with the new target cluster
$ echo $?
0
```

The above `make create-workload-cluster` workflow assumes that your kubeconfig context points to your cluster-api management cluster. As the stdout reports, you'll be able to connect to your newly built cluster by referring to a newly generated kubeconfig file in the current working directory.

A few notes:

- The URL referenced in a `KUBE_BINARY_URL` environment variable should be a tar'd + gzip'd file that contains files at these relative filepaths in the archive:
  - `kubernetes/node/bin/kubelet`
  - `kubernetes/node/bin/kubectl`
- It's not strictly required to include a reference to every component (for example, if you just want to test a custom build of kube-proxy, you may just define the `"KUBE_PROXY_IMAGE_URL"` environment variable), but we do assume that any referenced custom components passed to the cluster template are all built from the same source.
- Specify a value of `KUBERNETES_VERSION` that shares a common minor version heritage with the source commit, if possible. If you are testing against very recent main branch upstream Kubernetes commits, the guidance is to use the most recent version of Kubernetes that your capz workflow supports. The latest stable release can always be found [here](https://storage.googleapis.com/kubernetes-release/release/stable.txt). Additionally you can use the URL template to get the latest 1.XX release:
  - https://storage.googleapis.com/kubernetes-release/release/stable-1.XX.txt
