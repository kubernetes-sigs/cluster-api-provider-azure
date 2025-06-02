/*
Copyright 2025 The Kubernetes Authors.

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

package userassignedidentities

import (
	"context"
	"fmt"
	"strings"

	asomanagedidentityv1api20230131 "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

// UserAssignedIdentitySpec defines the specification for a user-assigned identity.
type UserAssignedIdentitySpec struct {
	Name          string
	ResourceGroup string
	Location      string
	Tags          map[string]*string
	ConfigMapName string
}

// ResourceName returns the name of the user-assigned identity.
func (s *UserAssignedIdentitySpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the resource group name.
func (s *UserAssignedIdentitySpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for user-assigned identities.
func (s *UserAssignedIdentitySpec) OwnerResourceName() string {
	return ""
}

// GetResourceID constructs the resource ID for the user-assigned identity.
func (s *UserAssignedIdentitySpec) GetResourceID(subscriptionID string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s",
		subscriptionID, s.ResourceGroup, s.Name)
}

// ParseUserAssignedIdentityResourceID parses a user-assigned identity resource ID and returns the parsed components.
func ParseUserAssignedIdentityResourceID(resourceID string) (*UserAssignedIdentitySpec, error) {
	parsed, err := azureutil.ParseResourceID(resourceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse resource ID")
	}

	// Validate that this is a user-assigned identity resource ID
	if !strings.Contains(resourceID, "/providers/Microsoft.ManagedIdentity/userAssignedIdentities/") {
		return nil, errors.Errorf("expected Microsoft.ManagedIdentity/userAssignedIdentities resource, got %s", resourceID)
	}

	return &UserAssignedIdentitySpec{
		Name:          parsed.Name,
		ResourceGroup: parsed.ResourceGroupName,
	}, nil
}

// ValidateCreate validates the user-assigned identity spec for creation.
func (s *UserAssignedIdentitySpec) ValidateCreate() error {
	if s.Name == "" {
		return errors.New("user-assigned identity name cannot be empty")
	}
	if s.ResourceGroup == "" {
		return errors.New("user-assigned identity resource group cannot be empty")
	}
	if s.Location == "" {
		return errors.New("user-assigned identity location cannot be empty")
	}
	return nil
}

// ResourceRef implements azure.ASOResourceSpecGetter.
func (s *UserAssignedIdentitySpec) ResourceRef() *asomanagedidentityv1api20230131.UserAssignedIdentity {
	return &asomanagedidentityv1api20230131.UserAssignedIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: azure.GetNormalizedKubernetesName(s.Name),
		},
	}
}

// Parameters implements azure.ASOResourceSpecGetter.
func (s *UserAssignedIdentitySpec) Parameters(_ context.Context, existing *asomanagedidentityv1api20230131.UserAssignedIdentity) (*asomanagedidentityv1api20230131.UserAssignedIdentity, error) {
	identity := existing
	if identity == nil {
		identity = &asomanagedidentityv1api20230131.UserAssignedIdentity{}
	}

	identity.Spec = asomanagedidentityv1api20230131.UserAssignedIdentity_Spec{
		AzureName: s.Name,
		Location:  ptr.To(s.Location),
		Owner: &genruntime.KnownResourceReference{
			Name: s.ResourceGroup,
		},
		Tags: convertTagsFromStringPtr(s.Tags),
		OperatorSpec: &asomanagedidentityv1api20230131.UserAssignedIdentityOperatorSpec{
			ConfigMaps: &asomanagedidentityv1api20230131.UserAssignedIdentityOperatorConfigMaps{
				ClientId: &genruntime.ConfigMapDestination{
					Name: s.ConfigMapName,
					Key:  "clientId",
				},
				PrincipalId: &genruntime.ConfigMapDestination{
					Name: s.ConfigMapName,
					Key:  "principalId",
				},
			},
		},
	}

	return identity, nil
}

// WasManaged implements azure.ASOResourceSpecGetter.
func (s *UserAssignedIdentitySpec) WasManaged(_ *asomanagedidentityv1api20230131.UserAssignedIdentity) bool {
	// User-assigned identities are always managed by CAPZ
	return true
}

// convertTagsFromStringPtr converts map[string]*string to map[string]string for ASO compatibility.
func convertTagsFromStringPtr(tags map[string]*string) map[string]string {
	if tags == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range tags {
		if v != nil {
			result[k] = *v
		}
	}
	return result
}
