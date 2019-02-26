#!/bin/bash
# Copyright 2018 The Kubernetes Authors.
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

# Directories.
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
OUTPUT_DIR=${OUTPUT_DIR:-${DIR}/out}
ENVSUBST=${ENVSUBST:-envsubst}

RANDOM_STRING=$(date | md5sum | head -c8)

# Azure settings.
export LOCATION="${LOCATION:-eastus2}"
export RESOURCE_GROUP="${RESOURCE_GROUP:-capi-${RANDOM_STRING}}"

# Cluster name.
export CLUSTER_NAME="${CLUSTER_NAME:-test1}"

# Manager image.
export MANAGER_IMAGE="${MANAGER_IMAGE:-quay.io/k8s/cluster-api-azure-controller:0.1.0-alpha.3}"
export MANAGER_IMAGE_PULL_POLICY=${MANAGER_IMAGE_PULL_POLICY:-IfNotPresent}

# Machine settings.
export CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-Standard_B2ms}"
export NODE_MACHINE_TYPE="${NODE_MACHINE_TYPE:-Standard_B2ms}"

# Credential locations.
SSH_KEY_FILE=${OUTPUT_DIR}/sshkey
CREDENTIALS_FILE=${OUTPUT_DIR}/credentials.sh

# Templates.
CLUSTER_TEMPLATE_FILE=${DIR}/cluster.yaml.template
CLUSTER_GENERATED_FILE=${OUTPUT_DIR}/cluster.yaml
# TODO: Change the machine template once nodes are implemented
MACHINES_TEMPLATE_FILE=${DIR}/machines_no_node.yaml.template
MACHINES_GENERATED_FILE=${OUTPUT_DIR}/machines.yaml
MANAGER_PATCH_TEMPLATE_FILE=${DIR}/azure_manager_image_patch.yaml.template
MANAGER_PATCH_GENERATED_FILE=${OUTPUT_DIR}/azure_manager_image_patch.yaml
ADDONS_FILE=${OUTPUT_DIR}/addons.yaml

# Overwrite flag.
OVERWRITE=0

SCRIPT=$(basename $0)
while test $# -gt 0; do
        case "$1" in
          -h|--help)
            echo "$SCRIPT - generates input yaml files for Cluster API on Azure"
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

if [ $OVERWRITE -ne 1 ] && [ -f $MACHINES_GENERATED_FILE ]; then
  echo File $MACHINES_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

if [ $OVERWRITE -ne 1 ] && [ -f $CLUSTER_GENERATED_FILE ]; then
  echo File $CLUSTER_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

mkdir -p ${OUTPUT_DIR}

# If CI, then use the CI service account
if [[ ( "${TF_BUILD:+isset}" == "isset" ) || ( ${CREATE_SP:-FALSE} ) ]]; then
  echo "Skipping creating service principal..."
else
  command -v az >/dev/null 2>&1 || \
  { echo >&2 "The Azure CLI is required. Please install it to continue."; exit 1; }

  echo Creating service principal...
  az ad sp create-for-rbac --name "cluster-api-${RANDOM_STRING}" --sdk-auth 2>/dev/null > tmp.auth
  echo Created service principal "cluster-api-${RANDOM_STRING}"

  TMP=$(grep "\"clientId\": " tmp.auth)
  CLIENT_ID=${TMP:15:36}
  TMP=$(grep "\"clientSecret\": " tmp.auth)
  CLIENT_SECRET=${TMP:19:36}
  TMP=$(grep "\"subscriptionId\": " tmp.auth)
  SUBSCRIPTION_ID=${TMP:21:36}
  TMP=$(grep "\"tenantId\": " tmp.auth)
  TENANT_ID=${TMP:15:36}
  rm tmp.auth
  printf "AZURE_CLIENT_ID=%s\n" "$CLIENT_ID" > "$CREDENTIALS_FILE"
  printf "AZURE_CLIENT_SECRET=%s\n" "$CLIENT_SECRET" >> "$CREDENTIALS_FILE"
  printf "AZURE_SUBSCRIPTION_ID=%s\n" "$SUBSCRIPTION_ID" >> "$CREDENTIALS_FILE"
  printf "AZURE_TENANT_ID=%s\n" "$TENANT_ID" >> "$CREDENTIALS_FILE"
fi

rm -f ${SSH_KEY_FILE} 2>/dev/null
ssh-keygen -t rsa -b 2048 -f ${SSH_KEY_FILE} -N '' 1>/dev/null

echo "Machine SSH key generated in ${SSH_KEY_FILE}"

export SSH_PUBLIC_KEY=$(cat ${SSH_KEY_FILE}.pub | base64 | tr -d '\r\n')
export SSH_PRIVATE_KEY=$(cat ${SSH_KEY_FILE} | base64 | tr -d '\r\n')

$ENVSUBST < $CLUSTER_TEMPLATE_FILE > "${CLUSTER_GENERATED_FILE}"
echo "Done generating ${CLUSTER_GENERATED_FILE}"

$ENVSUBST < $MACHINES_TEMPLATE_FILE > "${MACHINES_GENERATED_FILE}"
echo "Done generating ${MACHINES_GENERATED_FILE}"

$ENVSUBST < $MANAGER_PATCH_TEMPLATE_FILE > "${MANAGER_PATCH_GENERATED_FILE}"
echo "Done generating ${MANAGER_PATCH_GENERATED_FILE}"

cp  ${DIR}/addons.yaml.template ${ADDONS_FILE}
echo "Done copying ${ADDONS_FILE}"

echo -e "\nYour resource group is '${RESOURCE_GROUP}' in '${LOCATION}'"
echo -e "\nYour cluster name is '${CLUSTER_NAME}'"
