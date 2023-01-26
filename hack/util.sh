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

CURL_RETRIES=3

capz::util::get_latest_ci_version() {
    release="${1}"
    ci_version_url="https://dl.k8s.io/ci/latest-${release}.txt"
    if ! curl --retry "${CURL_RETRIES}" -fL "${ci_version_url}" > /dev/null; then
        ci_version_url="https://dl.k8s.io/ci/latest.txt"
    fi
    curl --retry "${CURL_RETRIES}" -sSL "${ci_version_url}"
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

capz::util::should_build_ccm() {
    if [[ -n "${TEST_CCM:-}" ]]; then
        echo "true" && return
    fi
    if [[ "${E2E_ARGS:-}" == "-kubetest.use-ci-artifacts" ]]; then
        echo "true" && return
    fi
    echo "false"
}

# all test regions must support AvailabilityZones
capz::util::get_random_region() {
    local REGIONS=("canadacentral" "eastus" "eastus2" "northeurope" "uksouth" "westeurope" "westus2" "westus3")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}
# all regions below must have GPU availability for the chosen GPU VM SKU
capz::util::get_random_region_gpu() {
    local REGIONS=("eastus" "eastus2" "northeurope" "uksouth" "westeurope" "westus2")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}
# all regions below must support ExtendedLocation
capz::util::get_random_region_edgezone() {
    local REGIONS=("canadacentral")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}

capz::util::generate_ssh_key() {
    # Generate SSH key.
    AZURE_SSH_PUBLIC_KEY_FILE=${AZURE_SSH_PUBLIC_KEY_FILE:-""}
    if [ -z "${AZURE_SSH_PUBLIC_KEY_FILE}" ]; then
        echo "generating sshkey for e2e"
        SSH_KEY_FILE=.sshkey
        rm -f "${SSH_KEY_FILE}" 2>/dev/null
        ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
        AZURE_SSH_PUBLIC_KEY_FILE="${SSH_KEY_FILE}.pub"
    fi
    AZURE_SSH_PUBLIC_KEY_B64=$(base64 < "${AZURE_SSH_PUBLIC_KEY_FILE}" | tr -d '\r\n')
    export AZURE_SSH_PUBLIC_KEY_B64
    # Windows sets the public key via cloudbase-init which take the raw text as input
    AZURE_SSH_PUBLIC_KEY=$(tr -d '\r\n' < "${AZURE_SSH_PUBLIC_KEY_FILE}")
    export AZURE_SSH_PUBLIC_KEY
}

capz::util::ensure_azure_envs() {
    : "${AZURE_SUBSCRIPTION_ID:?Environment variable empty or not defined.}"
    : "${AZURE_TENANT_ID:?Environment variable empty or not defined.}"
    : "${AZURE_CLIENT_ID:?Environment variable empty or not defined.}"
    : "${AZURE_CLIENT_SECRET:?Environment variable empty or not defined.}"
}
