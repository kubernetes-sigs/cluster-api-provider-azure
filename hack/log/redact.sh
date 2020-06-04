#!/usr/bin/env bash

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

# Verify the required Environment Variables are present.
: "${AZURE_SUBSCRIPTION_ID:?Environment variable empty or not defined.}"
: "${AZURE_TENANT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_SECRET:?Environment variable empty or not defined.}"

echo "================ REDACTING LOGS ================"

log_files=( $(find "${ARTIFACTS:-${PWD}/_artifacts}" -type f) )
redact_vars=(
    "${AZURE_CLIENT_ID}"
    "${AZURE_CLIENT_SECRET}"
    "${AZURE_SUBSCRIPTION_ID}"
    "${AZURE_TENANT_ID}"
    "${AZURE_STANDARD_JSON_B64:-}"
    "${AZURE_VMSS_JSON_B64:-}"
    "$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
    "$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
    "$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"
    "$(echo -n "$AZURE_CLIENT_SECRET" | base64 | tr -d '\n')"
)

for log_file in "${log_files[@]}"; do
    for redact_var in "${redact_vars[@]}"; do
        # LC_CTYPE=C and LANG=C will prevent "illegal byte sequence" error from sed
        if [[ "$(uname)" == "Darwin" ]]; then
            # sed on Mac OS requires an empty string for -i flag
            LC_CTYPE=C LANG=C sed -i "" "s|${redact_var}|===REDACTED===|g" "${log_file}" || true
        else
            LC_CTYPE=C LANG=C sed -i "s|${redact_var}|===REDACTED===|g" "${log_file}" || true
        fi
    done
done

echo "All sensitive variables are redacted"
