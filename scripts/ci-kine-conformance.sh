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

set +x
set -o errexit
set -o nounset
set -o pipefail

# shellcheck source=scripts/kine/performance.sh
source "$(dirname "${BASH_SOURCE[0]}")/kine/performance.sh"

capz::kine::az() {
  az "$@" --subscription "${AZURE_SUBSCRIPTION_ID}" --only-show-errors
}

capz::kine::certificates() {
  capz::kine::backend_defaults
  if [[ "${KINE_BACKEND}" != kine-postgres ]]; then
    echo "Local kubeadm etcd must not receive external datastore credentials." >&2
    return 1
  fi
  capz::kine::connection_defaults
  printf '%s\n' 'basicConstraints=critical,CA:FALSE' 'keyUsage=critical,digitalSignature,keyEncipherment' \
    'extendedKeyUsage=serverAuth' 'subjectAltName=IP:10.250.0.4,IP:127.0.0.1' > "${state_dir}/server.ext"
  printf '%s\n' 'basicConstraints=critical,CA:FALSE' 'keyUsage=critical,digitalSignature' \
    'extendedKeyUsage=clientAuth' > "${state_dir}/client.ext"
  openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 7 \
    -subj "/CN=capz-kine-ca" -addext "basicConstraints=critical,CA:TRUE" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" \
    -keyout "${state_dir}/ca.key" -out "${state_dir}/ca.crt" 2>"${state_dir}/openssl.log" ||
    { cat "${state_dir}/openssl.log" >&2; return 1; }
  local name
  for name in server client; do
    openssl req -new -newkey rsa:2048 -nodes -sha256 -subj "/CN=capz-kine-${name}" \
      -keyout "${state_dir}/${name}.key" -out "${state_dir}/${name}.csr" 2>"${state_dir}/openssl.log" ||
      { cat "${state_dir}/openssl.log" >&2; return 1; }
    openssl x509 -req -sha256 -days 7 -in "${state_dir}/${name}.csr" \
      -CA "${state_dir}/ca.crt" -CAkey "${state_dir}/ca.key" -CAcreateserial \
      -extfile "${state_dir}/${name}.ext" -out "${state_dir}/${name}.crt" 2>"${state_dir}/openssl.log" ||
      { cat "${state_dir}/openssl.log" >&2; return 1; }
  done
  KINE_CA_CERT_B64=$(base64 < "${state_dir}/ca.crt" | tr -d '\n')
  KINE_CLIENT_CERT_B64=$(base64 < "${state_dir}/client.crt" | tr -d '\n')
  KINE_CLIENT_KEY_B64=$(base64 < "${state_dir}/client.key" | tr -d '\n')
  export KINE_CA_CERT_B64 KINE_CLIENT_CERT_B64 KINE_CLIENT_KEY_B64
  local server_cert server_key disk_script check_script metrics_address=127.0.0.1 profiling=false
  if [[ "${KINE_PROFILE:-conformance}" == performance ]]; then
    metrics_address=10.250.0.4
    profiling="${KINE_COLLECT_PROFILES:-false}"
  fi
  disk_script=$(base64 < "${repo_root}/scripts/kine/mount-datastore-disk.sh" | tr -d '\n')
  check_script=$(base64 < "${repo_root}/scripts/kine/check-datastore.sh" | tr -d '\n')
  server_cert=$(base64 < "${state_dir}/server.crt" | tr -d '\n')
  server_key=$(base64 < "${state_dir}/server.key" | tr -d '\n')
  local storage_script connections_script
  storage_script=$(base64 < "${repo_root}/scripts/kine/configure-performance-storage.sh" | tr -d '\n')
  connections_script=$(base64 < "${repo_root}/scripts/kine/connections.sh" | tr -d '\n')
  local -a replacements=(
    -e "s|__CA_CERT__|${KINE_CA_CERT_B64}|g"
    -e "s|__SERVER_CERT__|${server_cert}|g"
    -e "s|__SERVER_KEY__|${server_key}|g"
    -e "s|__CLIENT_CERT__|${KINE_CLIENT_CERT_B64}|g"
    -e "s|__CLIENT_KEY__|${KINE_CLIENT_KEY_B64}|g"
    -e "s|__DISK_SCRIPT__|${disk_script}|g"
    -e "s|__CHECK_SCRIPT__|${check_script}|g"
    -e "s|__METRICS_BIND_ADDRESS__|${metrics_address}|g"
    -e "s|__METRICS_PROFILING__|${profiling}|g"
    -e "s|__STORAGE_SCRIPT__|${storage_script}|g"
    -e "s|__CONNECTIONS_SCRIPT__|${connections_script}|g"
    -e "s|__POSTGRES_MAX_CONNECTIONS__|${KINE_POSTGRES_MAX_CONNECTIONS}|g"
    -e "s|__KINE_MAX_OPEN_CONNECTIONS__|${KINE_DATASTORE_MAX_OPEN_CONNECTIONS}|g"
    -e "s|__KINE_MAX_IDLE_CONNECTIONS__|${KINE_DATASTORE_MAX_IDLE_CONNECTIONS}|g"
  )
  sed "${replacements[@]}" "${repo_root}/scripts/kine/cloud-init.yaml" > "${state_dir}/cloud-init.yaml"
}

