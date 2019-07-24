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
  - [Manual Testing](#manual-testing)
    - [Setting up the environment](#setting-up-the-environment)
    - [Building and pushing dev images](#building-and-pushing-dev-images)
    - [Build manifests](#build-manifests)
    - [Creating a test cluster](#creating-a-test-cluster)
  - [Submitting PRs and testing](#submitting-prs-and-testing)
    - [Executing unit tests](#executing-unit-tests)
  - [Automated Testing](#automated-testing)
    - [Mocks](#mocks)

<!-- /TOC -->

## Setting up

### Base requirements

1. Install [go][go]
   - Get the latest patch version for go v1.12.
2. Install [jq][jq]
   - `brew install jq` on MacOS.
3. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on MacOS.
4. Install [KIND][kind]
   - `GO111MODULE="on" go get sigs.k8s.io/kind@v0.3.0`.
5. Install [bazel][bazel]
6. Configure Python 2.7+ with [pyenv][pyenv] if your default is Python 3.x.
7. Install make.

[go]: https://golang.org/doc/install

### Get the source

`go get -d sigs.k8s.io/cluster-api-provider-azure`

Ensure you have updated the vendor directory with:

``` shell
cd "$(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-azure"
make vendor
```

### Get familiar with basic concepts

This provider is modeled after the upstream Cluster API project. To get familiar
with Cluster API resources, concepts and conventions, refer to the [Cluster API gitbook](https://kubernetes-sigs.github.io/cluster-api/).

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
- Run `make vendor`.

A few Makefile and scripts are offered to work with go modules:
- `make vendor` runs `hack/update-vendor.sh`, which uses a combination of `go mod` commands to store this project's dependencies under `vendor/`.
   - This step uses the `go.vendor` file to store a full copy of some imports/repositories. This is necessary given that go modules prunes non-Go files by default.
- `hack/ensure-go.sh` file checks that the Go version and environment variables are properly set.

### Manual Testing

#### Setting up the environment

Your environment must have the Azure credentials as outlined in the [getting
started prerequisites section](./getting-started.md#Prerequisites)

#### Building and pushing dev images

1. Login to your container registry using `docker login`.

    e.g., `docker login quay.io`
2. To build images with custom tags and push to your custom image registry,
   run the `make docker-build` as follows:

    ```bash
    REGISTRY="<container-registry>" MANAGER_IMAGE_TAG="<image-tag>" make docker-build
    ```

3. Push your docker images:
    ```bash
    REGISTRY="<container-registry>" MANAGER_IMAGE_TAG="<image-tag>" make docker-push
    ```

#### Build manifests

**NOTE:** It's expected that some set of Azure credentials are available at the time, either
as environment variable, or some other SDK-supported method.

Whenever you are working on a branch, you will need to generate manifests
using:

```bash
make clean # Clean up any previous manifests

REGISTRY="<container-registry>" MANAGER_IMAGE_TAG="<image-tag>" RESOURCE_GROUP="<resource-group>" CLUSTER_NAME="<cluster-name>" make manifests
```

You will then have a sample cluster and machine manifest in:
`/cmd/clusterctl/examples/azure/out` and a provider components file to use with clusterctl in
`cmd/clusterctl/examples/azure/out/provider-components.yaml`

#### Creating a test cluster

Generate custom binaries:
```bash
make binaries
```

Ensure kind has been reset:
```bash
make kind-reset
```

**Before continuing, please review the [documentation on manifests][manifests] to understand which manifests to use for various cluster scenarios.**

Launch a bootstrap cluster and then run the generated manifests creating a target cluster in Azure:
- Use `make create-cluster` to create a single control plane, single node cluster
- Use `make create-cluster-ha` to create a multi-node control plane, with no nodes

If you're interested in creating the cluster with verbose logging or further customize the manifests, you can instead run:

```bash
time clusterctl create cluster -v 10 \
--provider azure \
--bootstrap-type kind \
-m ./cmd/clusterctl/examples/azure/out/<machine-manifest> \
-c ./cmd/clusterctl/examples/azure/out/<cluster-manifest> \
-p ./cmd/clusterctl/examples/azure/out/provider-components.yaml \
-a ./cmd/clusterctl/examples/azure/out/addons.yaml
```

While clusterctl is running, you can optionally follow the controller logs in a separate window as follows:
```bash
export KUBECONFIG="$(kind get kubeconfig-path --name="clusterapi")" # Export the kind kubeconfig

time kubectl get po -o wide --all-namespaces -w # Watch pod creation until azure-provider-controller-manager-0 is available

kubectl logs azure-provider-controller-manager-0 -n azure-provider-system -f # Follow the controller logs
```

After this is finished you will have
a kubeconfig copied locally. You can debug most issues by SSHing into the
instances that have been created and reading `/var/lib/waagent/custom-script/download/0/stdout`.

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

Mocks are set up using Bazel, see [build](../../build)

If you then want to use these mocks with `go test ./...`, run

`make copy-genmocks`

<!-- References -->

[jq]: https://stedolan.github.io/jq/download/
[image_pull_secrets]: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
[gettext]: https://www.gnu.org/software/gettext/
[kind]: https://sigs.k8s.io/kind
[azure_cli]: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest
[bazel]: https://docs.bazel.build/versions/master/install.html
[manifests]: /docs/manifests.md
[pyenv]: https://github.com/pyenv/pyenv
