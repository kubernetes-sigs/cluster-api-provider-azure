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

# shellcheck source=hack/ensure-azcli.sh
source "${REPO_ROOT}/hack/ensure-azcli.sh"
# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"

: "${AZURE_STORAGE_ACCOUNT:?Environment variable empty or not defined.}"
: "${AZURE_STORAGE_KEY:?Environment variable empty or not defined.}"
: "${REGISTRY:?Environment variable empty or not defined.}"
# JOB_NAME is an enviornment variable set by a prow job -
# https://github.com/kubernetes/test-infra/blob/master/prow/jobs.md#job-environment-variables
: "${JOB_NAME:?Environment variable empty or not defined.}"

declare -a BINARIES=("kubeadm" "kubectl" "kubelet")
declare -a IMAGES=("kube-apiserver" "kube-controller-manager" "kube-proxy" "kube-scheduler")

setup() {
    KUBE_ROOT="$(go env GOPATH)/src/k8s.io/kubernetes"
    export KUBE_ROOT

    # shellcheck disable=SC1091
    # extract KUBE_GIT_VERSION from k/k
    # ref: https://github.com/kubernetes/test-infra/blob/de07aa4b89f1161778856dc0fed310bd816aad72/experiment/kind-conformance-image-e2e.sh#L112-L115
    source "${KUBE_ROOT}/hack/lib/version.sh"
    pushd "${KUBE_ROOT}" && kube::version::get_version_vars && popd
    : "${KUBE_GIT_VERSION:?Environment variable empty or not defined.}"
    export KUBE_GIT_VERSION

    # get the latest ci version for a particular release so that kubeadm is
    # able to pull existing images before being replaced by custom images
    major="$(echo "${KUBE_GIT_VERSION}" | grep -Po "(?<=v)[0-9]+")"
    minor="$(echo "${KUBE_GIT_VERSION}" | grep -Po "(?<=v${major}.)[0-9]+")"
    CI_VERSION="$(capz::util::get_latest_ci_version "${major}.${minor}")"
    export CI_VERSION
    export KUBERNETES_VERSION="${CI_VERSION}"

    # Docker tags cannot contain '+'
    # ref: https://github.com/kubernetes/kubernetes/blob/5491484aa91fd09a01a68042e7674bc24d42687a/build/lib/release.sh#L345-L346
    export IMAGE_TAG="${KUBE_GIT_VERSION/+/_}"
}

main() {
    if [[ "$(az storage container exists --name "${JOB_NAME}" --query exists)" == "false" ]]; then
        echo "Creating ${JOB_NAME} storage container"
        az storage container create --name "${JOB_NAME}" > /dev/null
        az storage container set-permission --name "${JOB_NAME}" --public-access container > /dev/null
    fi

    if [[ "${KUBE_BUILD_CONFORMANCE:-}" =~ [yY] ]]; then
        IMAGES+=("conformance")
        # consume by the conformance test suite
        export CONFORMANCE_IMAGE="${REGISTRY}/conformance:${IMAGE_TAG}"
    fi

    if [[ "$(can_reuse_artifacts)" == "false" ]]; then
        echo "Building Kubernetes"

        # TODO(chewong): support multi-arch and Windows build
        make -C "${KUBE_ROOT}" quick-release

        if [[ "${KUBE_BUILD_CONFORMANCE:-}" =~ [yY] ]]; then
            # rename conformance image since it is the only image that has an amd64 suffix
            mv "${KUBE_ROOT}"/_output/release-images/amd64/conformance-amd64.tar "${KUBE_ROOT}"/_output/release-images/amd64/conformance.tar
        fi

        for IMAGE_NAME in "${IMAGES[@]}"; do
            # extract docker image URL form `docker load` output
            OLD_IMAGE_URL="$(docker load --input "${KUBE_ROOT}/_output/release-images/amd64/${IMAGE_NAME}.tar" | grep -oP '(?<=Loaded image: )[^ ]*')"
            NEW_IMAGE_URL="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
            # retag and push images to ACR
            docker tag "${OLD_IMAGE_URL}" "${NEW_IMAGE_URL}" && docker push "${NEW_IMAGE_URL}"
        done

        for BINARY in "${BINARIES[@]}"; do
            az storage blob upload --container-name "${JOB_NAME}" --file "${KUBE_ROOT}/_output/dockerized/bin/linux/amd64/${BINARY}" --name "${KUBE_GIT_VERSION}/bin/linux/amd64/${BINARY}"
        done
    fi
}

# can_reuse_artifacts returns true if there exists Kubernetes artifacts built from a PR that we can reuse
can_reuse_artifacts() {
    for IMAGE_NAME in "${IMAGES[@]}"; do
        if ! docker pull "${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"; then
            echo "false" && return
        fi
    done

    for BINARY in "${BINARIES[@]}"; do
        if [[ "$(az storage blob exists --container-name "${JOB_NAME}" --name "${KUBE_GIT_VERSION}/bin/linux/amd64/${BINARY}" --query exists)" == "false" ]]; then
            echo "false" && return
        fi
    done

    echo "true"
}

cleanup() {
    if [[ -d "${KUBE_ROOT:-}" ]]; then
        make -C "${KUBE_ROOT}" clean || true
    fi
}

trap cleanup EXIT

setup
main
