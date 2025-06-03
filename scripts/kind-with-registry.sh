#!/usr/bin/env bash
# Copyright 2020 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

# Install kubectl and kind
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# shellcheck source=hack/ensure-azcli.sh
source "${REPO_ROOT}/hack/ensure-azcli.sh"
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh"

KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
KIND="${REPO_ROOT}/hack/tools/bin/kind"
AZWI="${REPO_ROOT}/hack/tools/bin/azwi"

# Remove aks_as_mgmt_settings from tilt-settings.yaml if it exists
TILT_SETTINGS_FILE="${REPO_ROOT}/tilt-settings.yaml"
if [ -f "$TILT_SETTINGS_FILE" ]; then
    # Check if yq is installed
    if ! command -v yq &> /dev/null; then
        echo "yq is required but not installed. Please install yq first."
        exit 1
    fi
    
    # Check if aks_as_mgmt_settings exists in the file
    if yq e 'has("aks_as_mgmt_settings")' "$TILT_SETTINGS_FILE" | grep -q "true"; then
        echo "Removing aks_as_mgmt_settings from tilt-settings.yaml"
        yq e 'del(.aks_as_mgmt_settings)' -i "$TILT_SETTINGS_FILE"
    fi
fi

AZWI_ENABLED="${AZWI_ENABLED:-true}"
RANDOM_SUFFIX="${RANDOM_SUFFIX:-$(od -An -N4 -tu4 /dev/urandom | tr -d ' ' | head -c 8)}"
export AZWI_STORAGE_ACCOUNT="capzcioidcissuer${RANDOM_SUFFIX}"
export AZWI_STORAGE_CONTAINER="\$web"
export AZWI_LOCATION="${AZURE_LOCATION:-southcentralus}"
export SERVICE_ACCOUNT_ISSUER="${SERVICE_ACCOUNT_ISSUER:-}"
export SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH="${SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH:-}"
export SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH="${SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH:-}"
export AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY="${AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY:-}"
export AZURE_IDENTITY_ID_FILEPATH="${AZURE_IDENTITY_ID_FILEPATH:-$REPO_ROOT/azure_identity_id}"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${KIND##*/}"

# Export desired cluster name; default is "capz"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capz}"
CONFORMANCE_FLAVOR="${CONFORMANCE_FLAVOR:-}"
export KIND_CLUSTER_NAME

if [[ "$("${KIND}" get clusters)" =~ .*"${KIND_CLUSTER_NAME}".* ]]; then
  echo "cluster already exists, moving on"
  exit 0
fi

# 1. Create registry container unless it already exists
reg_name='kind-registry'
reg_port="${KIND_REGISTRY_PORT:-5000}"
if [ "$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
  docker run \
    -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" \
    registry:2
fi

# Environment variable inputs
# SERVICE_ACCOUNT_ISSUER - BYO existing service account issuer
#    Accepts a URI string, e.g., https://${AZWI_STORAGE_ACCOUNT}.blob.core.windows.net/${AZWI_STORAGE_CONTAINER}/
#    Assumes that the required openid and jwks artifacts exist in the well-known locations at this URI
# SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH - A local filepath to a 2048 bit RSA public key
#    Defaults to capz-wi-sa.pub, must exist locally and match the signed artifacts if using BYO
# SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH - A local filepath to a 2048 bit RSA private key
#    Defaults to capz-wi-sa.key, must exist locally and match the signed artifacts if using BYO
#    If the above keypair filepaths environment variables are not included, a keypair will be created at runtime
#    Note: if a new keypair is created at runtime then you must not BYO service account issuer
# AZWI_RESOURCE_GROUP - Azure resource group where Workload Identity infra lives
# AZWI_LOCATION - Azure location for Workload Identity infra
# AZWI_STORAGE_ACCOUNT - Storage account in resource group $AZWI_RESOURCE_GROUP containing required artifacts
#    Must be configured for static website hosting
# AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY - BYO existing user-assigned identity
#    Should be a UUID that represents the clientID of the identity object
# USER_IDENTITY - Name to use when creating a new user-assigned identity
#    Required if not bringing your own identity via $AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY
# AZURE_IDENTITY_ID_FILEPATH - A local filepath to store the newly created user-assigned identity if not bringing your own
function checkAZWIENVPreReqsAndCreateFiles() {
  if [[ -z "${SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH}" || -z "${SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH}" ]]; then
    export SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH="${REPO_ROOT}/capz-wi-sa.pub"
    export SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH="${REPO_ROOT}/capz-wi-sa.key"
  fi
  if [ -z "${SERVICE_ACCOUNT_ISSUER}" ]; then
    # check if user is logged into azure cli
    if ! az account show > /dev/null 2>&1; then
        echo "Please login to Azure CLI using 'az login'"
        exit 1
    fi

    if [ -z "${AZWI_RESOURCE_GROUP}" ]; then
      echo "AZWI_RESOURCE_GROUP environment variable required - Azure resource group to store required Workload Identity artifacts"
      exit 1
    fi

    if [ "$(az group exists --name "${AZWI_RESOURCE_GROUP}" --output tsv)" == 'false' ]; then
      echo "Creating resource group '${AZWI_RESOURCE_GROUP}' in '${AZWI_LOCATION}'"
      az group create --name "${AZWI_RESOURCE_GROUP}" --location "${AZWI_LOCATION}" --output none --only-show-errors --tags creationTimestamp="${TIMESTAMP}" jobName="${JOB_NAME}" buildProvenance="${BUILD_PROVENANCE}"
    fi

    # Ensure that our connection to storage is inherited from the existing Azure login context
    unset AZURE_STORAGE_KEY
    unset AZURE_STORAGE_ACCOUNT

    if ! az storage account show --name "${AZWI_STORAGE_ACCOUNT}" --resource-group "${AZWI_RESOURCE_GROUP}" > /dev/null 2>&1; then
      echo "Creating storage account '${AZWI_STORAGE_ACCOUNT}' in '${AZWI_RESOURCE_GROUP}'"
      az storage account create --resource-group "${AZWI_RESOURCE_GROUP}" --name "${AZWI_STORAGE_ACCOUNT}" --allow-shared-key-access=false --output none --only-show-errors --tags creationTimestamp="${TIMESTAMP}" jobName="${JOB_NAME}" buildProvenance="${BUILD_PROVENANCE}"
      until az storage account show --name "${AZWI_STORAGE_ACCOUNT}" --resource-group "${AZWI_RESOURCE_GROUP}" > /dev/null 2>&1; do
        sleep 5
      done
      echo "Configuring storage account '${AZWI_STORAGE_ACCOUNT}' as static website"
      az storage blob service-properties update --account-name "${AZWI_STORAGE_ACCOUNT}" --static-website --auth-mode login
    fi

    if ! az storage container show --name "${AZWI_STORAGE_CONTAINER}" --account-name "${AZWI_STORAGE_ACCOUNT}" --auth-mode login > /dev/null 2>&1; then
      echo "Creating storage container '${AZWI_STORAGE_CONTAINER}' in '${AZWI_STORAGE_ACCOUNT}'"
      az storage container create --name "${AZWI_STORAGE_CONTAINER}" --account-name "${AZWI_STORAGE_ACCOUNT}" --output none --only-show-errors --auth-mode login
    fi

    SERVICE_ACCOUNT_ISSUER=$(az storage account show --name "${AZWI_STORAGE_ACCOUNT}" --resource-group "${AZWI_RESOURCE_GROUP}" -o json | jq -r .primaryEndpoints.web)
    export SERVICE_ACCOUNT_ISSUER
    AZWI_OPENID_CONFIG_FILEPATH="${REPO_ROOT}/openid-configuration.json"
    cat <<EOF > "${AZWI_OPENID_CONFIG_FILEPATH}"
{
  "issuer": "${SERVICE_ACCOUNT_ISSUER}",
  "jwks_uri": "${SERVICE_ACCOUNT_ISSUER}openid/v1/jwks",
  "response_types_supported": [
    "id_token"
  ],
  "subject_types_supported": [
    "public"
  ],
  "id_token_signing_alg_values_supported": [
    "RS256"
  ]
}
EOF
    openssl genrsa -out "${SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH}" 2048
    openssl rsa -in "${SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH}" -pubout -out "${SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH}"
    AZWI_JWKS_JSON_FILEPATH="${REPO_ROOT}/jwks.json"
    "${AZWI}" jwks --public-keys "${SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH}" --output-file "${AZWI_JWKS_JSON_FILEPATH}"

    echo "Uploading openid-configuration document to '${AZWI_STORAGE_ACCOUNT}' storage account"
    upload_to_blob "${AZWI_OPENID_CONFIG_FILEPATH}" ".well-known/openid-configuration"

    echo "Uploading jwks document to '${AZWI_STORAGE_ACCOUNT}' storage account"
    upload_to_blob "${AZWI_JWKS_JSON_FILEPATH}" "openid/v1/jwks"
  fi

  if [ -z "${AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY}" ]; then
    if [ -z "${USER_IDENTITY}" ]; then
        echo "USER_IDENTITY environment variable required if not bringing your own identity via AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY"
        exit 1
    fi

    az identity create -n "${USER_IDENTITY}" -g "${AZWI_RESOURCE_GROUP}" -l "${AZWI_LOCATION}" --output none --only-show-errors --tags creationTimestamp="${TIMESTAMP}" jobName="${JOB_NAME}" buildProvenance="${BUILD_PROVENANCE}"
    AZURE_IDENTITY_ID=$(az identity show -n "${USER_IDENTITY}" -g "${AZWI_RESOURCE_GROUP}" --query clientId -o tsv)
    AZURE_IDENTITY_ID_PRINCIPAL_ID=$(az identity show -n "${USER_IDENTITY}" -g "${AZWI_RESOURCE_GROUP}" --query principalId -o tsv)

    echo "${AZURE_IDENTITY_ID}" > "${AZURE_IDENTITY_ID_FILEPATH}"
    until az role assignment create --assignee-object-id "${AZURE_IDENTITY_ID_PRINCIPAL_ID}" --role "Contributor" --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}" --assignee-principal-type ServicePrincipal; do
      sleep 5
    done
    until az role assignment create --assignee-object-id "${AZURE_IDENTITY_ID_PRINCIPAL_ID}" --role "Role Based Access Control Administrator" --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}" --assignee-principal-type ServicePrincipal; do
      sleep 5
    done
    until az role assignment create --assignee-object-id "${AZURE_IDENTITY_ID_PRINCIPAL_ID}" --role "Storage Blob Data Reader" --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}" --assignee-principal-type ServicePrincipal; do
      sleep 5
    done

    echo "Creating federated credentials for capz-federated-identity"
    az identity federated-credential create -n "capz-federated-identity" \
      --identity-name "${USER_IDENTITY}" \
      -g "${AZWI_RESOURCE_GROUP}" \
      --issuer "${SERVICE_ACCOUNT_ISSUER}" \
      --audiences "api://AzureADTokenExchange" \
      --subject "system:serviceaccount:capz-system:capz-manager" --output none --only-show-errors

    echo "Creating federated credentials for aso-federated-identity"
    az identity federated-credential create -n "aso-federated-identity" \
      --identity-name "${USER_IDENTITY}" \
      -g "${AZWI_RESOURCE_GROUP}" \
      --issuer "${SERVICE_ACCOUNT_ISSUER}" \
      --audiences "api://AzureADTokenExchange" \
      --subject "system:serviceaccount:capz-system:azureserviceoperator-default" --output none --only-show-errors
  fi
}

