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

###############################################################################

set -o errexit
set -o nounset
set -o pipefail

# timestamp is in RFC-3339 format to match kubetest
export TIMESTAMP="${TIMESTAMP:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
export JOB_NAME="${JOB_NAME:-"cluster-api-provider-azure-e2e"}"
if [[ -n "${REPO_OWNER:-}" ]] && [[ -n "${REPO_NAME:-}" ]] && [[ -n "${PULL_BASE_SHA:-}" ]]; then
    export BUILD_PROVENANCE="https://github.com/${REPO_OWNER:-}/${REPO_NAME:-}/pull/${PULL_NUMBER:-}/commits/${PULL_PULL_SHA:-}"
else
    export BUILD_PROVENANCE="canary"
fi
