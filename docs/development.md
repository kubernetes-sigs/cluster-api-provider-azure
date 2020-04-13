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
  - [Manual Testing](#manual-testing)
    - [Creating a dev cluster](#creating-a-dev-cluster)
      - [Building and pushing dev images](#building-and-pushing-dev-images)
      - [Customizing the cluster deployment](#customizing-the-cluster-deployment)
      - [Creating the cluster](#creating-the-cluster)
    - [Debugging cluster creation](#debugging-cluster-creation)
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
   - `brew install jq` on MacOS.
   - `chocolatey install jq` on Windows.
   - `sudo apt-get install jq` on Linux.
3. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on MacOS.
   - [install instructions](gettextwindows) on Windows.
4. Install [KIND][kind]
   - `GO111MODULE="on" go get sigs.k8s.io/kind@v0.7.0`.
5. Install [Kustomize][kustomize]
   - `brew install kustomize` on MacOs.
   - `choco install kustomize` on Windows.
   - [install instructions][kustomizelinux] on Linux
6. Configure Python 2.7+ with [pyenv][pyenv] if your default is Python 3.x.
7. Install make.

[go]: https://golang.org/doc/install

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

A few Makefile and scripts are offered to work with go modules:

- `hack/ensure-go.sh` file checks that the Go version and environment variables are properly set.

### Setting up the environment

Your environment must have the Azure credentials as outlined in the [getting
started prerequisites section](./getting-started.md#Prerequisites)

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
      "AZURE_CLIENT_ID_B64": "$(echo "${AZURE_CLIENT_ID}" | tr -d '\n' | base64 | tr -d '\n')"
  }
}
EOF
```

To build a kind cluster and start Tilt, just run:

```shell
make tilt-up
```

Once your kind management cluster is up and running, you can [deploy a workload cluster](#deploying-a-workload-cluster).

To tear down the kind cluster built by the command above, just run:

```shell
make kind-reset
```

#### Tilt for dev in both CAPZ and CAPI

If you want to develop in both CAPI and CAPZ at the same time, then this is the path for you.

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow the [instructions](https://cluster-api.sigs.k8s.io/developer/tilt.html) in the cluster-api repo.  The instructions will walk you through cloning the Cluster API (capi) repository and configuring Tilt to use `kind` to deploy the cluster api management components.

> you may wish to checkout out the correct version of capi to match the [version used in capz](go.mod)

Note that `tilt up` will be run from the `cluster-api repository` directory and the `tilt-settings.json` file will point back to the `cluster-api-provider-azure` repository directory.  Any changes you make to the source code in `cluster-api` or `cluster-api-provider-azure` repositories will automatically redeployed to the `kind` cluster.

After you have cloned both repositories your folder structure should look like:

```tree
|-- src/cluster-api-provider-azure
|-- src/cluster-api (run `tilt up` here)
```

After configuring the environment variables, you can run the following to generate you `tilt-settings.json` file:

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
      "AZURE_CLIENT_ID_B64": "$(echo "${AZURE_CLIENT_ID}" | tr -d '\n' | base64 | tr -d '\n')"
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

### Manual Testing

#### Creating a dev cluster

The steps below are provided in a convenient script in [hack/create-dev-cluster.sh](../hack/create-dev-cluster.sh). Be sure to set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, and `AZURE_TENANT_ID` before running. Optionally, you can override the different cluster configuration variables. For example, to override the workload cluster name:

```bash
CLUSTER_NAME=<my-capz-cluster-name> ./hack/create-dev-cluster.sh
```

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
export KUBERNETES_VERSION="v1.17.4"

# Generate SSH key.
# If you want to provide your own key, skip this step and set AZURE_SSH_PUBLIC_KEY to your existing file.
SSH_KEY_FILE=.sshkey
rm -f "${SSH_KEY_FILE}" 2>/dev/null
ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
echo "Machine SSH key generated in ${SSH_KEY_FILE}"
export AZURE_SSH_PUBLIC_KEY=$(cat "${SSH_KEY_FILE}.pub" | base64 | tr -d '\r\n')
```

##### Creating the cluster

⚠️ Make sure you followed the previous two steps to build the dev image and set the required environment variables before proceding.

Ensure dev environment has been reset:

```bash
make clean
make kind-reset
```

Create the cluster:

```bash
make create-cluster
```

#### Debugging cluster creation

While cluster build out is running, you can optionally follow the controller logs in a separate window as follows:

```bash
time kubectl get po -o wide --all-namespaces -w # Watch pod creation until azure-provider-controller-manager-0 is available

kubectl logs -n capz-system azure-provider-controller-manager-0 manager -f # Follow the controller logs
```

An error such as the following in the manager could point to a miss match between a current capi with old capz version:

```
E0320 23:33:33.288073       1 controller.go:258] controller-runtime/controller "msg"="Reconciler error" "error"="failed to create AzureMachine VM: failed to create nic capz-cluster-control-plane-7z8ng-nic for machine capz-cluster-control-plane-7z8ng: unable to determine NAT rule for control plane network interface: strconv.Atoi: parsing \"capz-cluster-control-plane-7z8ng\": invalid syntax"  "controller"="azuremachine" "request"={"Namespace":"default","Name":"capz-cluster-control-plane-7z8ng"}
```

After the workload cluster is finished deploying you will have a kubeconfig in `./kubeconfig`.
You can debug most issues by SSHing into the VMs that have been created and
reading `/var/lib/waagent/custom-script/download/0/stdout`.

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

Mocks for the services tests are generated using [GoMock][gomock]

To generate the mocks you can run

```bash
make generate-go
```

#### E2E Testing

To run E2E locally, set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID` and run:

```bash
./scripts/ci-e2e.sh
```

You can optionally set `AZURE_SSH_PUBLIC_KEY_FILE` to use your own SSH key.

#### Conformance Testing

To run Conformance locally, set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID` and run:

```bash
./scripts/ci-conformance.sh
```

You can optionally set the following variables:

| Variable                    | Description                                                                                                    |
| --------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `AZURE_SSH_PUBLIC_KEY_FILE` | Use your own SSH key.                                                                                          |
| `SKIP_CREATE_CLUSTER`       | Skip cluster creation.                                                                                         |
| `SKIP_TESTS`                | Skip running Kubernetes E2E tests.                                                                             |
| `SKIP_CLEANUP`              | Skip deleting the cluster after the tests finish running.                                                      |
| `KUBECONFIG`                | Provide your existing cluster kubeconfig filepath. If no kubeconfig is provided, `./kubeconfig` will be used.  |
| `SKIP`                      | Regexp for test cases to skip.                                                                                 |
| `FOCUS`                     | Regexp for which test cases to run.                                                                            |
| `PARALLEL`                  | Skip serial tests and set --ginkgo-parallel.                                                                  |
| `USE_CI_ARTIFACTS`          | Use a CI version of Kubernetes, ie. not a released version (eg. `v1.19.0-alpha.1.426+0926c9c47677e9`)          |
| `CI_VERSION`                | Provide a custom CI version of Kubernetes. By default, the latest master commit will be used.                  |

You can also customize the configuration of the CAPZ cluster (assuming that `SKIP_CREATE_CLUSTER` is not set). See [Customizing the cluster deployment](#customizing-the-cluster-deployment) for more details.

<!-- References -->

[jq]: https://stedolan.github.io/jq/download/
[image_pull_secrets]: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
[gettext]: https://www.gnu.org/software/gettext/
[gettextwindows]: https://mlocati.github.io/articles/gettext-iconv-windows.html
[go.mod]: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/master/go.mod
[kind]: https://sigs.k8s.io/kind
[azure_cli]: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest
[manifests]: /docs/manifests.md
[pyenv]: https://github.com/pyenv/pyenv
[kustomize]: https://github.com/kubernetes-sigs/kustomize
[kustomizelinux]: https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md
[gomock]: https://github.com/golang/mock
