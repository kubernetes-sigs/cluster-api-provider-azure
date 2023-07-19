# Troubleshooting Guide

Common issues users might run into when using Cluster API Provider for Azure. This list is work-in-progress. Feel free to open a PR to add to it if you find that useful information is missing.

## Examples of troubleshooting real-world issues

### No Azure resources are getting created

This is likely due to missing or invalid Azure credentials. 

Check the CAPZ controller logs on the management cluster:

```bash
kubectl logs deploy/capz-controller-manager -n capz-system manager
```

If you see an error similar to this:

```
azure.BearerAuthorizer#WithAuthorization: Failed to refresh the Token for request to https://management.azure.com/subscriptions/123/providers/Microsoft.Compute/skus?%24filter=location+eq+%27eastus2%27&api-version=2019-04-01: StatusCode=401 -- Original Error: adal: Refresh request failed. Status Code = '401'. Response body: {\"error\":\"invalid_client\",\"error_description\":\"AADSTS7000215: Invalid client secret is provided.
```

Make sure the provided Service Principal client ID and client secret are correct and that the password has not expired.

### The AzureCluster infrastructure is provisioned but no virtual machines are coming up

Your Azure subscription might have no quota for the requested VM size in the specified Azure location.

Check the CAPZ controller logs on the management cluster:

```bash
kubectl logs deploy/capz-controller-manager -n capz-system manager
```

If you see an error similar to this:
```
"error"="failed to reconcile AzureMachine: failed to create virtual machine: failed to create VM capz-md-0-qkg6m in resource group capz-fkl3tp: compute.VirtualMachinesClient#CreateOrUpdate: Failure sending request: StatusCode=0 -- Original Error: autorest/azure: Service returned an error. Status=\u003cnil\u003e Code=\"OperationNotAllowed\" Message=\"Operation could not be completed as it results in exceeding approved standardDSv3Family Cores quota.
```

Follow the [these steps](https://learn.microsoft.com/azure/azure-resource-manager/templates/error-resource-quota). Alternatively, you can specify another Azure location and/or VM size during cluster creation.

### A virtual machine is running but the k8s node did not join the cluster

Check the AzureMachine (or AzureMachinePool if using a MachinePool) status:
```bash
kubectl get azuremachines -o wide
```

If you see an output like this:

```
NAME                                       READY   STATE
default-template-md-0-w78jt                false   Updating
```

This indicates that the bootstrap script has not yet succeeded. Check the AzureMachine `status.conditions` field for more information.

[Take a look at the cloud-init logs](#checking-cloud-init-logs-ubuntu) for further debugging.

### One or more control plane replicas are missing

Take a look at the KubeadmControlPlane controller logs and look for any potential errors:

```bash
kubectl logs deploy/capi-kubeadm-control-plane-controller-manager -n capi-kubeadm-control-plane-system manager
```

In addition, make sure all pods on the workload cluster are healthy, including pods in the `kube-system` namespace.

### Nodes are in NotReady state

Make sure you have installed a CNI on the workload cluster and that all the pods on the workload cluster are in running state.

### Load Balancer service fails to come up

Check the cloud-controller-manager logs on the workload cluster. 

If running the Azure cloud provider in-tree:

```
kubectl logs kube-controller-manager-<control-plane-node-name> -n kube-system 
```

If running the Azure cloud provider out-of-tree:

```
kubectl logs cloud-controller-manager -n kube-system 
```


## Watching Kubernetes resources

To watch progression of all Cluster API resources on the management cluster you can run:

```bash
kubectl get cluster-api
```

## Looking at controller logs

To check the CAPZ controller logs on the management cluster, run:

```bash
kubectl logs deploy/capz-controller-manager -n capz-system manager
```

### Checking cloud-init logs (Ubuntu)

Cloud-init logs can provide more information on any issues that happened when running the bootstrap script. 

#### Option 1: Using the Azure Portal 

Located in the virtual machine blade (if [enabled](https://learn.microsoft.com/en-us/troubleshoot/azure/virtual-machines/boot-diagnostics) for the VM), the boot diagnostics option is under the Support and Troubleshooting section in the Azure portal.

For more information, see [here](https://learn.microsoft.com/azure/virtual-machines/boot-diagnostics#boot-diagnostics-view)

#### Option 2: Using the Azure CLI

```bash
az vm boot-diagnostics get-boot-log --name MyVirtualMachine --resource-group MyResourceGroup
```

For more information, see [here](https://learn.microsoft.com/cli/azure/vm/boot-diagnostics?view=azure-cli-latest).

#### Option 3: With SSH

Using the ssh information provided during cluster creation (environment variable `AZURE_SSH_PUBLIC_KEY_B64`):


##### connect to first control node - capi is default linux user created by deployment
```
API_SERVER=$(kubectl get azurecluster capz-cluster -o jsonpath='{.spec.controlPlaneEndpoint.host}')
ssh capi@${API_SERVER}
```

##### list nodes
```
kubectl get azuremachines
NAME                               READY   STATE
capz-cluster-control-plane-2jprg   true    Succeeded
capz-cluster-control-plane-ck5wv   true    Succeeded
capz-cluster-control-plane-w4tv6   true    Succeeded
capz-cluster-md-0-s52wb            false   Failed
capz-cluster-md-0-w8xxw            true    Succeeded
```

##### pick node name from output above:
```
node=$(kubectl get azuremachine capz-cluster-md-0-s52wb -o jsonpath='{.status.addresses[0].address}')
ssh -J capi@${apiserver} capi@${node}
```

##### look at cloud-init logs
`less /var/log/cloud-init-output.log`

## Automated log collection

As part of CI there is a [log collection tool](https://github.com/kubernetes-sigs/cluster-api-provider-azure/tree/main/test/logger.go) <!-- markdown-link-check-disable-line -->
which you can also leverage to pull all the logs for machines which will dump logs to `${PWD}/_artifacts}` by default. The following works 
if your kubeconfig is configured with the management cluster.  See the tool for more settings.

```bash
go run -tags e2e ./test/logger.go --name <workload-cluster-name> --namespace <workload-cluster-namespace>
```

There are also some [provided scripts](https://github.com/kubernetes-sigs/cluster-api-provider-azure/tree/main/hack/debugging) that can help automate a few common tasks.
