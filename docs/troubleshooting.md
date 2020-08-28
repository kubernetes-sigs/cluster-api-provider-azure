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

Using the ssh information provided during cluster creation (environment variable `AZURE_SSH_PUBLIC_KEY_B64`), you can debug most issues by SSHing into the VMs that have been created:

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
