# k8s Cluster API Azure Provider [![Build Status](https://travis-ci.org/platform9/azure-provider.svg?branch=master)](https://travis-ci.org/platform9/azure-provider) [![Go Report Card](https://goreportcard.com/badge/github.com/platform9/azure-provider)](https://goreportcard.com/report/github.com/platform9/azure-provider)

## Usage
1. Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest)
2. Log into the Azure CLI, `az login`
3. (Optional) Modify the templates found in `configtemplates/` 
4. Run `generate-yaml.sh`   _Note: `generate-yaml.sh` creates an Azure service principal which will not be deleted automatically._
5. Obtain `clusterctl` from the [cluster-api repository](https://github.com/kubernetes-sigs/cluster-api). You can either:
    * Build from source by cloning the repo and running `go build` while in the `cluster-api/clusterctl` directory.
    * Use one of the pre-built binaries found in the releases of the repository.
6. Use the configs generated in `generatedconfigs/` with `clusterctl`
    * Example: `./clusterctl --provider azure -m generatedconfigs/machines.yaml -c generatedconfigs/cluster.yaml -p generatedconfigs/provider-components.yaml -a generatedconfigs/addons.yaml`

## Notes
cluster-api should be vendored when testing, either in Travis or locally, but should not be versioned in git. This allows the cluster-api to import `azure-provider` while avoiding a circular dependency. To vendor the cluster-api for testing purposes, un-ignore it in `Gopkg.toml` and run `dep ensure -add sigs.k8s.io/cluster-api/pkg -vendor-only`. After adding it, `Gopkg.lock` will reference it. Prior to comitting, this should be manually removed.