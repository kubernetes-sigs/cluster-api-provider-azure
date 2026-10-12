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

capz::kine::az() {
  az "$@" --subscription "${AZURE_SUBSCRIPTION_ID}" --only-show-errors
}

capz::kine::certificates() {
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
  local server_cert server_key
  server_cert=$(base64 < "${state_dir}/server.crt" | tr -d '\n')
  server_key=$(base64 < "${state_dir}/server.key" | tr -d '\n')
  sed -e "s|__CA_CERT__|${KINE_CA_CERT_B64}|g" \
    -e "s|__SERVER_CERT__|${server_cert}|g" \
    -e "s|__SERVER_KEY__|${server_key}|g" \
    -e "s|__CLIENT_CERT__|${KINE_CLIENT_CERT_B64}|g" \
    -e "s|__CLIENT_KEY__|${KINE_CLIENT_KEY_B64}|g" \
    "${repo_root}/scripts/kine/cloud-init.yaml" > "${state_dir}/cloud-init.yaml"
}

capz::kine::provision() {
  fixture_started=true
  capz::kine::az group create --name "${KINE_RESOURCE_GROUP}" --location "${AZURE_LOCATION}" \
    --tags "capz-kine-run=${run_id}" --output none
  capz::kine::az network vnet create --resource-group "${KINE_RESOURCE_GROUP}" \
    --name kine-vnet --address-prefixes 10.250.0.0/24 \
    --subnet-name datastore --subnet-prefixes 10.250.0.0/24 --output none
  capz::kine::az network nsg create --resource-group "${KINE_RESOURCE_GROUP}" --name kine-nsg --output none
  capz::kine::az network nsg rule create --resource-group "${KINE_RESOURCE_GROUP}" \
    --nsg-name kine-nsg --name allow-control-plane --priority 100 --direction Inbound \
    --access Allow --protocol Tcp --source-address-prefixes 10.0.0.0/24 \
    --source-port-ranges '*' --destination-address-prefixes '*' --destination-port-ranges 2379 --output none
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
  capz::kine::az network nic create --resource-group "${KINE_RESOURCE_GROUP}" \
    --name kine-nic --vnet-name kine-vnet --subnet datastore \
    --private-ip-address 10.250.0.4 --network-security-group kine-nsg --output none
  capz::kine::az vm create --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
    --nics kine-nic --image Canonical:ubuntu-24_04-lts:server:latest \
    --size "${KINE_VM_SIZE:-Standard_D2s_v5}" --admin-username azureuser \
    --ssh-key-values "${state_dir}/ssh.pub" --custom-data "${state_dir}/cloud-init.yaml" \
    --os-disk-size-gb 64 --storage-sku StandardSSD_LRS --output none
  vm_created=true
  capz::kine::az vm run-command invoke --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
    --command-id RunShellScript --scripts \
    'set -eu; timeout 1200 cloud-init status --wait; /usr/local/bin/check-kine' \
    --output json > "${artifact_dir}/readiness.json"
  # Run Command can return HTTP success even when the guest script failed.
  if ! jq -r '.value[].message' "${artifact_dir}/readiness.json" | grep -qx KINE_READY; then
    echo "Kine gRPC readiness failed; see ${artifact_dir}/readiness.json." >&2
    return 1
  fi
}

capz::kine::cleanup() {
  local result=$1 exists owner
  trap - EXIT
  if [[ "${vm_created}" == true ]]; then
    if ! capz::kine::az vm run-command invoke --resource-group "${KINE_RESOURCE_GROUP}" --name kine \
      --command-id RunShellScript --scripts \
      'journalctl --no-pager -u kine -n 60; tail -n 60 /var/log/cloud-init-output.log; cloud-init status --long; printf "cloud-init status exit code: %s\n" "$?"' \
      --output json > "${artifact_dir}/datastore-logs.json"; then
      echo "Failed to collect Kine VM logs." >&2
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
  local tool
  for tool in az openssl ssh-keygen jq; do
    command -v "${tool}" >/dev/null || { echo "Required tool not found: ${tool}" >&2; return 1; }
  done
  : "${AZURE_SUBSCRIPTION_ID:?Set the Azure subscription used by CAPZ and az CLI}"
  if [[ "${CONFORMANCE_FLAVOR:-kine-postgres}" != kine-postgres ]]; then
    echo "This wrapper requires CONFORMANCE_FLAVOR=kine-postgres." >&2
    return 1
  fi
  if [[ "${WINDOWS:-false}" != false || "${IP_FAMILY:-ipv4}" != ipv4 ||
        "${TEST_K8S:-false}" == true || "${E2E_ARGS:-}" == *use-ci-artifacts* ||
        "${E2E_ARGS:-}" == *use-pr-artifacts* ]]; then
    echo "The Kine scaffold supports Linux/IPv4 and released Kubernetes images only." >&2
    return 1
  fi
  export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.35.4}"
  if [[ ! "${KUBERNETES_VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Pin KUBERNETES_VERSION to a release such as v1.35.4." >&2
    return 1
  fi
  run_id=$(openssl rand -hex 8)
  export CLUSTER_NAME="${CLUSTER_NAME:-capz-kine-${run_id}}"
  if [[ ! "${CLUSTER_NAME}" =~ ^[a-z][a-z0-9-]{0,38}[a-z0-9]$ ]]; then
    echo "CLUSTER_NAME must be 2-40 lowercase letters, digits, or hyphens, starting with a letter." >&2
    return 1
  fi
  export KINE_RESOURCE_GROUP="${CLUSTER_NAME}-kine"
  export CONFORMANCE_FLAVOR=kine-postgres
  export CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT="${CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT:-1}"
  repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
  cd "${repo_root}"
  # shellcheck source=hack/util.sh
  source "${repo_root}/hack/util.sh"
  AZURE_LOCATION="${AZURE_LOCATION:-$(capz::util::get_random_region)}"
  export AZURE_LOCATION
  echo "Using AZURE_LOCATION: ${AZURE_LOCATION}"
  umask 077
  artifact_dir="${ARTIFACTS:-${repo_root}/_artifacts}/kine-${CLUSTER_NAME}"
  mkdir -p "${artifact_dir}"
  local group exists
  for group in "${CLUSTER_NAME}" "${KINE_RESOURCE_GROUP}"; do
    exists=$(capz::kine::az group exists --name "${group}" --output tsv)
    if [[ "${exists}" != false ]]; then
      echo "Refusing to reuse resource group ${group}; choose a fresh CLUSTER_NAME." >&2
      return 1
    fi
  done
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
  echo "Provisioning Kine v0.17.2 / PostgreSQL 16 in ${KINE_RESOURCE_GROUP}."
  capz::kine::provision
  bash "${repo_root}/scripts/ci-conformance.sh"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  capz::kine::main "$@"
fi
