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

package v1alpha3

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetDefaultImageSKUID(t *testing.T) {
	g := NewWithT(t)

	var tests = []struct {
		k8sVersion     string
		expectedResult string
		expectedError  bool
	}{
		{
			k8sVersion:     "v1.14.9",
			expectedResult: "k8s-1dot14dot9-ubuntu-1804",
			expectedError:  false,
		},
		{
			k8sVersion:     "v1.14.10",
			expectedResult: "k8s-1dot14dot10-ubuntu-1804",
			expectedError:  false,
		},
		{
			k8sVersion:     "v1.15.6",
			expectedResult: "k8s-1dot15dot6-ubuntu-1804",
			expectedError:  false,
		},
		{
			k8sVersion:     "v1.15.7",
			expectedResult: "k8s-1dot15dot7-ubuntu-1804",
			expectedError:  false,
		},
		{
			k8sVersion:     "v1.16.3",
			expectedResult: "k8s-1dot16dot3-ubuntu-1804",
			expectedError:  false,
		},
		{
			k8sVersion:     "v1.16.4",
			expectedResult: "k8s-1dot16dot4-ubuntu-1804",
			expectedError:  false,
		},
		{
			k8sVersion:     "1.12.0",
			expectedResult: "k8s-1dot12dot0-ubuntu-1804",
			expectedError:  false,
		},
		{
			k8sVersion:     "1.1.notvalid.semver",
			expectedResult: "",
			expectedError:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.k8sVersion, func(t *testing.T) {
			id, err := getDefaultImageSKUID(test.k8sVersion)

			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(id).To(Equal(test.expectedResult))
		})
	}
}
