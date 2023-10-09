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

# Install kubectl, helm and kustomize
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
HELM="${REPO_ROOT}/hack/tools/bin/helm"
KIND="${REPO_ROOT}/hack/tools/bin/kind"
KUSTOMIZE="${REPO_ROOT}/hack/tools/bin/kustomize"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${HELM##*/}" "${KIND##*/}" "${KUSTOMIZE##*/}"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capz}"
export KIND_CLUSTER_NAME
# export the variables so they are available in bash -c wait_for_nodes below
export KUBECTL
export HELM

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"
# shellcheck source=hack/util.sh
source "${REPO_ROOT}/hack/util.sh"

setup() {
    if [[ -n "${KUBERNETES_VERSION:-}" ]] && [[ -n "${CI_VERSION:-}" ]]; then
        echo "You may not set both \$KUBERNETES_VERSION and \$CI_VERSION, use one or the other to configure the version/build of Kubernetes to use"
        exit 1
    fi
    # setup REGISTRY for custom images.
    : "${REGISTRY:?Environment variable empty or not defined.}"
    "${REPO_ROOT}/hack/ensure-acr-login.sh"
    if [[ "$(capz::util::should_build_ccm)" == "true" ]]; then
        # shellcheck source=scripts/ci-build-azure-ccm.sh
        source "${REPO_ROOT}/scripts/ci-build-azure-ccm.sh"
        echo "Will use the ${IMAGE_REGISTRY}/${CCM_IMAGE_NAME}:${IMAGE_TAG_CCM} cloud-controller-manager image for external cloud-provider-cluster"
        echo "Will use the ${IMAGE_REGISTRY}/${CNM_IMAGE_NAME}:${IMAGE_TAG_CNM} cloud-node-manager image for external cloud-provider-azure cluster"

        CCM_IMG_ARGS=(--set cloudControllerManager.imageRepository="${IMAGE_REGISTRY}"
        --set cloudNodeManager.imageRepository="${IMAGE_REGISTRY}"
        --set cloudControllerManager.imageName="${CCM_IMAGE_NAME}"
        --set cloudNodeManager.imageName="${CNM_IMAGE_NAME}"
        --set-string cloudControllerManager.imageTag="${IMAGE_TAG_CCM}"
        --set-string cloudNodeManager.imageTag="${IMAGE_TAG_CNM}")
    fi

    if [[ "$(capz::util::should_build_kubernetes)" == "true" ]]; then
        # shellcheck source=scripts/ci-build-kubernetes.sh
        source "${REPO_ROOT}/scripts/ci-build-kubernetes.sh"
    fi

    if [[ "${KUBERNETES_VERSION:-}" =~ "latest" ]]; then
        CI_VERSION_URL="https://dl.k8s.io/ci/${KUBERNETES_VERSION}.txt"
        export CI_VERSION="${CI_VERSION:-$(curl --retry 3 -sSL "${CI_VERSION_URL}")}"
    fi
    if [[ -n "${CI_VERSION:-}" ]]; then
        echo "Using CI_VERSION ${CI_VERSION}"
        export KUBERNETES_VERSION="${CI_VERSION}"
    fi
    echo "Using KUBERNETES_VERSION ${KUBERNETES_VERSION:-}"

    if [[ -z "${CLUSTER_TEMPLATE:-}" ]]; then
        select_cluster_template
    fi
    echo "Using cluster template: ${CLUSTER_TEMPLATE}"

    export CLUSTER_NAME="${CLUSTER_NAME:-capz-$(
        head /dev/urandom | LC_ALL=C tr -dc a-z0-9 | head -c 6
        echo ''
    )}"
    export AZURE_RESOURCE_GROUP="${CLUSTER_NAME}"
    export AZURE_LOCATION="${AZURE_LOCATION:-$(capz::util::get_random_region)}"
    echo "Using AZURE_LOCATION: ${AZURE_LOCATION}"
    export AZURE_LOCATION_GPU="${AZURE_LOCATION_GPU:-$(capz::util::get_random_region_gpu)}"
    echo "Using AZURE_LOCATION_GPU: ${AZURE_LOCATION_GPU}"
    export AZURE_LOCATION_EDGEZONE="${AZURE_LOCATION_EDGEZONE:-$(capz::util::get_random_region_edgezone)}"
    echo "Using AZURE_LOCATION_EDGEZONE: ${AZURE_LOCATION_EDGEZONE}"
    # Need a cluster with at least 2 nodes
    export CONTROL_PLANE_MACHINE_COUNT="${CONTROL_PLANE_MACHINE_COUNT:-1}"
    export CCM_COUNT="${CCM_COUNT:-1}"
    export WORKER_MACHINE_COUNT="${WORKER_MACHINE_COUNT:-2}"
    export EXP_CLUSTER_RESOURCE_SET="true"

    # TODO figure out a better way to account for expected Windows node count
    if [[ -n "${TEST_WINDOWS:-}" ]]; then
        export WINDOWS_WORKER_MACHINE_COUNT="${WINDOWS_WORKER_MACHINE_COUNT:-2}"
    fi
}

