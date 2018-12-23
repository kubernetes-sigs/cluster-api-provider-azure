/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package actuators

import (
	//"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/compute"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute/computeapi"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-10-01/network/networkapi"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources/resourcesapi"
)

// AzureClients contains all the azure clients used by the scopes.
type AzureClients struct {
	// Compute
	VMAPI    computeapi.VirtualMachinesClientAPI
	DisksAPI computeapi.DisksClientAPI

	// Network
	LBAPI networkapi.LoadBalancersClientAPI

	// Resources
	DeploymentsAPI resourcesapi.DeploymentsClientAPI
}