capz::kine::provision() {
  capz::kine::backend_defaults
  if [[ "${KINE_BACKEND}" != kine-postgres ]]; then
    echo "Only Kine/PostgreSQL provisions an external datastore VM." >&2
    return 1
  fi
  fixture_started=true
  capz::kine::az group create --name "${KINE_RESOURCE_GROUP}" --location "${AZURE_LOCATION}" \
    --tags "capz-kine-run=${run_id}" "creationTimestamp=${TIMESTAMP}" \
    "jobName=${JOB_NAME}" "buildProvenance=${BUILD_PROVENANCE}" --output none
  capz::kine::az network vnet create --resource-group "${KINE_RESOURCE_GROUP}" \
    --name kine-vnet --address-prefixes 10.250.0.0/24 \
    --subnet-name datastore --subnet-prefixes 10.250.0.0/24 --output none
  capz::kine::az network nsg create --resource-group "${KINE_RESOURCE_GROUP}" --name kine-nsg --output none
  capz::kine::az network nsg rule create --resource-group "${KINE_RESOURCE_GROUP}" \
    --nsg-name kine-nsg --name allow-control-plane --priority 100 --direction Inbound \
    --access Allow --protocol Tcp --source-address-prefixes 10.0.0.0/24 \
    --source-port-ranges '*' --destination-address-prefixes '*' --destination-port-ranges 2379 --output none
  if [[ "${KINE_PROFILE:-conformance}" == performance ]]; then
    capz::kine::az network nsg rule create --resource-group "${KINE_RESOURCE_GROUP}" \
      --nsg-name kine-nsg --name allow-kine-metrics --priority 110 --direction Inbound \
      --access Allow --protocol Tcp --source-address-prefixes 10.0.1.0/24 \
      --source-port-ranges '*' --destination-address-prefixes 10.250.0.4/32 --destination-port-ranges 8080 --output none
  fi
  capz::kine::az network nsg rule create --resource-group "${KINE_RESOURCE_GROUP}" \
    --nsg-name kine-nsg --name deny-other-inbound --priority 200 --direction Inbound \
    --access Deny --protocol '*' --source-address-prefixes '*' --source-port-ranges '*' \
    --destination-address-prefixes '*' --destination-port-ranges '*' --output none

  # Explicit outbound access is needed for package installation and the Azure VM agent.
  capz::kine::az network public-ip create --resource-group "${KINE_RESOURCE_GROUP}" \
    --name kine-egress --sku Standard --allocation-method Static --output none
  capz::kine::az network nat gateway create --resource-group "${KINE_RESOURCE_GROUP}" \
    --name kine-nat --public-ip-addresses kine-egress --output none
  capz::kine::az network vnet subnet update --resource-group "${KINE_RESOURCE_GROUP}" \
    --vnet-name kine-vnet --name datastore --nat-gateway kine-nat --output none
  capz::kine::az disk create --resource-group "${KINE_RESOURCE_GROUP}" --name kine-data \
    --location "${AZURE_LOCATION}" --zone "${KINE_ZONE}" --sku UltraSSD_LRS \
    --size-gb "${KINE_DISK_SIZE_GB}" --disk-iops-read-write "${KINE_DISK_IOPS}" \
    --disk-mbps-read-write "${KINE_DISK_MBPS}" --output none
  capz::kine::az network nic create --resource-group "${KINE_RESOURCE_GROUP}" \
    --name kine-nic --vnet-name kine-vnet --subnet datastore \
    --private-ip-address 10.250.0.4 --network-security-group kine-nsg --accelerated-networking true --output none
  capz::kine::az vm create --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
    --nics kine-nic --image "${KINE_VM_IMAGE:-Canonical:ubuntu-24_04-lts:server:latest}" \
    --size "${KINE_VM_SIZE}" --admin-username azureuser \
    --ssh-key-values "${state_dir}/ssh.pub" --custom-data "${state_dir}/cloud-init.yaml" \
    --os-disk-size-gb 128 --os-disk-caching ReadWrite --storage-sku Premium_LRS \
    --zone "${KINE_ZONE}" --disk-controller-type SCSI --ultra-ssd-enabled true \
    --attach-data-disks kine-data --data-disk-caching None --output none
  vm_created=true
  capz::kine::az vm show --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
    --query '{size:hardwareProfile.vmSize,zone:zones,image:storageProfile.imageReference,controller:storageProfile.diskControllerType,dataDisks:storageProfile.dataDisks}' \
    --output json > "${artifact_dir}/vm-configuration.json"
  capz::kine::az disk show --resource-group "${KINE_RESOURCE_GROUP}" --name kine-data \
    --output json |
    jq '{sku:.sku.name,zone:.zones,size:.diskSizeGB,iops:.diskIOPSReadWrite,mbps:.diskMBpsReadWrite}' \
      > "${artifact_dir}/disk-configuration.json"
  if ! jq -e --argjson iops "${KINE_DISK_IOPS}" --argjson mbps "${KINE_DISK_MBPS}" \
    '.sku == "UltraSSD_LRS" and .iops == $iops and .mbps == $mbps' \
    "${artifact_dir}/disk-configuration.json" >/dev/null; then
    echo "Provisioned disk does not match requested performance." >&2
    echo "Expected: sku=UltraSSD_LRS, iops=${KINE_DISK_IOPS}, mbps=${KINE_DISK_MBPS}" >&2
    printf 'Observed: ' >&2
    jq -c '{sku, iops, mbps}' "${artifact_dir}/disk-configuration.json" >&2
    return 1
  fi
  jq -e '.controller == "SCSI" and (.dataDisks | length) == 1 and .dataDisks[0].lun == 0 and .dataDisks[0].caching == "None"' \
    "${artifact_dir}/vm-configuration.json" >/dev/null ||
    { echo "Expected a SCSI VM with one uncached data disk at LUN 0." >&2; return 1; }
  capz::kine::az vm run-command invoke --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
    --command-id RunShellScript --scripts \
    "set -eu; timeout 1200 cloud-init status --wait; /usr/local/bin/check-datastore ${KINE_BACKEND}" \
    --output json > "${artifact_dir}/readiness.json"
  # Run Command can return HTTP success even when the guest script failed.
  if ! jq -r '.value[].message' "${artifact_dir}/readiness.json" | grep -qx KINE_READY; then
    echo "${KINE_BACKEND} gRPC readiness failed; see ${artifact_dir}/readiness.json." >&2
    return 1
  fi
}