select_cluster_template() {
    if [[ "$(capz::util::should_build_kubernetes)" == "true" ]]; then
        export CLUSTER_TEMPLATE="test/dev/cluster-template-custom-builds.yaml"
    elif [[ -n "${CI_VERSION:-}" ]]; then
        # export cluster template which contains the manifests needed for creating the Azure cluster to run the tests
        export CLUSTER_TEMPLATE="test/ci/cluster-template-prow-ci-version.yaml"
    else
        export CLUSTER_TEMPLATE="test/ci/cluster-template-prow.yaml"
    fi

    if [[ "${EXP_MACHINE_POOL:-}" == "true" ]]; then
        if [[ "${CLUSTER_TEMPLATE}" =~ "prow" ]]; then
            export CLUSTER_TEMPLATE="${CLUSTER_TEMPLATE/prow/prow-machine-pool}"
        elif [[ "${CLUSTER_TEMPLATE}" =~ "custom-builds" ]]; then
            export CLUSTER_TEMPLATE="${CLUSTER_TEMPLATE/custom-builds/custom-builds-machine-pool}"
        fi
    fi
}

create_cluster() {
    "${REPO_ROOT}/hack/create-dev-cluster.sh"
    if [ ! -f "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" ]; then
        echo "Unable to find kubeconfig for kind mgmt cluster ${KIND_CLUSTER_NAME}"
        exit 1
    fi
}

# get_cidrs derives the CIDR from the Cluster's '.spec.clusterNetwork.pods.cidrBlocks' metadata
# any retryable operation in this function must return a non-zero exit code on failure so that we can
# retry it using a `until get_cidrs; do sleep 5; done` pattern;
# and any statement must be idempotent so that subsequent retry attempts can make forward progress.
get_cidrs() {
    # Get cluster CIDRs from Cluster object
    CIDR0=$(${KUBECTL} --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[0]}') || return 1
    export CIDR0
    CIDR_LENGTH=$(${KUBECTL} --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks}' | jq '. | length') || return 1
    if [[ "${CIDR_LENGTH}" == "2" ]]; then
        CIDR1=$(${KUBECTL} get cluster --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[1]}') || return 1
        export CIDR1
    fi
}

# get_cloud_provider determines if the Cluster is using an intree or external cloud-provider from the KubeadmConfigSpec.
# any retryable operation in this function must return a non-zero exit code on failure so that we can
# retry it using a `until get_cloud_provider; do sleep 5; done` pattern;
# and any statement must be idempotent so that subsequent retry attempts can make forward progress.
get_cloud_provider() {
    CLOUD_PROVIDER=$("${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" get kubeadmcontrolplane -l cluster.x-k8s.io/cluster-name="${CLUSTER_NAME}" -o=jsonpath='{.items[0].spec.kubeadmConfigSpec.clusterConfiguration.controllerManager.extraArgs.cloud-provider}') || return 1
    if [[ "${CLOUD_PROVIDER:-}" = "azure" ]]; then
        IN_TREE="true"
        export IN_TREE
    fi
}

