#!/bin/bash

# TODO: check for az cli to be installed in local
# wait for AKS VNet to be in the state created

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# shellcheck source=hack/common-vars.sh
source "${REPO_ROOT}/hack/common-vars.sh"

source "${REPO_ROOT}/aks-mgmt-vars.env"

echo \"--------Peering VNETs--------\"
az network vnet wait --resource-group ${AKS_RESOURCE_GROUP} --name ${AKS_MGMT_VNET_NAME} --created --timeout 180
export MGMT_VNET_ID=$(az network vnet show --resource-group ${AKS_RESOURCE_GROUP} --name ${AKS_MGMT_VNET_NAME} --query id --output tsv)
echo \" 1/8 ${AKS_MGMT_VNET_NAME} found \"

# wait for workload VNet to be created
az network vnet wait --resource-group ${CLUSTER_NAME} --name ${CLUSTER_NAME}-vnet --created --timeout 180
export WORKLOAD_VNET_ID=$(az network vnet show --resource-group ${CLUSTER_NAME} --name ${CLUSTER_NAME}-vnet --query id --output tsv)
echo \" 2/8 ${CLUSTER_NAME}-vnet found \"

# peer mgmt vnet
az network vnet peering create --name mgmt-to-${CLUSTER_NAME} --resource-group ${AKS_RESOURCE_GROUP} --vnet-name ${AKS_MGMT_VNET_NAME} --remote-vnet \"${WORKLOAD_VNET_ID}\" --allow-vnet-access true --allow-forwarded-traffic true --only-show-errors --output none
az network vnet peering wait --name mgmt-to-${CLUSTER_NAME} --resource-group ${AKS_RESOURCE_GROUP} --vnet-name ${AKS_MGMT_VNET_NAME} --created --timeout 300 --only-show-errors --output none
echo \" 3/8 mgmt-to-${CLUSTER_NAME} peering created in ${AKS_MGMT_VNET_NAME}\"

# peer workload vnet
az network vnet peering create --name ${CLUSTER_NAME}-to-mgmt --resource-group ${CLUSTER_NAME} --vnet-name ${CLUSTER_NAME}-vnet --remote-vnet \"${MGMT_VNET_ID}\" --allow-vnet-access true --allow-forwarded-traffic true --only-show-errors --output none
az network vnet peering wait --name ${CLUSTER_NAME}-to-mgmt --resource-group ${CLUSTER_NAME} --vnet-name ${CLUSTER_NAME}-vnet --created --timeout 300 --only-show-errors --output none
echo \" 4/8 ${CLUSTER_NAME}-to-mgmt peering created in ${CLUSTER_NAME}-vnet\"

# create private DNS zone
az network private-dns zone create --resource-group ${CLUSTER_NAME} --name ${AZURE_LOCATION}.cloudapp.azure.com --only-show-errors --output none
az network private-dns zone wait --resource-group ${CLUSTER_NAME} --name ${AZURE_LOCATION}.cloudapp.azure.com --created --timeout 300 --only-show-errors --output none
echo \" 5/8 ${AZURE_LOCATION}.cloudapp.azure.com private DNS zone created in ${CLUSTER_NAME}\"

# link private DNS Zone to workload vnet
az network private-dns link vnet create --resource-group ${CLUSTER_NAME} --zone-name ${AZURE_LOCATION}.cloudapp.azure.com --name ${CLUSTER_NAME}-to-mgmt --virtual-network \"${WORKLOAD_VNET_ID}\" --registration-enabled false --only-show-errors --output none
az network private-dns link vnet wait --resource-group ${CLUSTER_NAME} --zone-name ${AZURE_LOCATION}.cloudapp.azure.com --name ${CLUSTER_NAME}-to-mgmt --created --timeout 300 --only-show-errors --output none
echo \" 6/8 workload cluster vnet ${CLUSTER_NAME}-vnet linked with private DNS zone\"

# link private DNS Zone to mgmt vnet
az network private-dns link vnet create --resource-group ${CLUSTER_NAME} --zone-name ${AZURE_LOCATION}.cloudapp.azure.com --name mgmt-to-${CLUSTER_NAME} --virtual-network \"${MGMT_VNET_ID}\" --registration-enabled false --only-show-errors --output none
az network private-dns link vnet wait --resource-group ${CLUSTER_NAME} --zone-name ${AZURE_LOCATION}.cloudapp.azure.com --name mgmt-to-${CLUSTER_NAME} --created --timeout 300 --only-show-errors --output none
echo \" 7/8 management cluster vnet ${AKS_MGMT_VNET_NAME} linked with private DNS zone\"

# create private DNS zone record
# TODO: 10.0.0.100 should be customizable
az network private-dns record-set a add-record --resource-group ${CLUSTER_NAME} --zone-name ${AZURE_LOCATION}.cloudapp.azure.com --record-set-name ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX} --ipv4-address 10.0.0.100 --only-show-errors --output none
echo \" 8/8 ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX} private DNS zone record created\n\"