capz::kine::collect_compactions() {
  capz::kine::backend_defaults
  if ! capz::kine::az vm run-command invoke --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
    --command-id RunShellScript --scripts /usr/local/bin/collect-kine-compactions \
    --output json > "${artifact_dir}/compaction-command.json"; then
    echo "Failed to collect ${KINE_BACKEND} compaction history." >&2
    return 1
  fi
  local encoded
  encoded=$(jq -er '[.value[]?.message |
    capture("(?m)^KINE_COMPACTIONS_BEGIN\\n(?<data>[A-Za-z0-9+/=]+)\\nKINE_COMPACTIONS_END$").data] |
    if length == 1 then .[0] else error("Missing or truncated compaction history") end' \
    "${artifact_dir}/compaction-command.json") ||
    { echo "${KINE_BACKEND} compaction history is incomplete; see compaction-command.json." >&2; return 1; }
  if ! printf '%s' "${encoded}" | openssl base64 -d -A | gzip -dc > "${artifact_dir}/compaction.log.tmp"; then
    rm -f "${artifact_dir}/compaction.log.tmp"
    echo "${KINE_BACKEND} compaction history could not be decoded completely." >&2
    return 1
  fi
  mv "${artifact_dir}/compaction.log.tmp" "${artifact_dir}/compaction.log"
}

