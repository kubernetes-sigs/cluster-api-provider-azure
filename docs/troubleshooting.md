# Troubleshooting Guide

Common issue users might have when using Cluster API Provider for Azure.

## Debugging cluster creation
You will need to review the logs for the components of the control plane nodes and for the workload clusters.  Start your investigation with reviewing the logs of the control plane then move onto the workload cluster that is created.

## Review logs of control plane
While cluster buildout is running, you can follow the controller logs in a separate window like this:

```bash
kubectl get po -o wide --all-namespaces -w # Watch pod creation until azure-provider-controller-manager-0 is available

kubectl logs -n capz-system azure-provider-controller-manager-0 manager -f # Follow the controller logs
```

An error such as the following in the manager could point to a mismatch between a current CAPI and an old CAPZ version:

```
E0320 23:33:33.288073       1 controller.go:258] controller-runtime/controller "msg"="Reconciler error" "error"="failed to create AzureMachine VM: failed to create nic capz-cluster-control-plane-7z8ng-nic for machine capz-cluster-control-plane-7z8ng: unable to determine NAT rule for control plane network interface: strconv.Atoi: parsing \"capz-cluster-control-plane-7z8ng\": invalid syntax"  "controller"="azuremachine" "request"={"Namespace":"default","Name":"capz-cluster-control-plane-7z8ng"}
```

### Remoting to workload clusters
After the workload cluster is finished deploying you will have a kubeconfig in `./kubeconfig`.

Using the ssh information provided during cluster creation (environment variable `AZURE_SSH_PUBLIC_KEY`), you can debug most issues by SSHing into the VMs that have been created:

```
# connect to first control node - capi is default linux user created by deployment
API_SERVER=$(kubectl get azurecluster capz-cluster -o jsonpath='{.status.network.apiServerIp.dnsName}')
ssh capi@${API_SERVER}

# list nodes
kubectl get azuremachines
NAME                               READY   STATE
capz-cluster-control-plane-2jprg   true    Succeeded
capz-cluster-control-plane-ck5wv   true    Succeeded
capz-cluster-control-plane-w4tv6   true    Succeeded
capz-cluster-md-0-s52wb            true    Succeeded
capz-cluster-md-0-w8xxw            true    Succeeded

# pick node name from output above:
node=$(kubectl get azuremachine capz-cluster-md-0-s52wb -o jsonpath='{.status.addresses[0].address}') 
ssh -J capi@${apiserver} capi@${node} 
```

> There are some [provided scripts](/hack/debugging/Readme.md) that can help automate a few common tasks.

Reviewing the following logs on the workload cluster can help with troubleshooting:

- `less /var/lib/waagent/custom-script/download/0/stdout`
- `journalctl -u cloud-final`
- `less /var/log/cloud-init-output.log`
- `journalctl -u kubelet`

## Automated log collection

As part of [CI](../scripts/ci-e2e.sh) there is a [log collection script](hack/../../hack/log/log-dump.sh) which you can also leverage to pull all the logs for machines which will dump logs to `${PWD}/_artifacts}` by default:

```bash
./hack/log/log-dump.sh
```
## Examples of troubleshooting real-world issues

### Nodes did not come online

If as a result of a new cluster create operation, or a new machine or machinepool resource to an existing cluster, one or more nodes did not join the cluster, you can use some of the above guidance to SSH into the VM(s) and debug what happened. First, let's find out which VMs were created but failed to join the cluster by introspecting all VMs in the cluster resource group, and comparing them to the nodes present in the cluster:

```
$ export CLUSTER_RESOURCE_GROUP=my-cluster-rg
$ export VM_PREFIX=my-cluster-md-0-
$ export KUBECONFIG=/Users/me/.kube/my-cluster.kubeconfig
$ $ for vm in $(az vm list -g $CLUSTER_RESOURCE_GROUP |  jq -r --arg VM_PREFIX "${VM_PREFIX}" '.[] | select (.name | startswith($VM_PREFIX)).name'); do kubectl get node $vm 2>&1 >/dev/null && continue || echo node $vm did not join the cluster; done
Error from server (NotFound): nodes "my-cluster-md-0-8qlrg" not found
node my-cluster-md-0-8qlrg did not join the cluster
```

(The above uses the `az` command line tool to talk to Azure, and the `jq` utility to parse JSON output. Use your preferred toolchain following the general pattern.)

So, above we discover that the VM `my-cluster-md-0-8qlrg` is present in the resource group, but not as a node in the cluster. Let's hop on to the VM and look around.

We'll assume we have SSH access onto the control plane VM behind the apiserver as described above. Add the SSH private key to your local ssh client keychain so that you can log into any node from the control plane VM:

```
$ ssh-add -D
$ ssh-add ~/.ssh/my_private_key_rsa
$ ssh -A -i ~/.ssh/id_rsa capi@$(kubectl get azurecluster my-cluster -o jsonpath='{.status.network.apiServerIp.dnsName}')
capi@my-cluster-control-plane-68xfs:~$ ssh my-cluster-md-0-8qlrg
capi@my-cluster-md-0-8qlrg:~$
```

Now we're on the VM that didn't join the cluster. Let's look at the bootstrap logs on the cluster for error data.

```
capi@my-cluster-md-0-8qlrg:~$ less /var/lib/waagent/custom-script/download/0/stdout
<inspect VM bootstrap script data>
capi@my-cluster-md-0-8qlrg:~$ journalctl -u cloud-final
<inspect cloud-final systemd logs>
capi@my-cluster-md-0-8qlrg:~$ less /var/log/cloud-init-output.log
<inspect cloud-init data>
capi@my-cluster-md-0-8qlrg:~$ journalctl -u kubelet
<inspect kubelet systemd logs>
```
