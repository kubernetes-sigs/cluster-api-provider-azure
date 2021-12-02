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

# To run locally, set AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID, AZURE_TENANT_ID

set -o errexit
set -o nounset
set -o pipefail

# Install kubectl
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
cd "${REPO_ROOT}" && make "${KUBECTL##*/}"
# export the variable so it is available in bash -c wait_for_nodes below
export KUBECTL

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=hack/ensure-kustomize.sh
source "${REPO_ROOT}/hack/ensure-kustomize.sh"
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"
# shellcheck source=hack/util.sh
source "${REPO_ROOT}/hack/util.sh"

get_random_region() {
    local REGIONS=("eastus" "eastus2" "northcentralus" "northeurope" "uksouth" "westeurope" "westus2")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}

setup() {
    # setup REGISTRY for custom images.
    : "${REGISTRY:?Environment variable empty or not defined.}"
    "${REPO_ROOT}/hack/ensure-acr-login.sh"
    if [[ -z "${CLUSTER_TEMPLATE:-}" ]]; then
        select_cluster_template
    fi

    export CLUSTER_NAME="${CLUSTER_NAME:-capz-$(head /dev/urandom | LC_ALL=C tr -dc a-z0-9 | head -c 6 ; echo '')}"
    export AZURE_RESOURCE_GROUP="${CLUSTER_NAME}"
    export AZURE_LOCATION="${AZURE_LOCATION:-$(get_random_region)}"
    # Need a cluster with at least 2 nodes
    export CONTROL_PLANE_MACHINE_COUNT="${CONTROL_PLANE_MACHINE_COUNT:-1}"
    export WORKER_MACHINE_COUNT="${WORKER_MACHINE_COUNT:-2}"
    export EXP_CLUSTER_RESOURCE_SET="true"
}

select_cluster_template() {
    if [[ "$(capz::util::should_build_kubernetes)" == "true" ]]; then
        # shellcheck source=scripts/ci-build-kubernetes.sh
        source "${REPO_ROOT}/scripts/ci-build-kubernetes.sh"
        export CLUSTER_TEMPLATE="test/dev/cluster-template-custom-builds.yaml"
    elif [[ -n "${CI_VERSION:-}" ]] || [[ -n "${USE_CI_ARTIFACTS:-}" ]]; then
        # export cluster template which contains the manifests needed for creating the Azure cluster to run the tests
        GOPATH="$(go env GOPATH)"
        KUBERNETES_BRANCH="$(cd "${GOPATH}/src/k8s.io/kubernetes" && git rev-parse --abbrev-ref HEAD)"
        if [[ "${KUBERNETES_BRANCH:-}" =~ "release-" ]]; then
            CI_VERSION_URL="https://dl.k8s.io/ci/latest-${KUBERNETES_BRANCH/release-}.txt"
        else
            CI_VERSION_URL="https://dl.k8s.io/ci/latest.txt"
        fi
        export CLUSTER_TEMPLATE="test/ci/cluster-template-prow-ci-version.yaml"
        export CI_VERSION="${CI_VERSION:-$(curl -sSL ${CI_VERSION_URL})}"
        export KUBERNETES_VERSION="${CI_VERSION}"
    else
        export CLUSTER_TEMPLATE="test/ci/cluster-template-prow.yaml"
    fi

    if [[ -n "${TEST_CCM:-}" ]]; then
        export CLUSTER_TEMPLATE="test/ci/cluster-template-prow-external-cloud-provider.yaml"
        # shellcheck source=scripts/ci-build-azure-ccm.sh
        source "${REPO_ROOT}/scripts/ci-build-azure-ccm.sh"
        echo "Using CCM image ${AZURE_CLOUD_CONTROLLER_MANAGER_IMG} and CNM image ${AZURE_CLOUD_NODE_MANAGER_IMG} to build external cloud provider cluster"
    fi

    if [[ "${EXP_MACHINE_POOL:-}" == "true" ]]; then
        if [[ "${CLUSTER_TEMPLATE}" =~ "prow" ]]; then
            export CLUSTER_TEMPLATE="${CLUSTER_TEMPLATE/prow/prow-machine-pool}"
        elif [[ "${CLUSTER_TEMPLATE}" =~ "custom-builds" ]]; then
            export CLUSTER_TEMPLATE="${CLUSTER_TEMPLATE/custom-builds/custom-builds-machine-pool}"
        fi
    fi

    # this requires k8s 1.22+
    if [[ -n "${TEST_WINDOWS:-}" ]]; then
        export WINDOWS_WORKER_MACHINE_COUNT="${WINDOWS_WORKER_MACHINE_COUNT:-2}"
        export K8S_FEATURE_GATES="WindowsHostProcessContainers=true"
    fi
}

create_cluster() {
    "${REPO_ROOT}/hack/create-dev-cluster.sh"
}

wait_for_nodes() {
    echo "Waiting for ${CONTROL_PLANE_MACHINE_COUNT} control plane machine(s) and ${WORKER_MACHINE_COUNT} worker machine(s) to become Ready"

    # Ensure that all nodes are registered with the API server before checking for readiness
    local total_nodes="$((CONTROL_PLANE_MACHINE_COUNT + WORKER_MACHINE_COUNT))"
    while [[ $("${KUBECTL}" get nodes -ojson | jq '.items | length') -ne "${total_nodes}" ]]; do
        sleep 10
    done

    "${KUBECTL}" wait --for=condition=Ready node --all --timeout=5m
    "${KUBECTL}" get nodes -owide
}

# cleanup all resources we use
cleanup() {
    timeout 1800 "${KUBECTL}" delete cluster "${CLUSTER_NAME}" || true
    make kind-reset || true
}

on_exit() {
    unset KUBECONFIG
    "${REPO_ROOT}/hack/log/log-dump.sh" || true
    # cleanup
    if [[ -z "${SKIP_CLEANUP:-}" ]]; then
        cleanup
    fi
}

# setup all required variables and images
setup

trap on_exit EXIT
export ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"

# create cluster
create_cluster

# export the target cluster KUBECONFIG if not already set
export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"

export -f wait_for_nodes
timeout --foreground 1800 bash -c wait_for_nodes

if [[ "${#}" -gt 0 ]]; then
    # disable error exit so we can run post-command cleanup
    set +o errexit
    "${@}"
    EXIT_VALUE="${?}"
    exit ${EXIT_VALUE}
fi
