#!/bin/bash

# Copyright 2026 The Kubernetes Authors.
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

# This script builds and pushes the kuberay-operator Docker image from the
# marosset/kuberay workload-poc branch. It is sourced by ci-e2e.sh when
# running the NativeScheduling e2e tests.
#
# After sourcing, the following environment variables are exported:
#   KUBERAY_SOURCE_DIR          - path to the cloned kuberay repo
#   KUBERAY_OPERATOR_IMAGE_TAG  - tag applied to the built image

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1
REPO_ROOT="$(pwd)"

: "${REGISTRY:?Environment variable empty or not defined.}"

KUBERAY_SOURCE_REPO="${KUBERAY_SOURCE_REPO:-https://github.com/marosset/kuberay.git}"
KUBERAY_SOURCE_BRANCH="${KUBERAY_SOURCE_BRANCH:-workload-poc}"

# Clone kuberay source if not already present.
# Use an absolute path so it resolves correctly regardless of cwd (e.g., from Ginkgo test binaries).
KUBERAY_SOURCE_DIR="${KUBERAY_SOURCE_DIR:-${REPO_ROOT}/_kuberay-source}"
if [[ ! -d "${KUBERAY_SOURCE_DIR}" ]]; then
    echo "Cloning kuberay from ${KUBERAY_SOURCE_REPO} (branch: ${KUBERAY_SOURCE_BRANCH})"
    git clone --depth 1 --branch "${KUBERAY_SOURCE_BRANCH}" "${KUBERAY_SOURCE_REPO}" "${KUBERAY_SOURCE_DIR}"
else
    echo "Using existing kuberay source at ${KUBERAY_SOURCE_DIR}"
fi

# Determine the image tag from the kuberay source commit.
pushd "${KUBERAY_SOURCE_DIR}" > /dev/null
KUBERAY_OPERATOR_IMAGE_TAG="${KUBERAY_OPERATOR_IMAGE_TAG:-$(git rev-parse --short=7 HEAD)}"
popd > /dev/null

KUBERAY_OPERATOR_IMAGE="${REGISTRY}/kuberay-operator:${KUBERAY_OPERATOR_IMAGE_TAG}"

echo "Building kuberay-operator image: ${KUBERAY_OPERATOR_IMAGE}"
docker build -t "${KUBERAY_OPERATOR_IMAGE}" "${KUBERAY_SOURCE_DIR}/ray-operator"

echo "Pushing kuberay-operator image: ${KUBERAY_OPERATOR_IMAGE}"
docker push "${KUBERAY_OPERATOR_IMAGE}"

export KUBERAY_SOURCE_DIR
export KUBERAY_OPERATOR_IMAGE_TAG

echo "kuberay-operator image built and pushed successfully"
echo "  KUBERAY_SOURCE_DIR=${KUBERAY_SOURCE_DIR}"
echo "  KUBERAY_OPERATOR_IMAGE_TAG=${KUBERAY_OPERATOR_IMAGE_TAG}"
echo "  REGISTRY=${REGISTRY}"
