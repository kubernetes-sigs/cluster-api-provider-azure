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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}"

CM_NAMES=("calico-addon" "calico-ipv6-addon" "calico-dual-stack-addon" "calico-windows-addon")
CM_FILES=("calico.yaml" "calico-ipv6.yaml" "calico-dual-stack.yaml" "windows/calico")
for i in "${!CM_NAMES[@]}"; do
	"${KUBECTL}" create configmap "${CM_NAMES[i]}" --from-file="${REPO_ROOT}/templates/addons/${CM_FILES[i]}" --dry-run=client -o yaml | kubectl apply -f -
done
