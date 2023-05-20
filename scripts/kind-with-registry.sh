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

# Install kubectl and kind
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
KIND="${REPO_ROOT}/hack/tools/bin/kind"
AZWI_ENABLED=${AZWI:-}
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${KIND##*/}"

# Export desired cluster name; default is "capz"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capz}"
export KIND_CLUSTER_NAME

if [[ "$("${KIND}" get clusters)" =~ .*"${KIND_CLUSTER_NAME}".* ]]; then
  echo "cluster already exists, moving on"
  exit 0
fi

# 1. Create registry container unless it already exists
reg_name='kind-registry'
reg_port="${KIND_REGISTRY_PORT:-5000}"
if [ "$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
  docker run \
    -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" \
    registry:2
fi

# To use workload identity, service account signing key pairs base64 encoded should be exposed via the
# env variables. The function creates the key pair files after reading it from the env variables.
function checkAZWIENVPreReqsAndCreateFiles() {
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
  SERVICE_ACCOUNT_ISSUER="${SERVICE_ACCOUNT_ISSUER:-https://oidcissuercapzci.blob.core.windows.net/oidc-capzci/}"
}

# This function create a kind cluster for Workload identity which requires key pairs path
# to be mounted on the kind cluster and hence extra mount flags are required.
function createKindForAZWI() {
  echo "creating azwi kind"
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
    [plugins."io.containerd.grpc.v1.cri".registry]
       config_path = "/etc/containerd/certs.d"
EOF
}

# 2. Create kind cluster with containerd registry config dir enabled
# TODO: kind will eventually enable this by default and this patch will
# be unnecessary.
#
# See:
# https://github.com/kubernetes-sigs/kind/issues/2875
# https://github.com/containerd/containerd/blob/main/docs/cri/config.md#registry-configuration
# See: https://github.com/containerd/containerd/blob/main/docs/hosts.md
if [ "$AZWI_ENABLED" == 'true' ]
 then
   echo "azwi is enabled..."
   checkAZWIENVPreReqsAndCreateFiles
   createKindForAZWI
else
  echo "azwi is not enabled..."
 cat <<EOF | ${KIND} create cluster --name "${KIND_CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
EOF
fi

# 3. Add the registry config to the nodes
#
# This is necessary because localhost resolves to loopback addresses that are
# network-namespace local.
# In other words: localhost in the container is not localhost on the host.
#
# We want a consistent name that works from both ends, so we tell containerd to
# alias localhost:${reg_port} to the registry container when pulling images
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${reg_port}"
for node in $(${KIND} get nodes); do
  docker exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | docker exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${reg_name}:5000"]
EOF
done

# 4. Connect the registry to the cluster network if not already connected
# This allows kind to bootstrap the network but ensures they're on the same network
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
  docker network connect "kind" "${reg_name}"
fi

# 5. Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
"${KIND}" get kubeconfig -n "${KIND_CLUSTER_NAME}" > "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig"
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

# Wait 90s for the control plane node to be ready
"${KUBECTL}" --kubeconfig "${REPO_ROOT}/${KIND_CLUSTER_NAME}.kubeconfig" wait node "${KIND_CLUSTER_NAME}-control-plane" --for=condition=ready --timeout=90s
