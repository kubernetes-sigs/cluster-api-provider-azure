# Developing Cluster API Provider Azure <!-- omit in toc -->

## Contents <!-- omit in toc -->

<!-- Below is generated using VSCode yzhang.markdown-all-in-one >

<!-- TOC depthFrom:2 -->
- [Setting up](#setting-up)
  - [Base requirements](#base-requirements)
  - [Get the source](#get-the-source)
  - [Dev manifest files](#dev-manifest-files)
  - [Dev images](#dev-images)
    - [Container registry](#container-registry)
- [Developing](#developing)
  - [Manual Testing](#manual-testing)
    - [Setting up the environment](#setting-up-the-environment)
    - [Dev manifests](#dev-manifests)
    - [Running clusterctl](#running-clusterctl)
  - [Automated Testing](#automated-testing)
    - [Mocks](#mocks)

<!-- /TOC -->

## Setting up

### Base requirements

1. Install [go][go]
   - Get the latest patch version for go v1.11.
2. Install [jq][jq]
   - `brew install jq` on MacOS.
3. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on MacOS.
4. Install [KIND][kind]
   - `go get sigs.k8s.io/kind`.
5. Install [bazel][bazel]

[go]: https://golang.org/doc/install

### Get the source

`go get -d sigs.k8s.io/cluster-api-provider-azure`

Ensure you have updated the vendor directory with:

``` shell
cd "$(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-azure"
make dep-ensure
```

### Dev manifest files

Part of running cluster-api-provider-azure is generating manifests to run.
Generating dev manifests allows you to test dev images instead of the default
releases.

### Dev images

#### Container registry

Any public container registry can be leveraged for storing cluster-api-provider-azure container images.

## Developing

Change some code!

### Manual Testing

#### Setting up the environment

Your environment must have the Azure credentials as outlined in the [getting
started prerequisites section](./getting-started.md#Prerequisites)

#### Dev manifests

The dev version of the manifests can be generated with

`make manifests-dev examples-dev`

#### Running clusterctl

`make create-cluster` will launch a bootstrap cluster and then run the generated
manifests creating a target cluster in Azure. After this is finished you will have
a kubeconfig copied locally. You can debug most issues by SSHing into the
instances that have been created and reading `/var/log/startup.log`.

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
