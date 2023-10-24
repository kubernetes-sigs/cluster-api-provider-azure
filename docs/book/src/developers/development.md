# Developing Cluster API Provider Azure <!-- omit in toc -->

## Contents <!-- omit in toc -->

<!-- Below is generated using VSCode yzhang.markdown-all-in-one >

<!-- TOC depthFrom:2 -->

- [Setting up](#setting-up)
  - [Base requirements](#base-requirements)
  - [Get the source](#get-the-source)
  - [Get familiar with basic concepts](#get-familiar-with-basic-concepts)
  - [Dev manifest files](#dev-manifest-files)
  - [Dev images](#dev-images)
    - [Container registry](#container-registry)
- [Developing](#developing)
  - [Modules and dependencies](#modules-and-dependencies)
  - [Setting up the environment](#setting-up-the-environment)
  - [Tilt Requirements](#tilt-requirements)
  - [Using Tilt](#using-tilt)
    - [Tilt for dev in CAPZ](#tilt-for-dev-in-capz)
    - [Tilt for dev in both CAPZ and CAPI](#tilt-for-dev-in-both-capz-and-capi)
    - [Deploying a workload cluster](#deploying-a-workload-cluster)
    - [Viewing Telemetry](#viewing-telemetry)
    - [Debugging](#debugging)
  - [Manual Testing](#manual-testing)
    - [Creating a dev cluster](#creating-a-dev-cluster)
      - [Building and pushing dev images](#building-and-pushing-dev-images)
      - [Customizing the cluster deployment](#customizing-the-cluster-deployment)
      - [Creating the cluster](#creating-the-cluster)
  - [Instrumenting Telemetry](#instrumenting-telemetry)
    - [Distributed Tracing](#distributed-tracing)
    - [Metrics](#metrics)
  - [Submitting PRs and testing](#submitting-prs-and-testing)
    - [Executing unit tests](#executing-unit-tests)
  - [Automated Testing](#automated-testing)
    - [Mocks](#mocks)
    - [E2E Testing](#e2e-testing)
    - [Conformance Testing](#conformance-testing)
    - [Running custom test suites on CAPZ clusters](#running-custom-test-suites-on-capz-clusters)

<!-- /TOC -->

## Setting up

### Base requirements

1. Install [go][go]
   - Get the latest patch version for go v1.20.
2. Install [jq][jq]
   - `brew install jq` on macOS.
   - `sudo apt install jq` on Windows + WSL2
   - `sudo apt install jq` on Ubuntu Linux.
3. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on macOS.
   - `sudo apt install gettext` on Windows + WSL2.
   - `sudo apt install gettext` on Ubuntu Linux.
4. Install [KIND][kind]
   - `GO111MODULE="on" go get sigs.k8s.io/kind@v0.18.0`.
5. Install [Kustomize][kustomize]
   - `brew install kustomize` on macOS.
   - [install instructions](https://kubectl.docs.kubernetes.io/installation/kustomize/) on Windows + WSL2.
   - [install instructions][kustomizelinux] on Linux.
6. Install Python 3.x or 2.7.x, if neither is already installed.
7. Install pip
   - [pip installation instruction](https://pip.pypa.io/en/stable/installation/#installation)
8. Install make.
   - `brew install make` on MacOS.
   - `sudo apt install make` on Windows + WSL2.
   - `sudo apt install make` on Linux.
9. Install [timeout][timeout]
   - `brew install coreutils` on macOS.
10. Install [pre-commit framework](https://pre-commit.com/#installation)
   - `brew install pre-commit` Or `pip install pre-commit`. Installs pre-commit globally.
   - run `pre-commit install` at the root of the project to install pre-commit hooks to read `.pre-commit-config.yaml`
   - *Note*: use `git commit --no-verify` to skip running pre-commit workflow as and when needed.

When developing on Windows, it is suggested to set up the project on Windows + WSL2 and the file should be checked out on as wsl file system for better results.

### Get the source

```shell
git clone https://github.com/kubernetes-sigs/cluster-api-provider-azure
cd cluster-api-provider-azure
```

### Get familiar with basic concepts

This provider is modeled after the upstream Cluster API project. To get familiar
with Cluster API resources, concepts and conventions ([such as CAPI and CAPZ](https://cluster-api.sigs.k8s.io/reference/glossary.html#c)), refer to the [Cluster API Book](https://cluster-api.sigs.k8s.io/).

### Dev manifest files

Part of running cluster-api-provider-azure is generating manifests to run.
Generating dev manifests allows you to test dev images instead of the default
releases.

### Dev images

#### Container registry

Any public container registry can be leveraged for storing cluster-api-provider-azure container images.

## Developing

Change some code!

### Modules and dependencies

This repository uses [Go Modules](https://github.com/golang/go/wiki/Modules) to track and vendor dependencies.

To pin a new dependency:

- Run `go get <repository>@<version>`.
- (Optional) Add a `replace` statement in `go.mod`.

Makefile targets and scripts are offered to work with go modules:

- `make verify-modules` checks whether go module files are out of date.
- `make modules` runs `go mod tidy` to ensure proper vendoring.
- `hack/ensure-go.sh` checks that the Go version and environment variables are properly set.

### Setting up the environment

Your must have the Azure credentials as outlined in the [getting started prerequisites](../topics/getting-started.md#Prerequisites) section.

### Tilt Requirements

Install [Tilt](https://docs.tilt.dev/install.html):
 - `brew install tilt-dev/tap/tilt` on macOS or Linux
 - `scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket` & `scoop install tilt` on Windows
 - for alternatives you can follow the installation instruction for [macOS](https://docs.tilt.dev/install.html#macos),
   [Linux](https://docs.tilt.dev/install.html#linux) or [Windows](https://docs.tilt.dev/install.html#windows)

After the installation is done, verify that you have installed it correctly with: `tilt version`

Install [Helm](https://helm.sh/docs/intro/install/):
 - `brew install helm` on MacOS
 - `choco install kubernetes-helm` on Windows
 - [Install Instruction](https://helm.sh/docs/intro/install/#from-source-linux-macos) on Linux

You would require installation of Helm for successfully setting up Tilt.

### Using Tilt

Both of the [Tilt](https://tilt.dev) setups below will get you started developing CAPZ in a local kind cluster.
The main difference is the number of components you will build from source and the scope of the changes you'd like to make.
If you only want to make changes in CAPZ, then follow [CAPZ instructions](#tilt-for-dev-in-capz). This will
save you from having to build all of the images for CAPI, which can take a while. If the scope of your
development will span both CAPZ and CAPI, then follow the [CAPI and CAPZ instructions](#tilt-for-dev-in-both-capz-and-capi).

#### Tilt for dev in CAPZ

If you want to develop in CAPZ and get a local development cluster working quickly, this is the path for you.

Create a file named `tilt-settings.yaml` in the root of the CAPZ repository with the following contents:

```yaml
kustomize_substitutions:
  AZURE_SUBSCRIPTION_ID: <subscription-id>
  AZURE_TENANT_ID: <tenant-id>
  AZURE_CLIENT_SECRET: <client-secret>
  AZURE_CLIENT_ID: <client-id>
```

You should have these values saved from the [getting started prerequisites](../topics/getting-started.md#Prerequisites) section.

To build a kind cluster and start Tilt, just run:

```shell
make tilt-up
```
By default, the Cluster API components deployed by Tilt have experimental features turned off.
If you would like to enable these features, add `extra_args` as specified in [The Cluster API Book](https://cluster-api.sigs.k8s.io/developer/tilt.html#create-a-tilt-settingsjson-file).

Once your kind management cluster is up and running, you can [deploy a workload cluster](#deploying-a-workload-cluster).

To tear down the kind cluster built by the command above, just run:

```shell
make kind-reset
```

#### Tilt for dev in both CAPZ and CAPI

If you want to develop in both CAPI and CAPZ at the same time, then this is the path for you.

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow the [instructions](https://cluster-api.sigs.k8s.io/developer/tilt.html) in the cluster-api repo.  The instructions will walk you through cloning the Cluster API (CAPI) repository and configuring Tilt to use `kind` to deploy the cluster api management components.

> you may wish to checkout out the correct version of CAPI to match the [version used in CAPZ][go.mod]

Note that `tilt up` will be run from the `cluster-api repository` directory and the `tilt-settings.yaml` file will point back to the `cluster-api-provider-azure` repository directory.  Any changes you make to the source code in `cluster-api` or `cluster-api-provider-azure` repositories will automatically redeployed to the `kind` cluster.

After you have cloned both repositories, your folder structure should look like:

```tree
|-- src/cluster-api-provider-azure
|-- src/cluster-api (run `tilt up` here)
```

After configuring the environment variables, run the following to generate your `tilt-settings.yaml` file:

```shell
cat <<EOF > tilt-settings.yaml
default_registry: "${REGISTRY}"
provider_repos:
- ../cluster-api-provider-azure
enable_providers:
- azure
- docker
- kubeadm-bootstrap
- kubeadm-control-plane
kustomize_substitutions:
  AZURE_SUBSCRIPTION_ID: <subscription-id>
  AZURE_TENANT_ID: <tenant-id>
  AZURE_CLIENT_SECRET: <client-secret>
  AZURE_CLIENT_ID: <client-id>
EOF
```

Make sure to replace the credentials with the values from the [getting started prerequisites](../topics/getting-started.md#Prerequisites) section.

> `$REGISTRY` should be in the format `docker.io/<dockerhub-username>`

The cluster-api management components that are deployed are configured at the `/config` folder of each repository respectively. Making changes to those files will trigger a redeploy of the management cluster components.

#### Deploying a workload cluster

⚠️ Note that when developing with `tilt` as described above, some `clusterctl` commands won't work. Specifically, `clusterctl config` and `clusterctl generate` may fail. These commands expect specific releases of CAPI and CAPZ to be installed, but the `tilt` environment dynamically updates and installs these components from your local code. `clusterctl get kubeconfig` will still work, however.

After your kind management cluster is up and running with Tilt, you can deploy a workload cluster by opening the `tilt` web UI and clicking the clockwise arrow icon ⟳ on a resource listed, such as "aks-aad," "ipv6," or "windows."

Deploying a workload cluster from Tilt UI is also termed as flavor cluster deployment. Note that each time a flavor is deployed, it deploys a new workload cluster in addition to the existing ones. All the workload clusters must be manually deleted by the user.
Please refer to [Running flavor clusters as a tilt resource](../../../../templates/flavors/README.md#Running-flavor-clusters-as-a-tilt-resource) to learn more about this.

Or you can [configure workload cluster settings](#customizing-the-cluster-deployment) and deploy a workload cluster with the following command:

```bash
make create-workload-cluster
```

To delete the cluster:

```bash
make delete-workload-cluster
```

> Check out the [troubleshooting guide](../topics/troubleshooting.md) for common errors you might run into.

#### Viewing Telemetry

The CAPZ controller emits tracing and metrics data. When run in Tilt, the KinD management cluster is
provisioned with development deployments of OpenTelemetry for collecting distributed traces, Jaeger
for viewing traces, and Prometheus for scraping and visualizing metrics.

The OpenTelemetry, Jaeger, and Prometheus deployments are for development purposes only. These
illustrate the hooks for tracing and metrics, but lack the robustness of production cluster
deployments. For example, the Jaeger "all-in-one" component only keeps traces in memory, not in a
persistent store.

To view traces in the Jaeger interface, wait until the Tilt cluster is fully initialized. Then open
the Tilt web interface, select the "traces: jaeger-all-in-one" resource, and click "View traces"
near the top of the screen. Or visit http://localhost:16686/ in your browser. <!-- markdown-link-check-disable-line -->

To view traces in App Insights, follow the
[tracing documentation](../../../../hack/observability/opentelemetry/readme.md) before running
`make tilt-up`. Then open the Azure Portal in your browser. Find the App Insights resource you
specified in `AZURE_INSTRUMENTATION_KEY`, choose "Transaction search" on the left, and click
"Refresh" to see recent trace data.

To view metrics in the Prometheus interface, open the Tilt web interface, select the
"metrics: prometheus-operator" resource, and click "View metrics" near the top of the screen. Or
visit http://localhost:9090/ in your browser. <!-- markdown-link-check-disable-line -->

To view cluster resources using the [Cluster API Visualizer](https://github.com/Jont828/cluster-api-visualizer), select the "visualize-cluster" resource and click "View visualization" or visit "http://localhost:8000/" in your browser. <!-- markdown-link-check-disable-line -->

#### Debugging

You can debug CAPZ (or another provider / core CAPI) by running the controllers with delve. When developing using Tilt this is easily done by using the **debug** configuration section in your **tilt-settings.yaml** file. For example:

```yaml
default_registry: "${REGISTRY}"
provider_repos:
- ../cluster-api-provider-azure
enable_providers:
- azure
- docker
- kubeadm-bootstrap
- kubeadm-control-plane
kustomize_substitutions:
  AZURE_SUBSCRIPTION_ID: <subscription-id>
  AZURE_TENANT_ID: <tenant-id>
  AZURE_CLIENT_SECRET: <client-secret>
  AZURE_CLIENT_ID: <client-id>
debug:
  azure:
    continue: true
    port: 30000
```

> Note you can list multiple controllers or **core** CAPI and expose metrics as well in the debug section. Full details of the options can be seen [here](https://cluster-api.sigs.k8s.io/developer/tilt.html).

If you then start Tilt you can connect to delve via the port defined (i.e. 30000 in the sample). If you are using VSCode then you can use a launch configuration similar to this:

```json
{
   "name": "Connect to CAPZ",
   "type": "go",
   "request": "attach",
   "mode": "remote",
   "remotePath": "",
   "port": 30000,
   "host": "127.0.0.1",
   "showLog": true,
   "trace": "log",
   "logOutput": "rpc"
}
```

### Manual Testing

#### Creating a dev cluster

The steps below are provided in a convenient script in [hack/create-dev-cluster.sh](../../../../hack/create-dev-cluster.sh). Be sure to set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, and `AZURE_TENANT_ID` before running. Optionally, you can override the different cluster configuration variables. For example, to override the workload cluster name:

```bash
CLUSTER_NAME=<my-capz-cluster-name> ./hack/create-dev-cluster.sh
```

   NOTE: `CLUSTER_NAME` can only include letters, numbers, and hyphens and can't be longer than 44 characters.

##### Building and pushing dev images

1. To build images with custom tags,
   run the `make docker-build` as follows:

   ```bash
   export REGISTRY="<container-registry>"
   export MANAGER_IMAGE_TAG="<image-tag>" # optional - defaults to `dev`.
   PULL_POLICY=IfNotPresent make docker-build
   ```

2. (optional) Push your docker images:

   2.1. Login to your container registry using `docker login`.

   e.g., `docker login quay.io`

   2.2. Push to your custom image registry:

   ```bash
   REGISTRY=${REGISTRY} MANAGER_IMAGE_TAG=${MANAGER_IMAGE_TAG:="dev"} make docker-push
   ```

   NOTE: `make create-cluster` will fetch the manager image locally and load it onto the kind cluster if it is present.

##### Customizing the cluster deployment

Here is a list of required configuration parameters (the full list is available in `templates/cluster-template.yaml`):

```bash
# Cluster settings.
export CLUSTER_NAME="capz-cluster"
export AZURE_VNET_NAME=${CLUSTER_NAME}-vnet

# Azure settings.
export AZURE_LOCATION="southcentralus"
export AZURE_RESOURCE_GROUP=${CLUSTER_NAME}
export AZURE_SUBSCRIPTION_ID_B64="$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
export AZURE_TENANT_ID_B64="$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_ID_B64="$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_SECRET_B64="$(echo -n "$AZURE_CLIENT_SECRET" | base64 | tr -d '\n')"

# Machine settings.
export CONTROL_PLANE_MACHINE_COUNT=3
export AZURE_CONTROL_PLANE_MACHINE_TYPE="Standard_B2s"
export AZURE_NODE_MACHINE_TYPE="Standard_B2s"
export WORKER_MACHINE_COUNT=2
export KUBERNETES_VERSION="v1.25.6"

# Identity secret.
export AZURE_CLUSTER_IDENTITY_SECRET_NAME="cluster-identity-secret"
export CLUSTER_IDENTITY_NAME="cluster-identity"
export AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE="default"

# Generate SSH key.
# If you want to provide your own key, skip this step and set AZURE_SSH_PUBLIC_KEY_B64 to your existing file.
SSH_KEY_FILE=.sshkey
rm -f "${SSH_KEY_FILE}" 2>/dev/null
ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
echo "Machine SSH key generated in ${SSH_KEY_FILE}"
# For Linux the ssh key needs to be b64 encoded because we use the azure api to set it
# Windows doesn't support setting ssh keys so we use cloudbase-init to set which doesn't require base64
export AZURE_SSH_PUBLIC_KEY_B64=$(cat "${SSH_KEY_FILE}.pub" | base64 | tr -d '\r\n')
export AZURE_SSH_PUBLIC_KEY=$(cat "${SSH_KEY_FILE}.pub" | tr -d '\r\n')
```

⚠️ Please note the generated templates include default values and therefore require the use of `clusterctl` to create the cluster
or the use of `envsubst` to replace these values

##### Creating the cluster

⚠️ Make sure you followed the previous two steps to build the dev image and set the required environment variables before proceeding.

Ensure dev environment has been reset:

```bash
make clean kind-reset
```

Create the cluster:

```bash
make create-cluster
```

> Check out the [troubleshooting](../topics/troubleshooting.md) guide for common errors you might run into.

### Instrumenting Telemetry
Telemetry is the key to operational transparency. We strive to provide insight into the internal behavior of the
system through observable traces and metrics.

#### Distributed Tracing
Distributed tracing provides a hierarchical view of how and why an event occurred. CAPZ is instrumented to trace each
controller reconcile loop. When the reconcile loop begins, a trace span begins and is stored in loop `context.Context`.
As the context is passed on to functions below, new spans are created, tied to the parent span by the parent span ID.
The spans form a hierarchical representation of the activities in the controller.

These spans can also be propagated across service boundaries. The span context can be passed on through metadata such as
HTTP headers. By propagating span context, it creates a distributed, causal relationship between services and functions.

For tracing, we use [OpenTelemetry](https://github.com/open-telemetry).

Here is an example of staring a span in the beginning of a controller reconcile.

```go
ctx, logger, done := tele.StartSpanWithLogger(ctx, "controllers.AzureMachineReconciler.Reconcile",
   tele.KVP("namespace", req.Namespace),
   tele.KVP("name", req.Name),
   tele.KVP("kind", "AzureMachine"),
)
defer done()
```

The code above creates a context with a new span stored in the context.Context value bag. If a span already existed in
the `ctx` argument, then the new span would take on the parentID of the existing span, otherwise the new span
becomes a "root span", one that does not have a parent. The span is also created with labels, or tags, which
provide metadata about the span and can be used to query in many distributed tracing systems.

It also creates a logger that logs messages both to the span and `STDOUT`. The span is not returned directly, but closure of the span is handled by the final `done` value. This is a simple nil-ary function (`func()`) that should be called  as appropriate. Most likely, this should be done in a defer -- as shown in the above code sample -- to ensure that the span is closed at the end of your function or scope.

>Consider adding tracing if your func accepts a context.

#### Metrics
Metrics provide quantitative data about the operations of the controller. This includes cumulative data like
counters, single numerical values like guages, and distributions of counts / samples like histograms & summaries.

In CAPZ we expose metrics using the Prometheus client. The Kubebuilder project provides
[a guide for metrics and for exposing new ones](https://book.kubebuilder.io/reference/metrics.html#publishing-additional-metrics).

### Submitting PRs and testing

Pull requests and issues are highly encouraged!
If you're interested in submitting PRs to the project, please be sure to run some initial checks prior to submission:

```bash
make lint # Runs a suite of quick scripts to check code structure
make lint-fix # Runs a suite of quick scripts to fix lint errors
make verify # Runs a suite of verifying binaries
make test # Runs tests on the Go code
```

#### Executing unit tests

`make test` executes the project's unit tests. These tests do not stand up a
Kubernetes cluster, nor do they have external dependencies.

### Automated Testing

#### Mocks

Mocks for the services tests are generated using [GoMock][gomock].

To generate the mocks you can run

```bash
make generate-go
```

#### E2E Testing

To run E2E locally, set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID`, and run:

```bash
./scripts/ci-e2e.sh
```

You can optionally set the following variables:

| Variable                   | Description                                                                                                                                                                                                                                                                                                                                                                | Default                                                                               |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| `E2E_CONF_FILE`            | The path of the [E2E configuration file](https://cluster-api.sigs.k8s.io/developer/e2e.html#defining-an-e2e-config-file).                                                                                                                                                                                                                                                  | `${GOPATH}/src/sigs.k8s.io/cluster-api-provider-azure/test/e2e/config/azure-dev.yaml` |
| `SKIP_LOG_COLLECTION`      | Set to `true` if you do not want logs to be collected after running E2E tests. This is highly recommended for developers with Azure subscriptions that block SSH connections.                                                                                                                                                                                              | `false`                                                                               |
| `SKIP_CLEANUP`             | Set to `true` if you do not want the bootstrap and workload clusters to be cleaned up after running E2E tests.                                                                                                                                                                                                                                                             | `false`                                                                               |
| `SKIP_CREATE_MGMT_CLUSTER` | Skip management cluster creation. If skipping management cluster creation you must specify `KUBECONFIG` and `SKIP_CLEANUP`                                                                                                                                                                                                                                                 | `false`                                                                               |
| `USE_LOCAL_KIND_REGISTRY`  | Use Kind local registry and run the subset of tests which don't require a remotely pushed controller image. If set, `REGISTRY` is also set to `localhost:5000/ci-e2e`.                                                                                                                                                                                                     | `true`                                                                                |
| `REGISTRY`                 | Registry to push the controller image.                                                                                                                                                                                                                                                                                                                                     | `capzci.azurecr.io/ci-e2e`                                                            |
| `IMAGE_NAME`               | The name of the CAPZ controller image.                                                                                                                                                                                                                                                                                                                                     | `cluster-api-azure-controller`                                                        |
| `CONTROLLER_IMG`           | The repository/full name of the CAPZ controller image.                                                                                                                                                                                                                                                                                                                     | `${REGISTRY}/${IMAGE_NAME}`                                                           |
| `ARCH`                     | The image architecture argument to pass to Docker, allows for cross-compiling.                                                                                                                                                                                                                                                                                             | `${GOARCH}`                                                                           |
| `TAG`                      | The tag of the CAPZ controller image. If `BUILD_MANAGER_IMAGE` is set, then `TAG` is set to `$(date -u '+%Y%m%d%H%M%S')` instead of `dev`.                                                                                                                                                                                                                                 | `dev`                                                                                 |
| `BUILD_MANAGER_IMAGE`      | Build the CAPZ controller image. If not set, then we will attempt to load an image from `${CONTROLLER_IMG}-${ARCH}:${TAG}`.                                                                                                                                                                                                                                                | `true`                                                                                |
| `CLUSTER_NAME`             | Name of an existing workload cluster.  Must be set to run specs against existing workload cluster. Use in conjunction with `SKIP_CREATE_MGMT_CLUSTER`, `GINKGO_FOCUS`, `CLUSTER_NAMESPACE` and `KUBECONFIG`. Must specify **only one** e2e spec to run against with `GINKGO_FOCUS` such as `export GINKGO_FOCUS=Creating.a.VMSS.cluster.with.a.single.control.plane.node`. |
| `CLUSTER_NAMESPACE`        | Namespace of an existing workload cluster.  Must be set to run specs against existing workload cluster. Use in conjunction with `SKIP_CREATE_MGMT_CLUSTER`, `GINKGO_FOCUS`, `CLUSTER_NAME` and `KUBECONFIG`. Must specify **only one** e2e spec to run against with `GINKGO_FOCUS` such as `export GINKGO_FOCUS=Creating.a.VMSS.cluster.with.a.single.control.plane.node`. |
| `KUBECONFIG`               | Used with `SKIP_CREATE_MGMT_CLUSTER` set to true. Location of kubeconfig for the management cluster you would like to use. Use `kind get kubeconfig --name capz-e2e > kubeconfig.capz-e2e` to get the capz e2e kind cluster config                                                                                                                                         | '~/.kube/config'                                                                      |

You can also customize the configuration of the CAPZ cluster created by the E2E tests (except for `CLUSTER_NAME`, `AZURE_RESOURCE_GROUP`, `AZURE_VNET_NAME`, `CONTROL_PLANE_MACHINE_COUNT`, and `WORKER_MACHINE_COUNT`, since they are generated by individual test cases). See [Customizing the cluster deployment](#customizing-the-cluster-deployment) for more details.

#### Conformance Testing

To run the Kubernetes Conformance test suite locally, you can run

```bash
./scripts/ci-conformance.sh
```

Optional settings are:

| Environment Variable | Default Value | Description                                                                                        |
| -------------------- | ------------- | -------------------------------------------------------------------------------------------------- |
| `WINDOWS`            | `false`       | Run conformance against Windows nodes                                                              |
| `CONFORMANCE_NODES`  | `1`           | Number of parallel ginkgo nodes to run                                                             |
| `CONFORMANCE_FLAVOR` | `""`          | The flavor of the cluster to run conformance against. If not set, the default flavor will be used. |
| `IP_FAMILY`          | `IPv4`        | Set to `IPv6` to run conformance against single-stack IPv6, or `dual` for dual-stack.              |

With the following environment variables defined, you can build a CAPZ cluster from the HEAD of Kubernetes main branch or release branch, and run the Conformance test suite against it.

| Environment Variable      | Value                                                                                                                                                                                                    |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2E_ARGS`                | `-kubetest.use-ci-artifacts`                                                                                                                                                                             |
| `KUBERNETES_VERSION`      | `latest` - extract Kubernetes version from https://dl.k8s.io/ci/latest.txt (main's HEAD)<br>`latest-1.25` - extract Kubernetes version from https://dl.k8s.io/ci/latest-1.25.txt (release branch's HEAD) |
| `WINDOWS_SERVER_VERSION`  | Optional, can be `windows-2019` (default) or `windows-2022`                                                                                                                                              |
| `KUBETEST_WINDOWS_CONFIG` | Optional, can be `upstream-windows-serial-slow.yaml`, when not specified `upstream-windows.yaml` is used                                                                                                 |
| `WINDOWS_CONTAINERD_URL`  | Optional, can be any url to a `tar.gz` file containing binaries for containerd in the same format as upstream package                                                                                    |

With the following environment variables defined, CAPZ runs `./scripts/ci-build-kubernetes.sh` as part of `./scripts/ci-conformance.sh`, which allows developers to build Kubernetes from source and run the Kubernetes Conformance test suite against a CAPZ cluster based on the custom build:

| Environment Variable      | Value                                                                    |
| ------------------------- | ------------------------------------------------------------------------ |
| `AZURE_STORAGE_ACCOUNT`   | Your Azure storage account name                                          |
| `AZURE_STORAGE_KEY`       | Your Azure storage key                                                   |
| `USE_LOCAL_KIND_REGISTRY` | `false`                                                                  |
| `REGISTRY`                | Your Registry                                                            |
| `TEST_K8S`                | `true`                                                                   |

#### Running custom test suites on CAPZ clusters

To run a custom test suite on a CAPZ cluster locally, set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID` and run:

```bash
./scripts/ci-entrypoint.sh bash -c "cd ${GOPATH}/src/github.com/my-org/my-project && make e2e"
```

You can optionally set the following variables:

| Variable                    | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| --------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `AZURE_SSH_PUBLIC_KEY_FILE` | Use your own SSH key.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `SKIP_CLEANUP`              | Skip deleting the cluster after the tests finish running.                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `KUBECONFIG`                | Provide your existing cluster kubeconfig filepath. If no kubeconfig is provided, `./kubeconfig` will be used.                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `KUBERNETES_VERSION`        | Desired Kubernetes version to test. You can pass in a definitive released version, e.g., "v1.24.0". If you want to use pre-released CI bits of a particular release you may use the "latest-" prefix, e.g., "latest-1.24"; you may use the very latest built CI bits from the kubernetes/kubernetes master branch by passing in "latest". If you provide a `KUBERNETES_VERSION` environment variable, you may not also use `CI_VERSION` (below). Use only one configuration variable to declare the version of Kubernetes to test. |
| `CI_VERSION`                | Provide a custom CI version of Kubernetes (e.g., `v1.25.0-alpha.0.597+aa49dffc7f24dc`). If not specified, this will be determined from the KUBERNETES_VERSION above if it is an unreleased version. If you provide a `CI_VERSION` environment variable, you may not also use `KUBERNETES_VERSION` (above).                                                                                                                                                                                                                         |
| `TEST_CCM`                  | Build a cluster that uses custom versions of the Azure cloud-provider cloud-controller-manager and node-controller-manager images                                                                                                                                                                                                                                                                                                                                                                                                  |
| `EXP_MACHINE_POOL`          | Use [Machine Pool](../topics/machinepools.md) for worker machines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `TEST_WINDOWS`              | Build a cluster that has Windows worker nodes.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `REGISTRY`                  | Registry to push any custom k8s images or cloud provider images built.                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `CLUSTER_TEMPLATE`          | Use a custom cluster template. It can be a path to a template under templates/, a path on the host or a link. If the value is not set, the script will choose the appropriate cluster template based on existing environment variables.                                                                                                                                                                                                                                                                                            |
| `CCM_COUNT`                 | Set the number of cloud-controller-manager only when `TEST_CCM` is set. Besides it should not be more than control plane Node number.                                                                                                                                                                                                                                                                                                                                                                                              |

You can also customize the configuration of the CAPZ cluster (assuming that `SKIP_CREATE_WORKLOAD_CLUSTER` is not set). See [Customizing the cluster deployment](#customizing-the-cluster-deployment) for more details.

<!-- References -->

[go]: https://golang.org/doc/install
[jq]: https://stedolan.github.io/jq/download/
[image_pull_secrets]: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
[gettext]: https://www.gnu.org/software/gettext/
[gettextwindows]: https://mlocati.github.io/articles/gettext-iconv-windows.html
[go.mod]: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/go.mod
[kind]: https://sigs.k8s.io/kind
[azure_cli]: https://learn.microsoft.com/cli/azure/install-azure-cli?view=azure-cli-latest
[manifests]: /docs/manifests.md
[kustomize]: https://github.com/kubernetes-sigs/kustomize
[kustomizelinux]: https://kubectl.docs.kubernetes.io/installation/kustomize/
[gomock]: https://github.com/golang/mock
[timeout]: http://man7.org/linux/man-pages/man1/timeout.1.html
