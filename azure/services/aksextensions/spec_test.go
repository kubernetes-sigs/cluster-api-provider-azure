/*
Copyright 2023 The Kubernetes Authors.

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

package aksextensions

import (
	"context"
	"testing"

	asokubernetesconfigurationv1 "github.com/Azure/azure-service-operator/v2/api/kubernetesconfiguration/v1api20230501"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakeAKSExtension = asokubernetesconfigurationv1.Extension{
		Spec: asokubernetesconfigurationv1.Extension_Spec{
			AksAssignedIdentity: &asokubernetesconfigurationv1.Extension_Properties_AksAssignedIdentity_Spec{
				Type: (*asokubernetesconfigurationv1.Extension_Properties_AksAssignedIdentity_Type_Spec)(&fakeAKSExtensionSpec1.AKSAssignedIdentityType),
			},
			AzureName:               fakeAKSExtensionSpec1.Name,
			AutoUpgradeMinorVersion: ptr.To(true),
			ConfigurationSettings: map[string]string{
				"fake-key": "fake-value",
			},
			ExtensionType: fakeAKSExtensionSpec1.ExtensionType,
			Owner: &genruntime.ArbitraryOwnerReference{
				ARMID: fakeAKSExtensionSpec1.Owner,
			},
			Plan: &asokubernetesconfigurationv1.Plan{
				Name:      ptr.To(fakeAKSExtensionSpec1.Plan.Name),
				Product:   ptr.To(fakeAKSExtensionSpec1.Plan.Product),
				Publisher: ptr.To(fakeAKSExtensionSpec1.Plan.Publisher),
				Version:   ptr.To(fakeAKSExtensionSpec1.Plan.Version),
			},
			ReleaseTrain: fakeAKSExtensionSpec1.ReleaseTrain,
			Version:      fakeAKSExtensionSpec1.Version,
			Identity: &asokubernetesconfigurationv1.Identity{
				Type: (*asokubernetesconfigurationv1.Identity_Type)(&fakeAKSExtensionSpec1.ExtensionIdentity),
			},
		},
	}
	fakeAKSExtensionSpec1 = AKSExtensionSpec{
		Name:                    "fake-aks-extension",
		Namespace:               "fake-namespace",
		AKSAssignedIdentityType: "SystemAssigned",
		AutoUpgradeMinorVersion: ptr.To(true),
		ConfigurationSettings: map[string]string{
			"fake-key": "fake-value",
		},
		ExtensionType: ptr.To("fake-extension-type"),
		ReleaseTrain:  ptr.To("fake-release-train"),
		Version:       ptr.To("fake-version"),
		Owner:         "fake-owner",
		Plan: &infrav1.ExtensionPlan{
			Name: "fake-plan-name",
		},
		ExtensionIdentity: "SystemAssigned",
	}
	fakeAKSExtensionStatus = asokubernetesconfigurationv1.Extension_STATUS{
		Name:              ptr.To(fakeAKSExtensionSpec1.Name),
		ProvisioningState: ptr.To(asokubernetesconfigurationv1.ProvisioningStateDefinition_STATUS_Succeeded),
	}
)

func getASOAKSExtension(changes ...func(*asokubernetesconfigurationv1.Extension)) *asokubernetesconfigurationv1.Extension {
	aksExtension := fakeAKSExtension.DeepCopy()
	for _, change := range changes {
		change(aksExtension)
	}
	return aksExtension
}

func TestAzureAKSExtensionSpec_Parameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *AKSExtensionSpec
		existing      *asokubernetesconfigurationv1.Extension
		expect        func(g *WithT, result asokubernetesconfigurationv1.Extension)
		expectedError string
	}{
		{
			name:     "Creating a new AKS Extension",
			spec:     &fakeAKSExtensionSpec1,
			existing: nil,
			expect: func(g *WithT, result asokubernetesconfigurationv1.Extension) {
				g.Expect(result).To(Not(BeNil()))

				// ObjectMeta is populated later in the codeflow
				g.Expect(result.ObjectMeta).To(Equal(metav1.ObjectMeta{}))

				// Spec is populated from the spec passed in
				g.Expect(result.Spec).To(Equal(getASOAKSExtension().Spec))
			},
		},
		{
			name: "user updates to AKS Extension resource and capz should overwrite it",
			spec: &fakeAKSExtensionSpec1,
			existing: getASOAKSExtension(
				// user added AutoUpgradeMinorVersion which should be overwritten by capz
				func(aksExtension *asokubernetesconfigurationv1.Extension) {
					aksExtension.Spec.AutoUpgradeMinorVersion = ptr.To(false)
				},
				// user added Status
				func(aksExtension *asokubernetesconfigurationv1.Extension) {
					aksExtension.Status = fakeAKSExtensionStatus
				},
			),
			expect: func(g *WithT, result asokubernetesconfigurationv1.Extension) {
				g.Expect(result).To(Not(BeNil()))

				// ObjectMeta is populated later in the codeflow
				g.Expect(result.ObjectMeta).To(Equal(metav1.ObjectMeta{}))

				// Spec is populated from the spec passed in
				g.Expect(result.Spec).To(Equal(getASOAKSExtension().Spec))

				// Status should be carried over
				g.Expect(result.Status).To(Equal(fakeAKSExtensionStatus))
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, *result)
		})
	}
}
