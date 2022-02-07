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

# Install kubectl
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
cd "${REPO_ROOT}" && make "${KUBECTL##*/}"

# shellcheck source=hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"

export ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
mkdir -p "${ARTIFACTS}/management-cluster" "${ARTIFACTS}/workload-cluster"

export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"

get_node_name() {
    local -r pod_name="${1}"
    # shellcheck disable=SC1083
    "${KUBECTL}" get pod "${pod_name}" -ojsonpath={.spec.nodeName}
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
    "${KUBECTL}" apply -f "${REPO_ROOT}/hack/log/log-dump-daemonset.yaml"
    "${KUBECTL}" wait pod -l app=log-dump-node --for=condition=Ready --timeout=5m

    IFS=" " read -ra log_dump_pods <<< "$(kubectl get pod -l app=log-dump-node -ojsonpath='{.items[*].metadata.name}')"
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

dump_workload_cluster_logs_windows() {
    echo "Deploying log-dump-daemonset-windows"
    "${KUBECTL}" apply -f "${REPO_ROOT}/hack/log/log-dump-daemonset-windows.yaml"
    echo "Waiting for log-dump-daemonset-windows"
    "${KUBECTL}" wait pod -l app=log-dump-node-windows --for=condition=Ready --timeout=5m

    IFS=" " read -ra log_dump_pods <<< "$(kubectl get pod -l app=log-dump-node-windows -ojsonpath='{.items[*].metadata.name}')"

    for log_dump_pod in "${log_dump_pods[@]}"; do
        local node_name
        node_name="$(get_node_name "${log_dump_pod}")"
        echo "Getting logs for node ${node_name}"

        local log_dump_dir="${ARTIFACTS}/workload-cluster/${node_name}"
        mkdir -p "${log_dump_dir}"

        # make a new folder to copy logs to since files cannot be read to directly
        "${KUBECTL}" exec "${log_dump_pod}" -- cmd.exe /c mkdir log
        "${KUBECTL}" exec "${log_dump_pod}" -- cmd.exe /c xcopy /s c:\\var\\log\\kubelet c:\\log\\
        "${KUBECTL}" exec "${log_dump_pod}" -- cmd.exe /c xcopy /s c:\\var\\log\\pods c:\\log\\

        # Get a list of all of the files to copy with dir
        # /s - recurse
        # /B - bare format (no heading info or summaries)
        # /A-D - exclude directories
        IFS=" " read -ra log_dump_files <<< "$(kubectl exec "${log_dump_pod}" -- cmd.exe /c dir /s /B /A-D log | tr '\n' ' ' | tr -d '\r' )"
        echo "Collecting pod logs"

        for log_dump_file in "${log_dump_files[@]}"; do
            echo "    Getting logfile ${log_dump_file}"
            # reverse slashes and remove c:\log\ from paths
            fixed_dump_file_path="$(echo "${log_dump_file//\\//}" | cut -d "/" -f3-)"
            dir="$(dirname "${fixed_dump_file_path}")"
            file="$(basename "${fixed_dump_file_path}")"
            mkdir -p "${log_dump_dir}"/"${dir}"
            "${KUBECTL}" exec "${log_dump_pod}" -- cmd.exe /c type "${log_dump_file}" > "${log_dump_dir}"/"${dir}"/"${file}"
        done

        echo "Exported logs for node \"${node_name}\""
    done

}

cleanup() {
    "${KUBECTL}" delete -f "${REPO_ROOT}/hack/log/log-dump-daemonset.yaml" || true
    "${KUBECTL}" delete -f "${REPO_ROOT}/hack/log/log-dump-daemonset-windows.yaml" || true
    # shellcheck source=hack/log/redact.sh
    source "${REPO_ROOT}/hack/log/redact.sh"
}

trap cleanup EXIT

echo "================ DUMPING LOGS FOR MANAGEMENT CLUSTER ================"
dump_mgmt_cluster_logs

echo "================ DUMPING LOGS FOR WORKLOAD CLUSTER (Linux) =========="
dump_workload_cluster_logs

if [[ -z "${TEST_WINDOWS}" ]]; then
    echo "TEST_WINDOWS envvar not set, skipping log collection for Windows nodes."
else
    echo "================ DUMPING LOGS FOR WORKLOAD CLUSTER (Windows) ========"
    dump_workload_cluster_logs_windows
fi
