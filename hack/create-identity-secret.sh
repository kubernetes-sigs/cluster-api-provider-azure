#!/bin/bash
# Copyright 2021 The Kubernetes Authors.
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

# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"

if ! "${KUBECTL}" get secret -n default "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" > /dev/null; then
  "${KUBECTL}" create secret generic -n default "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" --from-literal=clientSecret="${AZURE_CLIENT_SECRET}"
fi

"${KUBECTL}" label secret -n default "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" "clusterctl.cluster.x-k8s.io/move-hierarchy=true" --overwrite=true
