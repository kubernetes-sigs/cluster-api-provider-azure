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
# shellcheck source=hack/ensure-azcli.sh
source "${REPO_ROOT}/hack/ensure-azcli.sh" # install az cli and login using WI
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh" # set the right timestamp and job name

KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
AZWI="${REPO_ROOT}/hack/tools/bin/azwi"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${AZWI##*/}"

export MGMT_CLUSTER_NAME="${MGMT_CLUSTER_NAME:-aks-mgmt-capz-${RANDOM_SUFFIX}}" # management cluster name
export AKS_RESOURCE_GROUP="${AKS_RESOURCE_GROUP:-aks-mgmt-capz-${RANDOM_SUFFIX}}" # resource group name
export AKS_NODE_RESOURCE_GROUP="node-${AKS_RESOURCE_GROUP}"
export AKS_MGMT_KUBERNETES_VERSION="${AKS_MGMT_KUBERNETES_VERSION:-v1.30.2}"
export AZURE_LOCATION="${AZURE_LOCATION:-westus2}"
export AKS_NODE_VM_SIZE="${AKS_NODE_VM_SIZE:-"Standard_B2s"}"
export AKS_NODE_COUNT="${AKS_NODE_COUNT:-2}"
export AZURE_NODE_MACHINE_TYPE="${AZURE_NODE_MACHINE_TYPE:-"Standard_B2s"}"
export MGMT_CLUSTER_KUBECONFIG="${MGMT_CLUSTER_KUBECONFIG:-$REPO_ROOT/aks-mgmt.config}"
export AZURE_IDENTITY_ID_FILEPATH="${AZURE_IDENTITY_ID_FILEPATH:-$REPO_ROOT/azure_identity_id}"
export AZWI_STORAGE_ACCOUNT="capzcioidcissuer${RANDOM_SUFFIX}"
export AZWI_STORAGE_CONTAINER="\$web"
export SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH="${SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH:-}"
export SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH="${SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH:-}"
export REGISTRY="${REGISTRY:-}"
export AKS_MGMT_VNET_NAME="${AKS_MGMT_VNET_NAME:-"aks-mgmt-vnet-${RANDOM_SUFFIX}"}"
export AKS_MGMT_VNET_CIDR="${AKS_MGMT_VNET_CIDR:-"20.255.0.0/16"}"
export AKS_MGMT_SERVICE_CIDR="${AKS_MGMT_SERVICE_CIDR:-"20.255.254.0/24"}"
export AKS_MGMT_DNS_SERVICE_IP="${AKS_MGMT_DNS_SERVICE_IP:-"20.255.254.100"}"
export AKS_MGMT_SUBNET_NAME="${AKS_MGMT_SUBNET_NAME:-"aks-mgmt-subnet-${RANDOM_SUFFIX}"}"
export AKS_MGMT_SUBNET_CIDR="${AKS_MGMT_SUBNET_CIDR:-"20.255.0.0/24"}"


export AZURE_SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID:-}"
export AZURE_CLIENT_ID="${AZURE_CLIENT_ID:-}"
export AZURE_TENANT_ID="${AZURE_TENANT_ID:-}"

# to suppress unbound variable error message
export APISERVER_LB_DNS_SUFFIX="${APISERVER_LB_DNS_SUFFIX:-}"
export AKS_MI_CLIENT_ID="${AKS_MI_CLIENT_ID:-}"
export AKS_MI_OBJECT_ID="${AKS_MI_OBJECT_ID:-}"
export AKS_MI_RESOURCE_ID="${AKS_MI_RESOURCE_ID:-}"
export MANAGED_IDENTITY_NAME="${MANAGED_IDENTITY_NAME:-}"
export MANAGED_IDENTITY_RG="${MANAGED_IDENTITY_RG:-}"
export ASO_CREDENTIAL_SECRET_MODE="${ASO_CREDENTIAL_SECRET_MODE:-}"
export SKIP_AKS_CREATE="${SKIP_AKS_CREATE:-false}"

