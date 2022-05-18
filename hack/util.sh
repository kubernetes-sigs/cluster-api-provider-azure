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

capz::util::get_latest_ci_version() {
    release="${1}"
    ci_version_url="https://dl.k8s.io/ci/latest-${release}.txt"
    if ! curl -fL "${ci_version_url}" > /dev/null; then
        ci_version_url="https://dl.k8s.io/ci/latest.txt"
    fi
    curl -sSL "${ci_version_url}"
}

capz::util::should_build_kubernetes() {
    if [[ -n "${TEST_K8S:-}" ]]; then
        echo "true" && return
    fi
    # JOB_TYPE, REPO_OWNER, and REPO_NAME are environment variables set by a prow job -
    # https://github.com/kubernetes/test-infra/blob/master/prow/jobs.md#job-environment-variables
    if [[ "${JOB_TYPE:-}" == "presubmit" ]] && [[ "${REPO_OWNER:-}/${REPO_NAME:-}" == "kubernetes/kubernetes" ]]; then
        echo "true" && return
    fi
    echo "false"
}

# all test regions must support AvailabilityZones
capz::util::get_random_region() {
    local REGIONS=("eastus" "eastus2" "northeurope" "uksouth" "westeurope" "westus2")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}
