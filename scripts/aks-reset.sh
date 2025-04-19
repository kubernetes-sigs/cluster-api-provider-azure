#!/usr/bin/env bash
# Copyright 2024 The Kubernetes Authors.
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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# shellcheck source=hack/ensure-azcli.sh
source "${REPO_ROOT}/hack/ensure-azcli.sh"

TILT_SETTINGS_FILE="${REPO_ROOT}/tilt-settings.yaml"
AZURE_IDENTITY_ID_FILEPATH="${REPO_ROOT}/azure_identity_id"
AKS_MGMT_CONFIG_FILE="${REPO_ROOT}/aks-mgmt.config"

function main() {
    if [ ! -f "$TILT_SETTINGS_FILE" ]; then
        echo "No tilt-settings.yaml found. Nothing to reset."
        exit 0
    fi

    # Check if yq is installed
    if ! command -v yq &> /dev/null; then
        echo "yq is required but not installed. Please install yq first."
        exit 1
    fi
    
    # Get the list of AKS contexts from tilt-settings.yaml
    local aks_contexts
    aks_contexts=$(yq '.allowed_contexts[]? | select(. == "*aks-mgmt-capz-*")' "$TILT_SETTINGS_FILE")
    
    if [ -z "$aks_contexts" ]; then
        echo "No AKS clusters found in tilt-settings.yaml. Nothing to reset."
        exit 0
    fi
    
    echo "Found AKS clusters to delete:"
    echo "$aks_contexts"
    
    # Delete each AKS cluster and its resource group
    while IFS= read -r context; do
        if [ -n "$context" ]; then
            # Extract resource group name from context (assuming pattern is consistent)
            local resource_group="${context}"
            
            echo "Deleting AKS cluster and resource group: $resource_group"
            
            # Delete the resource group (which includes the AKS cluster)
            if az group exists --name "$resource_group" | grep -q "true"; then
                az group delete --name "$resource_group" --yes --no-wait
                echo "Deletion of resource group $resource_group initiated (running in background)"
            else
                echo "Resource group $resource_group does not exist"
            fi
            
            # Remove the context from kubectl config
            if kubectl config get-contexts "$context" &>/dev/null; then
                kubectl config delete-context "$context" || true
            fi
        fi
    done <<< "$aks_contexts"
    
    # Clean up tilt-settings.yaml - remove AKS contexts
    yq eval -i 'del(.allowed_contexts[] | select(. == "*aks-mgmt-capz-*"))' "$TILT_SETTINGS_FILE"
    
    # Clean up aks_as_mgmt_settings
    yq eval -i 'del(.aks_as_mgmt_settings)' "$TILT_SETTINGS_FILE"
    
    # Clean up other AKS-related files
    if [ -f "$AZURE_IDENTITY_ID_FILEPATH" ]; then
        rm -f "$AZURE_IDENTITY_ID_FILEPATH"
        echo "Removed $AZURE_IDENTITY_ID_FILEPATH"
    fi
    
    if [ -f "$AKS_MGMT_CONFIG_FILE" ]; then
        rm -f "$AKS_MGMT_CONFIG_FILE"
        echo "Removed $AKS_MGMT_CONFIG_FILE"
    fi
    
    echo "AKS reset completed successfully"
}

main