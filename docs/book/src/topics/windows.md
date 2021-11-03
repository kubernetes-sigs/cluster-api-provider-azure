# Windows Clusters

## Overview

CAPZ enables you to create Windows Kubernetes clusters on Microsoft Azure. We recommend using Containerd for the Windows runtime in Cluster API for Azure.

### Using Containerd for Windows Clusters

To deploy a cluster using Windows, use the [Windows Containerd flavor template](../../../../templates/cluster-template-machinepool-windows-containerd.yaml).

#### Kube-proxy and CNIs for Containerd

Windows HostProcess Container support is in Alpha support in Kubernetes 1.22 and is planned to go to Beta in 1.23.  See the Windows [Hostprocess KEP](https://github.com/kubernetes/enhancements/tree/master/keps/sig-windows/1981-windows-privileged-container-support) for more details.  Kube-proxy and other CNI's  have been updated to use HostProcess containers directly.  The current implementation is using [kube-proxy and Calico CNI built by sig-windows](https://github.com/kubernetes-sigs/sig-windows-tools/pull/161). Sig-windows is working to upstream the kube-proxy, cni implementations, and better kubeadm support in the next few releases.

Current requirements:

- Kuberentes 1.22+
- containerd 1.6 Beta+ 
- `WindowsHostProcessContainers` feature-gate (currently in alpha) turned on for kube-apiserver and kubelet if using Kubernetes 1.22

These requirements are satisfied by the Windows Containerd Template and Azure Marketplace reference image `cncf-upstream:capi-windows:k8s-1dot22dot1-windows-2019-containerd:2021.10.15`

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

## Details

See the CAPI proposal for implementation details: https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20200804-windows-support.md

### VM and VMSS naming

Azure does not support creating Windows VM's with names longer than 15 characters ([see additional details historical restrictions](https://github.com/kubernetes-sigs/cluster-api/issues/2217#issuecomment-743336941)).  

When creating a cluster with `AzureMachine` if the AzureMachine is longer than 15 characters then the first 9 characters of the cluster name and appends the last 5 characters of the machine to create a unique machine name.  

When creating a cluster with `Machinepool` if the Machine Pool name is longer than 9 characters then the Machine pool uses the prefix `win` and appends the last 5 characters of the machine pool name.

### VM password and access
The VM password is [random generated](https://cloudbase-init.readthedocs.io/en/latest/plugins.html#setting-password-main)
by Cloudbase-init during provisioning of the VM. For Access to the VM you can use ssh which will be configured with SSH
public key you provided during deployment. 

To SSH:

```
ssh -t -i .sshkey -o 'ProxyCommand ssh -i .sshkey -W %h:%p capi@<api-server-ip>' capi@<windows-ip> 
```

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

### Using Docker EE and dockershim for Windows Clusters

> We recommend using [Containerd for Windows clusters](#using-containerd-for-windows-clusters)

Windows nodes can either run [Containerd (recommended)](#using-containerd-for-windows-clusters) or Docker EE as the container runtime.  
Docker EE requires the dockershim which will be [removed starting with Kubernetes 1.24](https://kubernetes.io/blog/2020/12/02/dockershim-faq/#when-will-dockershim-be-removed) and 
will be [maintained by Mirantis](https://www.mirantis.com/blog/mirantis-to-take-over-support-of-kubernetes-dockershim-2/) in the future.  We do not plan to support dockershim 
after its removal from upstream kubernetes in 1.24.

To deploy a cluster using Windows using dockershim, use the [Windows flavor template](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-windows.yaml).

#### Kube-proxy and CNIs for dockershim

Kube-proxy and Windows CNIs are deployed via Cluster Resource Sets.  Windows does not have a kube-proxy image due 
to not having Privileged containers which would provide access to the host.  The current solution is using wins.exe as 
demonstrated in the [Kubeadm support for Windows](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/adding-windows-nodes/) guide.  You may choose to run components as Windows services directly on the node but will require a [custom image](#image-creation) and modifications to the default Docker EE windows template.

Flannel is being used as the default CNI with Docker EE and dockershim.  An important note for Flannel vxlan deployments is that the MTU for the linux nodes must be set to 1400.  
This is because [Azure's VNET MTU is 1400](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-tcpip-performance-tuning#azure-and-vm-mtu) which can cause fragmentation on packets sent from the Linux node to Windows node resulting in dropped packets. 
To mitigate this we set the Linux eth0 port match 1400 and Flannel will automatically pick this up and [subtract 50](https://github.com/flannel-io/flannel/issues/1011) for the flannel network created.