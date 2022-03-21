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

	secretskeyvault "github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
)

// SecretSpec defines the specification for a secret.
type SecretSpec struct {
	Name      string
	VaultName string
	Value     string
}

// ResourceName returns the name of the secret.
func (s SecretSpec) ResourceName() string {
	return s.Name
}

// OwnerResourceName is the vault name that this secret belongs to.
func (s SecretSpec) OwnerResourceName() string {
	return s.VaultName
}

// ResourceGroupName returns the name of the resource group. no-op for secrets.
func (s SecretSpec) ResourceGroupName() string {
	return ""
}

// Parameters returns the parameters for the secret.
func (s SecretSpec) Parameters(existing interface{}) (params interface{}, err error) {
	if existing != nil {
		if _, ok := existing.(secretskeyvault.SecretBundle); !ok {
			return nil, errors.Errorf("%T is not a keyvault.SecretBundle", existing)
		}
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return secretskeyvault.SecretSetParameters{
		Value: to.StringPtr(base64.StdEncoding.EncodeToString([]byte(s.Value))),
	}, nil
}
