#!/bin/bash

if [ -z "$RESOURCE_GROUP" ]; then
    echo "must provide a RESOURCE_GROUP env var"
    exit 1;
fi

if [ -z "$REGION" ]; then
    echo "must provide a REGION env var"
    exit 1;
fi

if [ -z "$SUBSCRIPTION_ID" ]; then
    echo "must provide a SUBSCRIPTION_ID env var"
    exit 1;
fi

if [ -z "$AZURE_TENANT_ID" ]; then
    echo "must provide a AZURE_TENANT_ID env var"
    exit 1;
fi

if [ -z "$NAME" ]; then
    echo "must provide a NAME env var"
    exit 1;
fi

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

export AZURE_SUBSCRIPTION_ID="${SUBSCRIPTION_ID}"
export AZURE_LOCATION="${REGION}"
export AZURE_CUSTOM_VNET_RESOURCE_GROUP="${RESOURCE_GROUP}"
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-1.30.4}"
export NODES_PER_SYSTEM_POOL="${NODES_PER_SYSTEM_POOL:-1}"
export NODES_PER_USER_POOL="${NODES_PER_USER_POOL:-1}"
export SYSTEM_POOL_SKU="${USER_POOL_SKU:-Standard_D4s_v3}"
export AZURE_CONTROL_PLANE_MACHINE_TYPE="${AZURE_CONTROL_PLANE_MACHINE_TYPE:-Standard_D4s_v3}"
export AZURE_NODE_MACHINE_TYPE="${AZURE_NODE_MACHINE_TYPE:-Standard_D4s_v3}"
export USER_POOL_SKU="${USER_POOL_SKU:-Standard_D2s_v3}"
export CLUSTER_VNET="${CLUSTER_VNET:-${NAME}vnet}"
export AZURE_CUSTOM_VNET_NAME="${CLUSTER_VNET}"
export USER_IDENTITY="${USER_IDENTITY:-${NAME}identity}"
export CLUSTER_IDENTITY_TYPE="UserAssignedMSI"
export CLUSTER_IDENTITY_NAME="cluster-identity"
export BUILD_PROVENANCE="capz-dev"
export TIMESTAMP="now"
export JOB_NAME="capz-dev"
export CALICO_VERSION="v3.26.1"

