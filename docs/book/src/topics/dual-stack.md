# Dual-stack clusters

## Overview

CAPZ enables you to create [dual-stack](https://kubernetes.io/docs/concepts/services-networking/dual-stack/) Kubernetes cluster on Microsoft Azure.

- Dual-stack support is available for Kubernetes version 1.21.0 and later on Azure.

To deploy a cluster using dual-stack, use the [dual-stack flavor template](../../../../templates/cluster-template-dual-stack.yaml).

Things to try out after the cluster created:

- Nodes have 2 internal IPs, one from each IP family.

```bash
kubectl get node <node name> -o go-template --template='{{range .status.addresses}}{{printf "%s: %s \n" .type .address}}{{end}}'
Hostname: capi-dual-stack-md-0-j96nr 
InternalIP: 10.1.0.4 
InternalIP: 2001:1234:5678:9abd::4 
```

- Nodes have 2 `PodCIDRs`, one from each IP family.

```bash
kubectl get node <node name> -o go-template --template='{{range .spec.podCIDRs}}{{printf "%s\n" .}}{{end}}'
10.244.2.0/24
2001:1234:5678:9a42::/64
```

- Pods have 2 `PodIP`, one from each IP family.

```bash
kubectl get pods <pod name> -o go-template --template='{{range .status.podIPs}}{{printf "%s \n" .ip}}{{end}}' 
10.244.2.37 
2001:1234:5678:9a42::25 
```

- Able to reach other pods in cluster using IPv4 and IPv6.

```bash
# inside the nginx-pod
/ # ifconfig eth0
eth0      Link encap:Ethernet  HWaddr 8A:B2:32:92:4F:87
          inet addr:10.244.2.2  Bcast:0.0.0.0  Mask:255.255.255.255
          inet6 addr: 2001:1234:5678:9a42::2/128 Scope:Global
          inet6 addr: fe80::88b2:32ff:fe92:4f87/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:9 errors:0 dropped:0 overruns:0 frame:0
          TX packets:10 errors:0 dropped:1 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:906 (906.0 B)  TX bytes:840 (840.0 B)

/ # ping -c 2 10.244.1.2
PING 10.244.1.2 (10.244.1.2): 56 data bytes
64 bytes from 10.244.1.2: seq=0 ttl=62 time=1.366 ms
64 bytes from 10.244.1.2: seq=1 ttl=62 time=1.396 ms

--- 10.244.1.2 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 1.366/1.381/1.396 ms
/ # ping -c 2 2001:1234:5678:9a41::2
PING 2001:1234:5678:9a41::2 (2001:1234:5678:9a41::2): 56 data bytes
64 bytes from 2001:1234:5678:9a41::2: seq=0 ttl=62 time=1.264 ms
64 bytes from 2001:1234:5678:9a41::2: seq=1 ttl=62 time=1.233 ms

--- 2001:1234:5678:9a41::2 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 1.233/1.248/1.264 ms
```
