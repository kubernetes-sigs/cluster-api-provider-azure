//go:build e2e
// +build e2e

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

package e2e

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

func TestGetMaxVersion(t *testing.T) {
	g := NewWithT(t)
	tests := []struct {
		name               string
		orchestratorsInput []containerservice.OrchestratorVersionProfile
		maxVersionInput    string
		expectedFound      bool
		expectedMaxVersion string
	}{
		{
			name: "pick up the latest v1.23 version",
			orchestratorsInput: []containerservice.OrchestratorVersionProfile{
				{
					OrchestratorVersion: to.StringPtr("v1.24.6"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.24.3"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.12"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.8"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.15"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.11"),
				},
			},
			maxVersionInput:    "v1.23",
			expectedFound:      true,
			expectedMaxVersion: "v1.23.12",
		},
		{
			name: "pick up the latest 1.23 version (no v prefix)",
			orchestratorsInput: []containerservice.OrchestratorVersionProfile{
				{
					OrchestratorVersion: to.StringPtr("1.24.6"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.24.3"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.23.12"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.23.8"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.22.15"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.22.11"),
				},
			},
			maxVersionInput:    "1.23",
			expectedFound:      true,
			expectedMaxVersion: "v1.23.12",
		},
		{
			name: "don't enforce a max version",
			orchestratorsInput: []containerservice.OrchestratorVersionProfile{
				{
					OrchestratorVersion: to.StringPtr("v1.24.6"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.24.3"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.12"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.8"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.15"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.11"),
				},
			},
			maxVersionInput:    "",
			expectedFound:      true,
			expectedMaxVersion: "v1.24.6",
		},
		{
			name: "don't enforce a max version (no v prefix)",
			orchestratorsInput: []containerservice.OrchestratorVersionProfile{
				{
					OrchestratorVersion: to.StringPtr("1.24.6"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.24.3"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.23.12"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.23.8"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.22.15"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.22.11"),
				},
			},
			maxVersionInput:    "",
			expectedFound:      true,
			expectedMaxVersion: "v1.24.6",
		},
		{
			name: "max version is greater than any available version",
			orchestratorsInput: []containerservice.OrchestratorVersionProfile{
				{
					OrchestratorVersion: to.StringPtr("v1.24.6"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.24.3"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.12"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.8"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.15"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.11"),
				},
			},
			maxVersionInput:    "v1.25",
			expectedFound:      true,
			expectedMaxVersion: "v1.24.6",
		},
		{
			name: "max version is greater than any available version (no v prefix)",
			orchestratorsInput: []containerservice.OrchestratorVersionProfile{
				{
					OrchestratorVersion: to.StringPtr("1.24.6"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.24.3"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.23.12"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.23.8"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.22.15"),
				},
				{
					OrchestratorVersion: to.StringPtr("1.22.11"),
				},
			},
			maxVersionInput:    "1.25",
			expectedFound:      true,
			expectedMaxVersion: "v1.24.6",
		},
		{
			name: "all versions greater than max",
			orchestratorsInput: []containerservice.OrchestratorVersionProfile{
				{
					OrchestratorVersion: to.StringPtr("v1.24.6"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.24.3"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.12"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.23.8"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.15"),
				},
				{
					OrchestratorVersion: to.StringPtr("v1.22.11"),
				},
			},
			maxVersionInput:    "v1.21",
			expectedFound:      false,
			expectedMaxVersion: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxVersion, found := getMaxVersion(tt.orchestratorsInput, tt.maxVersionInput)

			g.Expect(found).To(Equal(tt.expectedFound))
			g.Expect(maxVersion).To(Equal(tt.expectedMaxVersion))
		})
	}
}

func TestSemverPrependV(t *testing.T) {
	g := NewWithT(t)
	tests := []struct {
		name          string
		input         string
		expected      string
		expectedError error
	}{
		{
			name:     "1.23",
			input:    "1.23",
			expected: "v1.23",
		},
		{
			name:     "v1.23",
			input:    "v1.23",
			expected: "v1.23",
		},
		{
			name:          "empty string",
			input:         "",
			expected:      "",
			expectedError: errors.New("not a valid semver"),
		},
		{
			name:          "not a valid semver",
			input:         "Hello, World!",
			expected:      "",
			expectedError: errors.New("not a valid semver"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := semverPrependV(tt.input)

			g.Expect(ret).To(Equal(tt.expected))
			if tt.expectedError != nil {
				g.Expect(err).ToNot(BeNil())
			}
		})
	}
}