function upload_to_blob() {
  local file_path=$1
  local blob_name=$2

  echo "Uploading ${file_path} to '${AZWI_STORAGE_ACCOUNT}' storage account"
  az storage blob upload \
      --container-name "${AZWI_STORAGE_CONTAINER}" \
      --file "${file_path}" \
      --name "${blob_name}" \
      --account-name "${AZWI_STORAGE_ACCOUNT}" \
      --output none --only-show-errors \
      --auth-mode login
}

# This function create a kind cluster for Workload identity which requires key pairs path
# to be mounted on the kind cluster and hence extra mount flags are required.
function createKindForAZWI() {
  echo "creating workload-identity-enabled kind configuration"
  cat <<EOF | "${KIND}" create cluster --name "${KIND_CLUSTER_NAME}" --config=-
  kind: Cluster
  apiVersion: kind.x-k8s.io/v1alpha4
  nodes:
  - role: control-plane
    extraMounts:
      - hostPath: "${SERVICE_ACCOUNT_SIGNING_PUB_FILEPATH}"
        containerPath: /etc/kubernetes/pki/sa.pub
      - hostPath: "${SERVICE_ACCOUNT_SIGNING_KEY_FILEPATH}"
        containerPath: /etc/kubernetes/pki/sa.key
    kubeadmConfigPatches:
    - |
      kind: ClusterConfiguration
      apiServer:
        extraArgs:
          service-account-issuer: ${SERVICE_ACCOUNT_ISSUER}
          service-account-key-file: /etc/kubernetes/pki/sa.pub
          service-account-signing-key-file: /etc/kubernetes/pki/sa.key
      controllerManager:
        extraArgs:
          service-account-private-key-file: /etc/kubernetes/pki/sa.key
  containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry]
       config_path = "/etc/containerd/certs.d"
