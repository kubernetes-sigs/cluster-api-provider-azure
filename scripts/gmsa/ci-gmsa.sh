#!/bin/bash

# Copyright 2022 The Kubernetes Authors.
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

###############################################################################

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=hack/ensure-azcli.sh
source "${REPO_ROOT}/hack/ensure-azcli.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh"

ENVSUBST="${REPO_ROOT}/hack/tools/bin/envsubst"
cd "${REPO_ROOT}" && make "${ENVSUBST##*/}"

CI_RG="${CI_RG:-capz-ci}"
GMSA_NODE_RG="${GMSA_NODE_RG:-gmsa-dc}"
AZURE_LOCATION="${AZURE_LOCATION:-westus2}"
GMSA_KEYVAULT="${CI_RG}-gmsa"

# The VM requires setup that needs Role Assignment permissions
# This script checks that all that has been configured properly before creating the Azure VM
main() {
    if [[ "$(az group exists --name "${CI_RG}")" == "false" ]]; then
        echo "Requires pre-requisite that resource group ${CI_RG} exists"
        exit 1
    fi

    keyvaultid=$(az keyvault show --name "${GMSA_KEYVAULT}" -g "$CI_RG" --query "id" || true)
    if [[ -z $keyvaultid ]]; then 
        echo "Requires pre-requisite that keyvault ${GMSA_KEYVAULT} exists"
        exit 1
    fi

    # Give permissions to write to keyvault during the domain creation to create secrets that will be used during test
    domainPricipalId=$(az identity show --name domain-vm-identity --resource-group "$CI_RG" --query 'principalId' -o tsv || true)
    domainId=$(az identity show --name domain-vm-identity --resource-group "$CI_RG" --query 'id' -o tsv || true)
    if [[ -z $domainPricipalId ]]; then
        echo "Requires pre-requisite that user identity 'domain-vm-identity' exists"
        exit 1
    fi

    # the powershell commandlet Get-AzUserAssignedIdentity requires ability to read subid which is granted via this custom role
    # see the setup-gmsa.sh for custom role creation
    customSubRole=$(az role assignment list --assignee "$domainPricipalId" --query "[?roleDefinitionName=='gMSA']" || true)
    if [[ $customSubRole == "[]" ]]; then
        echo "The domain-vm-identity must have custom role 'gMSA'"
        exit 1
    fi

    # this identity needs to be assigned to the the Worker nodes that is labeled during e2e set up.
    userId=$(az identity show --name gmsa-user-identity --resource-group "$CI_RG" --query 'principalId' -o tsv || true)
    if [[ -z $userId ]]; then 
        echo "Requires pre-requisite that user identity 'gmsa-user-identity' exists"
        exit 1
    fi

    echo "Pre-reqs are met for creating Domain vm"
    # the custom-data contains scripts to
    #  - turn this vm into a domain
    #  - vm is created in vnet that doesn't overlap with default capz cluster vnets
    #  - creates a domain admin and gmsa users 
    #  - uploads secrets to the keyvault
    #  - creates a gmsa yaml spec in location c:\gmsa\gmsa-cred-spec-gmsa-e2e.yml 
    # this is a random temp password which gets replaced by the cloudbase-init
    if [[ "$(az group exists --name "${GMSA_NODE_RG}")" == "false" ]]; then
        az group create --name "$GMSA_NODE_RG" --location "$AZURE_LOCATION" --tags creationTimestamp="$TIMESTAMP"
    fi

    winpass=$(openssl rand -base64 32)
    vmname="dc-${GMSA_ID}"
    vmid=$(az vm show -n "$vmname" -g "$GMSA_NODE_RG" --query "id" || true)
    if [[ -z $vmid ]]; then 
        echo "Creating Domain vm"
        GMSA_DOMAIN_ENVSUBST="${REPO_ROOT}/scripts/gmsa/domain.init"
        GMSA_DOMAIN_FILE="${REPO_ROOT}/scripts/gmsa/domain.init.tmpl"
        $ENVSUBST < "$GMSA_DOMAIN_FILE" > "$GMSA_DOMAIN_ENVSUBST"
        az vm create -l "$AZURE_LOCATION" -g "$GMSA_NODE_RG" -n "$vmname" \
            --image cncf-upstream:capi-windows:k8s-1dot23dot5-windows-2019-containerd:2022.03.30 \
            --admin-user 'azureuser' \
            --admin-password "$winpass" \
            --custom-data "${GMSA_DOMAIN_ENVSUBST}" \
            --assign-identity "$domainId" \
            --public-ip-address "" \
            --subnet-address-prefix 172.16.0.0/24 \
            --vnet-address-prefix 172.16.0.0/16 \
            --vnet-name "${vmname}-vnet" \
            --nsg "${vmname}-nsg" \
            --size Standard_D4s_v3
    fi

    bastionId=$(az network bastion show -n gmsa-bastion -g "$GMSA_NODE_RG" --query "id" || true)
    if [[ -z $bastionId && ${GMSA_BASTION:-} == "true" ]]; then
        echo "Create bastion for Domain vm"
        # Required inbound rules for AzureBastionSubnet
        # https://docs.microsoft.com/en-us/azure/bastion/bastion-nsg
        az network nsg rule create -g "$GMSA_NODE_RG" \
            -n Allow-HttpsInbound \
            --access allow \
            --destination-address-prefix '*' \
            --destination-port-range 443 \
            --direction inbound \
            --nsg-name "${vmname}-nsg" \
            --protocol tcp \
            --source-address-prefix Internet \
            --source-port-range '*' \
            --priority 120
        
        az network nsg rule create -g "$GMSA_NODE_RG" \
            -n Allow-GatewayManagerInboud \
            --access allow \
            --destination-address-prefix '*' \
            --destination-port-range 443 \
            --direction inbound \
            --nsg-name "${vmname}-nsg" \
            --protocol tcp \
            --source-address-prefix GatewayManager \
            --source-port-range '*' \
            --priority 130

        az network nsg rule create -g "$GMSA_NODE_RG" \
            -n Allow-BastionHostCommunication \
            --access allow \
            --destination-address-prefix VirtualNetwork \
            --destination-port-range 8080 5701 \
            --direction inbound \
            --nsg-name "${vmname}-nsg" \
            --protocol '*' \
            --source-address-prefix VirtualNetwork \
            --source-port-range '*' \
            --priority 150
        
        # Required outbound rules for AzureBastionSubnet
        # https://docs.microsoft.com/en-us/azure/bastion/bastion-nsg
        az network nsg rule create -g "$GMSA_NODE_RG" \
            -n Allow-SshRdpOutbound \
            --access allow \
            --destination-address-prefix VirtualNetwork \
            --destination-port-range 22 3389 \
            --direction outbound \
            --nsg-name "${vmname}-nsg" \
            --protocol '*' \
            --source-address-prefix '*' \
            --source-port-range '*' \
            --priority 100

        az network nsg rule create -g "$GMSA_NODE_RG" \
            -n Allow-AzureCloudoutbound \
            --access allow \
            --destination-address-prefix AzureCloud \
            --destination-port-range 443 \
            --direction outbound \
            --nsg-name "${vmname}-nsg" \
            --protocol 'tcp' \
            --source-address-prefix '*' \
            --source-port-range '*' \
            --priority 150
        
        az network nsg rule create -g "$GMSA_NODE_RG" \
            -n Allow-BastionCommunication \
            --access allow \
            --destination-address-prefix VirtualNetwork \
            --destination-port-range 8080 5701 \
            --direction outbound \
            --nsg-name "${vmname}-nsg" \
            --protocol '*' \
            --source-address-prefix VirtualNetwork \
            --source-port-range '*' \
            --priority 120
        
        az network nsg rule create -g "$GMSA_NODE_RG" \
            -n Allow-GetSessionInfomation \
            --access allow \
            --destination-address-prefix Internet \
            --destination-port-range 80 \
            --direction outbound \
            --nsg-name "${vmname}-nsg" \
            --protocol '*' \
            --source-address-prefix '*' \
            --source-port-range '*' \
            --priority 130

        az network vnet subnet create -g "$GMSA_NODE_RG" --vnet-name "${vmname}-vnet" -n AzureBastionSubnet \
                --address-prefixes 172.16.1.0/24 --network-security-group "${vmname}-nsg"
        
        az network public-ip create --resource-group "$GMSA_NODE_RG" --name bastion-gmsa --sku Standard
        az network bastion create --name gmsa-bastion --public-ip-address bastion-gmsa --resource-group "$GMSA_NODE_RG" --vnet-name "${vmname}-vnet"
    fi
}

main