# copy_kubeadm_config_map copies the kubeadm configmap into the calico-system namespace.
# any retryable operation in this function must return a non-zero exit code on failure so that we can
# retry it using a `until copy_kubeadm_config_map; do sleep 5; done` pattern;
# and any statement must be idempotent so that subsequent retry attempts can make forward progress.
copy_kubeadm_config_map() {
    # Copy the kubeadm configmap to the calico-system namespace.
    # This is a workaround needed for the calico-node-windows daemonset
    # to be able to run in the calico-system namespace.
    # First, validate that the kubeadm-config configmap has been created.
    "${KUBECTL}" get configmap kubeadm-config --namespace=kube-system -o yaml || return 1
    "${KUBECTL}" create namespace calico-system --dry-run=client -o yaml | kubectl apply -f - || return 1
    if ! "${KUBECTL}" get configmap kubeadm-config --namespace=calico-system; then
        "${KUBECTL}" get configmap kubeadm-config --namespace=kube-system -o yaml | sed 's/namespace: kube-system/namespace: calico-system/' | "${KUBECTL}" apply -f - || return 1
    fi
}

# install_cloud_provider_azure installs OOT cloud-provider-azure componentry onto the Cluster.
# Any retryable operation in this function must return a non-zero exit code on failure so that we can
# retry it using a `until install_cloud_provider_azure; do sleep 5; done` pattern;
# and any statement must be idempotent so that subsequent retry attempts can make forward progress.
install_cloud_provider_azure() {
    CLOUD_CONFIG="/etc/kubernetes/azure.json"
    CONFIG_SECRET_NAME=""
    ENABLE_DYNAMIC_RELOADING=false
    if [[ -n "${LOAD_CLOUD_CONFIG_FROM_SECRET:-}" ]]; then
        CLOUD_CONFIG=""
        CONFIG_SECRET_NAME="azure-cloud-provider"
        ENABLE_DYNAMIC_RELOADING=true
        copy_secret || return 1
    fi

    CCM_CLUSTER_CIDR="${CIDR0}"
    if [[ -n "${CIDR1:-}" ]]; then
        CCM_CLUSTER_CIDR="${CIDR0}\,${CIDR1}"
    fi
    echo "CCM cluster CIDR: ${CCM_CLUSTER_CIDR:-}"

    export CCM_LOG_VERBOSITY="${CCM_LOG_VERBOSITY:-4}"
    echo "Installing cloud-provider-azure components via helm"
    "${HELM}" upgrade cloud-provider-azure --install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure \
        --set infra.clusterName="${CLUSTER_NAME}" \
        --set cloudControllerManager.replicas="${CCM_COUNT}" \
        --set cloudControllerManager.enableDynamicReloading="${ENABLE_DYNAMIC_RELOADING}"  \
        --set cloudControllerManager.cloudConfig="${CLOUD_CONFIG}" \
        --set cloudControllerManager.cloudConfigSecretName="${CONFIG_SECRET_NAME}" \
        --set cloudControllerManager.logVerbosity="${CCM_LOG_VERBOSITY}" \
        --set-string cloudControllerManager.clusterCIDR="${CCM_CLUSTER_CIDR}" "${CCM_IMG_ARGS[@]}" || return 1
}

# wait_for_nodes returns when all nodes in the workload cluster are Ready.
wait_for_nodes() {
    echo "Waiting for ${CONTROL_PLANE_MACHINE_COUNT} control plane machine(s), ${WORKER_MACHINE_COUNT} worker machine(s), and ${WINDOWS_WORKER_MACHINE_COUNT:-0} windows machine(s) to become Ready"

    # Ensure that all nodes are registered with the API server before checking for readiness
    local total_nodes="$((CONTROL_PLANE_MACHINE_COUNT + WORKER_MACHINE_COUNT + WINDOWS_WORKER_MACHINE_COUNT))"
    while [[ $("${KUBECTL}" get nodes -ojson | jq '.items | length') -ne "${total_nodes}" ]]; do
        sleep 10
    done

    until "${KUBECTL}" wait --for=condition=Ready node --all --timeout=15m; do
        sleep 5
    done
    until "${KUBECTL}" get nodes -o wide; do
        sleep 5
    done
}

