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

CLUSTER_NAME=${CLUSTER_NAME:-"test-$(date +%s)"}
ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"

# dump logs from kind and all the nodes
dump-logs() {
  # log version information
  echo "=== versions ==="
  echo "kind : $(kind version)" || true
  echo "kubectl: "
  kubectl --kubeconfig=$CLUSTER_NAME.kubeconfig version || true
  echo ""

  # dump all the info from the CAPI related CRDs
  mkdir -p $ARTIFACTS/logs
  kubectl get \
  clusters,azureclusters,machines,azuremachines,kubeadmconfigs,machinedeployments,azuremachinetemplates,kubeadmconfigtemplates,machinesets,kubeadmcontrolplanes \
  --all-namespaces -o yaml >> "${ARTIFACTS}/logs/capz.info" || true

  # dump images info
  {
   echo "images in docker"
   docker images
   echo "images from bootstrap using containerd CLI"
   docker exec kind-control-plane ctr -n k8s.io images list
   echo "images in bootstrap cluster using kubectl CLI"
   (kubectl get pods --all-namespaces -o json \
   | jq --raw-output '.items[].spec.containers[].image' | sort)
   echo "images in deployed cluster using kubectl CLI"
   (kubectl --kubeconfig=$CLUSTER_NAME.kubeconfig get pods --all-namespaces -o json \
   | jq --raw-output '.items[].spec.containers[].image' | sort)
 } >> "${ARTIFACTS}/logs/images.info"
  
  # dump cluster info for kind
  {
    echo "kind cluster-info"
    kubectl cluster-info dump
  } >> "${ARTIFACTS}/logs/kind-cluster.info"

  # dump cluster info for capz
  {
    echo "=== VMs in ${AZURE_RESOURCE_GROUP}  ==="
    az vm list --resource-group "${AZURE_RESOURCE_GROUP}"
    echo "=== cluster-info dump ==="
    kubectl --kubeconfig=$CLUSTER_NAME.kubeconfig cluster-info dump
  } >> "${ARTIFACTS}/logs/capz-cluster.info"

  # export all logs from kind
  kind "export" logs --name="kind" "${ARTIFACTS}/logs"

  nodes=$(az vm list --resource-group ${AZURE_RESOURCE_GROUP} --query "[?tags.\"sigs.k8s.io_cluster-api-provider-azure_cluster_capi-quickstart\" == 'owned'].name" -o tsv)
  declare -a nodeList=( $( echo $nodes | cut -d' ' -f1- ) )
  # We used to pipe this output to 'tail -n +2' but for some reason this was sometimes (all the time?) only finding the
  # bastion host. For now, omit the tail and gather logs for all VMs that have a private IP address. This will include
  # the bastion, but that's better than not getting logs from all the VMs.
  for node in "${nodeList[@]}"
  do
    echo "collecting logs from ${node}"
    dir="${ARTIFACTS}/logs/${node}"
    mkdir -p "${dir}"
    ssh-to-node "${node}" "sudo journalctl --output=short-precise -k"  "${dir}/kern.log" 
    ssh-to-node "${node}" "sudo journalctl --output=short-precise"  "${dir}/systemd.log" 
    ssh-to-node "${node}" "sudo crictl version && sudo crictl info"  "${dir}/containerd.info" 
    ssh-to-node "${node}" "sudo journalctl --no-pager -u cloud-final"  "${dir}/cloud-final.log" 
    ssh-to-node "${node}" "sudo journalctl --no-pager -u kubelet.service"  "${dir}/kubelet.log" 
    ssh-to-node "${node}" "sudo journalctl --no-pager -u containerd.service" "${dir}/containerd.log" 
  done
}

# SSH to a node by instance-id ($1) and run a command ($2).
function ssh-to-node() {
  local node="$1"
  local cmd="$2"
  local logfile="$3"

  output=$(az vm run-command invoke -g ${AZURE_RESOURCE_GROUP} -n ${node} --command-id RunShellScript --scripts ${cmd})
  message=$(echo -E $output | jq '.value[0].message')
  echo -e $message >> ${logfile}
}
