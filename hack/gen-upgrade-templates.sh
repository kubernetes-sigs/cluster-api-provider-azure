#!/usr/bin/env bash

# Copyright The Kubernetes Authors.
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

if [[ $# -ne 4 ]]; then
  echo "Usage: $0 <e2e-config> <output-directory> <kustomize> <yq>" >&2
  exit 1
fi

config=$1
output_dir=$2
KUSTOMIZE=$3
YQ=$4
REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
overlays="${REPO_ROOT}/test/e2e/data/infrastructure-azure/upgrade"

old_version=$("${YQ}" -er '.variables.OLD_PROVIDER_UPGRADE_VERSION' "${config}")
latest_version=$("${YQ}" -er '.variables.LATEST_PROVIDER_UPGRADE_VERSION' "${config}")
versions=("${old_version}")
if [[ "${latest_version}" != "${old_version}" ]]; then
  versions+=("${latest_version}")
fi

for version in "${versions[@]}"; do
  if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-+][a-zA-Z0-9.-]+)?$ ]]; then
    echo "Invalid CAPZ release tag: ${version}" >&2
    exit 1
  fi
  if ! UPGRADE_VERSION="${version}" "${YQ}" -e \
    '.providers[] | select(.name == "azure" and .type == "InfrastructureProvider") | .versions[] | select(.name == strenv(UPGRADE_VERSION))' \
    "${config}" > /dev/null; then
    echo "No Azure InfrastructureProvider entry for ${version} in ${config}" >&2
    exit 1
  fi
done

tmp_dir=$(mktemp -d)
capz::gen-upgrade-templates::cleanup() {
  rm -rf "${tmp_dir}"
}
trap capz::gen-upgrade-templates::cleanup EXIT

cp -R "${overlays}/templates" "${tmp_dir}/overlays"
cp -R "${overlays}/patches" "${tmp_dir}/patches"
git init --quiet "${tmp_dir}/source"

for version in "${versions[@]}"; do
  git -C "${tmp_dir}/source" fetch --quiet --depth=1 \
    https://github.com/kubernetes-sigs/cluster-api-provider-azure.git "refs/tags/${version}"
  git -C "${tmp_dir}/source" checkout --quiet --detach FETCH_HEAD
  echo "Generating upgrade templates for ${version} from $(git -C "${tmp_dir}/source" rev-parse HEAD)"
  mkdir -p "${tmp_dir}/rendered/${version}"
  for flavor in cluster-template-prow cluster-template-prow-machine-and-machine-pool cluster-template-aks; do
    "${KUSTOMIZE}" build "${tmp_dir}/overlays/${flavor}" --load-restrictor LoadRestrictionsNone \
      > "${tmp_dir}/rendered/${version}/${flavor}.yaml"
  done
done

# Keep the existing outputs if any release fails to fetch or render.
for version in "${versions[@]}"; do
  mkdir -p "${output_dir}/${version}"
  cp "${tmp_dir}/rendered/${version}/"*.yaml "${output_dir}/${version}/"
done
