#!/usr/bin/env bash

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

if [[ "${REGISTRY:-}" =~ \.azurecr\.io ]]; then
    # if we are using the prow Azure Container Registry, login.
    "${REPO_ROOT}/hack/ensure-azcli.sh"
    : "${AZURE_SUBSCRIPTION_ID:?Environment variable empty or not defined.}"
    az account set -s "${AZURE_SUBSCRIPTION_ID}"
    acrname="${REGISTRY%%.*}"
    az acr login --name "$acrname"
fi
