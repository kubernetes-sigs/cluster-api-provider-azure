# Getting started with cluster-api-provider-azure <!-- omit in toc -->

## Contents <!-- omit in toc -->

<!-- Below is generated using VSCode yzhang.markdown-all-in-one >

<!-- TOC depthFrom:2 -->

- [Prerequisites](#prerequisites)
  - [Requirements](#requirements)
  - [Optional](#optional)
  - [Setting up the environment](#setting-up-the-environment)
- [Troubleshooting](#troubleshooting)
  - [Bootstrap running, but resources aren't being created](#bootstrap-running-but-resources-arent-being-created)
  - [Resources are created but control plane is taking a long time to become ready](#resources-are-created-but-control-plane-is-taking-a-long-time-to-become-ready)
- [Building from master](#building-from-master)

<!-- /TOC -->

## Prerequisites

### Requirements

- Linux or MacOS (Windows isn't supported at the moment)
- A [Microsoft Azure account](https://azure.microsoft.com/en-us/)
- Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest)
- Install the [Kubernetes CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl/) 
- [KIND]
- [kustomize]
- make
- gettext (with `envsubst` in your PATH)
- md5sum
- bazel

### Optional

- [Homebrew][brew] (MacOS)
- [jq]
- [Go]

[brew]: https://brew.sh/
[go]: https://golang.org/dl/
[jq]: https://stedolan.github.io/jq/download/
[kind]: https://sigs.k8s.io/kind
[kustomize]: https://github.com/kubernetes-sigs/kustomize

### Setting up your Azure environment

An Azure Service Principal is needed for populating the controller manifests. This utilizes [environment-based authentication](https://docs.microsoft.com/en-us/go/azure/azure-sdk-go-authorization#use-environment-based-authentication).
  
  1. List your Azure subscriptions. 

   ```bash
  az account list -o table
   ```

  2. Save your Subscription ID in an enviroment variable.

  ```bash
  export AZURE_SUBSCRIPTION_ID="<SubscriptionId>"
  ```
   
  3. Create an Azure Service Principal by running the following command or skip this step and use a previously created Azure Service Principal. 

  ```bash
  az ad sp create-for-rbac --name SPClusterAPI
  ```

  4. Save the output from the above command in enviroment variables. 

  ```bash  
  export AZURE_TENANT_ID=<Tenant>
  export AZURE_CLIENT_ID=<AppId>
  export AZURE_CLIENT_SECRET=<Password>
  export AZURE_LOCATION="eastus"
  ```
<!--An alternative is to install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) and have the project's script create the service principal automatically. _Note that the service principals created by the scripts will not be deleted automatically._ -->

### Using images

By default, the code will use the Azure Marketplace "capi" offer.

You can also [build your own image](https://github.com/kubernetes-sigs/image-builder/tree/master/images/capi/packer/azure) and specify the image ID in the manifests generated in the AzureMachine specs.

## Troubleshooting

### Bootstrap running, but resources aren't being created

Logs can be tailed using [`kubectl`][kubectl]:

```bash
kubectl logs azure-provider-controller-manager-0 -n azure-provider-system -f
```

### Resources are created but control plane is taking a long time to become ready

You can check the custom script logs by SSHing into the VM created and reading `/var/lib/waagent/custom-script/download/0/{stdout,stderr}`.

[development]: /docs/development.md

## Building from master

If you're interested in developing cluster-api-provider-azure and getting the latest version from `master`, please follow the [development guide][development].
