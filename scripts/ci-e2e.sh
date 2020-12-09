#!/bin/bash

# Copyright 2019 The Kubernetes Authors.
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

###############################################################################

# This script is executed by presubmit `pull-cluster-api-provider-azure-e2e`
# To run locally, set AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID, AZURE_TENANT_ID

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=../hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=../hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=../hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# shellcheck source=../hack/ensure-kustomize.sh
source "${REPO_ROOT}/hack/ensure-kustomize.sh"
# shellcheck source=../hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"

# Verify the required Environment Variables are present.
: "${AZURE_SUBSCRIPTION_ID:?Environment variable empty or not defined.}"
: "${AZURE_TENANT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_SECRET:?Environment variable empty or not defined.}"

get_random_region() {
    local REGIONS=("eastus" "eastus2" "southcentralus" "westus2" "westeurope")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}

export LOCAL_ONLY=${LOCAL_ONLY:-"true"}

if [[ "${LOCAL_ONLY}" == "false" ]]; then
  export REGISTRY=${REGISTRY:-"capzci.azurecr.io/ci-e2e"}

  if [[ "${REGISTRY}" =~ azurecr\.io ]]; then
    # if we are using Azure Container Registry, login
    ./hack/ensure-azcli.sh
    az account set -s "${AZURE_SUBSCRIPTION_ID}"
    az acr login --name capzci
  fi
else
  export REGISTRY="localhost:5000/ci-e2e"
fi

defaultTag=$(date -u '+%Y%m%d%H%M%S')
export TAG="${defaultTag:-dev}"
export AZURE_ENVIRONMENT="AzurePublicCloud"
export GINKGO_NODES=3
export AZURE_SUBSCRIPTION_ID_B64="$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
export AZURE_TENANT_ID_B64="$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_ID_B64="$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_SECRET_B64="$(echo -n "$AZURE_CLIENT_SECRET" | base64 | tr -d '\n')"
export AZURE_LOCATION="${AZURE_LOCATION:-$(get_random_region)}"
export AZURE_CONTROL_PLANE_MACHINE_TYPE="${AZURE_CONTROL_PLANE_MACHINE_TYPE:-"Standard_D2s_v3"}"
export AZURE_NODE_MACHINE_TYPE="${AZURE_NODE_MACHINE_TYPE:-"Standard_D2s_v3"}"

# Generate SSH key.
AZURE_SSH_PUBLIC_KEY_FILE=${AZURE_SSH_PUBLIC_KEY_FILE:-""}
if [ -z "${AZURE_SSH_PUBLIC_KEY_FILE}" ]; then
    SSH_KEY_FILE=.sshkey
    rm -f "${SSH_KEY_FILE}" 2>/dev/null
    ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
    AZURE_SSH_PUBLIC_KEY_FILE="${SSH_KEY_FILE}.pub"
fi
export AZURE_SSH_PUBLIC_KEY_B64=$(cat "${AZURE_SSH_PUBLIC_KEY_FILE}" | base64 | tr -d '\r\n')

# timestamp is in RFC-3339 format to match kubetest
export TIMESTAMP="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
export JOB_NAME="${JOB_NAME:-"cluster-api-provider-azure-e2e"}"

cleanup() {
    ${REPO_ROOT}/hack/log/redact.sh || true
}

trap cleanup EXIT

if [[ "${LOCAL_ONLY}" == "true" ]]; then
  make test-e2e-local
else
  make test-e2e
fi
