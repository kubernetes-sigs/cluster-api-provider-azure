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
#set -o verbose

root=$(dirname "${BASH_SOURCE[0]}")/..
kustomize="${root}/hack/tools/bin/kustomize"
flavors_dir="${root}/templates/flavors/"
ci_dir="${root}/templates/test/ci/"
dev_dir="${root}/templates/test/dev/"

find "${flavors_dir}"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v base | xargs -I {} sh -c "${kustomize} build --load-restrictor LoadRestrictionsNone --reorder none ${flavors_dir}{} > ${root}/templates/cluster-template-{}.yaml"
# move the default template to the default file expected by clusterctl
mv "${root}/templates/cluster-template-default.yaml" "${root}/templates/cluster-template.yaml"

find "${ci_dir}"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v patches | xargs -I {} sh -c "${kustomize} build --load-restrictor LoadRestrictionsNone --reorder none ${ci_dir}{} > ${ci_dir}cluster-template-{}.yaml"
find "${dev_dir}"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v patches | xargs -I {} sh -c "${kustomize} build --load-restrictor LoadRestrictionsNone --reorder none ${dev_dir}{} > ${dev_dir}cluster-template-{}.yaml"
