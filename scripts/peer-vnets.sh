#!/usr/bin/env bash
# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit # exit immediately if a command exits with a non-zero status.
set -o nounset # exit when script tries to use undeclared variables.
set -o pipefail # make the pipeline fail if any command in it fails.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

source_tilt_settings() {

  found=false
  # echo "checking for tilt-settings.yaml..."
  # echo "args: $@"
  for arg in "$@"; do
      if [ "$arg" == "tilt-settings.yaml" ]; then
          found=true
          break
      fi
  done

  if [ "$found" == true ]; then
      echo "tilt-settings.yaml was passed as an argument."
  else
      echo "tilt-settings.yaml was not found."
      exit 1
  fi

  TILT_SETTINGS_FILE="$1"

  # Check that the file exists
  if [ ! -f "$TILT_SETTINGS_FILE" ]; then
    echo "File not found: $TILT_SETTINGS_FILE"
    exit 1
  fi

  echo "Reading variables from $TILT_SETTINGS_FILE under 'kustomize_substitutions'..."

  # Get the list of keys under kustomize_substitutions
  VAR_KEYS=$(yq e '.kustomize_substitutions | keys | .[]' "$TILT_SETTINGS_FILE" 2>/dev/null || true)

  # If there's no such key or it's empty, VAR_KEYS will be empty
  if [ -z "$VAR_KEYS" ]; then
    echo "No variables found under 'kustomize_substitutions'."
  else
    for key in $VAR_KEYS; do
      # Read the value of each key
      value=$(yq e ".kustomize_substitutions[\"$key\"]" "$TILT_SETTINGS_FILE")
      # Export the key/value pair
      export "$key=$value"
      echo "Exported $key=$value"
    done
  fi

  echo "All variables exported"
}


peer_vnets() {
  # ------------------------------------------------------------------------------
  # Peer Vnets
  # ------------------------------------------------------------------------------

  echo "--------Peering VNETs--------"
  az network vnet wait --resource-group ${AKS_RESOURCE_GROUP:-''} --name ${AKS_MGMT_VNET_NAME:-''} --created --timeout 180
  export MGMT_VNET_ID=$(az network vnet show --resource-group ${AKS_RESOURCE_GROUP:-''} --name ${AKS_MGMT_VNET_NAME:-''} --query id --output tsv)
  echo " 1/8 ${AKS_MGMT_VNET_NAME:-''} found and ${MGMT_VNET_ID} found"

  # wait for workload VNet to be created
  az network vnet wait --resource-group ${CLUSTER_NAME:-''} --name ${CLUSTER_NAME:-''}-vnet --created --timeout 180
  export WORKLOAD_VNET_ID=$(az network vnet show --resource-group ${CLUSTER_NAME:-''} --name ${CLUSTER_NAME:-''}-vnet --query id --output tsv)
  echo " 2/8 ${CLUSTER_NAME:-''}-vnet found and ${WORKLOAD_VNET_ID} found"

  # peer mgmt vnet
  az network vnet peering create --name mgmt-to-${CLUSTER_NAME:-''} --resource-group ${AKS_RESOURCE_GROUP:-''} --vnet-name ${AKS_MGMT_VNET_NAME:-''} --remote-vnet ${WORKLOAD_VNET_ID} --allow-vnet-access true --allow-forwarded-traffic true --only-show-errors --output none
  az network vnet peering wait --name mgmt-to-${CLUSTER_NAME:-''} --resource-group ${AKS_RESOURCE_GROUP:-''} --vnet-name ${AKS_MGMT_VNET_NAME:-''} --created --timeout 300 --only-show-errors --output none
  echo " 3/8 mgmt-to-${CLUSTER_NAME:-''} peering created in ${AKS_MGMT_VNET_NAME:-''}"

  # peer workload vnet
  az network vnet peering create --name ${CLUSTER_NAME:-''}-to-mgmt --resource-group ${CLUSTER_NAME:-''} --vnet-name ${CLUSTER_NAME:-''}-vnet --remote-vnet ${MGMT_VNET_ID} --allow-vnet-access true --allow-forwarded-traffic true --only-show-errors --output none
  az network vnet peering wait --name ${CLUSTER_NAME:-''}-to-mgmt --resource-group ${CLUSTER_NAME:-''} --vnet-name ${CLUSTER_NAME:-''}-vnet --created --timeout 300 --only-show-errors --output none
  echo " 4/8 ${CLUSTER_NAME:-''}-to-mgmt peering created in ${CLUSTER_NAME:-''}-vnet"

  # create private DNS zone
  az network private-dns zone create --resource-group ${CLUSTER_NAME:-''} --name ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com --only-show-errors --output none
  az network private-dns zone wait --resource-group ${CLUSTER_NAME:-''} --name ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com --created --timeout 300 --only-show-errors --output none
  echo " 5/8 ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com private DNS zone created in ${CLUSTER_NAME:-''}"

  # link private DNS Zone to workload vnet
  az network private-dns link vnet create --resource-group ${CLUSTER_NAME:-''} --zone-name ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com --name ${CLUSTER_NAME:-''}-to-mgmt --virtual-network ${WORKLOAD_VNET_ID} --registration-enabled false --only-show-errors --output none
  az network private-dns link vnet wait --resource-group ${CLUSTER_NAME:-''} --zone-name ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com --name ${CLUSTER_NAME:-''}-to-mgmt --created --timeout 300 --only-show-errors --output none
  echo " 6/8 workload cluster vnet ${CLUSTER_NAME:-''}-vnet linked with private DNS zone"

  # link private DNS Zone to mgmt vnet
  az network private-dns link vnet create --resource-group ${CLUSTER_NAME:-''} --zone-name ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com --name mgmt-to-${CLUSTER_NAME:-''} --virtual-network ${MGMT_VNET_ID} --registration-enabled false --only-show-errors --output none
  az network private-dns link vnet wait --resource-group ${CLUSTER_NAME:-''} --zone-name ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com --name mgmt-to-${CLUSTER_NAME:-''} --created --timeout 300 --only-show-errors --output none
  echo " 7/8 management cluster vnet ${AKS_MGMT_VNET_NAME:-''} linked with private DNS zone"

  # create private DNS zone record
  az network private-dns record-set a add-record --resource-group ${CLUSTER_NAME:-''} --zone-name ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com --record-set-name @ --ipv4-address ${AZURE_INTERNAL_LB_PRIVATE_IP} --only-show-errors --output none
  echo " 8/8 \"@\" private DNS zone record created to point ${CLUSTER_NAME:-''}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com to ${AZURE_INTERNAL_LB_PRIVATE_IP}"
}
