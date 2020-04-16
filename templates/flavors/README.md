# Flavors

In `clusterctl` the infrastructure provider authors can provide different type of cluster templates, 
or flavors; use the --flavor flag to specify which flavor to use; e.g
```shell
clusterctl config cluster my-cluster --kubernetes-version v1.18.2 \
    --flavor external-cloud-provider > my-cluster.yaml
```
See [`clusterctl` flavors docs](https://cluster-api.sigs.k8s.io/clusterctl/commands/config-cluster.html#flavors).

This directory contains each of the flavors for CAPZ. Each directory besides `base` will be used to
create a flavor by running `kustomize build` on the directory. The name of the directory will be
appended to the end of the cluster-template.yaml, e.g cluster-template-{directory-name}.yaml. That
flavor can be used by specifying `--flavor {directory-name}`.

To generate all CAPZ flavors, run `make generate-flavors`.
