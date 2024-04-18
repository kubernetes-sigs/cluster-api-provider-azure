#!/usr/bin/env bash

# Copyright 2014 The Kubernetes Authors.
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
# shellcheck source=hack/common-vars.sh
source "${REPO_ROOT}/hack/common-vars.sh"

make --directory="${REPO_ROOT}" "${KUSTOMIZE##*/}"

flavors_dir="${REPO_ROOT}/templates/flavors/"
ci_dir="${REPO_ROOT}/templates/test/ci/"
dev_dir="${REPO_ROOT}/templates/test/dev/"

for name in $(find "${flavors_dir}"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v base); do
  ${KUSTOMIZE} build --load-restrictor LoadRestrictionsNone "${flavors_dir}${name}" > "${REPO_ROOT}/templates/cluster-template-${name}.yaml"
done
# move the default template to the default file expected by clusterctl
mv "${REPO_ROOT}/templates/cluster-template-default.yaml" "${REPO_ROOT}/templates/cluster-template.yaml"

for name in $(find "${ci_dir}"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v patches); do
  ${KUSTOMIZE} build --load-restrictor LoadRestrictionsNone "${ci_dir}${name}" > "${ci_dir}cluster-template-${name}.yaml"
done

for name in $(find "${dev_dir}"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v patches); do
  ${KUSTOMIZE} build --load-restrictor LoadRestrictionsNone "${dev_dir}${name}" > "${dev_dir}cluster-template-${name}.yaml"
done
