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
    - [Customizing the cluster deployment](#customizing-the-cluster-deployment)
    - [Creating a test cluster](#creating-a-test-cluster)
  - [Submitting PRs and testing](#submitting-prs-and-testing)
    - [Executing unit tests](#executing-unit-tests)
  - [Automated Testing](#automated-testing)
    - [Mocks](#mocks)

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
   - [install instructions](kustomizelinux) on Linux
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
with Cluster API resources, concepts and conventions, refer to the [Cluster API Book](https://cluster-api.sigs.k8s.io/).

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

### Using Tilt

To use Tilt for a simplified development workflow, follow the [instructions](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/src/developer/tilt.md) in the cluster-api repo.

Add the output of the following as a section in your `tilt-settings.json`:

```shell
cat <<EOF
"kustomize_substitutions": {
   "AZURE_SUBSCRIPTION_ID_B64": "$(echo "${AZURE_SUBSCRIPTION_ID}" | tr -d '\n' | base64 | tr -d '\n')",
   "AZURE_TENANT_ID_B64": "$(echo "${AZURE_TENANT_ID}" | tr -d '\n' | base64 | tr -d '\n')",
   "AZURE_CLIENT_SECRET_B64": "$(echo "${AZURE_CLIENT_SECRET}" | tr -d '\n' | base64 | tr -d '\n')",
   "AZURE_CLIENT_ID_B64": "$(echo "${AZURE_CLIENT_ID}" | tr -d '\n' | base64 | tr -d '\n')"
  }
EOF
```

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

#### Customizing the cluster deployment

Here is a list of commonly overriden configuration parameters (the full list is available in `templates/cluster-template.yaml`):

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
export KUBERNETES_VERSION="1.16.7"

# Generate SSH key.
# If you want to provide your own key, skip this step and set AZURE_SSH_PUBLIC_KEY to your existing file.
SSH_KEY_FILE=.sshkey
rm -f "${SSH_KEY_FILE}" 2>/dev/null
ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
echo "Machine SSH key generated in ${SSH_KEY_FILE}"
export AZURE_SSH_PUBLIC_KEY=$(cat "${SSH_KEY_FILE}.pub" | base64 | tr -d '\r\n')
```

#### Creating a test cluster

Ensure kind has been reset:

```bash
make kind-reset
```

Create the cluster:

```bash
make create-cluster
```

While cluster build out is running, you can optionally follow the controller logs in a separate window as follows:

```bash
time kubectl get po -o wide --all-namespaces -w # Watch pod creation until azure-provider-controller-manager-0 is available

kubectl logs azure-provider-controller-manager-0 -n azure-provider-system -f # Follow the controller logs
```

After this is finished you will have a kubeconfig in `./kubeconfig`.
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

<!-- References -->

[jq]: https://stedolan.github.io/jq/download/
[image_pull_secrets]: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
[gettext]: https://www.gnu.org/software/gettext/
[gettextwindows]: https://mlocati.github.io/articles/gettext-iconv-windows.html
[kind]: https://sigs.k8s.io/kind
[azure_cli]: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest
[manifests]: /docs/manifests.md
[pyenv]: https://github.com/pyenv/pyenv
[kustomize]: https://github.com/kubernetes-sigs/kustomize
[kustomizelinux]: https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md
[gomock]: https://github.com/golang/mock
