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

package azure

import (
	"fmt"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
)

func TestGetDefaultImageSKUID(t *testing.T) {
	g := NewWithT(t)

	var tests = []struct {
		k8sVersion     string
		os             string
		osVersion      string
		expectedResult string
		expectedError  bool
	}{
		{
			k8sVersion:     "v1.14.9",
			expectedResult: "k8s-1dot14dot9-ubuntu-1804",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "1804",
		},
		{
			k8sVersion:     "v1.14.10",
			expectedResult: "k8s-1dot14dot10-ubuntu-1804",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "1804",
		},
		{
			k8sVersion:     "v1.15.6",
			expectedResult: "k8s-1dot15dot6-ubuntu-1804",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "1804",
		},
		{
			k8sVersion:     "v1.15.7",
			expectedResult: "k8s-1dot15dot7-ubuntu-1804",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "1804",
		},
		{
			k8sVersion:     "v1.16.3",
			expectedResult: "k8s-1dot16dot3-ubuntu-1804",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "1804",
		},
		{
			k8sVersion:     "v1.16.4",
			expectedResult: "k8s-1dot16dot4-ubuntu-1804",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "1804",
		},
		{
			k8sVersion:     "1.12.0",
			expectedResult: "k8s-1dot12dot0-ubuntu-1804",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "1804",
		},
		{
			k8sVersion:     "1.1.notvalid.semver",
			expectedResult: "",
			expectedError:  true,
		},
		{
			k8sVersion:     "v1.19.3",
			expectedResult: "k8s-1dot19dot3-windows-2019",
			expectedError:  false,
			os:             "windows",
			osVersion:      "2019",
		},
	}

	for _, test := range tests {
		t.Run(test.k8sVersion, func(t *testing.T) {
			id, err := getDefaultImageSKUID(test.k8sVersion, test.os, test.osVersion)

			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(id).To(Equal(test.expectedResult))
		})
	}
}

func TestAutoRestClientAppendUserAgent(t *testing.T) {
	g := NewWithT(t)
	userAgent := "cluster-api-provider-azure/2.29.2"

	type args struct {
		c         *autorest.Client
		extension string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should append extension to user agent if extension is not empty",
			args: args{
				c:         &autorest.Client{UserAgent: autorest.UserAgent()},
				extension: userAgent,
			},
			want: fmt.Sprintf("%s %s", autorest.UserAgent(), userAgent),
		},
		{
			name: "should no changed if extension is empty",
			args: args{
				c:         &autorest.Client{UserAgent: userAgent},
				extension: "",
			},
			want: userAgent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AutoRestClientAppendUserAgent(tt.args.c, tt.args.extension)

			g.Expect(tt.want).To(Equal(tt.args.c.UserAgent))
		})
	}
}
