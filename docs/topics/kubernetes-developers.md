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
apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
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
apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
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
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
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
