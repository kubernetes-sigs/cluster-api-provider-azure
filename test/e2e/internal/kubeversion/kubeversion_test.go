/*
Copyright The Kubernetes Authors.

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

package kubeversion

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestSelect(t *testing.T) {
	tests := []struct {
		name          string
		requested     string
		aksVersions   []string
		imageVersions []string
		expected      string
		expectedError string
	}{
		{
			name:          "latest falls back to newest image available in AKS",
			requested:     "latest",
			aksVersions:   []string{"1.35.10", "1.36.2", "1.36.3"},
			imageVersions: []string{"1.35.10", "1.36.2"},
			expected:      "v1.36.2",
		},
		{
			name:          "latest ignores images unavailable in AKS",
			requested:     "latest",
			aksVersions:   []string{"v1.35.10", "v1.36.2"},
			imageVersions: []string{"1.36.2", "1.37.0"},
			expected:      "v1.36.2",
		},
		{
			name:          "stable release selects latest common patch",
			requested:     "stable-1.35",
			aksVersions:   []string{"1.35.9", "1.35.10", "1.36.2"},
			imageVersions: []string{"1.35.9", "1.35.10", "1.36.1"},
			expected:      "v1.35.10",
		},
		{
			name:          "unavailable patch falls back within requested minor",
			requested:     "v1.35.11",
			aksVersions:   []string{"1.35.9", "1.35.10", "1.36.2"},
			imageVersions: []string{"1.35.9", "1.35.10", "1.36.2"},
			expected:      "v1.35.10",
		},
		{
			name:          "latest minus one selects preceding common version",
			requested:     "latest-1",
			aksVersions:   []string{"1.35.10", "1.36.1", "1.36.2"},
			imageVersions: []string{"1.35.10", "1.36.1", "1.36.2"},
			expected:      "v1.36.1",
		},
		{
			name:          "no intersection fails",
			requested:     "latest",
			aksVersions:   []string{"1.36.3"},
			imageVersions: []string{"1.36.2"},
			expectedError: "no stable Kubernetes versions available",
		},
		{
			name:          "stable release without a common minor fails",
			requested:     "stable-1.34",
			aksVersions:   []string{"1.35.10", "1.36.2"},
			imageVersions: []string{"1.35.10", "1.36.2"},
			expectedError: "no Kubernetes version available for v1.34",
		},
		{
			name:          "unparsable requested version fails",
			requested:     "not-a-version",
			aksVersions:   []string{"1.36.2"},
			imageVersions: []string{"1.36.2"},
			expectedError: `invalid Kubernetes version "not-a-version"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			version, err := Select(tt.requested, Common(tt.aksVersions, tt.imageVersions))
			if tt.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedError))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(version).To(Equal(tt.expected))
		})
	}
}

func TestCommon(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "canonicalizes, sorts, and dedupes the intersection",
			a:        []string{"1.36.2", "v1.36.2", "1.35.10", "1.36.10"},
			b:        []string{"v1.36.10", "1.35.10", "1.36.2"},
			expected: []string{"v1.35.10", "v1.36.2", "v1.36.10"},
		},
		{
			name:     "drops prereleases and unparsable versions",
			a:        []string{"1.36.2", "1.37.0-alpha.1", "not-a-version", ""},
			b:        []string{"1.36.2", "1.37.0-alpha.1", "not-a-version", ""},
			expected: []string{"v1.36.2"},
		},
		{
			name:     "returns empty when nothing is shared",
			a:        []string{"1.36.3"},
			b:        []string{"1.36.2"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(Common(tt.a, tt.b)).To(Equal(tt.expected))
		})
	}
}

func TestLatestStable(t *testing.T) {
	tests := []struct {
		name          string
		versions      []string
		offset        int
		expected      string
		expectedError string
	}{
		{
			name:     "unsorted and unprefixed input",
			versions: []string{"1.36.2", "1.35.10", "v1.36.10", "1.36.9"},
			expected: "v1.36.10",
		},
		{
			name:     "offset selects the preceding version",
			versions: []string{"1.36.2", "1.35.10", "v1.36.10", "1.36.9"},
			offset:   1,
			expected: "v1.36.9",
		},
		{
			name:     "prereleases and unparsable entries are dropped",
			versions: []string{"1.36.2", "1.37.0-alpha.1", "not-a-version", ""},
			expected: "v1.36.2",
		},
		{
			name:          "offset beyond available versions fails",
			versions:      []string{"1.36.2"},
			offset:        1,
			expectedError: "fewer than 2 stable Kubernetes versions available",
		},
		{
			name:          "no parseable versions fails",
			versions:      []string{"not-a-version"},
			expectedError: "no stable Kubernetes versions available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			version, err := LatestStable(tt.versions, tt.offset)
			if tt.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedError))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(version).To(Equal(tt.expected))
		})
	}
}
