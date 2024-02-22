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

# Install kubectl
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
KIND="${REPO_ROOT}/hack/tools/bin/kind"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${KIND##*/}"

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
export BUILD_MANAGER_IMAGE=${BUILD_MANAGER_IMAGE:-"true"}

if [[ "${USE_LOCAL_KIND_REGISTRY}" == "false" ]]; then
  : "${REGISTRY:?Environment variable empty or not defined.}"
  "${REPO_ROOT}/hack/ensure-acr-login.sh"
else
  export REGISTRY="localhost:5000/ci-e2e"
fi

if [[ "${BUILD_MANAGER_IMAGE}" == "true" ]]; then
  defaultTag=$(date -u '+%Y%m%d%H%M%S')
  export TAG="${defaultTag:-dev}"
fi

if [[ "$(capz::util::should_build_ccm)" == "true" ]]; then
  # shellcheck source=scripts/ci-build-azure-ccm.sh
  source "${REPO_ROOT}/scripts/ci-build-azure-ccm.sh"
  echo "Will use the ${IMAGE_REGISTRY}/${CCM_IMAGE_NAME}:${IMAGE_TAG_CCM} cloud-controller-manager image for external cloud-provider-cluster"
  echo "Will use the ${IMAGE_REGISTRY}/${CNM_IMAGE_NAME}:${IMAGE_TAG_CNM} cloud-node-manager image for external cloud-provider-azure cluster"
fi

if [[ "$(capz::util::should_build_cluster_autoscaler)" == "true" ]]; then
  # shellcheck source=scripts/ci-build-cluster-autoscaler.sh
  source "${REPO_ROOT}/scripts/ci-build-cluster-autoscaler.sh"
fi

export GINKGO_NODES=${GINKGO_NODES:-10}

export AZURE_LOCATION="${AZURE_LOCATION:-$(capz::util::get_random_region)}"
export AZURE_LOCATION_GPU="${AZURE_LOCATION_GPU:-$(capz::util::get_random_region_gpu)}"
export AZURE_LOCATION_EDGEZONE="${AZURE_LOCATION_EDGEZONE:-$(capz::util::get_random_region_edgezone)}"
export AZURE_CONTROL_PLANE_MACHINE_TYPE="${AZURE_CONTROL_PLANE_MACHINE_TYPE:-"Standard_B2s"}"
export AZURE_NODE_MACHINE_TYPE="${AZURE_NODE_MACHINE_TYPE:-"Standard_B2s"}"
CALICO_VERSION=$(make get-calico-version)
export CALICO_VERSION


capz::util::generate_ssh_key

capz::ci-e2e::cleanup() {
    "${REPO_ROOT}/hack/log/redact.sh" || true
}

trap capz::ci-e2e::cleanup EXIT
# Image is configured as `${CONTROLLER_IMG}-${ARCH}:${TAG}` where `CONTROLLER_IMG` is defaulted to `${REGISTRY}/${IMAGE_NAME}`.
if [[ "${BUILD_MANAGER_IMAGE}" == "false" ]]; then
  # Load an existing image, skip docker-build and docker-push.
  make test-e2e-skip-build-and-push
elif [[ "${USE_LOCAL_KIND_REGISTRY}" == "true" ]]; then
  # Build an image with kind local registry, skip docker-push. REGISTRY is set to `localhost:5000/ci-e2e`. TAG is set to `$(date -u '+%Y%m%d%H%M%S')`.
  make test-e2e-skip-push
else
  # Build an image and push to the registry. TAG is set to `$(date -u '+%Y%m%d%H%M%S')`.
  make test-e2e
fi