main() {

  echo "--------------------------------"
  echo "MGMT_CLUSTER_NAME:                    $MGMT_CLUSTER_NAME"
  echo "AKS_RESOURCE_GROUP:                   $AKS_RESOURCE_GROUP"
  echo "AKS_NODE_RESOURCE_GROUP:              $AKS_NODE_RESOURCE_GROUP"
  echo "AKS_MGMT_KUBERNETES_VERSION:          $AKS_MGMT_KUBERNETES_VERSION"
  echo "AZURE_LOCATION:                       $AZURE_LOCATION"
  echo "AKS_NODE_VM_SIZE:                     $AKS_NODE_VM_SIZE"
  echo "AZURE_NODE_MACHINE_TYPE:              $AZURE_NODE_MACHINE_TYPE"
  echo "AKS_NODE_COUNT:                       $AKS_NODE_COUNT"
  echo "MGMT_CLUSTER_KUBECONFIG:              $MGMT_CLUSTER_KUBECONFIG"
  echo "AZURE_IDENTITY_ID_FILEPATH:           $AZURE_IDENTITY_ID_FILEPATH"
  echo "AZWI_STORAGE_ACCOUNT:                 $AZWI_STORAGE_ACCOUNT"
  echo "AZWI_STORAGE_CONTAINER:               $AZWI_STORAGE_CONTAINER"
  echo "SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH: $SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH"
  echo "SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH: $SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH"
  echo "REGISTRY:                             $REGISTRY"
  echo "APISERVER_LB_DNS_SUFFIX:              $APISERVER_LB_DNS_SUFFIX"
  echo "AKS_MGMT_VNET_NAME:                   $AKS_MGMT_VNET_NAME"
  echo "AKS_MGMT_VNET_CIDR:                   $AKS_MGMT_VNET_CIDR"
  echo "AKS_MGMT_SERVICE_CIDR:                $AKS_MGMT_SERVICE_CIDR"
  echo "AKS_MGMT_DNS_SERVICE_IP:              $AKS_MGMT_DNS_SERVICE_IP"
  echo "AKS_MGMT_SUBNET_NAME:                 $AKS_MGMT_SUBNET_NAME"
  echo "AKS_MGMT_SUBNET_CIDR:                 $AKS_MGMT_SUBNET_CIDR"

  echo "AZURE_SUBSCRIPTION_ID:                $AZURE_SUBSCRIPTION_ID"
  echo "AZURE_CLIENT_ID:                      $AZURE_CLIENT_ID"
  echo "AZURE_TENANT_ID:                      $AZURE_TENANT_ID"
  echo "--------------------------------"

  # if using SKIP_AKS_CREATE=true, skip creating the AKS cluster
  if [[ "${SKIP_AKS_CREATE}" == "true" ]]; then
    echo "Skipping AKS cluster creation"
    return
  fi

  create_aks_cluster
  set_env_varaibles
}

