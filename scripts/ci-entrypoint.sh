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
    if [[ -n "${TEST_CCM:-}" ]]; then
        # shellcheck source=scripts/ci-build-azure-ccm.sh
        source "${REPO_ROOT}/scripts/ci-build-azure-ccm.sh"
        echo "Will use the ${IMAGE_REGISTRY}/${CCM_IMAGE_NAME}:${IMAGE_TAG} cloud-controller-manager image for external cloud-provider-cluster"
        echo "Will use the ${IMAGE_REGISTRY}/${CNM_IMAGE_NAME}:${IMAGE_TAG} cloud-node-manager image for external cloud-provider-azure cluster"
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
    # Need a cluster with at least 2 nodes
    export CONTROL_PLANE_MACHINE_COUNT="${CONTROL_PLANE_MACHINE_COUNT:-1}"
    export CCM_COUNT="${CCM_COUNT:-1}"
    export WORKER_MACHINE_COUNT="${WORKER_MACHINE_COUNT:-2}"
    export EXP_CLUSTER_RESOURCE_SET="true"

    # this requires k8s 1.22+
    if [[ -n "${TEST_WINDOWS:-}" ]]; then
        export WINDOWS_WORKER_MACHINE_COUNT="${WINDOWS_WORKER_MACHINE_COUNT:-2}"
        if [[ -n "${K8S_FEATURE_GATES:-}" ]]; then
            export K8S_FEATURE_GATES="${K8S_FEATURE_GATES:-},WindowsHostProcessContainers=true"
        else
            export K8S_FEATURE_GATES="WindowsHostProcessContainers=true"
        fi
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

    if [[ -n "${TEST_CCM:-}" ]]; then
        # replace 'prow' with 'prow-external-cloud-provider' in the template name if testing out-of-tree
        export CLUSTER_TEMPLATE="${CLUSTER_TEMPLATE/prow/prow-external-cloud-provider}"
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
}

install_addons() {
    # Get cluster CIDRs from Cluster object
    CIDR0=$(${KUBECTL} get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[0]}')
    CIDR1=$(${KUBECTL} get cluster "${CLUSTER_NAME}" -o=jsonpath='{.spec.clusterNetwork.pods.cidrBlocks[1]}' 2> /dev/null) || true

    # export the target cluster KUBECONFIG if not already set
    export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"

    # wait for the apiserver pod to be Ready.
    APISERVER_POD=$("${KUBECTL}" get pods -n kube-system -o name | grep apiserver)
    "${KUBECTL}" wait --for=condition=Ready -n kube-system "${APISERVER_POD}" --timeout=5m

    # Copy the kubeadm configmap to the calico-system namespace. This is a workaround needed for the calico-node-windows daemonset to be able to run in the calico-system namespace.
    "${KUBECTL}" create ns calico-system
    until "${KUBECTL}" get configmap kubeadm-config --namespace=kube-system
    do
        # Wait for the kubeadm-config configmap to exist.
        sleep 2
    done
    "${KUBECTL}" get configmap kubeadm-config --namespace=kube-system -o yaml \
    | sed 's/namespace: kube-system/namespace: calico-system/' \
    | "${KUBECTL}" create -f -

    # install Calico CNI
    echo "Installing Calico CNI via helm"
    if [[ "${CIDR0}" =~ .*:.* ]]; then
        echo "Cluster CIDR is IPv6"
        CALICO_VALUES_FILE="${REPO_ROOT}/templates/addons/calico-ipv6/values.yaml"
        CIDR_STRING_VALUES="installation.calicoNetwork.ipPools[0].cidr=${CIDR0}"
    elif [[ "${CIDR1}" =~ .*:.* ]]; then
        echo "Cluster CIDR is dual-stack"
        CALICO_VALUES_FILE="${REPO_ROOT}/templates/addons/calico-dual-stack/values.yaml"
        CIDR_STRING_VALUES="installation.calicoNetwork.ipPools[0].cidr=${CIDR0},installation.calicoNetwork.ipPools[1].cidr=${CIDR1}"
    else
        echo "Cluster CIDR is IPv4"
        CALICO_VALUES_FILE="${REPO_ROOT}/templates/addons/calico/values.yaml"
        CIDR_STRING_VALUES="installation.calicoNetwork.ipPools[0].cidr=${CIDR0}"
    fi

    "${HELM}" repo add projectcalico https://projectcalico.docs.tigera.io/charts
    "${HELM}" install calico projectcalico/tigera-operator -f "${CALICO_VALUES_FILE}" --set-string "${CIDR_STRING_VALUES}" --namespace tigera-operator --create-namespace

    # install cloud-provider-azure components, if using out-of-tree
    if [[ -n "${TEST_CCM:-}" ]]; then
        CLOUD_CONFIG="/etc/kubernetes/azure.json"
        CONFIG_SECRET_NAME=""
        ENABLE_DYNAMIC_RELOADING=false
        if [[ -n "${LOAD_CLOUD_CONFIG_FROM_SECRET:-}" ]]; then
            CLOUD_CONFIG=""
            CONFIG_SECRET_NAME="azure-cloud-provider"
            ENABLE_DYNAMIC_RELOADING=true
            copy_secret
        fi

        CCM_CLUSTER_CIDR="${CIDR0}"
        if [[ -n "${CIDR1}" ]]; then
            CCM_CLUSTER_CIDR="${CIDR0}\,${CIDR1}"
        fi
        echo "CCM cluster CIDR: ${CCM_CLUSTER_CIDR:-}"

        export CCM_LOG_VERBOSITY="${CCM_LOG_VERBOSITY:-4}"
        echo "Installing cloud-provider-azure components via helm"
        "${HELM}" install --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name \
            --set infra.clusterName="${CLUSTER_NAME}" \
            --set cloudControllerManager.imageRepository="${IMAGE_REGISTRY}" \
            --set cloudNodeManager.imageRepository="${IMAGE_REGISTRY}" \
            --set cloudControllerManager.imageName="${CCM_IMAGE_NAME}" \
            --set cloudNodeManager.imageName="${CNM_IMAGE_NAME}" \
            --set-string cloudControllerManager.imageTag="${IMAGE_TAG}" \
            --set-string cloudNodeManager.imageTag="${IMAGE_TAG}" \
            --set cloudControllerManager.replicas="${CCM_COUNT}" \
            --set cloudControllerManager.enableDynamicReloading="${ENABLE_DYNAMIC_RELOADING}"  \
            --set cloudControllerManager.cloudConfig="${CLOUD_CONFIG}" \
            --set cloudControllerManager.cloudConfigSecretName="${CONFIG_SECRET_NAME}" \
            --set cloudControllerManager.logVerbosity="${CCM_LOG_VERBOSITY}" \
            --set-string cloudControllerManager.clusterCIDR="${CCM_CLUSTER_CIDR}"
    fi

    export -f wait_for_nodes
    timeout --foreground 1800 bash -c wait_for_nodes

    echo "Waiting for all calico-system pods to be ready"
    "${KUBECTL}" wait --for=condition=Ready pod -n calico-system --all --timeout=10m

    echo "Waiting for all kube-system pods to be ready"
    "${KUBECTL}" wait --for=condition=Ready pod -n kube-system --all --timeout=10m
}

