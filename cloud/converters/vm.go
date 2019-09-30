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

package converters

import (
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
)

// SDKToVM converts an Azure SDK VirtualMachine to the CAPZ VM type.
func SDKToVM(v compute.VirtualMachine) (*infrav1.VM, error) {
	i := &infrav1.VM{
		ID:               to.String(v.ID),
		Name:             to.String(v.Name),
		State:            infrav1.VMState(to.String(v.ProvisioningState)),
		AvailabilityZone: to.StringSlice(v.Zones)[0],

		// TODO: Add more conversions once types are updated.
		//Identity: string(v.Identity),
	}

	if v.VirtualMachineProperties != nil && v.VirtualMachineProperties.HardwareProfile != nil {
		i.VMSize = string(v.VirtualMachineProperties.HardwareProfile.VMSize)
	}

	// TODO: Determine if we need any of this logic
	/*
		for _, sg := range v.SecurityGroups {
			i.SecurityGroupIDs = append(i.SecurityGroupIDs, *sg.GroupId)
		}

		if len(v.Tags) > 0 {
			i.Tags = converters.TagsToMap(v.Tags)
		}

		rootSize, err := s.getInstanceRootDeviceSize(v)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get root volume size for instance: %q", aws.StringValue(v.InstanceId))
		}
	*/

	return i, nil
}
