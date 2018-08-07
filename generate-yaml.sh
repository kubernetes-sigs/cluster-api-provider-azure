#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Generate a somewhat unique cluster name, UUID is not an option as service-accounts are limited to 30 characters in length
# and one has a 19 character prefix (i.e. 'machine-controller-'). Of the 11 remaining characters 6 are reserved for the human
# friendly cluster name, one for a dash, and 5 are left for this random string.
RANDOM_STRING=$(head -c5 < <(LC_ALL=C tr -dc 'a-zA-Z0-9' < /dev/urandom) | tr '[:upper:]' '[:lower:]')
# Human friendly cluster name, limited to 6 characters
HUMAN_FRIENDLY_CLUSTER_NAME=test1
CLUSTER_NAME=${HUMAN_FRIENDLY_CLUSTER_NAME}-${RANDOM_STRING}
RESOURCE_GROUP=clusterapi-${RANDOM_STRING}

OUTPUT_DIR=generatedconfigs
TEMPLATE_DIR=configtemplates
SSH_KEY_FILE=${OUTPUT_DIR}/sshkey

MACHINE_TEMPLATE_FILE=${TEMPLATE_DIR}/machines.yaml.template
MACHINE_GENERATED_FILE=${OUTPUT_DIR}/machines.yaml
CLUSTER_TEMPLATE_FILE=${TEMPLATE_DIR}/cluster.yaml.template
CLUSTER_GENERATED_FILE=${OUTPUT_DIR}/cluster.yaml
PROVIDERCOMPONENT_TEMPLATE_FILE=${TEMPLATE_DIR}/provider-components.yaml.template
PROVIDERCOMPONENT_GENERATED_FILE=${OUTPUT_DIR}/provider-components.yaml
ADDON_TEMPLATE_FILE=${TEMPLATE_DIR}/addons.yaml.template
ADDON_GENERATED_FILE=${OUTPUT_DIR}/addons.yaml

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

if [ $OVERWRITE -ne 1 ] && [ -f $MACHINE_GENERATED_FILE ]; then
  echo File $MACHINE_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

if [ $OVERWRITE -ne 1 ] && [ -f $CLUSTER_GENERATED_FILE ]; then
  echo File $CLUSTER_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

if [ $OVERWRITE -ne 1 ] && [ -f $PROVIDERCOMPONENT_GENERATED_FILE ]; then
  echo File $PROVIDERCOMPONENT_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

if [ $OVERWRITE -ne 1 ] && [ -f $ADDON_GENERATED_FILE ]; then
  echo File $ADDON_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

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


CLIENT_ID_ENC=$(echo -n $CLIENT_ID | base64)
CLIENT_SECRET_ENC=$(echo -n $CLIENT_SECRET | base64)
SUBSCRIPTION_ID_ENC=$(echo -n $SUBSCRIPTION_ID | base64)
TENANT_ID_ENC=$(echo -n $TENANT_ID | base64)
LOCATION="eastus"

rm tmp.auth

mkdir -p ${OUTPUT_DIR}

rm -f $SSH_KEY_FILE 2>/dev/null
ssh-keygen -t rsa -b 2048 -f $SSH_KEY_FILE -N '' 1>/dev/null

echo "Machine SSH key generated in ${SSH_KEY_FILE}"

SSH_PUBLIC_KEY=$(cat $SSH_KEY_FILE.pub | base64 | tr -d '\r\n')
SSH_PRIVATE_KEY=$(cat $SSH_KEY_FILE | base64 | tr -d '\r\n')


cat $MACHINE_TEMPLATE_FILE \
  | sed -e "s/\$LOCATION/$LOCATION/" \
  | sed -e "s/\$SSH_PUBLIC_KEY/$SSH_PUBLIC_KEY/" \
  | sed -e "s/\$SSH_PRIVATE_KEY/$SSH_PRIVATE_KEY/" \
  > $MACHINE_GENERATED_FILE

cat $CLUSTER_TEMPLATE_FILE \
  | sed -e "s/\$RESOURCE_GROUP/$RESOURCE_GROUP/" \
  | sed -e "s/\$CLUSTER_NAME/$CLUSTER_NAME/" \
  | sed -e "s/\$LOCATION/$LOCATION/" \
  > $CLUSTER_GENERATED_FILE


cat $PROVIDERCOMPONENT_TEMPLATE_FILE \
  | sed -e "s/\$AZURE_TENANT_ID/$TENANT_ID_ENC/" \
  | sed -e "s/\$AZURE_CLIENT_ID/$CLIENT_ID_ENC/" \
  | sed -e "s/\$AZURE_CLIENT_SECRET/$CLIENT_SECRET_ENC/" \
  | sed -e "s/\$AZURE_SUBSCRIPTION_ID/$SUBSCRIPTION_ID_ENC/" \
  > $PROVIDERCOMPONENT_GENERATED_FILE

# TODO: implement addon file
cat $ADDON_TEMPLATE_FILE \
  > $ADDON_GENERATED_FILE

echo -e "\nYour cluster name is '${CLUSTER_NAME}'"
