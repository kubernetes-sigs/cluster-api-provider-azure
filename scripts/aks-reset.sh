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
    
    # Get all contexts from tilt-settings.yaml
    local contexts
    contexts=$(yq '.allowed_contexts[]?' "$TILT_SETTINGS_FILE")
    
    if [ -z "$contexts" ]; then
        echo "No contexts found in tilt-settings.yaml. Nothing to reset."
        exit 0
    fi
    
    echo "Scanning all contexts from tilt-settings.yaml..."
    
    # Track which AKS clusters were found for cleanup
    local found_aks_clusters=()
    
    # Check each context to see if it's an AKS cluster
    while IFS= read -r context; do
        if [ -n "$context" ]; then
            echo "Checking context: $context"
            
            # Try to get the AKS cluster info from Azure
            if az aks show --name "$context" --resource-group "$context" &>/dev/null; then
                echo "Found AKS cluster: $context"
                found_aks_clusters+=("$context")
            else
                echo "Context $context is not an AKS cluster or doesn't exist in Azure."
            fi
        fi
    done <<< "$contexts"
    
    if [ ${#found_aks_clusters[@]} -eq 0 ]; then
        echo "No AKS clusters found. Nothing to reset."
        exit 0
    fi
    
    echo "Found ${#found_aks_clusters[@]} AKS clusters to delete:"
    printf "  %s\n" "${found_aks_clusters[@]}"
    
    # Delete each AKS cluster and its resource group
    for cluster in "${found_aks_clusters[@]}"; do
        echo "Deleting AKS cluster and resource group: $cluster"
        
        # Delete the resource group (which includes the AKS cluster)
        if az group exists --name "$cluster" | grep -q "true"; then
            az group delete --name "$cluster" --yes --no-wait
            echo "Deletion of resource group $cluster initiated (running in background)"
        else
            echo "Resource group $cluster does not exist"
        fi
        
        # Remove the context from kubectl config
        if kubectl config get-contexts "$cluster" &>/dev/null; then
            kubectl config delete-context "$cluster" || true
            echo "Removed kubectl context for $cluster"
        fi
    done
    
    # Remove the AKS contexts from tilt-settings.yaml
    for cluster in "${found_aks_clusters[@]}"; do
        yq eval -i "del(.allowed_contexts[] | select(. == \"$cluster\"))" "$TILT_SETTINGS_FILE"
    done
    
    # Clean up aks_as_mgmt_settings if it exists
    if yq eval '.aks_as_mgmt_settings' "$TILT_SETTINGS_FILE" | grep -qv "null"; then
        echo "Cleaning up aks_as_mgmt_settings from tilt-settings.yaml"
        yq eval -i 'del(.aks_as_mgmt_settings)' "$TILT_SETTINGS_FILE"
    fi
    
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
