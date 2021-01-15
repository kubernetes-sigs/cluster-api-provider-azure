# Flavors

In `clusterctl` the infrastructure provider authors can provide different type of cluster templates,
or flavors; use the --flavor flag to specify which flavor to use; e.g
```shell
clusterctl config cluster my-cluster --kubernetes-version v1.19.7 \
    --flavor external-cloud-provider > my-cluster.yaml
```
See [`clusterctl` flavors docs](https://cluster-api.sigs.k8s.io/clusterctl/commands/config-cluster.html#flavors).

This directory contains each of the flavors for CAPZ. Each directory besides `base` will be used to
create a flavor by running `kustomize build` on the directory. The name of the directory will be
appended to the end of the cluster-template.yaml, e.g cluster-template-{directory-name}.yaml. That
flavor can be used by specifying `--flavor {directory-name}`.

To generate all CAPZ flavors, run `make generate-flavors`.


## Running flavor clusters as a tilt resource

#### From CLI
run ```tilt up ${flavors}``` to spin up worker clusters in Azure represented by a Tilt local resource.  You can also modify flavors while Tilt is up by running ```tilt args ${flavors}```

#### From Tilt Config
Tilt will auto-detect all available flavors from the `templates` directory.

#### Requirements
Please note your tilt-settings.json must contain at minimum the following fields when using tilt resources to deploy cluster flavors:
```json
{
    "kustomize_substitutions": {
        "AZURE_SUBSCRIPTION_ID_B64": "******",
        "AZURE_TENANT_ID_B64": "******",
        "AZURE_CLIENT_SECRET_B64": "******",
        "AZURE_CLIENT_ID_B64": "******"
    }
}
```

#### Defining Variable Overrides
If you wish to override the default variables for flavor workers, you can specify them as part of your tilt-settings.json as seen in the example below.  Please note, the precedence of variables is as follows:

1. explicitly defined vars for each flavor i.e. worker-templates.flavors[0].AZURE_VNET_NAME
2. vars defined at 'metadata' level-- spans workers i.e. metadata.AZURE_VNET_NAME
3. programmatically defined default vars i.e. everything except azure tenant, client, subscription


```json
{
    "kustomize_substitutions": {
        "AZURE_SUBSCRIPTION_ID_B64": "****",
        "AZURE_TENANT_ID_B64": "****",
        "AZURE_CLIENT_SECRET_B64": "****",
        "AZURE_CLIENT_ID_B64": "****",
        "AZURE_ENVIRONMENT": "AzureChinaCloud"
    },
    "worker-templates": {
        "flavors": {
            "default": {
                "CLUSTER_NAME": "example-default-cluster-name",
                "AZURE_VNET_NAME": "example-vnet-one"
            },
            "system-assigned-identity": {
                "CLUSTER_NAME": "example-SAI-cluster-name",
                "AZURE_LOCATION": "westus",
                "AZURE_VNET_NAME": "example-vnet-two"
            }
        },
        "metadata": {
            "AZURE_LOCATION": "eastus",
            "AZURE_RESOURCE_GROUP": "test-resource-group-name",
            "CONTROL_PLANE_MACHINE_COUNT": "1",
            "KUBERNETES_VERSION": "v1.19.7",
            "AZURE_CONTROL_PLANE_MACHINE_TYPE": "Standard_D2s_v3",
            "WORKER_MACHINE_COUNT": "2",
            "AZURE_NODE_MACHINE_TYPE": "Standard_D2s_v3"
        }
    }
}
```

Here is a practical example: creating a GPU-enabled cluster requires N-series nodes. You can set an
N-series node type just for the `nvidia-gpu` flavor in `tilt-settings.json` to override any default:

```json
{
    "kustomize_substitutions": {
        "AZURE_SUBSCRIPTION_ID_B64": "****",
        "AZURE_TENANT_ID_B64": "****",
        "AZURE_CLIENT_SECRET_B64": "****",
        "AZURE_CLIENT_ID_B64": "****",
        "AZURE_ENVIRONMENT": "AzurePublicCloud"
    },
    "worker-templates": {
        "flavors": {
            "nvidia-gpu": {
                "AZURE_NODE_MACHINE_TYPE": "Standard_NC6s_v3"
            }
        },
        "metadata": {
            "AZURE_CONTROL_PLANE_MACHINE_TYPE": "Standard_D2s_v3",
            "AZURE_LOCATION": "southcentralus",
            "KUBERNETES_VERSION": "v1.19.7",
            "WORKER_MACHINE_COUNT": "1"
        }
    }
}
```
