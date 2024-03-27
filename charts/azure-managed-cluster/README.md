# CAPI Azure

## Prerequisites
- [Docker Desktop](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/)
- [ClusterCTL](https://cluster-api.sigs.k8s.io/clusterctl/overview.html) Version v1.6.1 or older
- [Helm](https://helm.sh) version v3.14.0 or later

## Prerequisites Installations
- Docker Desktop
 https://www.docker.com/products/docker-desktop 

- Install Kind
  https://kind.sigs.k8s.io/

- Install Clusterctl
  https://cluster-api.sigs.k8s.io/clusterctl/overview.html

- Install Helm3
- https://helm.sh/docs/intro/install/

## Deploy

Clone this repo

```bash
git clone https://github.com/ams0/azure-managed-cluster-capz-helm.git
cd azure-managed-cluster-capz-helm

```

Create an Azure Service Principal or Managed Identity

- Follow the steps on this link to create a service principal <https://capz.sigs.k8s.io/topics/identities#service-principal>
- Follow the steps on this link to create a managed identity <https://capz.sigs.k8s.io/topics/identities#managed-identity>

Create and Update Environment variables
Edit your ~/.bashrc file to include Azure Service Principal environment variables or just export on your current terminal session
```bash
export AZURE_CLIENT_ID="" (if using Service Principal)

export AZURE_CLIENT_SECRET="" (if using Service Principal)

export AZURE_SUBSCRIPTION_ID=""

export AZURE_TENANT_ID=""

```

Load Environment variables
```bash
source clusterctl.env
```

Create a KIND cluster(Kind clusters only work service principal)):

```bash
kind create cluster --name capi-helm
```


Initialize Cluster API and install Azure CAPZ provider version 

```bash
clusterctl init --infrastructure azure
```

Deploy a cluster with Helm (please customize parameters as required)

**Using Service Principal:**

```bash
helm install capz1 charts/azure-managed-cluster/  \
--namespace default \
--set subscriptionID="${AZURE_SUBSCRIPTION_ID}" \
--set identity.clientId="${AZURE_CLIENT_ID}" \
--set identity.clientSecret="${AZURE_CLIENT_SECRET}" \
--set identity.type=ServicePrincipal \
--set identity.tenantId="${AZURE_TENANT_ID}" \
--set cluster.resourceGroupName=aksclusters \
--set cluster.nodeResourceGroupName=capz1 \
--set cluster.name=aks1 \
--set agentpools.agentpool0.name=capz1np0 \
--set agentpools.agentpool0.nodecount=1 \
--set agentpools.agentpool0.sku=Standard_B4ms \
--set agentpools.agentpool0.osDiskSizeGB=100 \
--set agentpools.agentpool0.mode=System \
--set agentpools.agentpool1.name=capz1np1 \
--set agentpools.agentpool1.nodecount=1 \
--set agentpools.agentpool1.sku=Standard_B4ms \
--set agentpools.agentpool1.osDiskSizeGB=10 \
--set agentpools.agentpool1.mode=User 
```

or more simply (after you edit the values file with your own values):

```bash
helm install capz1 charts/azure-managed-cluster/ --values values.yaml \
--namespace default \
--set controlplane.sshPublicKey="$(cat ~/.ssh/id_rsa.pub)" \
--set subscriptionID="${AZURE_SUBSCRIPTION_ID}" \
--set identity.clientID="${AZURE_CLIENT_ID}" \
--set identity.tenantID="${AZURE_TENANT_ID}" \
--set identity.clientSecret="${AZURE_CLIENT_SECRET}" \
--set identity.type=ServicePrincipal 
```

**Using Managed Identity**

NB: Ensure the AZURE_USER_ASSIGNED_IDENTITY_RESOURCE_ID is set by getting the resource id of the managed identity created in Azure


```bash

helm install capz1 charts/azure-managed-cluster/  \
--namespace default \
--set subscriptionID="${AZURE_SUBSCRIPTION_ID}" \
--set identity.clientID="${AZURE_CLIENT_ID}" \
--set identity.tenantID="${AZURE_TENANT_ID}" \
--set identity.type=UserAssignedMSI \
--set identity.resourceID="${AZURE_USER_ASSIGNED_IDENTITY_RESOURCE_ID}" 



Check the status with:
```
kubectl get cluster-api
kubectl  logs -n capz-system -l control-plane=capz-controller-manager -c manager -f
```

Get the credentials

```
kubectl get secret capi-helm-kubeconfig -o yaml -o jsonpath={.data.value} | base64 --decode > aks1.kubeconfig
```

Test the cluster!

```
kubectl --kubeconfig=aks1.kubeconfig cluster-info
```



Clean up:

```bash
helm delete capz1
helm delete capz2 -n default2
kubectl delete namespace default2

kind delete clusters capi
kind delete clusters capi-helm
```
