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

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Function to print colored messages
print_header() {
    echo -e "\n${BOLD}${BLUE}-------- $1 --------${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_info() {
    echo -e "${CYAN}ℹ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}" >&2
}

print_step() {
    echo -e "${BOLD}${CYAN}Step $1:${NC} $2"
}

usage() {
    cat <<EOF
Usage: $(basename "$0") <tilt-settings.yaml>

This script peers Azure VNets and sets up DNS for AKS clusters.

It requires a tilt-settings.yaml file with aks_as_mgmt_settings dict populated with env variables created from running make aks-create. The following
environment variables will be sourced from the file:
    - AKS_RESOURCE_GROUP
    - AKS_MGMT_VNET_NAME
    - CLUSTER_NAME
    - CLUSTER_NAMESPACE
    - AZURE_INTERNAL_LB_PRIVATE_IP
    - APISERVER_LB_DNS_SUFFIX
    - AZURE_LOCATION
    - AKS_NODE_RESOURCE_GROUP

Additionally, you may optionally skip individual steps by setting these environment variables:

    SKIP_PEER_VNETS: Set to "true" to skip the VNET peering operations.
    SKIP_CREATE_PRIVATE_DNS_ZONE: Set to "true" to skip the private DNS zone creation.
    SKIP_NSG_RULES: Set to "true" to skip the NSG rule checking and updates.


Examples:
    Run all steps:
        ./$(basename "$0") tilt-settings.yaml

    Skip the VNET peering operations:
        SKIP_PEER_VNETS=true ./$(basename "$0") tilt-settings.yaml

    Skip the NSG rule check and update:
        SKIP_NSG_RULES=true ./$(basename "$0") tilt-settings.yaml

    Skip the VNET peering operations and the NSG rule check and update:
        SKIP_PEER_VNETS=true SKIP_NSG_RULES=true ./$(basename "$0") tilt-settings.yaml

    Skip the VNET peering operations and the private DNS zone creation:
        SKIP_PEER_VNETS=true SKIP_CREATE_PRIVATE_DNS_ZONE=true ./$(basename "$0") tilt-settings.yaml

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

    # Function to process settings from a specific section
    process_settings() {
        local section="$1"
        echo "Reading variables from $TILT_SETTINGS_FILE under '$section'..."

        # Get the list of keys under the section
        local VAR_KEYS
        VAR_KEYS=$(yq e ".$section | keys | .[]" "$TILT_SETTINGS_FILE" 2>/dev/null || true)

        # If there's no such key or it's empty, VAR_KEYS will be empty
        if [ -z "$VAR_KEYS" ]; then
            echo "No variables found under '$section'."
        else
            for key in $VAR_KEYS; do
                # Read the value of each key
                value=$(yq e ".${section}[\"$key\"]" "$TILT_SETTINGS_FILE")
                # Export the key/value pair
                export "$key=$value"
                echo "Exported $key=$value"
            done
        fi
    }

    # Process both sections
    process_settings "aks_as_mgmt_settings"
    process_settings "kustomize_substitutions"

    echo "All variables exported"
}

# Check that all required environment variables are set
check_required_vars() {
    required_vars=(
        "AKS_RESOURCE_GROUP"
        "AKS_MGMT_VNET_NAME"
        "CLUSTER_NAME"
        "CLUSTER_NAMESPACE"
        "AZURE_INTERNAL_LB_PRIVATE_IP"
        "APISERVER_LB_DNS_SUFFIX"
        "AZURE_LOCATION"
        "AKS_NODE_RESOURCE_GROUP"
    )

    print_info "Checking required environment variables..."
    for var in "${required_vars[@]}"; do
        [ -z "${!var:-}" ] && error "$var is not set"
    done
    print_success "All required environment variables are set"

    # Add timeout variable for better maintainability
    WAIT_TIMEOUT=600

    # Add DNS zone variable to avoid repetition
    DNS_ZONE="${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com"
}

