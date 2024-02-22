#!/bin/bash

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

# cluster-autoscaler image
export CLUSTER_AUTOSCALER_IMAGE_NAME=cluster-autoscaler-amd64

setup() {
    CLUSTER_AUTOSCALER_ROOT="${CLUSTER_AUTOSCALER_ROOT:-""}"
    if [[ -z "${CLUSTER_AUTOSCALER_ROOT}" ]]; then
        CLUSTER_AUTOSCALER_ROOT="$(go env GOPATH)/src/k8s.io/autoscaler/cluster-autoscaler"
        export CLUSTER_AUTOSCALER_ROOT
    fi

    # the azure-cloud-provider repo expects IMAGE_REGISTRY.
    export IMAGE_REGISTRY=${REGISTRY}
    pushd "${CLUSTER_AUTOSCALER_ROOT}" && TAG=$(git rev-parse --short=7 HEAD) &&
      export TAG && popd
    export CLUSTER_AUTOSCALER_IMAGE="${REGISTRY}/${CLUSTER_AUTOSCALER_IMAGE_NAME}:${TAG}"
    # We use CLUSTER_AUTOSCALER_IMAGE_REPO to pass to the helm install command
    export CLUSTER_AUTOSCALER_IMAGE_REPO="${REGISTRY}/${CLUSTER_AUTOSCALER_IMAGE_NAME}"
    # We use CLUSTER_AUTOSCALER_IMAGE_TAG to pass to the helm install command
    export CLUSTER_AUTOSCALER_IMAGE_TAG="${TAG}"
    echo "Image registry is ${REGISTRY}"
    echo "Image Tag is ${TAG}"
    echo "Image reference is ${CLUSTER_AUTOSCALER_IMAGE}"
}

main() {
    if [[ "$(can_reuse_artifacts)" =~ "false" ]]; then
        echo "Building Linux amd64 cluster-autoscaler"
        export GOFLAGS=-mod=mod
        make -C "${CLUSTER_AUTOSCALER_ROOT}" build
        make -C "${CLUSTER_AUTOSCALER_ROOT}" make-image
        docker push "${CLUSTER_AUTOSCALER_IMAGE}"
    fi
}

# can_reuse_artifacts returns true if there exists CCM artifacts built from a PR that we can reuse
can_reuse_artifacts() {
    if ! docker pull "${CLUSTER_AUTOSCALER_IMAGE}"; then
        echo "false" && return
    fi
    echo "true"
}

capz::ci-build-cluster-autoscaler::cleanup() {
    echo "cluster-autoscaler cleanup"
    if [[ -d "${CLUSTER_AUTOSCALER_ROOT:-}" ]]; then
        make -C "${CLUSTER_AUTOSCALER_ROOT}" clean || true
    fi
}

trap capz::ci-build-cluster-autoscaler::cleanup EXIT

setup
main
