#!/bin/bash
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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# shellcheck source=hack/util.sh
source "${REPO_ROOT}/hack/util.sh"

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capz}"
KIND="${KIND:-${REPO_ROOT}/hack/tools/bin/kind}"
KUBECTL="${KUBECTL:-${REPO_ROOT}/hack/tools/bin/kubectl}"

# Verify the required Environment Variables are present.
capz::util::ensure_azure_envs

make envsubst

export REGISTRY="${REGISTRY:-registry.local/fake}"

export CLUSTER_CREATE_ATTEMPTS="${CLUSTER_CREATE_ATTEMPTS:-3}"

# Cluster settings.
export CLUSTER_NAME="${CLUSTER_NAME:-capz-test}"
export AZURE_VNET_NAME=${CLUSTER_NAME}-vnet

# Azure settings.
export AZURE_LOCATION="${AZURE_LOCATION:-southcentralus}"
export AZURE_RESOURCE_GROUP=${CLUSTER_NAME}

AZURE_SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID:=}"
AZURE_TENANT_ID="${AZURE_TENANT_ID:=}"
AZURE_CLIENT_ID="${AZURE_CLIENT_ID:=}"

AZURE_SUBSCRIPTION_ID_B64="$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
AZURE_TENANT_ID_B64="$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
AZURE_CLIENT_ID_B64="$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"

export AZURE_SUBSCRIPTION_ID_B64 AZURE_TENANT_ID_B64 AZURE_CLIENT_ID_B64

# Machine settings.
export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-3}
export AZURE_CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-Standard_B2s}"
export AZURE_NODE_MACHINE_TYPE="${NODE_MACHINE_TYPE:-Standard_B2s}"
export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-2}
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.35.4}"
export CLUSTER_TEMPLATE="${CLUSTER_TEMPLATE:-cluster-template.yaml}"

# identity secret settings.
export CLUSTER_IDENTITY_NAME=${CLUSTER_IDENTITY_NAME:="cluster-identity"}
export ASO_CREDENTIAL_SECRET_NAME=${ASO_CREDENTIAL_SECRET_NAME:="aso-credentials"}

# Generate SSH key.
capz::util::generate_ssh_key

echo "================ DOCKER BUILD ==============="
PULL_POLICY=IfNotPresent make modules docker-build

# cleanup_previous_attempt deletes any workload cluster and Azure RG left behind
# by a previous create_cluster iteration, so retries start with a clean slate.
cleanup_previous_attempt() {
    local kind_kubeconfig="${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig"

    if [[ -x "${KIND}" ]] && [[ -r "${kind_kubeconfig}" ]] && \
       "${KIND}" get clusters 2>/dev/null | grep -qxF "${KIND_CLUSTER_NAME}"; then
        echo "================ DELETE WORKLOAD CLUSTER (graceful) ==============="
        if timeout 1200 "${KUBECTL}" --kubeconfig "${kind_kubeconfig}" \
            delete cluster "${CLUSTER_NAME}" -n default --wait=true --ignore-not-found=true; then
            return 0
        fi
        echo "Graceful cluster delete failed or timed out; falling back to az group delete"
    fi

    # Synchronous so attempt N+1 doesn't race a still-deleting RG.
    if command -v az >/dev/null 2>&1 \
        && [[ -n "${AZURE_RESOURCE_GROUP:-}" ]] \
        && [[ "${AZURE_RESOURCE_GROUP}" == "${CLUSTER_NAME}" ]] \
        && [[ "$(az group exists --name "${AZURE_RESOURCE_GROUP}" 2>/dev/null)" == "true" ]]; then
        echo "================ AZURE RG CLEANUP (fallback) ==============="
        timeout 1800 az group delete --name "${AZURE_RESOURCE_GROUP}" --yes \
            || echo "WARNING: az group delete failed; subsequent attempt may see leftover Azure resources"
    fi
}

create_cluster() {
    cleanup_previous_attempt

    echo "================ MAKE CLEAN ==============="
    make clean

    echo "================ KIND RESET ==============="
    make kind-reset

    echo "================ INSTALL TOOLS ==============="
    make install-tools

    echo "================ CREATE CLUSTER ==============="
    make create-cluster
}

retries=$CLUSTER_CREATE_ATTEMPTS
until create_cluster; do
  if ((--retries == 0)); then
    exit 1
  fi
done