create_aks_cluster() {
  resource_group_exists=$(az group exists --name "${AKS_RESOURCE_GROUP}" --output tsv)
  if [ "${resource_group_exists}" == 'true' ]; then
    echo "resource group \"${AKS_RESOURCE_GROUP}\" already exists, moving on"
  else
    echo "creating resource group ${AKS_RESOURCE_GROUP}"
    az group create --name "${AKS_RESOURCE_GROUP}" \
    --location "${AZURE_LOCATION}" \
    --output none --only-show-errors \
    --tags creationTimestamp="${TIMESTAMP}" jobName="${JOB_NAME}" buildProvenance="${BUILD_PROVENANCE}"

    echo "creating vnet for the resource group ${AKS_RESOURCE_GROUP}"
    az network vnet create \
      --resource-group "${AKS_RESOURCE_GROUP}"\
      --name "${AKS_MGMT_VNET_NAME}" \
      --address-prefix "${AKS_MGMT_VNET_CIDR}" \
      --subnet-name "${AKS_MGMT_SUBNET_NAME}" \
      --subnet-prefix "${AKS_MGMT_SUBNET_CIDR}" \
      --output none --only-show-errors \
      --tags creationTimestamp="${TIMESTAMP}" jobName="${JOB_NAME}" buildProvenance="${BUILD_PROVENANCE}"
  fi

  aks_exists=$(az aks show --name "${MGMT_CLUSTER_NAME}" --resource-group "${AKS_RESOURCE_GROUP}" 2>&1 || true) # true because we want to continue if the command fails
  if echo "$aks_exists" | grep -E -q "Resource(NotFound|GroupNotFound)"; then
    echo "creating aks cluster ${MGMT_CLUSTER_NAME} in the resource group ${AKS_RESOURCE_GROUP}"
    az aks create --name "${MGMT_CLUSTER_NAME}" \
    --resource-group "${AKS_RESOURCE_GROUP}" \
    --location "${AZURE_LOCATION}" \
    --kubernetes-version "${AKS_MGMT_KUBERNETES_VERSION}" \
    --node-count "${AKS_NODE_COUNT}" \
    --node-vm-size "${AKS_NODE_VM_SIZE}" \
    --node-resource-group "${AKS_NODE_RESOURCE_GROUP}" \
    --vm-set-type VirtualMachineScaleSets \
    --generate-ssh-keys \
    --network-plugin azure \
    --vnet-subnet-id "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AKS_RESOURCE_GROUP}/providers/Microsoft.Network/virtualNetworks/${AKS_MGMT_VNET_NAME}/subnets/${AKS_MGMT_SUBNET_NAME}" \
    --service-cidr "${AKS_MGMT_SERVICE_CIDR}" \
    --dns-service-ip "${AKS_MGMT_DNS_SERVICE_IP}" \
    --max-pods 60 \
    --tags creationTimestamp="${TIMESTAMP}" jobName="${JOB_NAME}" buildProvenance="${BUILD_PROVENANCE}" \
    --output none --only-show-errors;
  elif echo "$aks_exists" | grep -q "${MGMT_CLUSTER_NAME}"; then
    echo "cluster ${MGMT_CLUSTER_NAME} already exists in RG ${AKS_RESOURCE_GROUP}, moving on"
  else
    echo "error : ${aks_exists}"
    exit 1
  fi

  # check and save kubeconfig
  echo -e "\n"
  echo "saving credentials of cluster ${MGMT_CLUSTER_NAME} in ${REPO_ROOT}/${MGMT_CLUSTER_KUBECONFIG}"
  az aks get-credentials --name "${MGMT_CLUSTER_NAME}" --resource-group "${AKS_RESOURCE_GROUP}" \
  --file "${REPO_ROOT}/${MGMT_CLUSTER_KUBECONFIG}" --only-show-errors

  az aks get-credentials --name "${MGMT_CLUSTER_NAME}" --resource-group "${AKS_RESOURCE_GROUP}" \
  --overwrite-existing --only-show-errors

  # echo "fetching Client ID for ${MGMT_CLUSTER_NAME}"
  AKS_MI_CLIENT_ID=$(az aks show -n "${MGMT_CLUSTER_NAME}" -g "${AKS_RESOURCE_GROUP}" --output json \
  --only-show-errors | jq -r '.identityProfile.kubeletidentity.clientId')
  export AKS_MI_CLIENT_ID
  echo "mgmt client identity: ${AKS_MI_CLIENT_ID}"
  echo "${AKS_MI_CLIENT_ID}" > "${AZURE_IDENTITY_ID_FILEPATH}"

  # echo "fetching Object ID for ${MGMT_CLUSTER_NAME}"
  AKS_MI_OBJECT_ID=$(az aks show -n "${MGMT_CLUSTER_NAME}" -g "${AKS_RESOURCE_GROUP}" --output json \
  --only-show-errors | jq -r '.identityProfile.kubeletidentity.objectId')
  export AKS_MI_OBJECT_ID
  echo "mgmt object identity: ${AKS_MI_OBJECT_ID}"

  # echo "fetching Resource ID for ${MGMT_CLUSTER_NAME}"
  AKS_MI_RESOURCE_ID=$(az aks show -n "${MGMT_CLUSTER_NAME}" -g "${AKS_RESOURCE_GROUP}" --output json \
  --only-show-errors | jq -r '.identityProfile.kubeletidentity.resourceId')
  export AKS_MI_RESOURCE_ID
  echo "mgmt resource identity: ${AKS_MI_RESOURCE_ID}"

  # save resource identity name and resource group
  MANAGED_IDENTITY_NAME=$(az identity show --ids "${AKS_MI_RESOURCE_ID}" --output json | jq -r '.name')
  # export MANAGED_IDENTITY_NAME
  echo "mgmt resource identity name: ${MANAGED_IDENTITY_NAME}"
  USER_IDENTITY=$MANAGED_IDENTITY_NAME
  export USER_IDENTITY

  MANAGED_IDENTITY_RG=$(az identity show --ids "${AKS_MI_RESOURCE_ID}" --output json | jq -r '.resourceGroup')
  export MANAGED_IDENTITY_RG
  echo "mgmt resource identity resource group: ${MANAGED_IDENTITY_RG}"

  echo "assigning contributor role to the service principal"
  until az role assignment create --assignee-object-id "${AKS_MI_OBJECT_ID}" --role "Contributor" \
  --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}" --assignee-principal-type ServicePrincipal --output none \
  --only-show-errors; do
    echo "retrying to assign role to the service principal"
    sleep 5
  done

  echo "using ASO_CREDENTIAL_SECRET_MODE as podidentity"
  ASO_CREDENTIAL_SECRET_MODE="podidentity"
}

