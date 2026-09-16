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

if [[ $# -ne 2 ]]; then
  echo "Usage: $0 <kustomize> <yq>" >&2
  exit 1
fi

KUSTOMIZE=$1
YQ=$2
REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
generator="${REPO_ROOT}/hack/gen-upgrade-templates.sh"
tmp_dir=$(mktemp -d)
capz::verify-upgrade-templates::cleanup() {
  rm -rf "${tmp_dir}"
}
trap capz::verify-upgrade-templates::cleanup EXIT

fixture="${tmp_dir}/fixture"
config="${tmp_dir}/config.yaml"
output="${tmp_dir}/output"
mkdir "${tmp_dir}/generator-tmp"

# Redirect only this test's upstream fetches to the local fixture.
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL="${tmp_dir}/gitconfig"
git config --file "${GIT_CONFIG_GLOBAL}" "url.${fixture}.insteadOf" \
  https://github.com/kubernetes-sigs/cluster-api-provider-azure.git
git init --quiet "${fixture}"
git -C "${fixture}" config user.name "CAPZ template test"
git -C "${fixture}" config user.email "capz-template-test@example.invalid"
git -C "${fixture}" config commit.gpgsign false

mkdir -p "${fixture}/templates/test/ci/prow-windows" \
  "${fixture}/templates/test/ci/prow-aks-aso" \
  "${fixture}/templates/flavors/aks-aso" \
  "${fixture}/test/e2e/data/infrastructure-azure/v1beta1/bases"
for flavor in prow-windows prow-aks-aso; do
  cat > "${fixture}/templates/test/ci/${flavor}/kustomization.yaml" <<'EOF'
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- resources.yaml
EOF
done

for version in v0.0.1 v0.0.2; do
  cat > "${fixture}/templates/test/ci/prow-windows/resources.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: default-source
data:
  release: ${version}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: \${CLUSTER_IDENTITY_NAME}
  namespace: default
spec:
  type: WorkloadIdentity
  clientID: release-client
EOF
  cat > "${fixture}/test/e2e/data/infrastructure-azure/v1beta1/bases/mp.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: pool-source
data:
  release: ${version}
EOF
  cat > "${fixture}/templates/test/ci/prow-aks-aso/resources.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: aks-source
data:
  release: ${version}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureASOManagedControlPlane
metadata:
  name: \${CLUSTER_NAME}
spec:
  resources:
  - kind: ManagedCluster
    spec: {}
EOF
  cat > "${fixture}/templates/flavors/aks-aso/credentials.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: \${ASO_CREDENTIAL_SECRET_NAME}
  annotations:
    release-source: ${version}
stringData:
  AUTH_MODE: workloadidentity
  AZURE_CLIENT_ID: release-client
EOF
  git -C "${fixture}" add .
  git -C "${fixture}" commit --quiet -m "${version}"
  git -C "${fixture}" tag "${version}"
done

rm "${fixture}/test/e2e/data/infrastructure-azure/v1beta1/bases/mp.yaml"
git -C "${fixture}" add -u
git -C "${fixture}" commit --quiet -m "Missing pool source"
git -C "${fixture}" tag v0.0.3

cat > "${config}" <<'EOF'
variables:
  OLD_PROVIDER_UPGRADE_VERSION: v0.0.1
  LATEST_PROVIDER_UPGRADE_VERSION: v0.0.2
providers:
- name: azure
  type: InfrastructureProvider
  versions:
  - name: v0.0.1
  - name: v0.0.2
EOF
cp "${config}" "${tmp_dir}/valid-config.yaml"

capz::verify-upgrade-templates::generate() {
  TMPDIR="${tmp_dir}/generator-tmp" "${generator}" "${config}" "${output}" "${KUSTOMIZE}" "${YQ}"
}

capz::verify-upgrade-templates::assert_value() {
  local actual
  actual=$("${YQ}" -r "$2" "$1")
  if [[ "${actual}" != "$3" ]]; then
    echo "Unexpected value in $1 for $2: expected '$3', got '${actual}'" >&2
    exit 1
  fi
}

capz::verify-upgrade-templates::generate
# Compare literal clusterctl placeholders.
# shellcheck disable=SC2016
for version in v0.0.1 v0.0.2; do
  for flavor in cluster-template-prow cluster-template-prow-machine-and-machine-pool; do
    file="${output}/${version}/${flavor}.yaml"
    capz::verify-upgrade-templates::assert_value "${file}" \
      'select(.metadata.name == "default-source") | .data.release' "${version}"
    capz::verify-upgrade-templates::assert_value "${file}" \
      'select(.kind == "AzureClusterIdentity") | .spec.type' 'UserAssignedMSI'
    capz::verify-upgrade-templates::assert_value "${file}" \
      'select(.kind == "AzureClusterIdentity") | .spec.clientID' '${AZURE_CLIENT_ID_CLOUD_PROVIDER}'
    capz::verify-upgrade-templates::assert_value "${file}" \
      'select(.kind == "AzureClusterIdentity") | .spec.resourceID' \
      '/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${CI_RG:=capz-ci}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/${USER_IDENTITY:=cloud-provider-user-identity}'
  done
  capz::verify-upgrade-templates::assert_value "${output}/${version}/cluster-template-prow-machine-and-machine-pool.yaml" \
    'select(.metadata.name == "pool-source") | .data.release' "${version}"
  file="${output}/${version}/cluster-template-aks.yaml"
  capz::verify-upgrade-templates::assert_value "${file}" \
    'select(.metadata.name == "aks-source") | .data.release' "${version}"
  capz::verify-upgrade-templates::assert_value "${file}" \
    'select(.kind == "Secret") | .metadata.annotations.release-source' "${version}"
  capz::verify-upgrade-templates::assert_value "${file}" \
    'select(.kind == "Secret") | .stringData.AUTH_MODE' 'podidentity'
  capz::verify-upgrade-templates::assert_value "${file}" \
    'select(.kind == "Secret") | .stringData.AZURE_CLIENT_ID' '${AZURE_CLIENT_ID_CLOUD_PROVIDER}'
  capz::verify-upgrade-templates::assert_value "${file}" \
    'select(.kind == "AzureASOManagedControlPlane") | .spec.resources[0].spec.azureName' \
    '${CLUSTER_NAME/clusterctl-upgrade-workload-/capz-upgrade-}'
done

cp -R "${output}" "${tmp_dir}/expected"
capz::verify-upgrade-templates::generate
diff -r "${tmp_dir}/expected" "${output}"

for version in v0.0.9 v0.0.3; do
  cp "${tmp_dir}/valid-config.yaml" "${config}"
  UPGRADE_VERSION="${version}" "${YQ}" -i \
    '.variables.LATEST_PROVIDER_UPGRADE_VERSION = strenv(UPGRADE_VERSION) | .providers[0].versions[1].name = strenv(UPGRADE_VERSION)' "${config}"
  if capz::verify-upgrade-templates::generate > "${tmp_dir}/failure.log" 2>&1; then
    echo "Expected generation to fail for ${version}" >&2
    exit 1
  fi
  diff -r "${tmp_dir}/expected" "${output}"
done

cp "${tmp_dir}/valid-config.yaml" "${config}"
"${YQ}" -i '.variables.LATEST_PROVIDER_UPGRADE_VERSION = "v0.0.9"' "${config}"
if capz::verify-upgrade-templates::generate > "${tmp_dir}/failure.log" 2>&1; then
  echo "Expected generation to fail without a matching Azure provider" >&2
  exit 1
fi
grep -q "No Azure InfrastructureProvider entry for v0.0.9" "${tmp_dir}/failure.log"
diff -r "${tmp_dir}/expected" "${output}"

if [[ -n "$(find "${tmp_dir}/generator-tmp" -mindepth 1 -print -quit)" ]]; then
  echo "Temporary generator files were not cleaned up" >&2
  exit 1
fi
echo "Release-specific upgrade template checks passed"
