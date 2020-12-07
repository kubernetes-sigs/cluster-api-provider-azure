# Debugging scripts
Some helpful scripts for debugging various setups


## Kubectl plugins
Any scripts that start with kubectl can be used as [kubectl plugins](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/).  They can also be used directly as with any other script.

To use as plugins, copy the files to a folder in your path such as:

```bash
cp hack/debugging/kubectl-* /usr/local/bin
```

To use as a script:

```bash
./hack/debugging/kubectl-capz-ssh
```

### capz-ssh
Quickly ssh to a node to debug VM join issues.


To connect to an Azure Machine:

```bash
# find the azure cluster and azure machine you wish to ssh too
$ kubectl get azuremachine
NAME                                 READY   STATE
capz-cluster-0-control-plane-5b5fc   true    Succeeded
capz-cluster-0-control-plane-vg8pl   true    Succeeded
capz-cluster-0-control-plane-z5pst   true    Succeeded
capz-cluster-0-md-0-fljwt            true    Succeeded
capz-cluster-0-md-0-wbx2r            true    Succeeded


$ kubectl capz ssh -am capz-cluster-3-control-plane-rcmkh
```

To connect to an instance of Azure Machine Pool, you can do the following:

- If you don't know the instance name you can run the list command to get all
instances of a Machine Pool

```bash
$ ./hack/debugging/kubectl-capz-ssh  --list-azure-machine-pool  --azure-machine-pool machinepool-template-mp-0
Utility tool to ssh'ing into CAPZ nodes

Listing Azure Machine Pool machinepool-template-mp-0
ID      ComputerName
========================================
2       machinepool-template-mp-0000002
3       machinepool-template-mp-0000003
```

- Then you can connect to a node using the `ID`

```bash
$ ./hack/debugging/kubectl-capz-ssh  --azure-machine-pool machinepool-template-mp-0 --azure-machine-pool-id 2
```

### capz-map
There are many different CRDs required to deploy a machine (such as azmachine, capimachine, and kubeadm bootstrap).  View how all the configurations are mapped together:

```bash
$ kubectl capz map
AzureCluster: capz-cluster-0
AzureMachine: capz-cluster-0-control-plane-5b5fc
Machine: capz-cluster-0-control-plane-xhbjh
Kubeadmconfig: capz-cluster-0-control-plane-g8gql
```
