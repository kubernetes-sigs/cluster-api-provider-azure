#!/bin/bash
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

# Verify the required Environment Variables are present.
: "${AZURE_SUBSCRIPTION_ID:?Environment variable empty or not defined.}"
: "${AZURE_TENANT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_ID:?Environment variable empty or not defined.}"
: "${AZURE_CLIENT_SECRET:?Environment variable empty or not defined.}"

make envsubst

export REGISTRY="${REGISTRY:-registry.local/fake}"

# Cluster settings.
export CLUSTER_NAME="${CLUSTER_NAME:-capz-test}"
export AZURE_VNET_NAME=${CLUSTER_NAME}-vnet
export AZURE_ENVIRONMENT="AzurePublicCloud"

# Azure settings.
export AZURE_LOCATION="${AZURE_LOCATION:-southcentralus}"
export AZURE_RESOURCE_GROUP=${CLUSTER_NAME}
export AZURE_SUBSCRIPTION_ID_B64="$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
export AZURE_TENANT_ID_B64="$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_ID_B64="$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_SECRET_B64="$(echo -n "$AZURE_CLIENT_SECRET" | base64 | tr -d '\n')"

# Machine settings.
export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-3}
export AZURE_CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-Standard_D2s_v3}"
export AZURE_NODE_MACHINE_TYPE="${NODE_MACHINE_TYPE:-Standard_D2s_v3}"
export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-2}
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.18.6}"
export TEMPLATE_PATH="${TEMPLATE_PATH:-cluster-template.yaml}"

# Generate SSH key.
SSH_KEY_FILE=${SSH_KEY_FILE:-""}
if ! [ -n "$SSH_KEY_FILE" ]; then
    SSH_KEY_FILE=.sshkey
    rm -f "${SSH_KEY_FILE}" 2>/dev/null
    ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
    echo "Machine SSH key generated in ${SSH_KEY_FILE}"
fi
export AZURE_SSH_PUBLIC_KEY=$(cat "${SSH_KEY_FILE}.pub" | base64 | tr -d '\r\n')

export AZURE_JSON_B64=$(echo '{
    "cloud": "${AZURE_ENVIRONMENT:="AzurePublicCloud"}",
    "tenantId": "${AZURE_TENANT_ID}",
    "subscriptionId": "${AZURE_SUBSCRIPTION_ID}",
    "aadClientId": "${AZURE_CLIENT_ID}",
    "aadClientSecret": "${AZURE_CLIENT_SECRET}",
    "resourceGroup": "${CLUSTER_NAME}",
    "securityGroupName": "${CLUSTER_NAME}-node-nsg",
    "location": "${AZURE_LOCATION}",
    "vmType": "vmss",
    "vnetName": "${AZURE_VNET_NAME:=$CLUSTER_NAME-vnet}",
    "vnetResourceGroup": "${CLUSTER_NAME}",
    "subnetName": "${CLUSTER_NAME}-node-subnet",
    "routeTableName": "${CLUSTER_NAME}-node-routetable",
    "loadBalancerSku": "standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}' | "${PWD}/hack/tools/bin/envsubst" | base64 | tr -d '\r\n')

echo "================ DOCKER BUILD ==============="
PULL_POLICY=IfNotPresent make modules docker-build

echo "================ MAKE CLEAN ==============="
make clean

echo "================ KIND RESET ==============="
make kind-reset

echo "================ CREATE CLUSTER ==============="
make create-cluster
