#!/usr/bin/env bash

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

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"

export ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
mkdir -p "${ARTIFACTS}/management-cluster" "${ARTIFACTS}/workload-cluster"

export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"

get_node_name() {
    local -r pod_name="${1}"
    # shellcheck disable=SC1083
    kubectl get pod "${pod_name}" -ojsonpath={.spec.nodeName}
}

dump_mgmt_cluster_logs() {
    # Assume the first kind cluster is the management cluster
    local -r mgmt_cluster_name="$(kind get clusters | head -n 1)"
    if [[ -z "${mgmt_cluster_name}" ]]; then
        echo "No kind cluster is found"
        return
    fi

    kind get kubeconfig --name "${mgmt_cluster_name}" > "${PWD}/kind.kubeconfig"
    local -r kubectl_kind="kubectl --kubeconfig=${PWD}/kind.kubeconfig"

    local -r resources=(
        "clusters"
        "azureclusters"
        "machines"
        "azuremachines"
        "kubeadmconfigs"
        "machinedeployments"
        "azuremachinetemplates"
        "kubeadmconfigtemplates"
        "machinesets"
        "kubeadmcontrolplanes"
        "machinepools"
        "azuremachinepools"
    )
    mkdir -p "${ARTIFACTS}/management-cluster/resources"
    for resource in "${resources[@]}"; do
        ${kubectl_kind} get --all-namespaces "${resource}" -oyaml > "${ARTIFACTS}/management-cluster/resources/${resource}.log" || true
    done

    {
        echo "images in docker"
        docker images
        echo "images in bootstrap cluster using kubectl CLI"
        (${kubectl_kind} get pods --all-namespaces -ojson \
        | jq --raw-output '.items[].spec.containers[].image' | sort)
        echo "images in deployed cluster using kubectl CLI"
        (${kubectl_kind} get pods --all-namespaces -ojson \
        | jq --raw-output '.items[].spec.containers[].image' | sort)
    } > "${ARTIFACTS}/management-cluster/images.info"

    {
        echo "kind cluster-info"
        ${kubectl_kind} cluster-info dump
    } > "${ARTIFACTS}/management-cluster/kind-cluster.info"

    kind export logs --name="${mgmt_cluster_name}" "${ARTIFACTS}/management-cluster"
}

dump_workload_cluster_logs() {
    echo "Deploying log-dump-daemonset"
    kubectl apply -f "${REPO_ROOT}/hack/log/log-dump-daemonset.yaml"
    kubectl wait pod -l app=log-dump-node --for=condition=Ready --timeout=5m

    local -r log_dump_pods=()
    IFS=" " read -r -a log_dump_pods <<< "$(kubectl get pod -l app=log-dump-node -ojsonpath='{.items[*].metadata.name}')"
    local log_dump_commands=(
        "journalctl --output=short-precise -u kubelet > kubelet.log"
        "journalctl --output=short-precise -u containerd > containerd.log"
        "journalctl --output=short-precise -k > kern.log"
        "journalctl --output=short-precise > journal.log"
        "cat /var/log/cloud-init.log > cloud-init.log"
        "cat /var/log/cloud-init-output.log > cloud-init-output.log"
    )

    if [[ "$(uname)" == "Darwin" ]]; then
        # tar on Mac OS does not support --wildcards flag
        log_dump_commands+=( "tar -cf - var/log/pods --ignore-failed-read | tar xf - --strip-components=2 -C . '*kube-system*'" )
    else
        log_dump_commands+=( "tar -cf - var/log/pods --ignore-failed-read | tar xf - --strip-components=2 -C . --wildcards '*kube-system*'" )
    fi

    for log_dump_pod in "${log_dump_pods[@]}"; do
        local node_name
        node_name="$(get_node_name "${log_dump_pod}")"

        local log_dump_dir="${ARTIFACTS}/workload-cluster/${node_name}"
        mkdir -p "${log_dump_dir}"
        pushd "${log_dump_dir}" > /dev/null
        for cmd in "${log_dump_commands[@]}"; do
            bash -c "kubectl exec ${log_dump_pod} -- ${cmd}" &
        done

        popd > /dev/null
        echo "Exported logs for node \"${node_name}\""
    done

    # Wait for log-dumping commands running in the background to complete
    wait
}

cleanup() {
    kubectl delete -f "${REPO_ROOT}/hack/log/log-dump-daemonset.yaml" || true
    # shellcheck source=hack/log/redact.sh
    source "${REPO_ROOT}/hack/log/redact.sh"
}

trap cleanup EXIT

echo "================ DUMPING LOGS FOR MANAGEMENT CLUSTER ================"
dump_mgmt_cluster_logs

echo "================ DUMPING LOGS FOR WORKLOAD CLUSTER ================"
dump_workload_cluster_logs
