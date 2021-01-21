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
  - [Using Tilt](#using-tilt)
    - [Tilt for dev in CAPZ](#tilt-for-dev-in-capz)
    - [Tilt for dev in both CAPZ and CAPI](#tilt-for-dev-in-both-capz-and-capi)
    - [Deploying a workload cluster](#deploying-a-workload-cluster)
    - [Viewing Telemetry](#viewing-telemetry)
  - [Instrumenting Telemetry](#instrumenting-telemetry)
    - [Distributed Tracing](#distributed-tracing)
    - [Metrics](#metrics)
  - [Manual Testing](#manual-testing)
    - [Creating a dev cluster](#creating-a-dev-cluster)
      - [Building and pushing dev images](#building-and-pushing-dev-images)
      - [Customizing the cluster deployment](#customizing-the-cluster-deployment)
      - [Creating the cluster](#creating-the-cluster)
  - [Submitting PRs and testing](#submitting-prs-and-testing)
    - [Executing unit tests](#executing-unit-tests)
  - [Automated Testing](#automated-testing)
    - [Mocks](#mocks)
    - [E2E Testing](#e2e-testing)
    - [Conformance Testing](#conformance-testing)

<!-- /TOC -->

## Setting up

### Base requirements

1. Install [go][go]
   - Get the latest patch version for go v1.13.
2. Install [jq][jq]
   - `brew install jq` on macOS.
   - `chocolatey install jq` on Windows.
   - `sudo apt install jq` on Ubuntu Linux.
3. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on macOS.
   - [install instructions][gettextwindows] on Windows.
   - `sudo apt install gettext` on Ubuntu Linux.
4. Install [KIND][kind]
   - `GO111MODULE="on" go get sigs.k8s.io/kind@v0.7.0`.
5. Install [Kustomize][kustomize]
   - `brew install kustomize` on macOS.
   - `choco install kustomize` on Windows.
   - [install instructions][kustomizelinux] on Linux
6. Install Python 3.x or 2.7.x, if neither is already installed.
7. Install make.
8. Install [timeout][timeout]
   - `brew install coreutils` on macOS.

### Get the source

```shell
go get -d sigs.k8s.io/cluster-api-provider-azure
cd "$(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-azure"
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

This repositories uses [Go Modules](https://github.com/golang/go/wiki/Modules) to track and vendor dependencies.

To pin a new dependency:

- Run `go get <repository>@<version>`.
- (Optional) Add a `replace` statement in `go.mod`.

Makefile targets and scripts are offered to work with go modules:

- `make verify-modules` checks whether go module files are out of date.
- `make modules` runs `go mod tidy` to ensure proper vendoring.
- `hack/ensure-go.sh` checks that the Go version and environment variables are properly set.

### Setting up the environment

Your environment must have the Azure credentials as outlined in the [getting
started prerequisites](./getting-started.md#Prerequisites) section.

### Using Tilt

Both of the [Tilt](https://tilt.dev) setups below will get you started developing CAPZ in a local kind cluster.
The main difference is the number of components you will build from source and the scope of the changes you'd like to make.
If you only want to make changes in CAPZ, then follow [CAPZ instructions](#tilt-for-dev-in-capz). This will
save you from having to build all of the images for CAPI, which can take a while. If the scope of your
development will span both CAPZ and CAPI, then follow the [CAPI and CAPZ instructions](#tilt-for-dev-in-both-capz-and-capi).

#### Tilt for dev in CAPZ

If you want to develop in CAPZ and get a local development cluster working quickly, this is the path for you.

From the root of the CAPZ repository and after configuring the environment variables, you can run the following to generate your `tilt-settings.json` file:

```shell
cat <<EOF > tilt-settings.json
{
  "kustomize_substitutions": {
      "AZURE_SUBSCRIPTION_ID_B64": "$(echo "${AZURE_SUBSCRIPTION_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_TENANT_ID_B64": "$(echo "${AZURE_TENANT_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_CLIENT_SECRET_B64": "$(echo "${AZURE_CLIENT_SECRET}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_CLIENT_ID_B64": "$(echo "${AZURE_CLIENT_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_ENVIRONMENT": "AzurePublicCloud"
  }
}
EOF
```

To build a kind cluster and start Tilt, just run:

```shell
make tilt-up
```
By default, the Cluster API components deployed by Tilt have experimental features turned off.
If you would like to enable these features, add `extra_args` as specified in [The Cluster API Book](https://cluster-api.sigs.k8s.io/developer/tilt.html#create-a-tilt-settingsjson-file).

Once your kind management cluster is up and running, you can [deploy a workload cluster](#deploying-a-workload-cluster).

You can also [deploy a flavor cluster as a local tilt resource](../templates/flavors/README.md#Running-flavor-clusters-as-a-tilt-resource).

To tear down the kind cluster built by the command above, just run:

```shell
make kind-reset
```

#### Tilt for dev in both CAPZ and CAPI

If you want to develop in both CAPI and CAPZ at the same time, then this is the path for you.

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow the [instructions](https://cluster-api.sigs.k8s.io/developer/tilt.html) in the cluster-api repo.  The instructions will walk you through cloning the Cluster API (CAPI) repository and configuring Tilt to use `kind` to deploy the cluster api management components.

> you may wish to checkout out the correct version of CAPI to match the [version used in CAPZ](go.mod)

Note that `tilt up` will be run from the `cluster-api repository` directory and the `tilt-settings.json` file will point back to the `cluster-api-provider-azure` repository directory.  Any changes you make to the source code in `cluster-api` or `cluster-api-provider-azure` repositories will automatically redeployed to the `kind` cluster.

After you have cloned both repositories, your folder structure should look like:

```tree
|-- src/cluster-api-provider-azure
|-- src/cluster-api (run `tilt up` here)
```

After configuring the environment variables, run the following to generate your `tilt-settings.json` file:

```shell
cat <<EOF > tilt-settings.json
{
  "default_registry": "${REGISTRY}",
  "provider_repos": ["../cluster-api-provider-azure"],
  "enable_providers": ["azure", "docker", "kubeadm-bootstrap", "kubeadm-control-plane"],
  "kustomize_substitutions": {
      "AZURE_SUBSCRIPTION_ID_B64": "$(echo "${AZURE_SUBSCRIPTION_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_TENANT_ID_B64": "$(echo "${AZURE_TENANT_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_CLIENT_SECRET_B64": "$(echo "${AZURE_CLIENT_SECRET}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_CLIENT_ID_B64": "$(echo "${AZURE_CLIENT_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "AZURE_ENVIRONMENT": "AzurePublicCloud"
  }
}
EOF
```

> `$REGISTRY` should be in the format `docker.io/<dockerhub-username>`

The cluster-api management components that are deployed are configured at the `/config` folder of each repository respectively. Making changes to those files will trigger a redeploy of the management cluster components.

#### Deploying a workload cluster

After your kind management cluster is up and running with Tilt, you can [configure workload cluster settings](#customizing-the-cluster-deployment) and deploy a workload cluster with the following:

```bash
make create-workload-cluster
```

To delete the cluster:

```bash
make delete-workload-cluster
```

> Check out the [troubleshooting guide](troubleshooting.md) for common errors you might run into.

#### Viewing Telemetry
The CAPZ controller emits tracing and metrics data. When run in Tilt, the KinD cluster is provisioned with a development
deployment of Jaeger, for distributed tracing, and Prometheus for metrics scraping and visualization.

The Jaeger and Prometheus deployments are for development purposes only. These illustrate the hooks for tracing and
metrics, but lack the robustness of production cluster deployments. For example, Jaeger in "all-in-one" mode with only
in-memory persistence of traces.

After the Tilt cluster has been initialized, to view distributed traces in Jaeger open a browser to
`http://localhost:8080`.

To view metrics, run `kubectl port-forward -n capz-system prometheus-prometheus-0 9090` and open
`http://localhost:9090` to see the Prometheus UI.

### Manual Testing

#### Creating a dev cluster

The steps below are provided in a convenient script in [hack/create-dev-cluster.sh](../hack/create-dev-cluster.sh). Be sure to set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, and `AZURE_TENANT_ID` before running. Optionally, you can override the different cluster configuration variables. For example, to override the workload cluster name:

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
   REGISTRY="<container-registry>" MANAGER_IMAGE_TAG="<image-tag>" make docker-push
   ```

   NOTE: `make create-cluster` will fetch the manager image locally and load it onto the kind cluster if it is present.

##### Customizing the cluster deployment

Here is a list of required configuration parameters (the full list is available in `templates/cluster-template.yaml`):

```bash
# Cluster settings.
export CLUSTER_NAME="capz-cluster"
export AZURE_VNET_NAME=${CLUSTER_NAME}-vnet

# Azure cloud settings
# To use the default public cloud, otherwise set to AzureChinaCloud|AzureGermanCloud|AzureUSGovernmentCloud
export AZURE_ENVIRONMENT="AzurePublicCloud"

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
export AZURE_NODE_MACHINE_TYPE="Standard_D2s_v3"
export WORKER_MACHINE_COUNT=2
export KUBERNETES_VERSION="v1.19.7"

# Generate SSH key.
# If you want to provide your own key, skip this step and set AZURE_SSH_PUBLIC_KEY_B64 to your existing file.
SSH_KEY_FILE=.sshkey
rm -f "${SSH_KEY_FILE}" 2>/dev/null
ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
echo "Machine SSH key generated in ${SSH_KEY_FILE}"
export AZURE_SSH_PUBLIC_KEY_B64=$(cat "${SSH_KEY_FILE}.pub" | base64 | tr -d '\r\n')
```

⚠️ Please note the generated templates include default values and therefore require the use of `clusterctl` to create the cluster
or the use of `envsubst` to replace these values

##### Creating the cluster

⚠️ Make sure you followed the previous two steps to build the dev image and set the required environment variables before proceding.

Ensure dev environment has been reset:

```bash
make clean kind-reset
```

Create the cluster:

```bash
make create-cluster
```

> Check out the [troubleshooting](troubleshooting.md) guide for common errors you might run into.

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
ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachineReconciler.Reconcile",
    trace.WithAttributes(
        label.String("namespace", req.Namespace),
        label.String("name", req.Name),
        label.String("kind", "AzureMachine"),
    ))
defer span.End()
```
The code above creates a context with a new span stored in the context.Context value bag. If a span already existed in
the `ctx` arguement, then the new span would take on the parentID of the existing span, otherwise the new span
becomes a "root span", one that does not have a parent. The span is also created with labels, or tags, which
provide metadata about the span and can be used to query in many distributed tracing systems.

You should consider adding tracing if your func accepts a context.

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

| Variable                   | Description                                                                                                               | Default                                                                               |
|----------------------------|---------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|
| `E2E_CONF_FILE`            | The path of the [E2E configuration file](https://cluster-api.sigs.k8s.io/developer/e2e.html#defining-an-e2e-config-file). | `${GOPATH}/src/sigs.k8s.io/cluster-api-provider-azure/test/e2e/config/azure-dev.yaml` |
| `SKIP_CLEANUP`             | Set to `true` if you do not want the bootstrap and workload clusters to be cleaned up after running E2E tests.            | `false`                                                                               |
| `SKIP_CREATE_MGMT_CLUSTER` | Skip management cluster creation. If skipping managment cluster creation you must specify `KUBECONFIG` and `SKIP_CLEANUP` | `false`                                                                               |
| `LOCAL_ONLY`               | Use Kind local registry and run the subset of tests which don't require a remotely pushed controller image.               | `true`                                                                                |
| `REGISTRY`                 | Registry to push the controller image.                                                                                    | `capzci.azurecr.io/ci-e2e`                                                            |
| `KUBECONFIG`               | Used with `SKIP_CREATE_MGMT_CLUSTER` set to true. Location of kubeconfig for the management cluster you would like to use. Use `kind get kubeconfig --name capz-e2e > kubeconfig.capz-e2e` to get the capz e2e kind cluster config | '~/.kube/config'                                                                                    |

You can also customize the configuration of the CAPZ cluster created by the E2E tests (except for `CLUSTER_NAME`, `AZURE_RESOURCE_GROUP`, `AZURE_VNET_NAME`, `CONTROL_PLANE_MACHINE_COUNT`, and `WORKER_MACHINE_COUNT`, since they are generated by individual test cases). See [Customizing the cluster deployment](#customizing-the-cluster-deployment) for more details.

#### Conformance Testing

To run Conformance locally, set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID` and run:

```bash
./scripts/ci-entrypoint.sh
```

You can optionally set the following variables:

| Variable                       | Description                                                                                                   |
|--------------------------------|---------------------------------------------------------------------------------------------------------------|
| `AZURE_SSH_PUBLIC_KEY_FILE`    | Use your own SSH key.                                                                                         |
| `SKIP_CREATE_WORKLOAD_CLUSTER` | Skip workload cluster creation.                                                                               |
| `SKIP_UPSTREAM_E2E_TESTS`      | Skip running upstream Kubernetes E2E tests.                                                                   |
| `SKIP_CLEANUP`                 | Skip deleting the cluster after the tests finish running.                                                      |
| `KUBECONFIG`                   | Provide your existing cluster kubeconfig filepath. If no kubeconfig is provided, `./kubeconfig` will be used.     |
| `SKIP`                         | Regexp for test cases to skip.                                                                                |
| `FOCUS`                        | Regexp for which test cases to run.                                                                           |
| `PARALLEL`                     | Skip serial tests and set --ginkgo-parallel.                                                                  |
| `USE_CI_ARTIFACTS`             | Use a CI version of Kubernetes, ie. not a released version (eg. `v1.19.0-alpha.1.426+0926c9c47677e9`)         |
| `CI_VERSION`                   | Provide a custom CI version of Kubernetes. By default, the latest master commit will be used.                 |
| `EXP_MACHINE_POOL`             | Use [Machine Pool](topics/machinepools.md) for worker machines.                                               |

You can also customize the configuration of the CAPZ cluster (assuming that `SKIP_CREATE_WORKLOAD_CLUSTER` is not set). See [Customizing the cluster deployment](#customizing-the-cluster-deployment) for more details.

In addition to upstream E2E, you can append custom commands to `./scripts/ci-entrypoint.sh` to run E2E from other projects against a CAPZ cluster:

```bash
export SKIP_UPSTREAM_E2E_TESTS="false"
./scripts/ci-entrypoint.sh bash -c "cd ${GOPATH}/src/github.com/my-org/my-project && make e2e"
```

<!-- References -->

[go]: https://golang.org/doc/install
[jq]: https://stedolan.github.io/jq/download/
[image_pull_secrets]: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
[gettext]: https://www.gnu.org/software/gettext/
[gettextwindows]: https://mlocati.github.io/articles/gettext-iconv-windows.html
[go.mod]: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/master/go.mod
[kind]: https://sigs.k8s.io/kind
[azure_cli]: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest
[manifests]: /docs/manifests.md
[kustomize]: https://github.com/kubernetes-sigs/kustomize
[kustomizelinux]: https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md
[gomock]: https://github.com/golang/mock
[timeout]: http://man7.org/linux/man-pages/man1/timeout.1.html
