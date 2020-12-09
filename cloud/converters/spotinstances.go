/*
Copyright 2020 The Kubernetes Authors.

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

package converters

import (
	"strconv"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// GetSpotVMOptions takes the spot vm options
// and returns the individual vm priority, eviction policy and billing profile
func GetSpotVMOptions(spotVMOptions *infrav1.SpotVMOptions) (compute.VirtualMachinePriorityTypes, compute.VirtualMachineEvictionPolicyTypes, *compute.BillingProfile, error) {
	// Spot VM not requested, return zero values to apply defaults
	if spotVMOptions == nil {
		return "", "", nil, nil
	}
	var billingProfile *compute.BillingProfile
	if spotVMOptions.MaxPrice != nil {
		maxPrice, err := strconv.ParseFloat(*spotVMOptions.MaxPrice, 64)
		if err != nil {
			return "", "", nil, err
		}
		billingProfile = &compute.BillingProfile{
			MaxPrice: &maxPrice,
		}
	}
	return compute.Spot, compute.Deallocate, billingProfile, nil
}