capz::kine::cleanup() {
  local result=$1 exists owner
  trap - EXIT
  capz::kine::backend_defaults
  if [[ "${vm_created}" == true ]]; then
    local logs='journalctl --no-pager -u kine -n 60; tail -n 60 /var/log/cloud-init-output.log; cloud-init status --long; printf "cloud-init status exit code: %s\n" "$?"'
    if ! capz::kine::az vm run-command invoke --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
      --command-id RunShellScript --scripts "${logs}" \
      --output json > "${artifact_dir}/datastore-logs.json"; then
      echo "Failed to collect ${KINE_BACKEND} VM logs." >&2
      result=1
    fi
    if [[ "${KINE_PROFILE:-conformance}" == performance ]] && ! capz::kine::collect_compactions; then
      result=1
    fi
  fi
  if [[ "${fixture_started}" == true && "${SKIP_CLEANUP:-false}" != true ]]; then
    if ! exists=$(capz::kine::az group exists --name "${CLUSTER_NAME}" --output tsv); then
      echo "Cannot establish workload deletion; preserving ${KINE_RESOURCE_GROUP}." >&2
      result=1
    elif [[ "${exists}" != false ]]; then
      echo "Workload resource group still exists; preserving ${KINE_RESOURCE_GROUP}." >&2
      result=1
    elif ! owner=$(capz::kine::az group show --name "${KINE_RESOURCE_GROUP}" --query 'tags."capz-kine-run"' --output tsv); then
      echo "Cannot verify fixture ownership; preserving ${KINE_RESOURCE_GROUP}." >&2
      result=1
    elif [[ "${owner}" != "${run_id}" ]]; then
      echo "Fixture ownership does not match this run; refusing to delete ${KINE_RESOURCE_GROUP}." >&2
      result=1
    elif ! capz::kine::az group delete --name "${KINE_RESOURCE_GROUP}" --yes; then
      echo "Failed to delete ${KINE_RESOURCE_GROUP}; manual cleanup is required." >&2
      result=1
    fi
  elif [[ "${fixture_started}" == true ]]; then
    echo "SKIP_CLEANUP=true: preserving datastore resource group ${KINE_RESOURCE_GROUP}."
  fi
  rm -f "${state_dir}/ca.key" "${state_dir}/ca.crt" "${state_dir}/ca.srl" \
    "${state_dir}/server.key" "${state_dir}/server.csr" "${state_dir}/server.crt" "${state_dir}/server.ext" \
    "${state_dir}/client.key" "${state_dir}/client.csr" "${state_dir}/client.crt" "${state_dir}/client.ext" \
    "${state_dir}/ssh" "${state_dir}/ssh.pub" "${state_dir}/cloud-init.yaml" "${state_dir}/openssl.log"
  rmdir "${state_dir}"
  exit "${result}"
}