set_env_varaibles(){
  cat <<EOF > tilt-settings-temp.yaml
kustomize_substitutions:
  AKS_RESOURCE_GROUP: "${AKS_RESOURCE_GROUP}"
  AKS_NODE_RESOURCE_GROUP: "${AKS_NODE_RESOURCE_GROUP}"
  AKS_MGMT_VNET_NAME: "${AKS_MGMT_VNET_NAME}"
  MGMT_CLUSTER_NAME: "${MGMT_CLUSTER_NAME}"
  AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY: "${AKS_MI_CLIENT_ID}"
  CI_RG: "${MANAGED_IDENTITY_RG}"
  USER_IDENTITY: "${MANAGED_IDENTITY_NAME}"
  CLUSTER_IDENTITY_TYPE: "UserAssignedMSI"
  ASO_CREDENTIAL_SECRET_MODE: "${ASO_CREDENTIAL_SECRET_MODE}"
  REGISTRY: "${REGISTRY}"
  APISERVER_LB_DNS_SUFFIX: "${APISERVER_LB_DNS_SUFFIX}"
allowed_contexts:
  - "$MGMT_CLUSTER_NAME"
  - "kind-capz"
azure_location: "${AZURE_LOCATION}"
EOF

# create tilt-settings.yaml if it does not exist
if [ -f tilt-settings.yaml ]; then
  echo "tilt-settings.yaml exists"
else
  echo "tilt-settings.yaml does not exist, creating one"
  touch tilt-settings.yaml
fi

# copy over the existing allowed_contexts to tilt-settings.yaml if it does not exist
allowed_contexts_exists=$(yq eval '.allowed_contexts' tilt-settings.yaml)
if [ "$allowed_contexts_exists" == "null" ]; then
  yq eval '.allowed_contexts = load("tilt-settings-temp.yaml") | .allowed_contexts' tilt-settings-temp.yaml > tilt-settings.yaml
fi

# extract allowed_contexts from tilt-settings.yaml
current_contexts=$(yq eval '.allowed_contexts' tilt-settings.yaml | sort -u)

# extract allowed_contexts from tilt-settings-new.yaml
new_contexts=$(yq eval '.allowed_contexts' tilt-settings-temp.yaml | sort -u)

# combine current and new contexts, keeping the union of both
combined_contexts=$(echo "$current_contexts"$'\n'"$new_contexts" | sort -u)

# create a temporary file since env($combined_contexts) is not supported in yq
echo "$combined_contexts" > combined_contexts.yaml

# update allowed_contexts in tilt-settings.yaml with the combined contexts
yq eval --inplace ".allowed_contexts = load(\"combined_contexts.yaml\")" tilt-settings.yaml

# merge the updated kustomize_substitution and azure_location with the existing one in tilt-settings.yaml
yq eval-all 'select(fileIndex == 0) *+ {"kustomize_substitutions": select(fileIndex == 1).kustomize_substitutions, "azure_location": select(fileIndex == 1).azure_location}' tilt-settings.yaml tilt-settings-temp.yaml > tilt-settings-new.yaml

mv tilt-settings-new.yaml tilt-settings.yaml
rm -r combined_contexts.yaml
rm -f tilt-settings-temp.yaml
}

main
