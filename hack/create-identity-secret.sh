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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"

kubectl create secret generic "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" --from-literal=clientSecret="${AZURE_CLIENT_SECRET}"
kubectl label secret "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" "clusterctl.cluster.x-k8s.io/move-hierarchy=true" --overwrite=true
