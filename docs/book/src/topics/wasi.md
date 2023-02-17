# WebAssembly / WASI Workloads

## Overview

CAPZ enables you to create WebAssembly (Wasm) / WASI pod workloads targeting either [Deislabs Slight](https://github.com/deislabs/spiderlightning) or [Fermyon Spin](https://github.com/fermyon/spin) frameworks for building and running fast, secure microservices on Kubernetes (v1.23.16+, v1.24.10+, v1.25.6+, v1.26.1+, and newer Kubernetes versions).

Both of the runtimes (slight and spin) for running Wasm workloads use [Wasmtime](https://wasmtime.dev) embedded in containerd shims via the [deislabs/containerd-wasm-shims](https://github.com/deislabs/containerd-wasm-shims) project which is built upon [containerd/runwasi](https://github.com/containerd/runwasi). These containerd shims enable Kubernetes to run Wasm workloads without needing to embed the Wasm runtime in each OCI image. 

## Slight (SpiderLightning)
Slight (or Spiderlightning) is an open source wasmtime-based runtime that provides cloud capabilities to Wasm microservices. These capabilities include key/value, pub/sub, and much more.

## Fermyon Spin
"Spin is an open source framework for building and running fast, secure, and composable cloud microservices with WebAssembly. It aims to be the easiest way to get started with WebAssembly microservices, and takes advantage of the latest developments in the WebAssembly component model and Wasmtime runtime."

### Applying the Wasm Runtime Classes
By default, CAPZ reference virtual machine images include containerd shims to run both `slight` and `spin` workloads. To inform Kubernetes about the ability to run these workloads on CAPZ nodes, you will need to apply a runtime class for each runtime (`slight` and `spin`) to your workload cluster.

```yaml
---
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: "wasmtime-slight-v1"
handler: "slight"
---
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: "wasmtime-spin-v1"
handler: "spin"
```

The preceding YAML document will register a runtime class for `slight` and `spin`, which will direct containerd to use the spin or slight shim when a pod workload is scheduled onto a cluster node.

### Running an Example Spin Workload
With the runtime classes registered, we can now schedule Wasm workloads on our nodes by applying the following YAML document to your workload cluster.

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wasm-spin
spec:
  replicas: 3
  selector:
    matchLabels:
      app: wasm-spin
  template:
    metadata:
      labels:
        app: wasm-spin
    spec:
      runtimeClassName: wasmtime-spin-v1
      containers:
        - name: spin-hello
          image: ghcr.io/deislabs/containerd-wasm-shims/examples/spin-rust-hello:latest
          command: ["/"]
          resources:
            requests:
              cpu: 10m
              memory: 10Mi
            limits:
              cpu: 500m
              memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: wasm-spin
spec:
  type: LoadBalancer
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app: wasm-spin
```

The preceding deployment and service will create a load-balanced "hello world" service with 3 Spin microservices. Note the `runtimeClassName` applied to the Deployment, `wasmtime-spin-v1`, which informs containerd on the cluster node to run the workload with the spin shim.

### A Running Spin Microservice
With the service and the deployment applied, you should now have a Spin microservice running in your workload cluster. If you run the following command against the workload cluster, you can find the IP for the `wasm-spin` service.

```shell
kubectl get services -w
NAME         TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)        AGE
kubernetes   ClusterIP      10.96.0.1       <none>          443/TCP        14m
wasm-spin    LoadBalancer   10.105.51.137   20.121.244.48   80:30197/TCP   3m8s
```

In the preceding output, we can see the `wasm-spin` service with an external IP of `20.121.244.48`. Your external IP will be a different IP address, but that is expected.

Next, let's curl the service and get a response from our Wasm microservice. You will need to replace the placeholder IP address with the external IP address from the preceding output.

```shell
curl http://20.121.244.48/hello
Hello world from Spin!
```

In the preceding output, we see the HTTP response from our Spin microservice, "Hello world from Spin!".

### Building a Spin or Slight Application
At this point, you might be asking "How do I build my own Wasm microservice?" Here are a couple pointers to help you get started.

#### Example `slight` Application
The [`slight` example in deislabs/containerd-wasm-shims repo](https://github.com/deislabs/containerd-wasm-shims/tree/ad323c4e773633630706cf1d354293dec90e61e6/images/slight) demonstrates a project layout for creating a container image consisting of a `slight` `app.wasm` and a `slightfile.toml`, both of which are needed to run the microservice.

To learn more about building `slight` applications, see [Deislabs Slight](https://github.com/deislabs/spiderlightning).

#### Example `spin` Application
The [`spin` example in deislabs/containerd-wasm-shims repo](https://github.com/deislabs/containerd-wasm-shims/tree/ad323c4e773633630706cf1d354293dec90e61e6/images/spin) demonstrates a project layout for creating a container image consisting of two `spin` apps, `spin_rust_hello.wasm` and `spin_go_hello.wasm`, and a `spin.toml` file.

To learn more about building `spin` applications, see [Fermyon Spin](https://github.com/fermyon/spin).

### Constraining Scheduling of Wasm Workloads
You may have a cluster where not all nodes are able to run Wasm workloads. In this case, you would want to constrain the nodes that are able to have Wasm workloads scheduled. 

If you would like to constrain the nodes that will run the Wasm workloads, you can apply a node label selector to the runtime classes, and apply node labels to the cluster nodes you'd like to run the workloads.

```yaml
---
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: "wasmtime-slight-v1"
handler: "slight"
scheduling:
  nodeSelector:
    "cluster.x-k8s.io/wasmtime-slight-v1": "true"
---
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: "wasmtime-spin-v1"
handler: "spin"
scheduling:
  nodeSelector:
    "cluster.x-k8s.io/wasmtime-spin-v1": "true"
```

In the preceding YAML, note the nodeSelector and the label. The Kubernetes scheduler will select nodes with the `cluster.x-k8s.io/wasmtime-slight-v1: "true"` or the `cluster.x-k8s.io/wasmtime-spin-v1: "true"` to determine where to schedule Wasm workloads.

You will also need to pair the above runtime classes with labels applied to your cluster nodes. To label your nodes, use a command like the following:

```bash
kubectl label nodes <your-node-name> <label>
```

Once you have applied node labels, you can safely schedule Wasm workloads to a constrained set of nodes in your cluster.
