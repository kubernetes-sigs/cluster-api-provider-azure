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

set -o errexit # exit immediately if a command exits with a non-zero status.
set -o nounset # exit when script tries to use undeclared variables.
set -o pipefail # make the pipeline fail if any command in it fails.

usage() {
    cat <<EOF
Usage: $(basename "$0") <tilt-settings.yaml>

This script peers Azure VNets and sets up DNS for AKS clusters.

It requires a tilt-settings.yaml file with kustomize_substitutions. The following
environment variables will be sourced from the file:
  - AKS_RESOURCE_GROUP
  - AKS_MGMT_VNET_NAME
  - CLUSTER_NAME
  - AZURE_INTERNAL_LB_PRIVATE_IP
  - APISERVER_LB_DNS_SUFFIX
  - AZURE_LOCATION
  - AKS_NODE_RESOURCE_GROUP

Additionally, you may optionally skip individual steps by setting these environment variables:

  SKIP_PEER_VNETS: Set to "true" to skip the VNET peering operations.
  SKIP_NSG_RULES: Set to "true" to skip the NSG rule checking and updates.

Examples:
  Run all steps:
      ./$(basename "$0") tilt-settings.yaml

  Skip the VNET peering operations:
      SKIP_PEER_VNETS=true ./$(basename "$0") tilt-settings.yaml

  Skip the NSG rule check and update:
      SKIP_NSG_RULES=true ./$(basename "$0") tilt-settings.yaml

EOF
    exit 1
}

error() {
    echo "ERROR: $1" >&2
    exit 1
}

