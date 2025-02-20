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

: "${AZURE_STORAGE_ACCOUNT:?Environment variable empty or not defined.}"
: "${REGISTRY:?Environment variable empty or not defined.}"

# cloud controller manager image
export CCM_IMAGE_NAME=azure-cloud-controller-manager
# cloud node manager image
export CNM_IMAGE_NAME=azure-cloud-node-manager
# container name
export AZURE_BLOB_CONTAINER_NAME="${AZURE_BLOB_CONTAINER_NAME:-"kubernetes-ci"}"

setup() {
    AZURE_CLOUD_PROVIDER_ROOT="${AZURE_CLOUD_PROVIDER_ROOT:-""}"
    if [[ -z "${AZURE_CLOUD_PROVIDER_ROOT}" ]]; then
        AZURE_CLOUD_PROVIDER_ROOT="$(go env GOPATH)/src/sigs.k8s.io/cloud-provider-azure"
        export AZURE_CLOUD_PROVIDER_ROOT
    fi

    # the azure-cloud-provider repo expects IMAGE_REGISTRY.
    export IMAGE_REGISTRY=${REGISTRY}
    pushd "${AZURE_CLOUD_PROVIDER_ROOT}" && IMAGE_TAG=$(git rev-parse --short=7 HEAD) &&
      IMAGE_TAG_CCM="${IMAGE_TAG_CCM:-${IMAGE_TAG}}" && IMAGE_TAG_CNM="${IMAGE_TAG_CNM:-${IMAGE_TAG}}" &&
      export IMAGE_TAG_CCM && export IMAGE_TAG_CNM && popd
    echo "Image registry is ${REGISTRY}"
    echo "Image Tag CCM is ${IMAGE_TAG_CCM}"
    echo "Image Tag CNM is ${IMAGE_TAG_CNM}"
    IMAGE_TAG_ACR_CREDENTIAL_PROVIDER="${IMAGE_TAG_ACR_CREDENTIAL_PROVIDER:-${IMAGE_TAG}}"
    export IMAGE_TAG_ACR_CREDENTIAL_PROVIDER
    echo "Image Tag ACR credential provider is ${IMAGE_TAG_ACR_CREDENTIAL_PROVIDER}"
}

main() {
    if ! can_reuse_artifacts; then
        echo "Building Linux Azure amd64 cloud controller manager"
        make -C "${AZURE_CLOUD_PROVIDER_ROOT}" ALL_ARCH.linux=amd64 push-manifest-controller-manager
        echo "Building Linux amd64 and Windows (hpc) amd64 cloud node managers"
        make -C "${AZURE_CLOUD_PROVIDER_ROOT}" WINDOWS_USE_HOST_PROCESS_CONTAINERS=true ALL_ARCH.linux=amd64 push-manifest-node-manager

        echo "Building and pushing Linux and Windows amd64 Azure ACR credential provider"
        make -C "${AZURE_CLOUD_PROVIDER_ROOT}" bin/azure-acr-credential-provider bin/azure-acr-credential-provider.exe

        if [[ "$(az storage container exists --name "${AZURE_BLOB_CONTAINER_NAME}" --query exists --output tsv --auth-mode login)" == "false" ]]; then
            echo "Creating ${AZURE_BLOB_CONTAINER_NAME} storage container"
            az storage container create --name "${AZURE_BLOB_CONTAINER_NAME}" --auth-mode login > /dev/null
            # if the storage account has public access disabled at the account level this will return 404
            AZURE_STORAGE_AUTH_MODE=login az storage container set-permission --name "${AZURE_BLOB_CONTAINER_NAME}" --public-access container > /dev/null
        fi

        az storage blob upload --overwrite --container-name "${AZURE_BLOB_CONTAINER_NAME}" --file "${AZURE_CLOUD_PROVIDER_ROOT}/bin/azure-acr-credential-provider" --name "${IMAGE_TAG_ACR_CREDENTIAL_PROVIDER}/azure-acr-credential-provider" --auth-mode login
        az storage blob upload --overwrite --container-name "${AZURE_BLOB_CONTAINER_NAME}" --file "${AZURE_CLOUD_PROVIDER_ROOT}/bin/azure-acr-credential-provider.exe" --name "${IMAGE_TAG_ACR_CREDENTIAL_PROVIDER}/azure-acr-credential-provider.exe" --auth-mode login
        az storage blob upload --overwrite --container-name "${AZURE_BLOB_CONTAINER_NAME}" --file "${AZURE_CLOUD_PROVIDER_ROOT}/examples/out-of-tree/credential-provider-config.yaml" --name "${IMAGE_TAG_ACR_CREDENTIAL_PROVIDER}/credential-provider-config.yaml" --auth-mode login
        az storage blob upload --overwrite --container-name "${AZURE_BLOB_CONTAINER_NAME}" --file "${AZURE_CLOUD_PROVIDER_ROOT}/examples/out-of-tree/credential-provider-config-win.yaml" --name "${IMAGE_TAG_ACR_CREDENTIAL_PROVIDER}/credential-provider-config-win.yaml" --auth-mode login
    fi
}

# can_reuse_artifacts returns true if there exists CCM artifacts built from a PR that we can reuse
can_reuse_artifacts() {
    declare -a IMAGES=("${CCM_IMAGE_NAME}:${IMAGE_TAG_CCM}" "${CNM_IMAGE_NAME}:${IMAGE_TAG_CNM}")
    for IMAGE in "${IMAGES[@]}"; do
        if ! docker manifest inspect "${REGISTRY}/${IMAGE}" >/dev/null; then
            return 1
        fi
    done

    if ! docker manifest inspect "${REGISTRY}/${CNM_IMAGE_NAME}:${IMAGE_TAG_CNM}" | grep -q "\"os\": \"windows\""; then
        return 1
    fi

    # Do not reuse the image if there is a Windows image built with older version of this script that did not
    # build the images as host-process-container images. Those images cannot be pulled on mis-matched Windows Server versions.
    if docker manifest inspect "${REGISTRY}/${CNM_IMAGE_NAME}:${IMAGE_TAG_CNM}" | grep -q "\"os.version\": \"10.0."; then
        return 1
    fi

    for BINARY in azure-acr-credential-provider azure-acr-credential-provider.exe credential-provider-config.yaml credential-provider-config-win.yaml; do
        if [[ "$(az storage blob exists --container-name "${AZURE_BLOB_CONTAINER_NAME}" --name "${IMAGE_TAG_ACR_CREDENTIAL_PROVIDER}/${BINARY}" --query exists --output tsv --auth-mode login)" == "false" ]]; then
        return 1
        fi
    done
}

capz::ci-build-azure-ccm::cleanup() {
    echo "cloud-provider-azure cleanup"
    if [[ -d "${AZURE_CLOUD_PROVIDER_ROOT:-}" ]]; then
        make -C "${AZURE_CLOUD_PROVIDER_ROOT}" clean || true
    fi
}

trap capz::ci-build-azure-ccm::cleanup EXIT

setup
main
