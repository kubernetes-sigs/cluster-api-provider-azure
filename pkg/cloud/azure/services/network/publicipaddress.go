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

package network

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
)

// GetPublicIPAddress retrieves the Public IP address resource.
func (s *Service) GetPublicIPAddress(resourceGroup string, IPName string) (network.PublicIPAddress, error) {
	return s.PublicIPAddressesClient.Get(s.ctx, resourceGroup, IPName, "")
}

// DeletePublicIPAddress deletes the Public IP address resource.
func (s *Service) DeletePublicIPAddress(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error) {
	return s.PublicIPAddressesClient.Delete(s.ctx, resourceGroup, IPName)
}

// WaitForPublicIPAddressDeleteFuture waits for the DeletePublicIPAddress operation to complete.
func (s *Service) WaitForPublicIPAddressDeleteFuture(future network.PublicIPAddressesDeleteFuture) error {
	return future.Future.WaitForCompletionRef(s.ctx, s.PublicIPAddressesClient.Client)
}
