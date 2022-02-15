#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
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

CI_RG="${CI_RG:-capz-ci}"
AZURE_LOCATION="${AZURE_LOCATION:-westus2}"
GMSA_KEYVAULT="${CI_RG}-gmsa"

: "${CI_CLIENT_ID:?Environment variable empty or not defined.}"

rolejson=$(cat <<-ROLE
{
    "Name": "gMSA",
    "Description": "Required permissions for gmsa to read properties of subscriptions and managed identities",
    "Actions": [
        "Microsoft.Resources/subscriptions/read",
        "Microsoft.ManagedIdentity/userAssignedIdentities/read"
    ],
    "AssignableScopes": ["/subscriptions/$AZURE_SUBSCRIPTION_ID"]
}
ROLE
)

main() {
    if [[ "$(az group exists --name "${CI_RG}")" == "false" ]]; then
        az group create --name "$CI_RG" --location "$AZURE_LOCATION"
    fi

    keyvaultid=$(az keyvault show --name "${GMSA_KEYVAULT}" -g "$CI_RG" --query "id" || true)
    if [[ -z $keyvaultid ]]; then 
        az keyvault create --name "${GMSA_KEYVAULT}" -g "$CI_RG"
    fi

    # Give permissions to vms identity to write to keyvault during the domain creation
    domainid=$(az identity show --name domain-vm-identity --resource-group "$CI_RG" --query 'principalId' -o tsv || true)
    if [[ -z $domainid ]]; then
        domainid=$(az identity create -g "$CI_RG" -n domain-vm-identity --query 'principalId' -o tsv)
    fi
    az keyvault set-policy --name "${GMSA_KEYVAULT}" --object-id "$domainid" --secret-permissions set   
    
    # The identity also needs to be able to read subscription id and managed identities
    # This is a custom role to make this least priviliged. 
    # The creator must have permissions to create roles and assignements.
    customSubRole=$(az role definition list --custom-role-only --query [].roleName -o tsv)
    if ! [[ $customSubRole =~ "gMSA" ]]; then
        echo "If the following fails you need to have someone with permissions create this role"
        az role definition create --role-definition "$rolejson"
    fi
    # on first run this takes ~1-2 mins
    until az role assignment create --role "gMSA" --assignee-object-id "$domainid"  --assignee-principal-type ServicePrincipal &> /dev/null
    do 
        echo "wait for role propgation"
        sleep 10
    done

    # create identity for the worker VMs to use to get keyvault secrets
    # this identity needs to be assigned to the the Worker nodes that is labeled during e2e set up.
    userId=$(az identity show --name gmsa-user-identity --resource-group "$CI_RG" --query 'principalId' -o tsv || true)
    if [[ -z $userId ]]; then 
        userId=$(az identity create -g "$CI_RG" -n gmsa-user-identity --query 'principalId' -o tsv)
    fi
    az keyvault set-policy --name "${GMSA_KEYVAULT}" --object-id "$userId" --secret-permissions get

    cloudproviderId=$(az identity show --name cloud-provider-user-identity --resource-group "$CI_RG" --query 'principalId' -o tsv || true)
    if [[ -z $cloudproviderId ]]; then 
        cloudproviderId=$(az identity create -g "$CI_RG" -n cloud-provider-user-identity --query 'principalId' -o tsv)
    fi

    # on first run this takes ~1-2 mins
    until az role assignment create --role "Contributor" --assignee-object-id "$cloudproviderId"  --assignee-principal-type ServicePrincipal &> /dev/null
    do 
        echo "wait for role propgation"
        sleep 10
    done

    until az role assignment create --role "AcrPull" --assignee-object-id "$cloudproviderId"  --assignee-principal-type ServicePrincipal &> /dev/null
    do 
        echo "wait for role propgation"
        sleep 10
    done

    # make sure the service CI principal has read access to set up tests
    ciSP=$(az ad sp show --id "$CI_CLIENT_ID" --query objectId -o tsv)
    az keyvault set-policy --name "${GMSA_KEYVAULT}" --object-id "$ciSP" --secret-permissions get delete list purge
}

main