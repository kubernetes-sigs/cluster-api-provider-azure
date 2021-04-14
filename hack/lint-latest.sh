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

set -o nounset
set -o pipefail

root=$(dirname "${BASH_SOURCE[0]}")
abs_root=$(cd "${root}/.." || exit 2; pwd)
found=$(find "${abs_root}" -type f -name '*.go' -print0 | xargs -0 grep -Ei 'github.com/Azure/azure-sdk-for-go/.+/latest/.+')
if [[ -n ${found} ]]; then
  echo "Found usages of the 'latest' floating Azure API version. Only specific versions of the Azure APIs are allowed. Replace the following occurrences of latest with a specific date version described here: https://github.com/Azure/azure-sdk-for-go#versioning."
  echo "${found}"
  exit 1
fi