EOF
}

# 2. Create kind cluster with containerd registry config dir enabled
# TODO: kind will eventually enable this by default and this patch will
# be unnecessary.
#
# See:
# https://github.com/kubernetes-sigs/kind/issues/2875
# https://github.com/containerd/containerd/blob/main/docs/cri/config.md#registry-configuration
# See: https://github.com/containerd/containerd/blob/main/docs/hosts.md
if [ "$AZWI_ENABLED" == 'true' ]
 then
   echo "workload-identity is enabled..."
   checkAZWIENVPreReqsAndCreateFiles
   createKindForAZWI
else
  echo "workload-identity is not enabled..."
 cat <<EOF | ${KIND} create cluster --name "${KIND_CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
EOF
fi

# 3. Add the registry config to the nodes
#
# This is necessary because localhost resolves to loopback addresses that are
# network-namespace local.
# In other words: localhost in the container is not localhost on the host.
#
# We want a consistent name that works from both ends, so we tell containerd to
# alias localhost:${reg_port} to the registry container when pulling images
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${reg_port}"
for node in $(${KIND} get nodes); do
  docker exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | docker exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${reg_name}:5000"]
EOF
done

# 4. Connect the registry to the cluster network if not already connected
# This allows kind to bootstrap the network but ensures they're on the same network
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
  docker network connect "kind" "${reg_name}"
fi

# 5. Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
"${KIND}" get kubeconfig -n "${KIND_CLUSTER_NAME}" > "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig"
cat <<EOF | "${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

# Wait 90s for the control plane node to be ready
"${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" wait node "${KIND_CLUSTER_NAME}-control-plane" --for=condition=ready --timeout=90s