az group create -n $RESOURCE_GROUP -l $REGION
az aks create -g $RESOURCE_GROUP -n $NAME-aks-init --kubernetes-version $KUBERNETES_VERSION -l $REGION -c $NODES_PER_SYSTEM_POOL -s $SYSTEM_POOL_SKU
az network vnet create -g $RESOURCE_GROUP --name $CLUSTER_VNET --address-prefixes 10.0.0.0/16 -o none
az network vnet subnet create -g $RESOURCE_GROUP --vnet-name $CLUSTER_VNET --name $CLUSTER_VNET-controlplane-subnet --address-prefixes 10.0.0.0/24 -o none
az network vnet subnet create -g $RESOURCE_GROUP --vnet-name $CLUSTER_VNET --name $CLUSTER_VNET-node-subnet --address-prefixes 10.0.1.0/24 -o none
az network vnet subnet create -g $RESOURCE_GROUP --vnet-name $CLUSTER_VNET --name $CLUSTER_VNET-bastion-subnet --address-prefixes 10.0.2.0/24 -o none
az network nsg create --resource-group $RESOURCE_GROUP --name $CLUSTER_VNET-nsg
az network nsg rule create --resource-group $RESOURCE_GROUP --nsg-name $CLUSTER_VNET-nsg --name allow_apiserver --priority 2201 --destination-address-prefixes '*' --destination-port-ranges 6443 --protocol Tcp --description "Allow API Server"
az network route-table create -g $RESOURCE_GROUP -n $CLUSTER_VNET-node-subnet-route-table
az network vnet subnet update --resource-group $RESOURCE_GROUP --vnet-name $CLUSTER_VNET --name $CLUSTER_VNET-controlplane-subnet --network-security-group $CLUSTER_VNET-nsg
az network vnet subnet update --resource-group $RESOURCE_GROUP --vnet-name $CLUSTER_VNET --name $CLUSTER_VNET-node-subnet --route-table $CLUSTER_VNET-node-subnet-route-table
az identity create -n $USER_IDENTITY -g $RESOURCE_GROUP -l $REGION --output none --only-show-errors
export AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY=$(az identity show -n "${USER_IDENTITY}" -g "${RESOURCE_GROUP}" --query clientId -o tsv)
AZURE_IDENTITY_ID_PRINCIPAL_ID=$(az identity show -n "${USER_IDENTITY}" -g "${RESOURCE_GROUP}" --query principalId -o tsv)
az role assignment create --assignee-object-id "${AZURE_IDENTITY_ID_PRINCIPAL_ID}" --role "Contributor" --scope "/subscriptions/${SUBSCRIPTION_ID}" --assignee-principal-type ServicePrincipal
az aks get-credentials -g $RESOURCE_GROUP -n $NAME-aks-init -f $HOME/.kube/$NAME-aks-init.kubeconfig --overwrite-existing
clusterctl init --kubeconfig=$HOME/.kube/$NAME-aks-init.kubeconfig --infrastructure azure
curl --retry 3 -sSL https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/releases/download/v0.2.5/addon-components.yaml | hack/tools/bin/envsubst | kubectl --kubeconfig=$HOME/.kube/$NAME-aks-init.kubeconfig apply -f -
export CLUSTER_NAME=$NAME-peered-mgmt
export KUBERNETES_VERSION=v1.30.4
export CI_RG=$RESOURCE_GROUP
cat $REPO_ROOT/templates/test/ci/cluster-template-prow-custom-vnet.yaml | hack/tools/bin/envsubst | kubectl apply --kubeconfig=$HOME/.kube/$NAME-aks-init.kubeconfig -f -
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "default.azurecluster.infrastructure.cluster.x-k8s.io": failed to call webhook: Post "https://capz-webhook-service.capz-system.svc:443/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azurecluster?timeout=10s": no endpoints available for service "capz-webhook-service"
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "default.azuremachinetemplate.infrastructure.cluster.x-k8s.io": failed to call webhook: Post "https://capz-webhook-service.capz-system.svc:443/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinetemplate?timeout=10s": no endpoints available for service "capz-webhook-service"
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "default.azuremachinetemplate.infrastructure.cluster.x-k8s.io": failed to call webhook: Post "https://capz-webhook-service.capz-system.svc:443/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinetemplate?timeout=10s": no endpoints available for service "capz-webhook-service"
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "validation.azureclusteridentity.infrastructure.cluster.x-k8s.io": failed to call webhook: Post "https://capz-webhook-service.capz-system.svc:443/validate-infrastructure-cluster-x-k8s-io-v1beta1-azureclusteridentity?timeout=10s": no endpoints available for service "capz-webhook-service"
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "helmchartproxy.kb.io": failed to call webhook: Post "https://caaph-webhook-service.caaph-system.svc:443/mutate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy?timeout=10s": no endpoints available for service "caaph-webhook-service"
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "helmchartproxy.kb.io": failed to call webhook: Post "https://caaph-webhook-service.caaph-system.svc:443/mutate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy?timeout=10s": no endpoints available for service "caaph-webhook-service"
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "helmchartproxy.kb.io": failed to call webhook: Post "https://caaph-webhook-service.caaph-system.svc:443/mutate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy?timeout=10s": no endpoints available for service "caaph-webhook-service"
# Error from server (InternalError): error when creating "STDIN": Internal error occurred: failed calling webhook "helmchartproxy.kb.io": failed to call webhook: Post "https://caaph-webhook-service.caaph-system.svc:443/mutate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy?timeout=10s": no endpoints available for service "caaph-webhook-service"
