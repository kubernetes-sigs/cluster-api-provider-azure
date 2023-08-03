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

set -o errexit
set -o nounset
set -o pipefail
set +o xtrace

parse_cred() {
    grep -E -o "\b$1[[:blank:]]*=[[:blank:]]*\"[^[:space:]\"]+\"" | cut -d '"' -f 2
}

# for Prow we use the provided AZURE_CREDENTIALS file.
# the file is expected to be in toml format.
if [[ -n "${AZURE_CREDENTIALS:-}" ]]; then
    AZURE_SUBSCRIPTION_ID="$(parse_cred SubscriptionID < "${AZURE_CREDENTIALS}")"
    AZURE_TENANT_ID="$(parse_cred TenantID < "${AZURE_CREDENTIALS}")"
    AZURE_CLIENT_ID="$(parse_cred ClientID < "${AZURE_CREDENTIALS}")"
    AZURE_CLIENT_SECRET="$(parse_cred ClientSecret < "${AZURE_CREDENTIALS}")"
    AZURE_STORAGE_ACCOUNT="$(parse_cred StorageAccountName < "${AZURE_CREDENTIALS}")"
    AZURE_STORAGE_KEY="$(parse_cred StorageAccountKey < "${AZURE_CREDENTIALS}")"

    export AZURE_SUBSCRIPTION_ID AZURE_TENANT_ID AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_STORAGE_ACCOUNT AZURE_STORAGE_KEY
fi
