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
set +o xtrace

# Install kubectl
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# shellcheck source=hack/common-vars.sh
source "${REPO_ROOT}/hack/common-vars.sh"

make --directory="${REPO_ROOT}" "${KUBECTL##*/}"

# Test cloud provider config with shorter cache ttl
CLOUD_PROVIDER_CONFIG="https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/tests/k8s-azure/manifest/cluster-api/cloud-config-vmss-short-cache-ttl.json"
if [[ -n "${CUSTOM_CLOUD_PROVIDER_CONFIG:-}" ]]; then
  CLOUD_PROVIDER_CONFIG="${CUSTOM_CLOUD_PROVIDER_CONFIG:-}"
fi

curl --retry 3 -sL -o tmp_azure_json "${CLOUD_PROVIDER_CONFIG}"
envsubst < tmp_azure_json > azure_json
"${KUBECTL}" delete secret "${CLUSTER_NAME}-control-plane-azure-json" -n default || true
"${KUBECTL}" create secret generic "${CLUSTER_NAME}-control-plane-azure-json" -n default \
  --from-file=control-plane-azure.json=azure_json \
  --from-file=worker-node-azure.json=azure_json
rm tmp_azure_json azure_json