wait_for_nodes() {
    echo "Waiting for ${CONTROL_PLANE_MACHINE_COUNT} control plane machine(s), ${WORKER_MACHINE_COUNT} worker machine(s), and ${WINDOWS_WORKER_MACHINE_COUNT:-0} windows machine(s) to become Ready"

    # Ensure that all nodes are registered with the API server before checking for readiness
    local total_nodes="$((CONTROL_PLANE_MACHINE_COUNT + WORKER_MACHINE_COUNT + WINDOWS_WORKER_MACHINE_COUNT))"
    while [[ $("${KUBECTL}" get nodes -ojson | jq '.items | length') -ne "${total_nodes}" ]]; do
        sleep 10
    done

    "${KUBECTL}" wait --for=condition=Ready node --all --timeout=5m
    "${KUBECTL}" get nodes -owide
}

copy_secret() {
    # point at the management cluster
    unset KUBECONFIG
    "${KUBECTL}" get secret "${CLUSTER_NAME}-control-plane-azure-json" -o jsonpath='{.data.control-plane-azure\.json}' | base64 --decode >azure_json

    # set KUBECONFIG back to the workload cluster
    export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"
    "${KUBECTL}" create secret generic "${CONFIG_SECRET_NAME}" -n kube-system \
        --from-file=cloud-config=azure_json
    rm azure_json
}

# cleanup all resources we use
cleanup() {
    timeout 1800 "${KUBECTL}" delete cluster "${CLUSTER_NAME}" || true
    make kind-reset || true
}

on_exit() {
    if [[ -n ${KUBECONFIG:-} ]]; then
        "${KUBECTL}" get nodes -owide || echo "Unable to get nodes"
        "${KUBECTL}" get pods -A -owide || echo "Unable to get pods"
    fi

    # unset kubeconfig which is currently pointing at workload cluster.
    # we want to be pointing at the management cluster (kind in this case)
    unset KUBECONFIG
    go run -tags e2e "${REPO_ROOT}"/test/logger.go --name "${CLUSTER_NAME}" --namespace default
    "${REPO_ROOT}/hack/log/redact.sh" || true
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

# install CNI and CCM
install_addons

if [[ "${#}" -gt 0 ]]; then
    # disable error exit so we can run post-command cleanup
    set +o errexit
    "${@}"
    EXIT_VALUE="${?}"
    exit ${EXIT_VALUE}
fi
