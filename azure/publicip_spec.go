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

package azure

// ResourceName returns the name of the public IP.
func (s PublicIPSpec) ResourceName() string {
	return s.Name
}

// OwnerResourceName is a no-op for public IPs.
func (s PublicIPSpec) OwnerResourceName() string {
	return ""
}

// ResourceGroupName returns the name of the resource group the public IP is in.
func (s PublicIPSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// Parameters returns the parameters for the route table.
func (s PublicIPSpec) Parameters(existing interface{}) (interface{}, error) {
	return nil, nil
}
