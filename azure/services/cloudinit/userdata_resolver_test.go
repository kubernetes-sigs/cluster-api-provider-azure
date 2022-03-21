/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudinit

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

var simpleUserDataResolver = SimpleUserDataResolver{
	Data: "bootstrap data",
}

func TestSimpleUserDataResolver(t *testing.T) {
	g := NewWithT(t)
	result, err := simpleUserDataResolver.ResolveUserData()
	g.Expect(err).To(BeNil())
	g.Expect(result).Should(Equal("bootstrap data"))
}

const bootstrapDataMime = `MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="BoundaryForTesting"

--BoundaryForTesting
content-type: text/cloud-boothook

#cloud-boothook
#!/bin/bash

# Copyright 2022 The Kubernetes Authors.
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

umask 006

IDENTITY="userID"
VAULT_NAME="my-cluster-vault"
SECRET_PREFIX="my-machine-bootstrap-secret"
CHUNKS="1"
FILE="/etc/secret-userdata.txt"
FINAL_INDEX=$((CHUNKS - 1))

# Log an error and exit.
# Args:
#   $1 Message to log with the error
#   $2 The error code to return
log::error_exit() {
  local message="${1}"
  local code="${2}"

  log::error "${message}"
  log::error "capz.cluster.x-k8s.io encrypted cloud-init script $0 exiting with status ${code}"
  exit "${code}"
}

log::success_exit() {
  log::info "capz.cluster.x-k8s.io encrypted cloud-init script $0 finished"
  exit 0
}

# Log an error but keep going.
log::error() {
  local message="${1}"
  timestamp=$(date --iso-8601=seconds)
  echo "!!! [${timestamp}] ${1}" >&2
  shift
  for message; do
    echo "    ${message}" >&2
  done
}

# Print a status line.  Formatted to show up in a stream of output.
log::info() {
  timestamp=$(date --iso-8601=seconds)
  echo "+++ [${timestamp}] ${1}"
  shift
  for message; do
    echo "    ${message}"
  done
}

check_azure_command() {
  local command="${1}"
  local code="${2}"
  local out="${3}"
  local sanitised="${out//[$'\t\r\n']/}"
  case ${code} in
  "0")
    log::info "Azure CLI reported successful execution for ${command}"
    ;;
  *)
    log::error "Azure CLI reported unknown error ${code} for ${command}"
    log::error "${sanitised}"
    ;;
  esac
}

delete_secret_value() {
  local id="${SECRET_PREFIX}-${1}"
  local out
  log::info "deleting secret from Azure Key Vault"
  set +o errexit
  set +o nounset
  set +o pipefail
  out=$(
    az keyvault secret delete --name "${id}" --vault-name "${VAULT_NAME}" 2>&1
  )
  local delete_return=$?
  set -o errexit
  set -o nounset
  set -o pipefail
  check_azure_command "KeyVault::DeleteSecret" "${delete_return}" "${out}"
  if [ ${delete_return} -ne 0 ]; then
    log::error_exit "Could not delete secret value" 2
  fi
}

delete_secrets() {
  for i in $(seq 0 ${FINAL_INDEX}); do
    delete_secret_value "$i"
  done
}

get_secret_value() {
  local chunk=$1
  local id="${SECRET_PREFIX}-${chunk}"

  log::info "getting userdata from Azure Key Vault"
  log::info "getting userdata from Azure Key Vault"

  local data
  set +o errexit
  set +o nounset
  set +o pipefail
  data=$(
    set +e
    set +o pipefail
    az keyvault secret show --name "${id}" --vault-name "${VAULT_NAME}" --query value 2>&1
  )
  local get_return=$?
  check_azure_command "KeyVault::GetSecretValue" "${get_return}" "${data}"
  set -o errexit
  set -o nounset
  set -o pipefail
  if [ ${get_return} -ne 0 ]; then
    log::error "could not get secret value, deleting secret"
    delete_secrets
    log::error_exit "could not get secret value, but secret was deleted" 1
  fi
  log::info "appending data to temporary file ${FILE}.gz"
  echo "${data}" | base64 -id >>${FILE}.gz
}

azure_login() {
  log::info "logging into azure console"

  local out
  set +o errexit
  set +o nounset
  set +o pipefail
  out=$(
    set +e
    set +o pipefail
    az login --identity -u "${IDENTITY}" 2>&1
  )
  local login_return=$?
  check_azure_command "AZ::Login" "${login_return}" "${out}"
  set -o errexit
  set -o nounset
  set -o pipefail
  if [ ${login_return} -ne 0 ]; then
    log::error "could not login to azure"
  fi
  log::info "login successful"
}

log::info "capz.cluster.x-k8s.io encrypted cloud-init script $0 started"
log::info "secret prefix: ${SECRET_PREFIX}"
log::info "secret count: ${CHUNKS}"

if test -f "${FILE}"; then
  log::info "encrypted userdata already written to disk"
  log::success_exit
fi

azure_login

for i in $(seq 0 "${FINAL_INDEX}"); do
  get_secret_value "$i"
done

#delete_secrets

log::info "decompressing userdata to ${FILE}"
gunzip "${FILE}.gz" --stdout | base64 -d > "${FILE}"
GUNZIP_RETURN=$?
if [ ${GUNZIP_RETURN} -ne 0 ]; then
  log::error_exit "could not unzip data" 4
fi

log::info "restarting cloud-init"
systemctl restart cloud-init
log::success_exit

--BoundaryForTesting
content-type: text/x-include-url

file:///etc/secret-userdata.txt

--BoundaryForTesting--
`

func TestGenerateInitDocument(t *testing.T) {
	g := NewWithT(t)
	result, err := GenerateInitDocument("userID", "my-cluster-vault", "my-machine-bootstrap-secret", 1,
		secretFetchScript, "BoundaryForTesting")
	g.Expect(err).To(BeNil())

	// remove carriage return for easier equality check
	resultWithoutCR := strings.ReplaceAll(string(result), "\r\n", "\n")
	g.Expect(resultWithoutCR).Should(Equal(bootstrapDataMime))
}
