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
# set -o verbose

root=$(dirname "${BASH_SOURCE[0]}")/..
gpudir=${root}/templates/flavors/nvidia-gpu
workdir=${gpudir}/tmp
namespace=gpu-operator-resources
# See https://github.com/NVIDIA/gpu-operator/releases for available versions of the gpu-operator chart.
gpu_operator_version=${1:-1.10.1}

helm repo add nvidia https://nvidia.github.io/gpu-operator \
    && helm repo update
helm template gpu-operator nvidia/gpu-operator \
    --create-namespace \
    --include-crds \
    --namespace ${namespace} \
    --output-dir "${workdir}" \
    --version "${gpu_operator_version}"
cat "${workdir}"/gpu-operator/crds/*.yaml > "${gpudir}"/clusterpolicy-crd.yaml
cat <<EOF > "${gpudir}"/gpu-operator-components.tmp
---
# Source: gpu-operator/templates/resources-namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
  labels:
    app.kubernetes.io/component: "gpu-operator"
    openshift.io/cluster-monitoring: "true"
EOF
for f in "${workdir}"/gpu-operator/charts/node-feature-discovery/templates/*.yaml "${workdir}"/gpu-operator/templates/*.yaml; do
    # Ensure that each resource has an explicit namespace.
    if grep -q '^ *namespace: .*' "${f}"; then
        cat "${f}" >> "${gpudir}"/gpu-operator-components.tmp
    else
        sed -r "s/^( *)name: .*/&\n\1namespace: ${namespace}/" "${f}" >> "${gpudir}"/gpu-operator-components.tmp
    fi
done
mv "${gpudir}"/gpu-operator-components.tmp "${gpudir}"/gpu-operator-components.yaml
rm -rf "${workdir}"

make -C "${root}" generate-flavors
