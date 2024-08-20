# IPv6 clusters

## Overview

CAPZ enables you to create IPv6 Kubernetes clusters on Microsoft Azure.

- IPv6 support is available for Kubernetes version 1.18.0 and later on Azure.
- IPv6 support is in beta as of Kubernetes version 1.18 in Kubernetes community.

To deploy a cluster using IPv6, use the [ipv6 flavor template](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-ipv6.yaml).

<aside class="note warning">

<h1> Warning </h1>

**Action required**: The Azure DNS nameserver is only IPv4. If the coredns pod runs on the pod network, it will fail to resolve. 
The workaround is to edit the coredns deployment and add `hostNetwork: true`, so it can leverage host routes for the v4 network to do the DNS resolution.
```bash
kubectl patch deploy/coredns -n kube-system --type=merge -p '{"spec": {"template": {"spec":{"hostNetwork": true}}}}'
```

</aside>

Things to try out after the cluster created:

- Nodes are Kubernetes version 1.18.0 or later
- Nodes have an IPv6 Internal-IP

```bash
kubectl get nodes -o wide
NAME                         STATUS   ROLES    AGE   VERSION   INTERNAL-IP              EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION     CONTAINER-RUNTIME
ipv6-0-control-plane-8xqgw   Ready    master   53m   v1.18.8   2001:1234:5678:9abc::4   <none>        Ubuntu 18.04.5 LTS   5.3.0-1034-azure   containerd://1.3.4
ipv6-0-control-plane-crpvf   Ready    master   49m   v1.18.8   2001:1234:5678:9abc::5   <none>        Ubuntu 18.04.5 LTS   5.3.0-1034-azure   containerd://1.3.4
ipv6-0-control-plane-nm5v9   Ready    master   46m   v1.18.8   2001:1234:5678:9abc::6   <none>        Ubuntu 18.04.5 LTS   5.3.0-1034-azure   containerd://1.3.4
ipv6-0-md-0-7k8vm            Ready    <none>   49m   v1.18.8   2001:1234:5678:9abd::5   <none>        Ubuntu 18.04.5 LTS   5.3.0-1034-azure   containerd://1.3.4
ipv6-0-md-0-mwfpt            Ready    <none>   50m   v1.18.8   2001:1234:5678:9abd::4   <none>        Ubuntu 18.04.5 LTS   5.3.0-1034-azure   containerd://1.3.4
```

- Nodes have 2 internal IPs, one from each IP family. IPv6 clusters on Azure run on dual-stack hosts. The IPv6 is the primary IP.

```bash
kubectl get nodes ipv6-0-md-0-7k8vm -o go-template --template='{{range .status.addresses}}{{printf "%s: %s \n" .type .address}}{{end}}'
Hostname: ipv6-0-md-0-7k8vm
InternalIP: 2001:1234:5678:9abd::5
InternalIP: 10.1.0.5
```

- Nodes have an IPv6 PodCIDR

```bash
kubectl get nodes ipv6-0-md-0-7k8vm -o go-template --template='{{.spec.podCIDR}}'
2001:1234:5678:9a40:200::/72
```

- Pods have an IPv6 IP

```bash
kubectl get pods nginx-f89759699-h65lt -o go-template --template='{{.status.podIP}}'
2001:1234:5678:9a40:300::1f
```

- Able to reach other pods in cluster using IPv6

```bash
# inside the nginx-pod
#  # ifconfig eth0
  eth0      Link encap:Ethernet  HWaddr 3E:DA:12:82:4C:C2
            inet6 addr: fe80::3cda:12ff:fe82:4cc2/64 Scope:Link
            inet6 addr: 2001:1234:5678:9a40:100::4/128 Scope:Global
            UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
            RX packets:15 errors:0 dropped:0 overruns:0 frame:0
            TX packets:20 errors:0 dropped:1 overruns:0 carrier:0
            collisions:0 txqueuelen:0
            RX bytes:1562 (1.5 KiB)  TX bytes:1832 (1.7 KiB)
# ping 2001:1234:5678:9a40::2
PING 2001:1234:5678:9a40::2 (2001:1234:5678:9a40::2): 56 data bytes
64 bytes from 2001:1234:5678:9a40::2: seq=0 ttl=62 time=1.690 ms
64 bytes from 2001:1234:5678:9a40::2: seq=1 ttl=62 time=1.009 ms
64 bytes from 2001:1234:5678:9a40::2: seq=2 ttl=62 time=1.388 ms
64 bytes from 2001:1234:5678:9a40::2: seq=3 ttl=62 time=0.925 ms
```

- Kubernetes services have IPv6 ClusterIP and ExternalIP

```bash
kubectl get svc
NAME            TYPE           CLUSTER-IP   EXTERNAL-IP           PORT(S)          AGE
kubernetes      ClusterIP      fd00::1      <none>                443/TCP          94m
nginx-service   LoadBalancer   fd00::4a12   2603:1030:805:2::b    80:32136/TCP     40m
```

- Able to reach the workload on IPv6 ExternalIP

NOTE: this will only work if your ISP has IPv6 enabled. Alternatively, you can connect from an Azure VM with IPv6.

```bash
curl [2603:1030:805:2::b] -v
* Rebuilt URL to: [2603:1030:805:2::b]/
*   Trying 2603:1030:805:2::b...
* TCP_NODELAY set
* Connected to 2603:1030:805:2::b (2603:1030:805:2::b) port 80 (#0)
> GET / HTTP/1.1
> Host: [2603:1030:805:2::b]
> User-Agent: curl/7.58.0
> Accept: */*
>
< HTTP/1.1 200 OK
< Server: nginx/1.17.0
< Date: Fri, 18 Sep 2020 23:07:12 GMT
< Content-Type: text/html
< Content-Length: 612
< Last-Modified: Tue, 21 May 2019 15:33:12 GMT
< Connection: keep-alive
< ETag: "5ce41a38-264"
< Accept-Ranges: bytes
```

## Known Limitations

The reference [ipv6 flavor](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-ipv6.yaml) takes care of most of these for you, but it is important to be aware of these if you decide to write your own IPv6 cluster template, or use a different bootstrap provider.

- Kubernetes version needs to be 1.18+

- The coredns pod needs to run on the host network, so it can leverage host routes for the v4 network to do the DNS resolution. The workaround is to edit the coredns deployment and add `hostNetwork: true`:
```bash
kubectl patch deploy/coredns -n kube-system --type=merge -p '{"spec": {"template": {"spec":{"hostNetwork": true}}}}'
```

- When using [Calico CNI](https://docs.projectcalico.org/reference/public-cloud/azure), the selected pod’s subnet should be part of your Azure virtual network IP range.
