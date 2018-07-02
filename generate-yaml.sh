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

# Generate a somewhat unique cluster name, UUID is not an option as service-accounts are limited to 30 characters in length
# and one has a 19 character prefix (i.e. 'machine-controller-'). Of the 11 remaining characters 6 are reserved for the human
# friendly cluster name, one for a dash, and 5 are left for this random string.
RANDOM_STRING=$(head -c5 < <(LC_ALL=C tr -dc 'a-zA-Z0-9' < /dev/urandom) | tr '[:upper:]' '[:lower:]')
# Human friendly cluster name, limited to 6 characters
HUMAN_FRIENDLY_CLUSTER_NAME=test1
CLUSTER_NAME=${HUMAN_FRIENDLY_CLUSTER_NAME}-${RANDOM_STRING}

OUTPUT_DIR=generatedconfigs
TEMPLATE_DIR=configtemplates

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

mkdir -p ${OUTPUT_DIR}

cat $MACHINE_TEMPLATE_FILE > $MACHINE_GENERATED_FILE
cat $CLUSTER_TEMPLATE_FILE > $CLUSTER_GENERATED_FILE
cat $PROVIDERCOMPONENT_TEMPLATE_FILE > $PROVIDERCOMPONENT_GENERATED_FILE
cat $ADDON_TEMPLATE_FILE > $ADDON_GENERATED_FILE

#cat $MACHINE_TEMPLATE_FILE \
#  | sed -e "s/\$ZONE/$ZONE/" \
#  > $MACHINE_GENERATED_FILE

#cat $CLUSTER_TEMPLATE_FILE \
#  | sed -e "s/\$GCLOUD_PROJECT/$GCLOUD_PROJECT/" \
#  | sed -e "s/\$CLUSTER_NAME/$CLUSTER_NAME/" \
#  > $CLUSTER_GENERATED_FILE

#cat $PROVIDERCOMPONENT_TEMPLATE_FILE \
#  | sed -e "s/\$MACHINE_CONTROLLER_SA_KEY/$MACHINE_CONTROLLER_SA_KEY/" \
#  | sed -e "s/\$CLUSTER_NAME/$CLUSTER_NAME/" \
#  | sed -e "s/\$MACHINE_CONTROLLER_SSH_USER/$MACHINE_CONTROLLER_SSH_USER/" \
#  | sed -e "s/\$MACHINE_CONTROLLER_SSH_PUBLIC/$MACHINE_CONTROLLER_SSH_PUBLIC/" \
#  | sed -e "s/\$MACHINE_CONTROLLER_SSH_PRIVATE/$MACHINE_CONTROLLER_SSH_PRIVATE/" \
#  > $PROVIDERCOMPONENT_GENERATED_FILE

#cat $ADDON_TEMPLATE_FILE \
#  | sed -e "s/\$GCLOUD_PROJECT/$GCLOUD_PROJECT/" \
#  | sed -e "s/\$CLUSTER_NAME/$CLUSTER_NAME/" \
#  | sed "s/\$LOADBALANCER_SA_KEY/$LOADBALANCER_SA_KEY/" \
#  > $ADDON_GENERATED_FILE

echo -e "\nYour cluster name is '${CLUSTER_NAME}'"
