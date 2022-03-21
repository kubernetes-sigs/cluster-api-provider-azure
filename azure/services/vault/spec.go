/*
Copyright 2022 The Kubernetes Authors.

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

package vault

import (
	"github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2019-09-01/keyvault"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// Spec defines the specification for a vault.
type Spec struct {
	Name          string
	ResourceGroup string
	Location      string
	ClusterName   string
	TenantID      string
}

// ResourceName returns the name of the vault.
func (s Spec) ResourceName() string {
	return s.Name
}

// OwnerResourceName is the owner resource name of this resource.
// no-op for Spec.
func (s Spec) OwnerResourceName() string {
	return ""
}

// ResourceGroupName returns the name of the resource group.
func (s Spec) ResourceGroupName() string {
	return s.ResourceGroup
}

// Parameters returns the parameters for the vault.
func (s Spec) Parameters(existing interface{}) (params interface{}, err error) {
	if existing != nil {
		if _, ok := existing.(keyvault.Vault); !ok {
			return nil, errors.Errorf("%T is not a keyvault.Vault", existing)
		}
		return nil, nil
	}

	tenantID := uuid.Must(uuid.FromString(s.TenantID))
	return keyvault.VaultCreateOrUpdateParameters{
		Location: to.StringPtr(s.Location),
		Properties: &keyvault.VaultProperties{
			TenantID: &tenantID,
			Sku: &keyvault.Sku{
				Family: to.StringPtr("A"),
				Name:   "standard",
			},
			EnableRbacAuthorization: to.BoolPtr(true),
			EnableSoftDelete:        to.BoolPtr(false),
		},
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Additional:  nil,
		})),
	}, nil
}
