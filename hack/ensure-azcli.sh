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

if [[ -z "$(command -v az)" ]]; then
  echo "installing Azure CLI"
  apt-get update && apt-get install -y ca-certificates curl apt-transport-https lsb-release gnupg
  curl --retry 3 -sL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor | tee /etc/apt/trusted.gpg.d/microsoft.gpg > /dev/null
  AZ_REPO=$(lsb_release -cs)
  echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ ${AZ_REPO} main" | tee /etc/apt/sources.list.d/azure-cli.list
  apt-get update && apt-get install -y azure-cli

  if [[ -n "${AZURE_FEDERATED_TOKEN_FILE:-}" ]]; then
    echo "Logging in with federated token"
    # AZURE_CLIENT_ID has been overloaded with Azure Workload ID in the preset-azure-cred-wi.
    # This is done to avoid exporting Azure Workload ID as AZURE_CLIENT_ID in the test scenarios.
    az login --service-principal -u "${AZURE_CLIENT_ID}" -t "${AZURE_TENANT_ID}" --federated-token "$(cat "${AZURE_FEDERATED_TOKEN_FILE}")" > /dev/null

    # Use --auth-mode "login" in az storage commands to use RBAC permissions of login identity. This is a well known ENV variable the Azure cli
    export AZURE_STORAGE_AUTH_MODE="login"
  else
    echo "AZURE_FEDERATED_TOKEN_FILE environment variable must be set to path location of token file"
    exit 1
  fi
fi
