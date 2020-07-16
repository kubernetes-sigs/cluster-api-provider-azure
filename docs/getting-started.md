# Getting started with cluster-api-provider-azure <!-- omit in toc -->

## Contents <!-- omit in toc -->

<!-- Below is generated using VSCode yzhang.markdown-all-in-one >

<!-- TOC depthFrom:2 -->

- [Prerequisites](#prerequisites)
  - [Requirements](#requirements)
  - [Optional](#optional)
  - [Setting up your Azure environment](#setting-up-your-azure-environment)
- [Troubleshooting](#troubleshooting)
  - [Bootstrap running, but resources aren't being created](#bootstrap-running-but-resources-arent-being-created)
  - [Resources are created but control plane is taking a long time to become ready](#resources-are-created-but-control-plane-is-taking-a-long-time-to-become-ready)
- [Building from master](#building-from-master)

<!-- /TOC -->

## Prerequisites

### Requirements

- Linux or macOS (Windows isn't supported at the moment)
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

  2. Save your Subscription ID in an environment variable.

  ```bash
  export AZURE_SUBSCRIPTION_ID="<SubscriptionId>"
  ```

  3. Create an Azure Service Principal by running the following command or skip this step and use a previously created Azure Service Principal.

  ```bash
  az ad sp create-for-rbac --name SPClusterAPI --role owner
  ```

  4. Save the output from the above command in environment variables.

  ```bash
  export AZURE_TENANT_ID="<Tenant>"
  export AZURE_CLIENT_ID="<AppId>"
  export AZURE_CLIENT_SECRET='<Password>'
  export AZURE_LOCATION="eastus" # this should be an Azure region that your subscription has quota for.
  ```

:warning: NOTE: If your password contains single quotes (`'`), make sure to escape them. To escape a single quote, close the quoting before it, insert the single quote, and re-open the quoting. 
For example, if your password is `foo'blah$`, you should do `export AZURE_CLIENT_SECRET='foo'\''blah$'`.

  5. Set the name of the AzureCloud to be used, the default value that would be used by most users is "AzurePublicCloud", other values are:

 - ChinaCloud: "AzureChinaCloud"
 - GermanCloud: "AzureGermanCloud"
 - PublicCloud: "AzurePublicCloud"
 - USGovernmentCloud: "AzureUSGovernmentCloud"

```bash
export AZURE_ENVIRONMENT="AzurePublicCloud"
```

<!--An alternative is to install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) and have the project's script create the service principal automatically. _Note that the service principals created by the scripts will not be deleted automatically._ -->

### Using images

By default, the code will use the Azure Marketplace "capi" offer. You can list the available images with:

```bash
az vm image list --publisher cncf-upstream --offer capi --all -o table
```

You can also [build your own image](https://image-builder.sigs.k8s.io/capi/providers/azure.html) and specify the image ID in the manifests generated in the AzureMachine specs.

## Troubleshooting

Please refer to the [troubleshooting guide][troubleshooting].

[troubleshooting]: /docs/troubleshooting.md

## Building from master

If you're interested in developing cluster-api-provider-azure and getting the latest version from `master`, please follow the [development guide][development].

[development]: /docs/development.md
