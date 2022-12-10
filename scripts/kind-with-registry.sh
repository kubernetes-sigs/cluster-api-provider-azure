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

# Install kubectl
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
KIND="${REPO_ROOT}/hack/tools/bin/kind"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${KIND##*/}"

# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capz}"
export KIND_CLUSTER_NAME

if [[ "$("${KIND}" get clusters)" =~ .*"${KIND_CLUSTER_NAME}".* ]]; then
  echo "cluster already exists, moving on"
  exit 0
fi

reg_name='kind-registry'
reg_port="${KIND_REGISTRY_PORT:-5000}"

# create registry container unless it already exists
running="$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
if [ "${running}" != 'true' ]; then
  docker run -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" registry:2
fi

SERVICE_ACCOUNT_ISSUER="${SERVICE_ACCOUNT_ISSUER:-https://oidcissuercapzci.blob.core.windows.net/oidc-capzci/}"

if [[ -z "${SERVICE_ACCOUNT_SIGNING_PUB}" ]]; then
  echo "'SERVICE_ACCOUNT_SIGNING_PUB' is not set."
  exit 1
fi

if [[ -z "${SERVICE_ACCOUNT_SIGNING_KEY}" ]]; then
  echo "'SERVICE_ACCOUNT_SIGNING_KEY' is not set."
  exit 1
fi

mkdir -p "$HOME"/azwi/creds
echo "${SERVICE_ACCOUNT_SIGNING_PUB}" > "$HOME"/azwi/creds/sa.pub
echo  "${SERVICE_ACCOUNT_SIGNING_KEY}" > "$HOME"/azwi/creds/sa.key
# create a cluster with the local registry enabled in containerd
cat <<EOF | "${KIND}" create cluster --name "${KIND_CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
    - hostPath: $HOME/azwi/creds/sa.pub
      containerPath: /etc/kubernetes/pki/sa.pub
    - hostPath: $HOME/azwi/creds/sa.key
      containerPath: /etc/kubernetes/pki/sa.key
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        service-account-issuer: ${SERVICE_ACCOUNT_ISSUER}
        service-account-key-file: /etc/kubernetes/pki/sa.pub
        service-account-signing-key-file: /etc/kubernetes/pki/sa.key
    controllerManager:
      extraArgs:
        service-account-private-key-file: /etc/kubernetes/pki/sa.key
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:5000"]
EOF

# connect the registry to the cluster network
# (the network may already be connected)
docker network connect "kind" "${reg_name}" || true

"${KIND}" get kubeconfig -n "${KIND_CLUSTER_NAME}" > "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig"

# Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | "${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

"${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" wait node "${KIND_CLUSTER_NAME}-control-plane" --for=condition=ready --timeout=90s
