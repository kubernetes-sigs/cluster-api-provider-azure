#!/usr/bin/env bash

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

capz::kine::performance_defaults() {
  export AZURE_LOCATION="${AZURE_LOCATION:-westus2}"
  export KINE_ZONE="${KINE_ZONE:-2}"
  [[ "${KINE_ZONE}" =~ ^[123]$ ]] || { echo "KINE_ZONE must be 1, 2, or 3." >&2; return 1; }
  # Kustomize strips template quotes; substitute a YAML string literal instead.
  export KINE_ZONE_YAML="\"${KINE_ZONE}\""
  export KINE_ZONE_1=false KINE_ZONE_2=false KINE_ZONE_3=false
  case "${KINE_ZONE}" in
    1) export KINE_ZONE_1=true ;;
    2) export KINE_ZONE_2=true ;;
    3) export KINE_ZONE_3=true ;;
  esac
  export KINE_VM_SIZE="${KINE_VM_SIZE:-Standard_E112ibs_v5}"
  export KINE_DISK_SIZE_GB="${KINE_DISK_SIZE_GB:-512}"
  export KINE_DISK_IOPS="${KINE_DISK_IOPS:-400000}"
  export KINE_DISK_MBPS="${KINE_DISK_MBPS:-10000}"
  export AZURE_CONTROL_PLANE_MACHINE_TYPE="${AZURE_CONTROL_PLANE_MACHINE_TYPE:-Standard_D32s_v5}"
  export AZURE_NODE_MACHINE_TYPE="${AZURE_NODE_MACHINE_TYPE:-Standard_D16s_v5}"
  export CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT="${CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT:-1}"
  export CONFORMANCE_WORKER_MACHINE_COUNT="${CONFORMANCE_WORKER_MACHINE_COUNT:-2}"
  local value
  for value in "${KINE_DISK_SIZE_GB}" "${KINE_DISK_IOPS}" "${KINE_DISK_MBPS}" \
    "${CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT}" "${CONFORMANCE_WORKER_MACHINE_COUNT}"; do
    if [[ ! "${value}" =~ ^[1-9][0-9]{0,6}$ ]]; then
      echo "Disk settings and machine counts must be positive integers (at most 7 digits)." >&2
      return 1
    fi
  done
  if (( KINE_DISK_SIZE_GB < 4 || KINE_DISK_SIZE_GB > 65536 ||
        KINE_DISK_IOPS < 100 || KINE_DISK_IOPS > 400000 || KINE_DISK_IOPS > KINE_DISK_SIZE_GB * 1000 ||
        KINE_DISK_MBPS > 10000 || KINE_DISK_MBPS * 4 > KINE_DISK_IOPS )); then
    echo "Requested disk capacity/IOPS/MBps exceeds Ultra Disk limits." >&2
    return 1
  fi
}

capz::kine::performance_preflight() {
  : "${artifact_dir:?Set the fixture artifact directory}" "${repo_root:?Set the repository root}"
  capz::kine::az vm list-skus --location "${AZURE_LOCATION}" --resource-type virtualMachines --all \
    --output json > "${artifact_dir}/vm-skus.json"
  capz::kine::az vm list-usage --location "${AZURE_LOCATION}" \
    --output json > "${artifact_dir}/quota.json"
  jq -n --arg location "${AZURE_LOCATION}" --arg zone "${KINE_ZONE}" \
    --arg datastore "${KINE_VM_SIZE}" --arg controlPlane "${AZURE_CONTROL_PLANE_MACHINE_TYPE}" \
    --arg worker "${AZURE_NODE_MACHINE_TYPE}" \
    --argjson controlPlanes "${CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT}" \
    --argjson workers "${CONFORMANCE_WORKER_MACHINE_COUNT}" \
    --slurpfile skus "${artifact_dir}/vm-skus.json" --slurpfile quota "${artifact_dir}/quota.json" \
    -f "${repo_root}/scripts/kine/performance-preflight.jq" > "${artifact_dir}/capacity-plan.json"
  cat "${artifact_dir}/capacity-plan.json"
  local image="${KINE_VM_IMAGE:-Canonical:ubuntu-24_04-lts:server:latest}" version
  capz::kine::az vm image show --location "${AZURE_LOCATION}" \
    --urn "${image}" \
    --output json > "${artifact_dir}/image.json"
  jq -e '.architecture == "x64" and .hyperVGeneration == "V2" and any(.features[]?; .name == "DiskControllerTypes" and (.value | contains("NVMe")))' \
    "${artifact_dir}/image.json" >/dev/null ||
    { echo "Datastore image must explicitly support x64/Gen2/NVMe." >&2; return 1; }
  version=$(jq -er '.name | if test("^[0-9]+\\.[0-9]+\\.[0-9]+$") then . else error("Invalid datastore image version") end' "${artifact_dir}/image.json")
  KINE_VM_IMAGE="${image%:*}:${version}"
  export KINE_VM_IMAGE
  printf 'location=%s\nzone=%s\nimage=%s\ndisk-size-gib=%s\ndisk-iops=%s\ndisk-mbps=%s\n' \
    "${AZURE_LOCATION}" "${KINE_ZONE}" "${KINE_VM_IMAGE}" "${KINE_DISK_SIZE_GB}" "${KINE_DISK_IOPS}" "${KINE_DISK_MBPS}" \
    > "${artifact_dir}/performance-settings.txt"
  echo "VM quota/capability preflight passed; allocation capacity and disk quotas are not reserved."
}
