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

random-string() {
    cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w ${1:-32} | head -n 1
}

# generate manifests needed for creating the Azure cluster to run the tests
add_kustomize_patch() {
    # Enable the bits to inject a script that can pull newer versions of kubernetes
    if ! grep -i -wq "patchesStrategicMerge" "templates/kustomization.yaml"; then
        echo "patchesStrategicMerge:" >> "templates/kustomization.yaml"
    fi
    if ! grep -i -wq "kustomizeversions" "templates/kustomization.yaml"; then
        echo "- kustomizeversions.yaml" >> "templates/kustomization.yaml"
    fi
}

# build Kubernetes E2E binaries
build_k8s() {
    # possibly enable bazel build caching before building kubernetes
    if [[ "${BAZEL_REMOTE_CACHE_ENABLED:-false}" == "true" ]]; then
    create_bazel_cache_rcs.sh || true
    fi

    pushd "$(go env GOPATH)/src/k8s.io/kubernetes"

    # make sure we have e2e requirements
    bazel build //cmd/kubectl //test/e2e:e2e.test //vendor/github.com/onsi/ginkgo/ginkgo

    # ensure the e2e script will find our binaries ...
    mkdir -p "${PWD}/_output/bin/"
    cp -f "${PWD}/bazel-bin/test/e2e/e2e.test" "${PWD}/_output/bin/e2e.test"
    cp -f "${PWD}/bazel-bin/vendor/github.com/onsi/ginkgo/ginkgo/darwin_amd64_stripped/ginkgo" "${PWD}/_output/bin/ginkgo"
    export KUBECTL_PATH="$(dirname "$(find "${PWD}/bazel-bin/" -name kubectl -type f)")/kubectl"
    PATH="${KUBECTL_PATH}:${PATH}"
    export PATH

    # attempt to release some memory after building
    sync || true
    echo 1 > /proc/sys/vm/drop_caches || true

    popd
}

create_cluster() {
    export CLUSTER_NAME="capz-conformance-$(head /dev/urandom | LC_ALL=C tr -dc a-z0-9 | head -c 6 ; echo '')"
    # Conformance test suite needs a cluster with at least 2 nodes
    export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-3}
    export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-2}
    export CI_VERSION=${CI_VERSION:-$(curl -sSL https://dl.k8s.io/ci/k8s-master.txt)}
    export REGISTRY=conformance
    ${REPO_ROOT}/hack/create-dev-cluster.sh
}

run_tests() {
    # export the target cluster KUBECONFIG if not already set
    export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"
    # ginkgo regexes
    SKIP="${SKIP:-}"
    FOCUS="${FOCUS:-"\\[Conformance\\]"}"
    # if we set PARALLEL=true, skip serial tests set --ginkgo-parallel
    if [[ "${PARALLEL:-false}" == "true" ]]; then
        export GINKGO_PARALLEL=y
        if [[ -z "${SKIP}" ]]; then
            SKIP="\\[Serial\\]"
        else
            SKIP="\\[Serial\\]|${SKIP}"
        fi
    fi

    # get the number of worker nodes
    NUM_NODES="$(kubectl get nodes --kubeconfig="$KUBECONFIG" \
    -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.taints}{"\n"}{end}' \
    | grep -cv "node-role.kubernetes.io/master" )"

    # wait for all the nodes to be ready
    kubectl wait --for=condition=Ready node --kubeconfig="$KUBECONFIG" --all || true

    # setting this env prevents ginkg e2e from trying to run provider setup
    export KUBERNETES_CONFORMANCE_TEST="y"
    # run the tests
    (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && ./hack/ginkgo-e2e.sh \
    '--provider=skeleton' "--num-nodes=${NUM_NODES}" \
    "--ginkgo.focus=${FOCUS}" "--ginkgo.skip=${SKIP}" \
    "--report-dir=${ARTIFACTS}" '--disable-log-dump=true')

    unset KUBECONFIG
    unset KUBERNETES_CONFORMANCE_TEST
}

get_logs() {
    # TODO collect more logs https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/474 
    kubectl logs deploy/capz-controller-manager -n capz-system manager > "${ARTIFACTS}/logs/capz-manager.log" || true
}

# cleanup all resources we use
cleanup() {
    timeout 600 kubectl \
        delete cluster "${CLUSTER_NAME}" || true
        timeout 600 kubectl \
        wait --for=delete cluster/"${CLUSTER_NAME}" || true
    make kind-reset || true
    # clean up e2e.test symlink
    (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && rm -f _output/bin/e2e.test) || true
}

on_exit() {
    unset KUBECONFIG
    get_logs
    # cleanup
    if [[ -z "${SKIP_CLEANUP:-}" ]]; then
        cleanup
    fi
}

trap on_exit EXIT 
ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
mkdir -p "${ARTIFACTS}/logs"

# create cluster
if [[ -z "${SKIP_CREATE_CLUSTER:-}" ]]; then
    if [[ -n ${CI_VERSION:-} || -n ${USE_CI_ARTIFACTS:-} ]]; then
        add_kustomize_patch
    fi
    create_cluster
fi

# build k8s binaries and run conformance tests
if [[ -z "${SKIP_TESTS:-}" ]]; then
    build_k8s
    run_tests
fi
