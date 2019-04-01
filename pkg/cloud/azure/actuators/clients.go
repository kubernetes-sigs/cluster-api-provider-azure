/*
Copyright 2019 The Kubernetes Authors.

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
	"fmt"
	"hash/fnv"

	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
)

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	SubscriptionID string
	Authorizer     autorest.Authorizer
}

// CreateOrUpdateNetworkAPIServerIP creates or updates public ip name and dns name
func CreateOrUpdateNetworkAPIServerIP(scope *Scope) {
	if scope.Network().APIServerIP.Name == "" {
		h := fnv.New32a()
		h.Write([]byte(fmt.Sprintf("%s/%s/%s", scope.SubscriptionID, scope.ResourceGroup().Name, scope.Cluster.Name)))
		scope.Network().APIServerIP.Name = azure.GeneratePublicIPName(scope.Cluster.Name, fmt.Sprintf("%x", h.Sum32()))
	}

	scope.Network().APIServerIP.DNSName = azure.GenerateFQDN(scope.Network().APIServerIP.Name, scope.ClusterConfig.Location)
}
