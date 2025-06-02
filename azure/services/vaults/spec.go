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

package vaults

import (
	"context"

	"github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	// VaultsResourceType is the resource type name for Key Vaults.
	VaultsResourceType = "vaults"
)

// VaultSpec defines the specification for a Key Vault.
type VaultSpec struct {
	Name          string
	ResourceGroup string
	Location      string
	TenantID      string
	Tags          map[string]string
}

// ResourceRef implements azure.ASOResourceSpecGetter.
func (s *VaultSpec) ResourceRef() *v1api20230701.Vault {
	return &v1api20230701.Vault{
		ObjectMeta: metav1.ObjectMeta{
			Name: azure.GetNormalizedKubernetesName(s.Name),
		},
	}
}

// Parameters returns the parameters for the Key Vault.
func (s *VaultSpec) Parameters(ctx context.Context, existing *v1api20230701.Vault) (*v1api20230701.Vault, error) {
	_, log, done := tele.StartSpanWithLogger(ctx, "keyvault.KeyVaultSpec.Parameters")
	defer done()

	params := existing
	if params == nil {
		params = &v1api20230701.Vault{}
	}

	params.Spec = v1api20230701.Vault_Spec{
		Location: &s.Location,
		Tags:     s.Tags,
		//AzureName: "",
		OperatorSpec: &v1api20230701.VaultOperatorSpec{
			ConfigMapExpressions: nil,
			SecretExpressions:    nil,
		},
		Owner: &genruntime.KnownResourceReference{
			Name: s.ResourceGroup,
		},
		Properties: &v1api20230701.VaultProperties{
			AccessPolicies:               []v1api20230701.AccessPolicyEntry{},
			EnabledForDeployment:         ptr.To(false),
			EnabledForDiskEncryption:     ptr.To(false),
			EnabledForTemplateDeployment: ptr.To(false),
			EnableSoftDelete:             ptr.To(true),
			SoftDeleteRetentionInDays:    ptr.To(int(90)),
			EnableRbacAuthorization:      ptr.To(true), // Use RBAC instead of access policies
			// CreateMode:                   nil,
			// EnablePurgeProtection:        nil,
			// NetworkAcls: &v1api20230701.NetworkRuleSet{
			// 	Bypass:              nil,
			// 	DefaultAction:       nil,
			// 	IpRules:             nil,
			// 	VirtualNetworkRules: nil,
			// },
			// ProvisioningState:   nil,
			// PublicNetworkAccess: nil,
			TenantId: &s.TenantID,
			Sku: &v1api20230701.Sku{
				Family: ptr.To(v1api20230701.Sku_Family_A),
				Name:   ptr.To(v1api20230701.Sku_Name_Standard),
			},
			// VaultUri: nil,
		},
	}
	log.V(2).Info("Creating new Key Vault", "name", s.Name, "location", s.Location)
	return params, nil
}

// WasManaged implements azure.ASOResourceSpecGetter.
func (s *VaultSpec) WasManaged(_ *v1api20230701.Vault) bool {
	// Vault identities are always managed by CAPZ
	return true
}
