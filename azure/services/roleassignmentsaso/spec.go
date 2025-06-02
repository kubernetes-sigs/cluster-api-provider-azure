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

package roleassignmentsaso

import (
	"context"
	"fmt"
	"strings"

	asoauthorizationv1api20220401 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

const (
	// NetworkProviderAzureCom is the provider namespace for Azure network resources.
	NetworkProviderAzureCom = "network.azure.com"
)

// KubernetesRoleAssignmentSpec defines the specification for a Kubernetes role assignment CRD.
type KubernetesRoleAssignmentSpec struct {
	Name                     string
	Namespace                string
	PrincipalIDConfigMapName string
	PrincipalIDConfigMapKey  string
	PrincipalType            string
	RoleDefinitionReference  string
	OwnerName                string
	OwnerGroup               string
	OwnerKind                string
	ClusterName              string
	Tags                     map[string]string
}

// ResourceName returns the name of the role assignment.
func (s *KubernetesRoleAssignmentSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns empty string as role assignments are not bound to resource groups in Kubernetes.
func (s *KubernetesRoleAssignmentSpec) ResourceGroupName() string {
	return ""
}

// OwnerResourceName returns the owner resource name.
func (s *KubernetesRoleAssignmentSpec) OwnerResourceName() string {
	return s.OwnerName
}

// ValidateCreate validates the role assignment spec for creation.
func (s *KubernetesRoleAssignmentSpec) ValidateCreate() error {
	if s.Name == "" {
		return errors.New("role assignment name cannot be empty")
	}
	if s.Namespace == "" {
		return errors.New("role assignment namespace cannot be empty")
	}
	if s.PrincipalIDConfigMapName == "" {
		return errors.New("principal ID config map name cannot be empty")
	}
	if s.PrincipalIDConfigMapKey == "" {
		return errors.New("principal ID config map key cannot be empty")
	}
	if s.PrincipalType == "" {
		return errors.New("principal type cannot be empty")
	}
	if s.RoleDefinitionReference == "" {
		return errors.New("role definition reference cannot be empty")
	}
	if s.OwnerName == "" {
		return errors.New("owner name cannot be empty")
	}
	if s.OwnerGroup == "" {
		return errors.New("owner group cannot be empty")
	}
	if s.OwnerKind == "" {
		return errors.New("owner kind cannot be empty")
	}
	return nil
}

// BuildRoleAssignmentName builds the role assignment name based on the template pattern.
// Template: ${USER}-${CS_CLUSTER_NAME}-${IDENTITY_NAME}-${ROLE_SUFFIX}-${SCOPE_SUFFIX}.
func BuildRoleAssignmentName(user, clusterName, identityName, roleSuffix, scopeSuffix string) string {
	return fmt.Sprintf("%s-%s-%s-%s-%s", user, clusterName, identityName, roleSuffix, scopeSuffix)
}

// ParseOwnerFromScope parses an Azure resource ID and returns the corresponding ASO owner group and kind.
func ParseOwnerFromScope(scope string) (ownerName, ownerGroup, ownerKind string, err error) {
	parsed, err := azureutil.ParseResourceID(scope)
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to parse scope resource ID")
	}

	resourceTypeString := parsed.ResourceType.String()

	// Map Azure resource types to ASO groups and kinds
	switch resourceTypeString {
	case "Microsoft.Network/networkSecurityGroups":
		return parsed.Name, NetworkProviderAzureCom, "NetworkSecurityGroup", nil
	case "Microsoft.Network/virtualNetworks/subnets":
		// For subnets, we need the virtual network name and subnet name
		// The resource ID format is: /subscriptions/.../resourceGroups/.../providers/Microsoft.Network/virtualNetworks/{vnetName}/subnets/{subnetName}
		parts := strings.Split(scope, "/")
		if len(parts) < 11 {
			return "", "", "", errors.Errorf("invalid subnet resource ID format: %s", scope)
		}
		vnetName := parts[8]    // The virtual network name
		subnetName := parts[10] // The subnet name
		// For ASO, subnets are represented as VirtualNetworksSubnet with the format {vnetName}-{subnetName}
		ownerName := fmt.Sprintf("%s-%s", vnetName, subnetName)
		return ownerName, NetworkProviderAzureCom, "VirtualNetworksSubnet", nil
	case "Microsoft.ManagedIdentity/userAssignedIdentities":
		return parsed.Name, "managedidentity.azure.com", "UserAssignedIdentity", nil
	case "Microsoft.KeyVault/vaults":
		return parsed.Name, "keyvault.azure.com", "Vault", nil
	case "Microsoft.Network/virtualNetworks":
		return parsed.Name, NetworkProviderAzureCom, "VirtualNetwork", nil
	default:
		return "", "", "", errors.Errorf("unsupported resource type for role assignment scope: %s", resourceTypeString)
	}
}

// ResourceRef implements azure.ASOResourceSpecGetter.
func (s *KubernetesRoleAssignmentSpec) ResourceRef() *asoauthorizationv1api20220401.RoleAssignment {
	return &asoauthorizationv1api20220401.RoleAssignment{
		ObjectMeta: metav1.ObjectMeta{
			Name: azure.GetNormalizedKubernetesName(s.Name),
		},
	}
}

// Parameters implements azure.ASOResourceSpecGetter.
func (s *KubernetesRoleAssignmentSpec) Parameters(_ context.Context, existing *asoauthorizationv1api20220401.RoleAssignment) (*asoauthorizationv1api20220401.RoleAssignment, error) {
	roleAssignment := existing
	if roleAssignment == nil {
		roleAssignment = &asoauthorizationv1api20220401.RoleAssignment{}
	}

	roleAssignment.Spec = asoauthorizationv1api20220401.RoleAssignment_Spec{
		Owner: &genruntime.ArbitraryOwnerReference{
			Name:  s.OwnerName,
			Group: s.OwnerGroup,
			Kind:  s.OwnerKind,
		},
		PrincipalIdFromConfig: &genruntime.ConfigMapReference{
			Name: s.PrincipalIDConfigMapName,
			Key:  s.PrincipalIDConfigMapKey,
		},
		PrincipalType: (*asoauthorizationv1api20220401.RoleAssignmentProperties_PrincipalType)(ptr.To(s.PrincipalType)),
		RoleDefinitionReference: &genruntime.ResourceReference{
			ARMID: s.RoleDefinitionReference,
		},
	}

	// Set metadata
	if roleAssignment.ObjectMeta.Labels == nil {
		roleAssignment.ObjectMeta.Labels = make(map[string]string)
	}
	// Add additional tags
	for k, v := range s.Tags {
		roleAssignment.ObjectMeta.Labels[k] = v
	}
	roleAssignment.ObjectMeta.Labels[clusterv1.ClusterNameLabel] = s.ClusterName

	return roleAssignment, nil
}

// WasManaged implements azure.ASOResourceSpecGetter.
func (s *KubernetesRoleAssignmentSpec) WasManaged(_ *asoauthorizationv1api20220401.RoleAssignment) bool {
	// Role assignments are always managed by CAPZ
	return true
}
