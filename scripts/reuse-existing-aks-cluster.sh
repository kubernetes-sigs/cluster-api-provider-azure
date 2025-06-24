#!/usr/bin/env bash
# Copyright 2025 The Kubernetes Authors.
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
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Initialize variables
KUBE_CONTEXT=""

# Functions to print colored messages
print_success() {
    echo -e "${GREEN} $1${NC}" >&2;
}

print_info() {
    echo -e "${CYAN} $1${NC}" >&2;
}

print_warning() {
    echo -e "${YELLOW} $1${NC}" >&2;
}


print_error() {
    echo -e "${RED} $1${NC}" >&2;
}

parse_args() {

    # Check if MGMT_CLUSTER_NAME is set, if yes, use it as the context
    if [ -n "${MGMT_CLUSTER_NAME:-}" ]; then
        KUBE_CONTEXT="${MGMT_CLUSTER_NAME}"
        print_info "using MGMT_CLUSTER_NAME: $KUBE_CONTEXT"

        if ! kubectl config use-context "$KUBE_CONTEXT" >/dev/null 2>&1; then
            print_error "Failed to switch to context '$KUBE_CONTEXT'"
            exit 1
        fi
    fi

    if [ -z "$KUBE_CONTEXT" ]; then
        KUBE_CONTEXT=$(kubectl config current-context)
        print_warning "no context provided – using current context: $KUBE_CONTEXT" >&2
    fi
}

check_aks_cluster_exists() {
    # Check if the AKS cluster exists
    if ! az aks show --name "$KUBE_CONTEXT" --resource-group "$KUBE_CONTEXT" >/dev/null 2>&1; then
        print_error "AKS cluster '$KUBE_CONTEXT' does not exist"
        exit 1
    fi
}

delete_resources() {
    local -r resources=("deployment" "secret" "serviceaccount")
    local -r namespaces=("capz-system" "caaph-system" "capi-kubeadm-bootstrap-system" "capi-kubeadm-control-plane-system" "capi-system")

    for resource in "${resources[@]}"; do
        print_info "Deleting all the ${resource}s from namespaces: ${namespaces[*]}"
        for namespace in "${namespaces[@]}"; do
            kubectl delete "${resource}" -n "${namespace}" --all
        done
    done
}

delete_crds() {
    # delete all the CRDs from the ASO_CRDS_PATH. ASO_CRDS_PATH is defined in Makefile.
    # ASO_CRDS_PATH has the path to the yaml that has all the CRDs required for ASO.
    print_info "Deleting all the CRDs from the ASO_CRDS_PATH using kubectl delete -f ${ASO_CRDS_PATH}"
    if ! kubectl delete -f "${ASO_CRDS_PATH}" --force 2>/dev/null; then
        print_warning "No ASO CRDs found or error deleting them, continuing..."
    else
        print_success "Successfully deleted ASO CRDs"
    fi

    # delete all the CRDs from the CRD_ROOT. CRD_ROOT is defined in Makefile.
    # CRD_ROOT leads to a directory with a list of CRD yaml files for CAPZ.
    print_info "Deleting all the CRDs from the CRD_ROOT"
    for crd_file in "${CRD_ROOT:-}"/*; do
        if [ -f "$crd_file" ]; then
            if ! kubectl delete -f "$crd_file" --force 2>/dev/null; then
                print_warning "Failed to delete CRD from $crd_file, continuing..."
            else
                print_success "Successfully deleted CRDs from $crd_file"
            fi
        fi
    done
}

main() {
    # Parse arguments and read into variables
    parse_args "$@"
    print_success "Successfully initialized with context: $KUBE_CONTEXT"

    check_aks_cluster_exists
    delete_resources

    if [ "${DELETE_CRDS:-}" == "true" ]; then
        delete_crds
    fi

    # Once the deployments are deleted, the AKS cluster is ready to be reused
    print_success "AKS cluster '$KUBE_CONTEXT' is ready to be reused"
}

# Only run main if script is executed directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
