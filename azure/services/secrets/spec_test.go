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

package secrets

import (
	"encoding/base64"
	"testing"

	secretskeyvault "github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
)

var (
	secretSpec = SecretSpec{
		Name:      "azure-machine-bootstrap-secret",
		VaultName: "my-cluster-vault",
		Value:     "secret_val",
	}
)

func TestSecretSpec_ResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(secretSpec.ResourceName()).Should(Equal("azure-machine-bootstrap-secret"))
}

func TestSecretSpec_ResourceGroupName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(secretSpec.ResourceGroupName()).Should(Equal(""))
}

func TestSecretSpec_OwnerResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(secretSpec.OwnerResourceName()).Should(Equal("my-cluster-vault"))
}

func TestSecretSpec_Parameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          SecretSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:          "new secret spec",
			expectedError: "",
			spec:          secretSpec,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(secretskeyvault.SecretSetParameters{
					Value: to.StringPtr(base64.StdEncoding.EncodeToString([]byte("secret_val"))),
				}))
			},
		},
		{
			name:          "existing secret",
			expectedError: "",
			spec:          secretSpec,
			existing:      secretskeyvault.SecretBundle{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:          "type cast error",
			expectedError: "string is not a keyvault.SecretBundle",
			spec:          secretSpec,
			existing:      "I'm not keyvault.SecretBundle",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				tc.expect(g, result)
			}
		})
	}
}