# wait_for_pods returns when all pods on the workload cluster are Running.
wait_for_pods() {
    echo "Waiting for all pod init containers scheduled in the cluster to be ready"
    while "${KUBECTL}" get pods --all-namespaces -o jsonpath="{.items[*].status.initContainerStatuses[*].ready}" | grep -q false; do
        echo "Not all pod init containers are Ready...."
        sleep 5
    done

    echo "Waiting for all pod containers scheduled in the cluster to be ready"
    while "${KUBECTL}" get pods --all-namespaces -o jsonpath="{.items[*].status.containerStatuses[*].ready}" | grep -q false; do
        echo "Not all pod containers are Ready...."
        sleep 5
    done
    until "${KUBECTL}" get pods --all-namespaces -o wide; do
        sleep 5
    done
}

install_addons() {
    until get_cidrs; do
        sleep 5
    done
    # export the target cluster KUBECONFIG if not already set
    export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"
    until copy_kubeadm_config_map; do
        sleep 5
    done
    # install cloud-provider-azure components, if using out-of-tree
    until get_cloud_provider; do
        sleep 5
    done
    if [[ -z "${IN_TREE:-}" ]]; then
        until install_cloud_provider_azure; do
            sleep 5
        done
    fi
    # In order to determine the successful outcome of CNI and cloud-provider-azure,
    # we need to wait a little bit for nodes and pods terminal state,
    # so we block successful return upon the cluster being fully operational.
    export -f wait_for_nodes
    timeout --foreground 1800 bash -c wait_for_nodes
    export -f wait_for_pods
    timeout --foreground 1800 bash -c wait_for_pods
}

copy_secret() {
    # point at the management cluster
    "${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" get secret "${CLUSTER_NAME}-control-plane-azure-json" -o jsonpath='{.data.control-plane-azure\.json}' | base64 --decode >azure_json || return 1

    # create the secret on the workload cluster
    "${KUBECTL}" create secret generic "${CONFIG_SECRET_NAME}" -n kube-system \
        --from-file=cloud-config=azure_json || return 1
    rm azure_json
}

capz::ci-entrypoint::on_exit() {
    if [[ -n ${KUBECONFIG:-} ]]; then
        "${KUBECTL}" get nodes -o wide || echo "Unable to get nodes"
        "${KUBECTL}" get pods -A -o wide || echo "Unable to get pods"
    fi
    # unset kubeconfig which is currently pointing at workload cluster.
    # we want to be pointing at the management cluster (kind in this case)
    unset KUBECONFIG
    go run -tags e2e "${REPO_ROOT}"/test/logger.go --name "${CLUSTER_NAME}" --namespace default
    "${REPO_ROOT}/hack/log/redact.sh" || true
    # cleanup all resources we use
    if [[ ! "${SKIP_CLEANUP:-}" == "true" ]]; then
        timeout 1800 "${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" delete cluster "${CLUSTER_NAME}" || echo "Unable to delete cluster ${CLUSTER_NAME}"
        make --directory="${REPO_ROOT}" kind-reset || true
    fi
}

# setup all required variables and images
setup

trap capz::ci-entrypoint::on_exit EXIT
export ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"

# create cluster
create_cluster

# install CNI and CCM
install_addons
echo "Cluster ${CLUSTER_NAME} created and fully operational"

if [[ "${#}" -gt 0 ]]; then
    # disable error exit so we can run post-command cleanup
    set +o errexit
    "${@}"
    EXIT_VALUE="${?}"
    exit ${EXIT_VALUE}
fi