# Peers the mgmt and workload clusters VNETs
peer_vnets() {
    print_header "Peering VNETs"

    # Get VNET IDs with improved error handling
    az network vnet wait --resource-group "${AKS_RESOURCE_GROUP}" --name "${AKS_MGMT_VNET_NAME}" --created --timeout "${WAIT_TIMEOUT}" || error "Timeout waiting for management VNET"
    MGMT_VNET_ID=$(az network vnet show --resource-group "${AKS_RESOURCE_GROUP}" --name "${AKS_MGMT_VNET_NAME}" --query id --output tsv) || error "Failed to get management VNET ID"
    print_step "1/4" "${AKS_MGMT_VNET_NAME} found and ${MGMT_VNET_ID} found"

    az network vnet wait --resource-group "${CLUSTER_NAME}" --name "${CLUSTER_NAME}-vnet" --created --timeout "${WAIT_TIMEOUT}" || error "Timeout waiting for workload VNET"
    WORKLOAD_VNET_ID=$(az network vnet show --resource-group "${CLUSTER_NAME}" --name "${CLUSTER_NAME}-vnet" --query id --output tsv) || error "Failed to get workload VNET ID"
    print_step "2/4" "${CLUSTER_NAME}-vnet found and ${WORKLOAD_VNET_ID} found"

    # Peer mgmt vnet with improved error handling
    az network vnet peering create \
        --name "mgmt-to-${CLUSTER_NAME}" \
        --resource-group "${AKS_RESOURCE_GROUP}" \
        --vnet-name "${AKS_MGMT_VNET_NAME}" \
        --remote-vnet "${WORKLOAD_VNET_ID}" \
        --allow-vnet-access true \
        --allow-forwarded-traffic true \
        --only-show-errors --output none || error "Failed to create management peering"
    print_step "3/4" "mgmt-to-${CLUSTER_NAME} peering created in ${AKS_MGMT_VNET_NAME}"

    # Peer workload vnet with improved error handling
    az network vnet peering create \
        --name "${CLUSTER_NAME}-to-mgmt" \
        --resource-group "${CLUSTER_NAME}" \
        --vnet-name "${CLUSTER_NAME}-vnet" \
        --remote-vnet "${MGMT_VNET_ID}" \
        --allow-vnet-access true \
        --allow-forwarded-traffic true \
        --only-show-errors --output none || error "Failed to create workload peering"
    print_step "4/4" "${CLUSTER_NAME}-to-mgmt peering created in ${CLUSTER_NAME}-vnet"
    print_success "VNET peering completed successfully"
}

# Creates a private DNS zone and links it to the workload and mgmt VNETs
create_private_dns_zone() {
    print_header "Creating private DNS zone"

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
    print_step "1/4" "${DNS_ZONE} private DNS zone created in ${CLUSTER_NAME}"

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
    print_step "2/4" "workload cluster vnet ${CLUSTER_NAME}-vnet linked with private DNS zone"

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
    print_step "3/4" "management cluster vnet ${AKS_MGMT_VNET_NAME} linked with private DNS zone"

    # Create private DNS zone record with improved error handling
    az network private-dns record-set a add-record \
        --resource-group "${CLUSTER_NAME}" \
        --zone-name "${DNS_ZONE}" \
        --record-set-name "@" \
        --ipv4-address "${AZURE_INTERNAL_LB_PRIVATE_IP}" \
        --only-show-errors --output none || error "Failed to create DNS record"
    print_step "4/4" "\"@\" private DNS zone record created to point ${DNS_ZONE} to ${AZURE_INTERNAL_LB_PRIVATE_IP}"
    print_success "Private DNS zone creation completed successfully"
}

# New function that waits for NSG rules with prefix "NRMS-Rule-101" in the relevant resource groups,
# then creates or modifies NRMS-Rule-101 to allow the specified ports.
wait_and_fix_nsg_rules() {
    local tcp_ports="443 5986 6443"
    local udp_ports="53 123"
    local timeout=3000     # seconds to wait per NSG for the appearance of an NRMS-Rule-101 rule
    local sleep_interval=10  # seconds between checks

    print_header "Checking and Updating NSG Rules"

    print_info "Waiting for NSG rules with prefix 'NRMS-Rule-101' to appear..."

    local resource_groups=("$AKS_NODE_RESOURCE_GROUP" "$AKS_RESOURCE_GROUP" "$CLUSTER_NAME")

    for rg in "${resource_groups[@]}"; do
        echo
        print_info "Processing NSGs in resource group: '$rg'"
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
            print_warning "No NSGs found in '$rg' yet, waiting..."
            sleep "$sleep_interval"
        done

        for nsg in $nsg_list; do
            echo
            print_info "Checking for NRMS-Rule-101 rules in NSG: '$nsg' (Resource Group: '$rg')"
            local rule_found=""
            local rule_start_time
            rule_start_time=$(date +%s)
            while :; do
                # Query NSG rules with names that start with "NRMS-Rule-101".
                rule_found=$(az network nsg rule list --resource-group "$rg" --nsg-name "$nsg" --query "[?starts_with(name, 'NRMS-Rule-101')].name" --output tsv)
                if [ -n "$rule_found" ]; then
                    print_success "Found NRMS rule(s): $rule_found in NSG '$nsg'"
                    break
                fi
                if (( $(date +%s) - rule_start_time >= timeout )); then
                    print_warning "Timeout waiting for NRMS-Rule-101 rules in NSG '$nsg' in RG '$rg'. Skipping NSG."
                    break
                fi
                print_warning "NRMS-Rule-101 rules not found in NSG '$nsg', waiting..."
                sleep "$sleep_interval"
            done

            # If an NRMS-Rule-101 rule is found in the NSG, then ensure NRMS-Rule-101 is updated.
            if [ -n "$rule_found" ]; then
                echo
                print_info "Configuring NRMS-Rule-101 in NSG '$nsg' (Resource Group: '$rg')"
                print_info "Allowed TCP ports: $tcp_ports"
                if az network nsg rule show --resource-group "$rg" --nsg-name "$nsg" --name "NRMS-Rule-101" --output none 2>/dev/null; then
                    # shellcheck disable=SC2086
                    az network nsg rule update \
                        --resource-group "$rg" \
                        --nsg-name "$nsg" \
                        --name "NRMS-Rule-101" \
                        --access Allow \
                        --direction Inbound \
                        --protocol "TCP" \
                        --destination-port-ranges $tcp_ports \
                        --destination-address-prefixes "*" \
                        --source-address-prefixes "*" \
                        --source-port-ranges "*" \
                        --only-show-errors --output none || error "Failed to update NRMS-Rule-101 in NSG '$nsg' in resource group '$rg'"
                    print_success "Successfully updated NRMS-Rule-101 in NSG '$nsg'"

                    echo
                    print_info "Configuring NRMS-Rule-103 in NSG '$nsg' (Resource Group: '$rg')"
                    print_info "Allowed UDP ports: $udp_ports"
                    # shellcheck disable=SC2086
                    az network nsg rule update \
                        --resource-group "$rg" \
                        --nsg-name "$nsg" \
                        --name "NRMS-Rule-103" \
                        --access Allow \
                        --direction Inbound \
                        --protocol "UDP" \
                        --destination-port-ranges $udp_ports \
                        --destination-address-prefixes "*" \
                        --source-address-prefixes "*" \
                        --source-port-ranges "*" \
                        --only-show-errors --output none || error "Failed to update NRMS-Rule-103 in NSG '$nsg' in resource group '$rg'"
                    print_success "Successfully updated NRMS-Rule-103 in NSG '$nsg'"
                fi
            fi
        done
    done
    print_success "NSG Rule Check and Update Complete"
}