capz::kine::main() {
  repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
  capz::kine::backend_defaults
  local flavor=kine-postgres legacy_flavor=kine-postgres
  if [[ "${KINE_BACKEND}" == kubeadm-etcd ]]; then flavor=kubeadm-etcd; fi
  case "${KINE_PROFILE:-conformance}" in
    conformance) ;;
    performance) flavor="${flavor}-performance"; legacy_flavor=kine-postgres-performance ;;
    *) echo "Unknown KINE_PROFILE: ${KINE_PROFILE}" >&2; return 1 ;;
  esac
  export KINE_COLLECT_PROFILES="${KINE_COLLECT_PROFILES:-false}"
  case "${KINE_COLLECT_PROFILES}" in
    true|false) ;;
    *) echo "KINE_COLLECT_PROFILES must be true or false." >&2; return 1 ;;
  esac
  local tool
  local -a tools=(az openssl jq)
  if [[ "${KINE_BACKEND}" == kine-postgres ]]; then tools+=(ssh-keygen gzip); fi
  for tool in "${tools[@]}"; do
    command -v "${tool}" >/dev/null || { echo "Required tool not found: ${tool}" >&2; return 1; }
  done
  : "${AZURE_SUBSCRIPTION_ID:?Set the Azure subscription used by CAPZ and az CLI}"
  if [[ "${CONFORMANCE_FLAVOR:-${flavor}}" != "${flavor}" ]]; then
    if [[ "${KINE_BACKEND}" == kubeadm-etcd && "${CONFORMANCE_FLAVOR}" == "${legacy_flavor}" ]]; then
      echo "Selecting CONFORMANCE_FLAVOR=${flavor} instead of legacy job flavor ${CONFORMANCE_FLAVOR} for local etcd."
    else
      echo "This wrapper requires CONFORMANCE_FLAVOR=${flavor}." >&2
      return 1
    fi
  fi
  if [[ "${WINDOWS:-false}" != false || "${IP_FAMILY:-ipv4}" != ipv4 ||
        "${TEST_K8S:-false}" == true || "${E2E_ARGS:-}" == *use-ci-artifacts* ||
        "${E2E_ARGS:-}" == *use-pr-artifacts* ]]; then
    echo "The Kine scaffold supports Linux/IPv4 and released Kubernetes images only." >&2
    return 1
  fi
  export KINE_KUBERNETES_VERSION="${KINE_KUBERNETES_VERSION:-v1.35.5}"
  if [[ ! "${KINE_KUBERNETES_VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Pin KINE_KUBERNETES_VERSION to a release such as v1.35.5." >&2
    return 1
  fi
  if [[ -n "${KUBERNETES_VERSION:-}" && "${KUBERNETES_VERSION}" != "${KINE_KUBERNETES_VERSION}" ]]; then
    echo "Kine overrides KUBERNETES_VERSION=${KUBERNETES_VERSION} with KINE_KUBERNETES_VERSION=${KINE_KUBERNETES_VERSION}."
  fi
  export KUBERNETES_VERSION="${KINE_KUBERNETES_VERSION}"
  export KINE_NODE_IMAGE_VERSION="${KUBERNETES_VERSION#v}"
  echo "Using Kine Kubernetes version: ${KUBERNETES_VERSION}; Azure Linux Gen2 node image version: ${KINE_NODE_IMAGE_VERSION}"
  run_id=$(openssl rand -hex 8)
  export CLUSTER_NAME="${CLUSTER_NAME:-capz-kine-${run_id}}"
  if [[ ! "${CLUSTER_NAME}" =~ ^[a-z][a-z0-9-]{0,38}[a-z0-9]$ ]]; then
    echo "CLUSTER_NAME must be 2-40 lowercase letters, digits, or hyphens, starting with a letter." >&2
    return 1
  fi
  if [[ "${KINE_BACKEND}" == kine-postgres ]]; then
    export KINE_RESOURCE_GROUP="${CLUSTER_NAME}-kine"
  else
    unset KINE_RESOURCE_GROUP KINE_CA_CERT_B64 KINE_CLIENT_CERT_B64 KINE_CLIENT_KEY_B64
  fi
  export CONFORMANCE_FLAVOR="${flavor}"
  capz::kine::performance_defaults
  cd "${repo_root}"
  # shellcheck source=hack/ensure-tags.sh
  source "${repo_root}/hack/ensure-tags.sh"
  echo "Using AZURE_LOCATION: ${AZURE_LOCATION}, KINE_ZONE: ${KINE_ZONE}"
  umask 077
  artifact_dir="${ARTIFACTS:-${repo_root}/_artifacts}/kine-${CLUSTER_NAME}"
  mkdir -p "${artifact_dir}"
  capz::kine::performance_preflight
  if [[ "${KINE_PREFLIGHT_ONLY:-false}" == true ]]; then
    return 0
  fi
  if [[ "${KINE_PROFILE:-conformance}" == performance ]]; then
    bash "${repo_root}/scripts/kine/run-clusterloader.sh" --prepare-only
  fi
  local group exists
  local -a groups=("${CLUSTER_NAME}")
  if [[ "${KINE_BACKEND}" == kine-postgres ]]; then groups+=("${KINE_RESOURCE_GROUP}"); fi
  for group in "${groups[@]}"; do
    exists=$(capz::kine::az group exists --name "${group}" --output tsv)
    if [[ "${exists}" != false ]]; then
      echo "Refusing to reuse resource group ${group}; choose a fresh CLUSTER_NAME." >&2
      return 1
    fi
  done
  if [[ "${KINE_BACKEND}" == kubeadm-etcd ]]; then
    printf 'subscription=%s\nworkload-resource-group=%s\ndatastore-topology=kubeadm-local\n' \
      "${AZURE_SUBSCRIPTION_ID}" "${CLUSTER_NAME}" > "${artifact_dir}/resources.txt"
    echo "Using kubeadm-managed local etcd; no standalone datastore VM, VNet, NAT, or client credentials will be created."
    bash "${repo_root}/scripts/ci-conformance.sh"
    return
  fi
  state_dir=$(mktemp -d "${TMPDIR:-/tmp}/capz-kine.XXXXXX")
  fixture_started=false
  vm_created=false
  trap 'capz::kine::cleanup $?' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  ssh-keygen -q -t rsa -b 2048 -N '' -f "${state_dir}/ssh"
  capz::kine::certificates
  printf 'subscription=%s\nworkload-resource-group=%s\ndatastore-resource-group=%s\n' \
    "${AZURE_SUBSCRIPTION_ID}" "${CLUSTER_NAME}" "${KINE_RESOURCE_GROUP}" > "${artifact_dir}/resources.txt"
  echo "Provisioning ${KINE_BACKEND} ${KINE_DATASTORE_VERSION} in ${KINE_RESOURCE_GROUP}."
  capz::kine::provision
  bash "${repo_root}/scripts/ci-conformance.sh"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  capz::kine::main "$@"
fi
