#!/bin/bash

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

set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

cd "${REPO_ROOT}" || exit

echo "*** Running go-apidiff ***"
APIDIFF=$(APIDIFF_OLD_COMMIT="${PULL_BASE_SHA}" make apidiff 2> /dev/null)

if grep -qE '^sigs\.k8s\.io/cluster-api-provider-azure/(exp/)?api/' <<< "$APIDIFF"; then
    echo "${APIDIFF}"
    exit 1
else
    echo "No files under api/ or exp/api/ changed."
fi
