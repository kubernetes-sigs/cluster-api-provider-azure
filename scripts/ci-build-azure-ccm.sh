#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"

: "${REGISTRY:?Environment variable empty or not defined.}"

# cloud controller manager image
declare CCM_IMAGE_NAME=azure-cloud-controller-manager
# cloud node manager image
declare CNM_IMAGE_NAME=azure-cloud-node-manager
declare -a IMAGES=("${CCM_IMAGE_NAME}" "${CNM_IMAGE_NAME}")

setup() {
    AZURE_CLOUD_PROVIDER_ROOT="$(go env GOPATH)/src/sigs.k8s.io/cloud-provider-azure"
    export AZURE_CLOUD_PROVIDER_ROOT
    # the azure-cloud-provider repo expects IMAGE_REGISTRY.
    export IMAGE_REGISTRY=${REGISTRY}
    pushd "${AZURE_CLOUD_PROVIDER_ROOT}" && IMAGE_TAG=$(git rev-parse --short=7 HEAD) && export IMAGE_TAG && popd
    echo "Image Tag is ${IMAGE_TAG}"
    export AZURE_CLOUD_CONTROLLER_MANAGER_IMG=${IMAGE_REGISTRY}/${CCM_IMAGE_NAME}:${IMAGE_TAG}
    export AZURE_CLOUD_NODE_MANAGER_IMG=${IMAGE_REGISTRY}/${CNM_IMAGE_NAME}:${IMAGE_TAG}
}

main() {
    if [[ "$(can_reuse_artifacts)" == "false" ]]; then
        echo "Building Azure cloud controller manager and cloud node manager..."
        make -C "${AZURE_CLOUD_PROVIDER_ROOT}" image push
    fi
}

# can_reuse_artifacts returns true if there exists CCM artifacts built from a PR that we can reuse
can_reuse_artifacts() {
    for IMAGE_NAME in "${IMAGES[@]}"; do
        if ! docker pull "${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"; then
            echo "false" && return
        fi
    done

    echo "true"
}

cleanup() {
    if [[ -d "${AZURE_CLOUD_PROVIDER_ROOT:-}" ]]; then
        make -C "${AZURE_CLOUD_PROVIDER_ROOT}" clean || true
    fi
}

trap cleanup EXIT

setup
main
