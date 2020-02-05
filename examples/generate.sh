#!/bin/bash
# Copyright 2019 The Kubernetes Authors.
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

# Directories.
SOURCE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
OUTPUT_DIR=${OUTPUT_DIR:-${SOURCE_DIR}/_out}

# Binaries
ENVSUBST=${ENVSUBST:-envsubst}
command -v "${ENVSUBST}" >/dev/null 2>&1 || echo -v "Cannot find ${ENVSUBST} in path."

RANDOM_STRING=$(date | md5sum | head -c8)

# Cluster.
export CLUSTER_NAME="${CLUSTER_NAME:-capz-example-${RANDOM_STRING}}"
export VNET_NAME="${VNET_NAME:-${CLUSTER_NAME}-vnet}"
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.16.6}"
export KUBERNETES_SEMVER="${KUBERNETES_VERSION#v}"

# Machine settings.
export CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-Standard_B2ms}"
export NODE_MACHINE_TYPE="${NODE_MACHINE_TYPE:-Standard_B2ms}"

# containerd
export CONTAINERD_VERSION="${CONTAINERD_VERSION:-1.3.0}"
export CONTAINERD_SHA256="${CONTAINERD_SHA256:-47653ab55b58668ce93704e47b727b41f57d296e4048d6860daf55d6b7c2bf18}"

# Outputs.
COMPONENTS_CLUSTER_API_GENERATED_FILE=${SOURCE_DIR}/provider-components/provider-components-cluster-api.yaml
COMPONENTS_KUBEADM_GENERATED_FILE=${SOURCE_DIR}/provider-components/provider-components-kubeadm.yaml
COMPONENTS_AZURE_GENERATED_FILE=${SOURCE_DIR}/provider-components/provider-components-azure.yaml

PROVIDER_COMPONENTS_GENERATED_FILE=${OUTPUT_DIR}/provider-components.yaml
CLUSTER_GENERATED_FILE=${OUTPUT_DIR}/cluster.yaml
CONTROLPLANE_GENERATED_FILE=${OUTPUT_DIR}/controlplane.yaml
MACHINEDEPLOYMENT_GENERATED_FILE=${OUTPUT_DIR}/machinedeployment.yaml
ENV_GENERATED_FILE=${OUTPUT_DIR}/.env
CERTMANAGER_COMPONENTS_GENERATED_FILE=${OUTPUT_DIR}/cert-manager.yaml

# Overwrite flag.
OVERWRITE=0

SCRIPT=$(basename "$0")
while test $# -gt 0; do
        case "$1" in
          -h|--help)
            echo "$SCRIPT - generates input yaml files for Cluster API on azure"
            echo " "
            echo "$SCRIPT [options]"
            echo " "
            echo "options:"
            echo "-h, --help                show brief help"
            echo "-f, --force-overwrite     if file to be generated already exists, force script to overwrite it"
            exit 0
            ;;
          -f)
            OVERWRITE=1
            shift
            ;;
          --force-overwrite)
            OVERWRITE=1
            shift
            ;;
          *)
            break
            ;;
        esac
done

if [ $OVERWRITE -ne 1 ] && [ -d "$OUTPUT_DIR" ]; then
  echo "ERR: Folder ${OUTPUT_DIR} already exists. Delete it manually before running this script."
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"

# Verify the required Environment Variables are present.
: "${AZURE_SUBSCRIPTION_ID:?Environment variable empty or not defined.}"
: "${AZURE_TENANT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_SECRET:?Environment variable empty or not defined.}"


AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP:-${CLUSTER_NAME}}
export AZURE_RESOURCE_GROUP

AZURE_LOCATION=${AZURE_LOCATION:?}
export AZURE_LOCATION

# Azure Credentials.
SSH_KEY_FILE=${OUTPUT_DIR}/sshkey
rm -f "${SSH_KEY_FILE}" 2>/dev/null
ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null

echo "Machine SSH key generated in ${SSH_KEY_FILE}"

export SSH_PUBLIC_KEY=$(cat "${SSH_KEY_FILE}.pub" | base64 | tr -d '\r\n')
export AZURE_SUBSCRIPTION_ID_B64="$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
export AZURE_TENANT_ID_B64="$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_ID_B64="$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_SECRET_B64="$(echo -n "$AZURE_CLIENT_SECRET" | base64 | tr -d '\n')"

# Download cert-manager component
curl -sL https://github.com/jetstack/cert-manager/releases/download/v0.11.0/cert-manager.yaml > "${CERTMANAGER_COMPONENTS_GENERATED_FILE}"
echo "Generated ${CERTMANAGER_COMPONENTS_GENERATED_FILE}"

# Generate cluster resources.
kustomize build "${SOURCE_DIR}/cluster" | envsubst > "${CLUSTER_GENERATED_FILE}"
echo "Generated ${CLUSTER_GENERATED_FILE}"

# Generate controlplane resources.
kustomize build "${SOURCE_DIR}/controlplane" | envsubst > "${CONTROLPLANE_GENERATED_FILE}"
echo "Generated ${CONTROLPLANE_GENERATED_FILE}"

# Generate machinedeployment resources.
kustomize build "${SOURCE_DIR}/machinedeployment" | envsubst >> "${MACHINEDEPLOYMENT_GENERATED_FILE}"
echo "Generated ${MACHINEDEPLOYMENT_GENERATED_FILE}"

# Generate Cluster API provider components file.
CAPI_BRANCH=${CAPI_BRANCH:-"master"}
kustomize build "github.com/kubernetes-sigs/cluster-api/config/default/?ref=${CAPI_BRANCH}" > "${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
kustomize build "github.com/kubernetes-sigs/cluster-api/bootstrap/kubeadm/config/default/?ref=${CAPI_BRANCH}" > "${COMPONENTS_KUBEADM_GENERATED_FILE}"
echo "---" >> "${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
cat ${SOURCE_DIR}/provider-components/provider-components-kubeadm.yaml >> "${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
echo "Generated ${COMPONENTS_CLUSTER_API_GENERATED_FILE} from cluster-api - ${CAPI_BRANCH}"

# Generate Cluster API provider components file.
# curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.2.9/cluster-api-components.yaml > "${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
# echo "Downloaded ${COMPONENTS_CLUSTER_API_GENERATED_FILE}"

# Generate Kubeadm Bootstrap Provider components file.
# curl -L https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/download/v0.1.5/bootstrap-components.yaml > "${COMPONENTS_KUBEADM_GENERATED_FILE}"
# echo "Downloaded ${COMPONENTS_KUBEADM_GENERATED_FILE}"

# Generate Azure Infrastructure Provider components file.
kustomize build "${SOURCE_DIR}/../config/default" | envsubst > "${COMPONENTS_AZURE_GENERATED_FILE}"
echo "Generated ${COMPONENTS_AZURE_GENERATED_FILE}"

# Generate a single provider components file.
kustomize build "${SOURCE_DIR}/provider-components" | envsubst > "${PROVIDER_COMPONENTS_GENERATED_FILE}"
echo "Generated ${PROVIDER_COMPONENTS_GENERATED_FILE}"
echo "WARNING: ${PROVIDER_COMPONENTS_GENERATED_FILE} includes Azure credentials"

echo "CLUSTER_NAME=${CLUSTER_NAME}" > "${ENV_GENERATED_FILE}"