# Waits for the controlplane of the workload cluster to be ready
wait_for_controlplane_ready() {
    print_header "Waiting for Workload Cluster Control Plane"

    print_info "Waiting for secret: ${CLUSTER_NAME}-kubeconfig to be available in the management cluster"
    until kubectl get secret "${CLUSTER_NAME}-kubeconfig" -n "${CLUSTER_NAMESPACE}" > /dev/null 2>&1; do
        sleep 5
    done
    kubectl get secret "${CLUSTER_NAME}-kubeconfig" -n "${CLUSTER_NAMESPACE}" -o jsonpath='{.data.value}' | base64 --decode > "./${CLUSTER_NAME}.kubeconfig"
    chmod 600 "./${CLUSTER_NAME}.kubeconfig"

    # Save the current (management) kubeconfig.
    # If KUBECONFIG was not set, assume the default is $HOME/.kube/config.
    MANAGEMENT_KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

    # Now switch to the workload cluster kubeconfig.
    export KUBECONFIG="./${CLUSTER_NAME}.kubeconfig"  # Set kubeconfig for subsequent kubectl commands

    print_info "Waiting for controlplane of the workload cluster to be ready..."

    # Wait for the API server to be responsive and for control plane nodes to be Ready
    until kubectl get nodes --selector='node-role.kubernetes.io/control-plane' --no-headers 2>/dev/null | grep -q "Ready"; do
        print_warning "Waiting for control plane nodes to be responsive and Ready..."
        sleep 10
    done

    # Reset KUBECONFIG back to the management cluster kubeconfig.
    export KUBECONFIG="$MANAGEMENT_KUBECONFIG"
    print_info "Reset KUBECONFIG to management cluster kubeconfig: $KUBECONFIG"
    print_success "Workload Cluster Control Plane is Ready"
}

main() {
    source_tilt_settings "$@"
    check_required_vars

    # SKIP_PEER_VNETS can be set to true to skip the VNET peering operations
    if [ "${SKIP_PEER_VNETS:-false}" != "true" ]; then
        peer_vnets
    else
        print_header "Skipping VNET Peering"
        print_info "Skipping peer_vnets as requested via SKIP_PEER_VNETS."
    fi

    # wait for controlplane of the workload cluster to be ready and then create the private DNS zone
    # SKIP_CREATE_PRIVATE_DNS_ZONE can be set to true to skip the private DNS zone creation
    if [ "${SKIP_CREATE_PRIVATE_DNS_ZONE:-false}" != "true" ]; then
        wait_for_controlplane_ready
        create_private_dns_zone
    else
        print_header "Skipping Private DNS Zone Creation"
        print_info "Skipping create_private_dns_zone as requested via SKIP_CREATE_PRIVATE_DNS_ZONE."
    fi

    # SKIP_NSG_RULES can be set to true to skip the NSG rule checking and updates
    if [ "${SKIP_NSG_RULES:-false}" != "true" ]; then
        wait_and_fix_nsg_rules
    else
        print_header "Skipping NSG Rule Updates"
        print_info "Skipping wait_and_fix_nsg_rules as requested via SKIP_NSG_RULES."
    fi
}

# Only run main if script is executed directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
