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

if [[ -z "$REPO_ROOT" ]]; then
  echo >&2 "REPO_ROOT must be set"
  exit 1
fi

# shellcheck disable=SC2034
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
# shellcheck disable=SC2034
KUSTOMIZE="${REPO_ROOT}/hack/tools/bin/kustomize"
# shellcheck disable=SC2034
ENVSUBST="${REPO_ROOT}/hack/tools/bin/envsubst"
# shellcheck disable=SC2034
YQ="${REPO_ROOT}/hack/tools/bin/yq"
