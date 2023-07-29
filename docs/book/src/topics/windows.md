# Windows Clusters

## Overview

CAPZ enables you to create Windows Kubernetes clusters on Microsoft Azure. We recommend using Containerd for the Windows runtime in Cluster API for Azure.

### Using Containerd for Windows Clusters

To deploy a cluster using Windows, use the [Windows flavor template](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-windows.yaml).

## Deploy a workload

After you Windows VM is up and running you can deploy a workload. Using the deployment file below:

```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: iis-1809
  labels:
    app: iis-1809
spec:
  replicas: 1
  template:
    metadata:
      name: iis-1809
      labels:
        app: iis-1809
    spec:
      containers:
      - name: iis
        image: mcr.microsoft.com/windows/servercore/iis:windowsservercore-ltsc2019
        resources:
          limits:
            cpu: 1
            memory: 800m
          requests:
            cpu: .1
            memory: 300m
        ports:
          - containerPort: 80
      nodeSelector:
        "kubernetes.io/os": windows
  selector:
    matchLabels:
      app: iis-1809
---
apiVersion: v1
kind: Service
metadata:
  name: iis
spec:
  type: LoadBalancer
  ports:
  - protocol: TCP
    port: 80
  selector:
    app: iis-1809
```

Save this file to iis.yaml then deploy it:

```
kubectl apply -f .\iis.yaml
```

Get the Service endpoint and curl the website:

```
kubectl get services
NAME         TYPE           CLUSTER-IP   EXTERNAL-IP   PORT(S)        AGE
iis          LoadBalancer   10.0.9.47    <pending>     80:31240/TCP   1m
kubernetes   ClusterIP      10.0.0.1     <none>        443/TCP        46m

curl <EXTERNAL-IP>
```

#### Kube-proxy and CNIs for Containerd

The Windows HostProcess Container feature is Alpha for Kubernetes v1.22 and Beta for v1.23.  See the Windows [Hostprocess KEP](https://github.com/kubernetes/enhancements/tree/master/keps/sig-windows/1981-windows-privileged-container-support) for more details.  Kube-proxy and other CNI's  have been updated to run in HostProcess containers.  The current implementation is using [kube-proxy and Calico CNI built by sig-windows](https://github.com/kubernetes-sigs/sig-windows-tools/pull/161). Sig-windows is working to upstream the kube-proxy, cni implementations, and improve kubeadm support in the next few releases.

Current requirements:

- Kubernetes 1.23+
- containerd 1.6+
- `WindowsHostProcessContainers` feature-gate (Beta / on-by-default for v1.23) turned on for kube-apiserver and kubelet

These requirements are satisfied by the Windows Containerd Template and Azure Marketplace reference image `cncf-upstream:capi-windows:k8s-1dot22dot1-windows-2019-containerd:2021.10.15`

## Details

See the CAPI proposal for implementation details: https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20200804-windows-support.md

### VM and VMSS naming

Azure does not support creating Windows VM's with names longer than 15 characters ([see additional details historical restrictions](https://github.com/kubernetes-sigs/cluster-api/issues/2217#issuecomment-743336941)).

When creating a cluster with `AzureMachine` if the AzureMachine is longer than 15 characters then the first 9 characters of the cluster name and appends the last 5 characters of the machine to create a unique machine name.

When creating a cluster with `Machinepool` if the Machine Pool name is longer than 9 characters then the Machine pool uses the prefix `win` and appends the last 5 characters of the machine pool name.

### VM password and access
The VM password is [random generated](https://cloudbase-init.readthedocs.io/en/latest/plugins.html#setting-password-main)
by Cloudbase-init during provisioning of the VM. For Access to the VM you can use ssh, which can be configured with a
public key you provide during deployment.
It's required to specify the SSH key using the `users` property in the Kubeadm config template. Specifying the `sshPublicKey` on `AzureMachine` / `AzureMachinePool` resources only works with Linux instances.

For example like this:
```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: test1-md-0
  namespace: default
spec:
  template:
    spec:
      ...
      users:
      - name: username
        groups: Administrators
        sshAuthorizedKeys:
        - "ssh-rsa AAAA..."
```

To SSH:

```
ssh -t -i .sshkey -o 'ProxyCommand ssh -i .sshkey -W %h:%p capi@<api-server-ip>' capi@<windows-ip>
```

Refer to [SSH Access for nodes](ssh-access.md) for more instructions on how to connect using SSH.

> There is also a [CAPZ kubectl plugin](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/hack/debugging/Readme.md) that automates the ssh connection using the Management cluster

To RDP you can proxy through the api server:

```
ssh -L 5555:<windows-ip>:3389 capi@<api-server-ip>
```

And then open an RDP client on your local machine to `localhost:5555`

### Image creation
The images are built using [image-builder](https://github.com/kubernetes-sigs/image-builder) and published the the Azure Market place. They use [Cloudbase-init](https://cloudbase-init.readthedocs.io/en/latest/) to bootstrap the machines via Kubeadm.

Find the latest published images:

```
az vm image list --publisher cncf-upstream --offer capi-windows -o table --all
Offer         Publisher      Sku                                     Urn                                                                           Version
------------  -------------  ----------------------------            ------------------------------------------------------------------            ----------
capi-windows  cncf-upstream  k8s-1dot22dot1-windows-2019-containerd  cncf-upstream:capi-windows:k8s-1dot22dot1-windows-2019-containerd:2021.10.15  2021.10.15
capi-windows  cncf-upstream  k8s-1dot22dot2-windows-2019-containerd  cncf-upstream:capi-windows:k8s-1dot22dot2-windows-2019-containerd:2021.10.15  2021.10.15
```

If you would like customize your images please refer to the documentation on building your own [custom images](custom-images.md).
