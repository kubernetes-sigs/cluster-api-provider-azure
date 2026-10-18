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
    - [Tilt for dev using internal load balancer (ILB) for intra-cluster node-apiserver traffic](./tilt-with-aks-as-mgmt-ilb.md#tilt-workflow-for-aks-as-management-cluster-with-internal-load-balancer)
      - [Flavors for dev using internal load balancer (ILB) for intra-cluster node-apiserver traffic](./tilt-with-aks-as-mgmt-ilb.md#flavors-leveraging-internal-load-balancer)
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
   - Get the latest patch version for go v1.22.
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

You must have the Azure credentials as outlined in the [getting started prerequisites](../getting-started-with-aks.md#Prerequisites) section.

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

Install the [Azure CLI][azure_cli] if you use Tilt workflows that invoke `az` against Azure (for example VNet peering when using an AKS management cluster with an internal load balancer; see [Tilt with AKS as management cluster (ILB)](tilt-with-aks-as-mgmt-ilb.md)). `make tilt-up` runs `make check-az-cli` first; if `az` is missing, it prints a warning that asks you to install the CLI and fix your `PATH` before continuing with Tilt. To print the resolved `az` path when it is installed, run `VERBOSE=1 make check-az-cli`.

### Using Tilt

Both of the [Tilt](https://tilt.dev) setups below will get you started developing CAPZ in a local kind cluster.
The main difference is the number of components you will build from source and the scope of the changes you'd like to make.
If you only want to make changes in CAPZ, then follow [CAPZ instructions](#tilt-for-dev-in-capz). This will
save you from having to build all of the images for CAPI, which can take a while. If the scope of your
development will span both CAPZ and CAPI, then follow the [CAPI and CAPZ instructions](#tilt-for-dev-in-both-capz-and-capi).

#### Tilt for dev in CAPZ

<aside class="note warning">
  <h1>Warning</h1>
  To use an internal load balancer (ILB) for intra-cluster node-apiserver traffic in your workload cluster, please refer to the
  <a href="tilt-with-aks-as-mgmt-ilb.md">Tilt with AKS as Management Cluster</a> guide.
</aside>


If you want to develop in CAPZ and get a local development cluster working quickly, this is the path for you.

Create a file named `tilt-settings.yaml` in the root of the CAPZ repository with the following contents:

```yaml
kustomize_substitutions:
  AZURE_SUBSCRIPTION_ID: <subscription-id>
  AZURE_TENANT_ID: <tenant-id>
  AZURE_CLIENT_SECRET: <client-secret>
  AZURE_CLIENT_ID: <client-id>
```

You should have these values saved from the [getting started prerequisites](../getting-started-with-aks.md#Prerequisites) section.

To build a kind cluster and start Tilt, just run:

```shell
make kind-create tilt-up
```

**Note**: You can also choose an AKS cluster as a management cluster by using `aks-create` instead of `kind-create`.

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

Make sure to replace the credentials with the values from the [getting started prerequisites](../getting-started-with-aks.md#Prerequisites) section.

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

> Check out the [self-managed](../self-managed/troubleshooting.md) and [managed](../managed/troubleshooting.md) troubleshooting guides for common errors you might run into.

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

#### AI-assisted debugging

This repository includes a [debugging skill](https://github.com/kubernetes-sigs/cluster-api-provider-azure/tree/main/.claude/skills/debug-capz-k8s/SKILL.md) <!-- markdown-link-check-disable-line --> for AI coding agents that can help triage CAPZ cluster failures interactively. The skill covers:

- Live cluster inspection via `kubectl` and `az` CLI
- Prow CI artifact analysis and build log triage
- Common failure patterns for Azure infrastructure, control plane, networking, and more
- VM-level debugging with `az vm run-command`

The skill is automatically loaded by coding agents that support it, such as [GitHub Copilot coding agent](https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent), [Claude Code](https://docs.anthropic.com/en/docs/claude-code), and [opencode](https://opencode.ai). Open a session in this repository and ask the agent to debug a failing cluster or CI run.

For general troubleshooting guidance, see also the [self-managed cluster troubleshooting](../self-managed/troubleshooting.md) and [managed cluster troubleshooting](../managed/troubleshooting.md) pages.

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
export AZURE_CONTROL_PLANE_MACHINE_TYPE="Standard_D2s_v3"
export AZURE_NODE_MACHINE_TYPE="Standard_B2s"
export WORKER_MACHINE_COUNT=2
export KUBERNETES_VERSION="v1.35.4"

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

> Check out the [self-managed](../self-managed/troubleshooting.md) and [managed](../managed/troubleshooting.md) troubleshooting guides for common errors you might run into

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

Generated GoMock files (`*_mock.go`) and Kubernetes generated Go files (`zz_generated*.go`)
are excluded from Codecov coverage in `.github/codecov.yml`. Handwritten code remains included.

#### E2E Testing

To run E2E locally, set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID`, and run:

```bash
./scripts/ci-e2e.sh
```

Note: Users that have a restrictive environment and want to leverage API Server ILB in their flavors and to run e2e tests locally should refer to the detailed explanation in [running e2e tests locally leveraging apiserver ilb solution](./tilt-with-aks-as-mgmt-ilb.md#running-e2e-tests-locally-using-api-server-ilbs-networking-solution)

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
| `IP_FAMILY`          | `ipv4`        | Set to `ipv6` to run conformance against single-stack IPv6, or `dual` for dual-stack.              |

With the following environment variables defined, you can build a CAPZ cluster from the HEAD of Kubernetes main branch or release branch, and run the Conformance test suite against it.

| Environment Variable      | Value                                                                                                                                                                                                    |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2E_ARGS`                | `-kubetest.use-ci-artifacts`                                                                                                                                                                             |
| `KUBERNETES_VERSION`      | `latest` - extract Kubernetes version from https://dl.k8s.io/ci/latest.txt (main's HEAD)<br>`latest-1.<MINOR>` - extract Kubernetes version from dl.k8s.io/ci/latest-1.<MINOR>.txt (release branch's HEAD). For end-of-life minor versions, a static final patch release is returned instead (see `capz::util::get_eol_k8s_version` in `hack/util.sh`). |
| `WINDOWS_SERVER_VERSION`  | Optional, can be `windows-2022` (default) or `windows-2025`                                                                                                                                              |
| `KUBETEST_WINDOWS_CONFIG` | Default is `upstream-windows.yaml`. CAPZ contains various other configuration recipes in the `test/e2e/data/` directory; you may use any of those by referencing their file names as the value of `KUBETEST_WINDOWS_CONFIG` (e.g., `conformance-fast.yaml`), or you may drop in your own config files into `test/e2e/data/` and reference those.                                                                                                 |
| `WINDOWS_CONTAINERD_URL`  | Optional, can be any url to a `tar.gz` file containing binaries for containerd in the same format as upstream package                                                                                    |

With the following environment variables defined, CAPZ runs `./scripts/ci-build-kubernetes.sh` as part of `./scripts/ci-conformance.sh`, which allows developers to build Kubernetes from source and run the Kubernetes Conformance test suite against a CAPZ cluster based on the custom build:

| Environment Variable      | Value                                                                    |
| ------------------------- | ------------------------------------------------------------------------ |
| `AZURE_STORAGE_ACCOUNT`   | Your Azure storage account name                                          |
| `AZURE_STORAGE_KEY`       | Your Azure storage key                                                   |
| `USE_LOCAL_KIND_REGISTRY` | `false`                                                                  |
| `REGISTRY`                | Your Registry                                                            |
| `TEST_K8S`                | `true`                                                                   |

##### Experimental Kine/PostgreSQL conformance

`make test-conformance-kine` wraps the existing conformance runner with an
experimental external datastore fixture. It creates a dedicated Ubuntu 24.04
AMD64 Azure VM running **Kine v0.17.2** and **PostgreSQL 16**, then selects the
`kine-postgres` test flavor. It does not change other flavors or use AKS.
All Kine/PostgreSQL runs, including the existing
`pull-cluster-api-provider-azure-conformance-kine-postgres` Prow job, use the
shared high-I/O infrastructure described below: E112ibs_v5 datastore VM,
dedicated NVMe Ultra Disk, and larger control-plane/worker VMs.
The Prow job still runs the complete conformance suite, not ClusterLoader2;
no test-infra configuration change is needed.
Kine's downloaded binary is checked against a pinned SHA-256 checksum.
Ubuntu image and PostgreSQL package patches follow the Ubuntu repositories.

Complete the normal conformance prerequisites above, and install/authenticate
the Azure CLI, OpenSSL 1.1.1 or later, `ssh-keygen`, and `jq`.
The Azure CLI and CAPZ identities must have permissions in the same subscription
to create/delete resource groups, VMs and networks, and create VNet peerings in
both resource groups. The workload cloud-provider identity also needs its usual
Azure permissions. No roles are granted automatically by this wrapper.

```bash
export AZURE_SUBSCRIPTION_ID=<subscription-id>
# Optional: override the fixed high-I/O placement defaults.
# export AZURE_LOCATION=westus2
# export KINE_ZONE=2
# Also configure the usual CAPZ credentials and management-cluster prerequisites.
export KUBERNETES_VERSION=v1.35.4
make test-conformance-kine
```

Unset or empty `AZURE_LOCATION` and `KINE_ZONE` default to `westus2` and `2`.
The datastore, control plane, and workers share that placement; explicit
overrides are preserved. The wrapper checks VM capabilities and combined
vCPU quota before creating resources. Unsupported placement or insufficient
quota fails the job without silently selecting cheaper infrastructure.
The wrapper derives `KINE_ZONE_YAML` from `KINE_ZONE` so worker failure domains
remain strings after template generation and variable substitution.

This command **creates billable resources and runs the full Kubernetes conformance
suite**. It is an experimental configuration, not a claim that this combination
has passed conformance. Released Linux/IPv4 Kubernetes images are supported;
Windows, IPv6/dual-stack, and CI/PR Kubernetes artifact builds are not wired into
this flavor. The default is one control-plane node and two workers; set
`CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT=3` to exercise multiple API servers
against the same datastore. The datastore itself remains single-instance.

The wrapper provisions a fresh `<cluster-name>-kine` resource group containing
the VM, its OS/data disks, VNet, NSG, and an outbound-only NAT gateway/public IP. There is
no public IP on the VM and no inbound SSH rule. The fixture uses `10.250.0.0/24`;
the workload VNet uses `10.0.0.0/16`, with explicit control-plane `10.0.0.0/24`
and node `10.0.1.0/24` subnets. CAPZ creates the bidirectional peering.
Only the control-plane subnet `10.0.0.0/24` can reach Kine's TCP port 2379.
Avoid overlapping addresses if using a peered management cluster.
PostgreSQL uses Unix-socket peer authentication, not a network-facing password.

Seven-day test certificates provide mTLS between API servers and Kine.
The CA signing key is never sent to the VM or management cluster. The VM gets
server credentials and a client credential for its smoke check; CABPK gets the CA
certificate and API-server client credentials via the standard external-etcd
Secrets. Private temporary inputs are deleted when the wrapper exits.
Azure VM custom data contains test credentials, and normal E2E artifacts can
contain kubeconfigs/bootstrap data: restrict access and do not publish them
unredacted. The existing log redactor also removes the direct base64 client key.

Before creating the workload cluster, the wrapper waits for cloud-init and
executes a Kubernetes-shaped gRPC create transaction followed by a read over mTLS.
It checks a success marker because Azure Run Command can return HTTP success
even if the guest script failed. The smoke check logs the `etcdctl` version,
service state, transaction/read exit statuses, and synthetic key/value results
before asserting success; it does not log credential contents.
Kubeadm ignores only `ExternalEtcdVersion`:
standalone Kine does not serve kubeadm's HTTP `/version` endpoint. API-server
readiness and conformance still exercise the actual storage path. The flavor
removes the unused local-etcd configuration and control-plane etcd data disk.

Datastore logs and resource names are written under
`$ARTIFACTS/kine-<cluster-name>` (default `_artifacts/kine-<cluster-name>`).
`datastore-logs.json` includes the Kine service journal, the tail of cloud-init
output, and `cloud-init status --long` with its exit code.
It then deletes the fixture resource group **only after the workload resource
group is gone and the fixture ownership tag matches this invocation**.
An Azure query/deletion failure is an error, not evidence that resources are gone.
If workload cleanup fails or `SKIP_CLEANUP=true`, the fixture is retained to avoid
destroying a surviving cluster's state. Retained resources continue to incur costs.
For manual cleanup, delete the workload through CAPI first, then delete the exact
fixture resource group listed in `resources.txt`:

```bash
az group delete --subscription <subscription-id> --name <cluster-name>-kine --yes
```

`CLUSTER_NAME` may be set to a fresh name (2-40 lowercase characters); existing
workload or fixture groups are rejected. `KINE_VM_SIZE` defaults to
`Standard_E112ibs_v5` and must remain an AMD64 NVMe/Ultra-capable size.
Both conformance and performance keep PostgreSQL data/WAL on the dedicated
512-GiB Ultra Disk, provisioned for 400,000 IOPS and 10,000 MB/s by default.
Each run has its own database;
there is no state migration, database HA, upgrade/failover testing, or performance
claim in this scaffold. Those are separate experiments from conformance.

Local checks do not provision Azure resources:

```bash
bash hack/gen-flavors.sh
go test ./test/kine -count=1
```

#### Running custom test suites on CAPZ clusters

##### Shared high-I/O Kine/PostgreSQL infrastructure and performance workload

`make test-conformance-kine` and `make test-performance-kine` use identical
high-I/O infrastructure and preflight checks. The latter runs ClusterLoader2's
Pod write/watch workload **instead of conformance**, reusing the same harness
for management/workload lifecycle and cleanup. `KINE_PROFILE` selects only the
workload (`conformance` by default, or `performance`); it does not select a
different storage tier. The two test flavors inherit the same infrastructure
template. A green performance run is not a conformance result or proof that a
maximum throughput was reached.

This is a **large, billable, single-datastore experiment**, not an HA configuration:

| Setting | Default |
| --- | --- |
| Datastore VM (`KINE_VM_SIZE`) | `Standard_E112ibs_v5`: 112 vCPU, 672 GiB |
| Datastore controller | NVMe, accelerated networking, Gen2 Ubuntu 24.04 |
| Data disk | One uncached Ultra Disk, containing both PostgreSQL data and WAL |
| `KINE_DISK_SIZE_GB` / `KINE_DISK_IOPS` / `KINE_DISK_MBPS` | `512` / `400000` / `10000` |
| Control plane (`AZURE_CONTROL_PLANE_MACHINE_TYPE`) | One `Standard_D64s_v6`: 64 vCPU, 256 GiB |
| Workers (`AZURE_NODE_MACHINE_TYPE`) | Two `Standard_D16s_v5`, hosting the selected test workload |
| Region / zone | `AZURE_LOCATION=westus2`, `KINE_ZONE=2`; explicit overrides preserved |
| PostgreSQL connection limit (`KINE_POSTGRES_MAX_CONNECTIONS`) | `1024` |
| Kine maximum open connections (`KINE_DATASTORE_MAX_OPEN_CONNECTIONS`) | PostgreSQL limit minus 64 (`960` by default) |
| Kine maximum idle connections (`KINE_DATASTORE_MAX_IDLE_CONNECTIONS`) | The smaller of `128` and the maximum open connections |

The limits' numeric format and relationships are validated before provisioning,
and the limits are written to one environment file used by both PostgreSQL
initialization and the Kine service. Overrides must be positive integers; an
unlimited Kine pool is rejected. Idle connections count toward the open-connection
limit, not in addition to it. At least 64 PostgreSQL connection slots must
remain outside Kine's pool budget, including PostgreSQL's reserved slots and
headroom for administration, health checks, and monitoring. The guest verifies
the effective PostgreSQL limit and that non-reserved slots exceed Kine's pool cap.
Requested limits are recorded in `performance-settings.txt`.
These are aggressive experimental defaults for the large datastore VM, not an
assertion that more concurrent connections always improve throughput.

The default footprint needs **208 additional regional vCPUs**: 112 in the
datastore VM family, 64 in the Dsv6 family, and 32 in the Dsv5 worker family.
Changing the machine counts changes
these requirements. This excludes any separately provisioned management cluster.
The control plane and workers are pinned to the selected datastore zone, so these
results do not model cross-zone HA latency.

For either workload, start with the read-only preflight; normal CAPZ credentials
and Azure CLI login are still required:

```bash
export AZURE_SUBSCRIPTION_ID=<subscription-id>
# Optional: override westus2 / zone 2.
# export AZURE_LOCATION=<region-with-required-skus>
# export KINE_ZONE=<1-or-2-or-3>
make preflight-performance-kine
```

This checks advertised VM/zone availability, subscription restrictions, x64,
accelerated networking, datastore NVMe/Ultra support, Gen2/NVMe image support,
and available regional and VM-family vCPU quota. Family requirements are summed
across all roles. Missing capability/quota information is an error.
It writes the capacity plan and requested settings under
`$ARTIFACTS/kine-<cluster-name>` without creating Azure resources.
Passing preflight does **not** reserve hardware or validate every Azure quota
(including managed-disk/network quotas); Azure allocation can still fail.

If quota is insufficient, deliberately reduce **both** the SKU and disk settings;
there is no automatic downgrade. For example, an E32bs_v5 experiment could use:

```bash
export KINE_VM_SIZE=Standard_E32bs_v5
export KINE_DISK_IOPS=170000
export KINE_DISK_MBPS=4500
make preflight-performance-kine
```

The effective storage ceiling is bounded by both the VM and disk. Preflight does
not infer NVMe-specific aggregate performance limits from incomplete SKU metadata;
verify these against the current Azure size documentation when overriding SKUs.
Requested and provisioned settings are archived, and the run rejects a non-NVMe
VM, unexpected data-disk attachment/caching, or mismatched provisioned disk IOPS
and throughput. Disk validation failures log the expected and observed SKU, IOPS,
and throughput. The resolved Ubuntu image version is frozen within each run;
image/package patch versions can still differ between runs.

After reviewing the capacity plan and cost, select the workload:

```bash
make test-conformance-kine
# Or run the performance workload on the same infrastructure:
make test-performance-kine
```

PostgreSQL is moved off the OS disk onto the dedicated data disk before Kine
starts. Initialization refuses a missing/non-NVMe disk, existing disk signatures,
or an already mounted disk; it never falls back to the OS disk. PostgreSQL's
systemd unit requires the mount on subsequent boots. Starting tuning uses 25%
of RAM for shared buffers, a 75% effective-cache estimate, 32-GiB max WAL,
15-minute checkpoints, WAL compression, and lower autovacuum/analyze scale
factors. These are starting settings, not a demonstrated optimum.
`fsync`, `synchronous_commit`, and `full_page_writes` remain enabled.

For performance runs only, ClusterLoader2 is downloaded from a fixed perf-tests revision with a checked
archive SHA-256 and built with Go **before** Azure resources are created. The
conformance entry point does not download, build, or invoke ClusterLoader2. The
workload uses upstream templates/generators with CAPZ-specific measurements and
larger writer CPU limits. Target Pods remain unscheduled; this measures API
mutations and watch fan-out, not Pod startup. Creation and deletion surround a
sustained patch phase; continuous create/delete churn is not yet included.

| Workload variable | Default |
| --- | --- |
| `CL2_PODS_BENCHMARK_POD_COUNT` | `10000` |
| `CL2_PODS_WRITE_REPLICAS` | `10` |
| `CL2_PODS_PATCH_QPS` | `100` per writer (1,000 aggregate offered QPS) |
| `CL2_PODS_WATCH_REPLICAS` | `10` |
| `CL2_PODS_WATCHES_PER_REPLICA` | `1` |
| `CL2_PODS_BENCHMARK_DURATION` | `60m` |

Increase QPS between runs while keeping population and watch count fixed, then
vary those separately. Use achieved successful throughput, PATCH p99 latency,
errors, and generator/API-server CPU to interpret saturation. The upstream
generator can accumulate in-flight requests under overload; its offered QPS is
not proof of sustainable throughput. Check Pod restarts and resource saturation.
The upstream API responsiveness measurement enforces its latency criteria.
`PodPatchErrorCount` also fails the load phase if Prometheus observes any non-2xx
PATCH response for the main Pod resource during the measurement window. It uses
the full-window counter increase, not the rolling-rate average; Pod subresources
such as `/status` are excluded. A missing request metric fails rather than being
treated as zero errors. This is a cluster-wide API metric, not a per-generator
client error count. The profile does not yet enforce an achieved-throughput floor.

Prometheus is explicitly enabled on workers with ephemeral storage, scrapes API
servers and kubelets, and is torn down after the workload. The adapted measurements
do not require an etcd server or etcd-only metrics. An additional ServiceMonitor
collects only the write/watch generators' cAdvisor CPU and memory metrics from
`test-*` namespaces, which the upstream monitor excludes. Other benchmark Pods
remain excluded, and system-container metrics are not duplicated.

The `LoadGeneratorCPU`, `LoadGeneratorCPUThrottling`, and `LoadGeneratorMemory`
reports contain average and maximum CPU usage (cores), percentage of CFS periods
throttled, and memory working set (MiB), respectively, grouped by namespace, Pod,
and container. CPU throttling is time denied by a CPU quota, not API-server request
throttling; the period percentage is not a percentage of CPU time lost.
Idle intervals with zero CFS periods produce a zero throttling ratio. Each query
requires samples, so a completely missing metric fails the measurement instead of
producing an empty success report. These measurements do not impose resource-use
thresholds or guarantee samples from every replica.

Results and workload settings are placed in the E2E artifact directory's
`clusterloader2` subdirectory. Generator request latency, client-side errors,
restarts/OOMs, and watch reconnects or lag are not yet measured by these reports.
Full PostgreSQL/Kine time-series telemetry, raw-disk calibration, direct datastore
benchmarks, automated rate sweeps, and an equivalent etcd baseline remain separate
follow-ups; this profile alone must not be reported as an established backend ceiling.

To prepare the pinned tool without Azure, or run it against an existing disposable
cluster:

```bash
bash scripts/kine/run-clusterloader.sh --prepare-only
KUBECONFIG=/absolute/path/to/workload.kubeconfig bash scripts/kine/run-clusterloader.sh --dry-run
KUBECONFIG=/absolute/path/to/workload.kubeconfig bash scripts/kine/run-clusterloader.sh
```

Only the last command mutates the existing cluster. It creates test resources and
a `monitoring` stack: do not use a production cluster or an existing monitoring
namespace. Upstream CL2's `--dry-run` still requires a reachable cluster and
intentionally exits nonzero after dumping the compiled test configuration.
Treat artifact kubeconfigs/bootstrap data as credentials. The fixture's
ownership checks and retain-on-workload-cleanup-failure behavior still apply;
retained Ultra disks and large VMs continue to incur costs.

##### Other custom suites

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
| `KUBERNETES_VERSION`        | Desired Kubernetes version to test. You can pass in a definitive released version, e.g., "v1.24.0". If you want to use pre-released CI bits of a particular release you may use the "latest-" prefix, e.g., "latest-1.24"; you may use the very latest built CI bits from the kubernetes/kubernetes master branch by passing in "latest". For end-of-life Kubernetes versions whose CI artifacts are no longer published, CAPZ sets `KUBERNETES_VERSION` to a static final patch release instead (see `capz::util::get_eol_k8s_version` in `hack/util.sh`). If you provide a `KUBERNETES_VERSION` environment variable, you may not also use `CI_VERSION` (below). Use only one configuration variable to declare the version of Kubernetes to test. |
| `CI_VERSION`                | Provide a custom CI version of Kubernetes (e.g., `v1.25.0-alpha.0.597+aa49dffc7f24dc`). If not specified, this will be determined from the KUBERNETES_VERSION above if it is an unreleased version. If you provide a `CI_VERSION` environment variable, you may not also use `KUBERNETES_VERSION` (above).                                                                                                                                                                                                                         |
| `TEST_CCM`                  | Build a cluster that uses custom versions of the Azure cloud-provider cloud-controller-manager and node-controller-manager images                                                                                                                                                                                                                                                                                                                                                                                                  |
| `EXP_MACHINE_POOL`          | Use [Machine Pool](../self-managed/machinepools.md) for worker machines. Defaults to true.                                                                                                                                                                                                                                                                                                                                                                   |
| `TEST_WINDOWS`              | Set to "true" to build a cluster that has Windows worker nodes.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
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
