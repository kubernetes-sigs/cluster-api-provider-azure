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

###############################################################################

# This script is executed by presubmit `pull-cluster-api-provider-azure-e2e`
# To run locally, set AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID, AZURE_TENANT_ID

set -o errexit
set -o nounset
set -o pipefail

# Install kubectl
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
KIND="${REPO_ROOT}/hack/tools/bin/kind"
KUSTOMIZE="${REPO_ROOT}/hack/tools/bin/kustomize"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${KIND##*/}" "${KUSTOMIZE##*/}"

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"
# shellcheck source=hack/util.sh
source "${REPO_ROOT}/hack/util.sh"

# Verify the required Environment Variables are present.
capz::util::ensure_azure_envs

export LOCAL_ONLY=${LOCAL_ONLY:-"true"}
export USE_LOCAL_KIND_REGISTRY=${USE_LOCAL_KIND_REGISTRY:-${LOCAL_ONLY}} 

if [[ "${USE_LOCAL_KIND_REGISTRY}" == "true" ]]; then
  export REGISTRY="localhost:5000/ci-e2e"
else
  : "${REGISTRY:?Environment variable empty or not defined.}"
  "${REPO_ROOT}"/hack/ensure-acr-login.sh
  if [[ "$(capz::util::should_build_kubernetes)" == "true" ]]; then
    export E2E_ARGS="-kubetest.use-pr-artifacts"
    export KUBE_BUILD_CONFORMANCE="y"
    source "${REPO_ROOT}/scripts/ci-build-kubernetes.sh"
  fi

  if [[ "$(capz::util::should_build_ccm)" == "true" ]]; then
    # shellcheck source=scripts/ci-build-azure-ccm.sh
    source "${REPO_ROOT}/scripts/ci-build-azure-ccm.sh"
    echo "Will use the ${IMAGE_REGISTRY}/${CCM_IMAGE_NAME}:${IMAGE_TAG} cloud-controller-manager image for external cloud-provider-cluster"
    echo "Will use the ${IMAGE_REGISTRY}/${CNM_IMAGE_NAME}:${IMAGE_TAG} cloud-node-manager image for external cloud-provider-azure cluster"
  fi
fi

defaultTag=$(date -u '+%Y%m%d%H%M%S')
export TAG="${defaultTag:-dev}"
export GINKGO_NODES=1

export AZURE_LOCATION="${AZURE_LOCATION:-$(capz::util::get_random_region)}"
export AZURE_LOCATION_GPU="${AZURE_LOCATION_GPU:-$(capz::util::get_random_region_gpu)}"
export AZURE_LOCATION_EDGEZONE="${AZURE_LOCATION_EDGEZONE:-$(capz::util::get_random_region_edgezone)}"
export AZURE_CONTROL_PLANE_MACHINE_TYPE="${AZURE_CONTROL_PLANE_MACHINE_TYPE:-"Standard_B2s"}"
export AZURE_NODE_MACHINE_TYPE="${AZURE_NODE_MACHINE_TYPE:-"Standard_B2s"}"
export WINDOWS="${WINDOWS:-false}"

# Generate SSH key.
capz::util::generate_ssh_key

cleanup() {
    "${REPO_ROOT}/hack/log/redact.sh" || true
}

trap cleanup EXIT

if [[ "${WINDOWS}" == "true" ]]; then
  make test-windows-upstream
else
  make test-conformance
fi