source_tilt_settings() {
    [ $# -eq 0 ] && usage

    TILT_SETTINGS_FILE="$1"
    [ ! -f "$TILT_SETTINGS_FILE" ] && error "File not found: $TILT_SETTINGS_FILE"

    echo "Reading variables from $TILT_SETTINGS_FILE under 'kustomize_substitutions'..."

    # Get the list of keys under kustomize_substitutions
    VAR_KEYS=$(yq e '.kustomize_substitutions | keys | .[]' "$TILT_SETTINGS_FILE" 2>/dev/null || true)

    # If there's no such key or it's empty, VAR_KEYS will be empty
    if [ -z "$VAR_KEYS" ]; then
        echo "No variables found under 'kustomize_substitutions'."
    else
        for key in $VAR_KEYS; do
            # Read the value of each key
            value=$(yq e ".kustomize_substitutions[\"$key\"]" "$TILT_SETTINGS_FILE")
            # Export the key/value pair
            export "$key=$value"
            echo "Exported $key=$value"
        done
    fi

    echo "All variables exported"
}


peer_vnets() {
    required_vars=(
        "AKS_RESOURCE_GROUP"
        "AKS_MGMT_VNET_NAME"
        "CLUSTER_NAME"
        "AZURE_INTERNAL_LB_PRIVATE_IP"
        "APISERVER_LB_DNS_SUFFIX"
        "AZURE_LOCATION"
        "AKS_NODE_RESOURCE_GROUP"
    )

    echo "Checking required environment variables..."
    for var in "${required_vars[@]}"; do
        [ -z "${!var:-}" ] && error "$var is not set"
    done
    echo "All required environment variables are set"

    # Add timeout variable for better maintainability
    WAIT_TIMEOUT=300

    # Add DNS zone variable to avoid repetition
    DNS_ZONE="${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com"

    echo "--------Peering VNETs--------"

    # Get VNET IDs with improved error handling
    az network vnet wait --resource-group "${AKS_RESOURCE_GROUP}" --name "${AKS_MGMT_VNET_NAME}" --created --timeout 180 || error "Timeout waiting for management VNET"
    MGMT_VNET_ID=$(az network vnet show --resource-group "${AKS_RESOURCE_GROUP}" --name "${AKS_MGMT_VNET_NAME}" --query id --output tsv) || error "Failed to get management VNET ID"
    echo " 1/8 ${AKS_MGMT_VNET_NAME} found and ${MGMT_VNET_ID} found"

    az network vnet wait --resource-group "${CLUSTER_NAME}" --name "${CLUSTER_NAME}-vnet" --created --timeout 180 || error "Timeout waiting for workload VNET"
    WORKLOAD_VNET_ID=$(az network vnet show --resource-group "${CLUSTER_NAME}" --name "${CLUSTER_NAME}-vnet" --query id --output tsv) || error "Failed to get workload VNET ID"
    echo " 2/8 ${CLUSTER_NAME}-vnet found and ${WORKLOAD_VNET_ID} found"

    # Peer mgmt vnet with improved error handling
    az network vnet peering create \
        --name "mgmt-to-${CLUSTER_NAME}" \
        --resource-group "${AKS_RESOURCE_GROUP}" \
        --vnet-name "${AKS_MGMT_VNET_NAME}" \
        --remote-vnet "${WORKLOAD_VNET_ID}" \
        --allow-vnet-access true \
        --allow-forwarded-traffic true \
        --only-show-errors --output none || error "Failed to create management peering"
    echo " 3/8 mgmt-to-${CLUSTER_NAME} peering created in ${AKS_MGMT_VNET_NAME}"

    # Peer workload vnet with improved error handling
    az network vnet peering create \
        --name "${CLUSTER_NAME}-to-mgmt" \
        --resource-group "${CLUSTER_NAME}" \
        --vnet-name "${CLUSTER_NAME}-vnet" \
        --remote-vnet "${MGMT_VNET_ID}" \
        --allow-vnet-access true \
        --allow-forwarded-traffic true \
        --only-show-errors --output none || error "Failed to create workload peering"
    echo " 4/8 ${CLUSTER_NAME}-to-mgmt peering created in ${CLUSTER_NAME}-vnet"

    # Create private DNS zone with improved error handling
    az network private-dns zone create \
        --resource-group "${CLUSTER_NAME}" \
        --name "${DNS_ZONE}" \
        --only-show-errors --output none || error "Failed to create private DNS zone"
    az network private-dns zone wait \
        --resource-group "${CLUSTER_NAME}" \
        --name "${DNS_ZONE}" \
        --created --timeout "${WAIT_TIMEOUT}" \
        --only-show-errors --output none || error "Timeout waiting for private DNS zone"
    echo " 5/8 ${DNS_ZONE} private DNS zone created in ${CLUSTER_NAME}"

    # Link private DNS Zone to workload vnet with improved error handling
    az network private-dns link vnet create \
        --resource-group "${CLUSTER_NAME}" \
        --zone-name "${DNS_ZONE}" \
        --name "${CLUSTER_NAME}-to-mgmt" \
        --virtual-network "${WORKLOAD_VNET_ID}" \
        --registration-enabled false \
        --only-show-errors --output none || error "Failed to create workload DNS link"
    az network private-dns link vnet wait \
        --resource-group "${CLUSTER_NAME}" \
        --zone-name "${DNS_ZONE}" \
        --name "${CLUSTER_NAME}-to-mgmt" \
        --created --timeout "${WAIT_TIMEOUT}" \
        --only-show-errors --output none || error "Timeout waiting for workload DNS link"
    echo " 6/8 workload cluster vnet ${CLUSTER_NAME}-vnet linked with private DNS zone"

    # Link private DNS Zone to mgmt vnet with improved error handling
    az network private-dns link vnet create \
        --resource-group "${CLUSTER_NAME}" \
        --zone-name "${DNS_ZONE}" \
        --name "mgmt-to-${CLUSTER_NAME}" \
        --virtual-network "${MGMT_VNET_ID}" \
        --registration-enabled false \
        --only-show-errors --output none || error "Failed to create management DNS link"
    az network private-dns link vnet wait \
        --resource-group "${CLUSTER_NAME}" \
        --zone-name "${DNS_ZONE}" \
        --name "mgmt-to-${CLUSTER_NAME}" \
        --created --timeout "${WAIT_TIMEOUT}" \
        --only-show-errors --output none || error "Timeout waiting for management DNS link"
    echo " 7/8 management cluster vnet ${AKS_MGMT_VNET_NAME} linked with private DNS zone"

    # Create private DNS zone record with improved error handling
    az network private-dns record-set a add-record \
        --resource-group "${CLUSTER_NAME}" \
        --zone-name "${DNS_ZONE}" \
        --record-set-name "@" \
        --ipv4-address "${AZURE_INTERNAL_LB_PRIVATE_IP}" \
        --only-show-errors --output none || error "Failed to create DNS record"
    echo " 8/8 \"@\" private DNS zone record created to point ${DNS_ZONE} to ${AZURE_INTERNAL_LB_PRIVATE_IP}"
}

# New function that waits for NSG rules with prefix "NRMS-*" in the relevant resource groups,
# then creates or modifies rule-101 to allow the specified ports.
wait_and_fix_nsg_rules() {
    local allow_ports="22,443,5986,6443,53,123"
    local timeout=3000     # seconds to wait per NSG for the appearance of an NRMS-* rule
    local sleep_interval=10  # seconds between checks

    echo "Waiting for NSG rules with prefix 'NRMS-' to appear..."

    local resource_groups=("$AKS_RESOURCE_GROUP" "$AKS_NODE_RESOURCE_GROUP" "$CLUSTER_NAME")

    for rg in "${resource_groups[@]}"; do
        echo "Processing NSGs in resource group '$rg'..."
        local nsg_list=""
        local rg_start_time
        rg_start_time=$(date +%s)
        # Wait until at least one NSG is present in the resource group.
        while :; do
            nsg_list=$(az network nsg list --resource-group "$rg" --query "[].name" --output tsv)
            if [ -n "$nsg_list" ]; then
                break
            fi
            if (( $(date +%s) - rg_start_time >= timeout )); then
                error "Timeout waiting for NSGs in resource group '$rg'"
            fi
            echo "No NSGs found in '$rg' yet, waiting..."
            sleep "$sleep_interval"
        done

        for nsg in $nsg_list; do
            echo "Checking for NRMS-* rules in NSG '$nsg' in resource group '$rg'..."
            local rule_found=""
            local rule_start_time
            rule_start_time=$(date +%s)
            while :; do
                # Query NSG rules with names that start with "NRMS-".
                rule_found=$(az network nsg rule list --resource-group "$rg" --nsg-name "$nsg" --query "[?starts_with(name, 'NRMS-')].name" --output tsv)
                if [ -n "$rule_found" ]; then
                    echo "Found NRMS rule(s): $rule_found in NSG '$nsg'"
                    break
                fi
                if (( $(date +%s) - rule_start_time >= timeout )); then
                    echo "Timeout waiting for NRMS-* rules in NSG '$nsg' in RG '$rg'. Skipping NSG."
                    break
                fi
                echo "NRMS-* rules not found in NSG '$nsg', waiting..."
                sleep "$sleep_interval"
            done

            # If an NRMS-* rule is found in the NSG, then ensure rule-101 is enabled.
            if [ -n "$rule_found" ]; then
                echo "Ensuring rule-101 is configured in NSG '$nsg' of RG '$rg' to allow ports $allow_ports..."
                if az network nsg rule show --resource-group "$rg" --nsg-name "$nsg" --name "rule-101" --output none 2>/dev/null; then
                    az network nsg rule update \
                        --resource-group "$rg" \
                        --nsg-name "$nsg" \
                        --name "rule-101" \
                        --access Allow \
                        --direction Inbound \
                        --protocol Tcp \
                        --destination-port-ranges "$allow_ports" \
                        --only-show-errors --output none || error "Failed to update rule-101 in NSG '$nsg' in resource group '$rg'"
                    echo "Updated rule-101 in NSG '$nsg' in resource group '$rg'."
                else
                    az network nsg rule create \
                        --resource-group "$rg" \
                        --nsg-name "$nsg" \
                        --name "rule-101" \
                        --priority 101 \
                        --direction Inbound \
                        --access Allow \
                        --protocol Tcp \
                        --destination-port-ranges "$allow_ports" \
                        --only-show-errors --output none || error "Failed to create rule-101 in NSG '$nsg' in resource group '$rg'"
                    echo "Created rule-101 in NSG '$nsg' in resource group '$rg'."
                fi
            fi
        done
    done
    echo "NSG NRMS rule check and modification complete."
}

main() {
    source_tilt_settings "$@"

    if [ "${SKIP_PEER_VNETS:-false}" != "true" ]; then
        peer_vnets
    else
        echo "Skipping peer_vnets as requested via SKIP_PEER_VNETS."
    fi

    if [ "${SKIP_NSG_RULES:-false}" != "true" ]; then
        wait_and_fix_nsg_rules
    else
        echo "Skipping wait_and_fix_nsg_rules as requested via SKIP_NSG_RULES."
    fi
}

# Only run main if script is executed directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
