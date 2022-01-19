/*
Copyright 2021 The Kubernetes Authors.

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

package converters

import (
	"net/http"
	"testing"

	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	sdkFuture, _ = azureautorest.NewFutureFromResponse(&http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Request: &http.Request{
			Method: http.MethodDelete,
		},
	})

	validFuture = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   "test-service",
		Name:          "test-group",
		ResourceGroup: "test-group",
		Data:          "eyJtZXRob2QiOiJERUxFVEUiLCJwb2xsaW5nTWV0aG9kIjoiTG9jYXRpb24iLCJscm9TdGF0ZSI6IkluUHJvZ3Jlc3MifQ==",
	}

	emptyDataFuture = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   "test-service",
		Name:          "test-group",
		ResourceGroup: "test-group",
		Data:          "",
	}

	decodedDataFuture = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   "test-service",
		Name:          "test-group",
		ResourceGroup: "test-group",
		Data:          "this is not b64 encoded",
	}

	invalidFuture = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   "test-service",
		Name:          "test-group",
		ResourceGroup: "test-group",
		Data:          "ZmFrZSBiNjQgZnV0dXJlIGRhdGEK",
	}
)

func Test_SDKToFuture(t *testing.T) {
	cases := []struct {
		name         string
		future       azureautorest.FutureAPI
		futureType   string
		service      string
		resourceName string
		rgName       string
		expect       func(*GomegaWithT, *infrav1.Future, error)
	}{
		{
			name:         "valid future",
			future:       &sdkFuture,
			futureType:   infrav1.DeleteFuture,
			service:      "test-service",
			resourceName: "test-resource",
			rgName:       "test-group",
			expect: func(g *GomegaWithT, f *infrav1.Future, err error) {
				g.Expect(err).Should(BeNil())
				g.Expect(f).Should(BeEquivalentTo(&infrav1.Future{
					Type:          infrav1.DeleteFuture,
					ServiceName:   "test-service",
					Name:          "test-resource",
					ResourceGroup: "test-group",
					Data:          "eyJtZXRob2QiOiJERUxFVEUiLCJwb2xsaW5nTWV0aG9kIjoiIiwicG9sbGluZ1VSSSI6IiIsImxyb1N0YXRlIjoiU3VjY2VlZGVkIiwicmVzdWx0VVJJIjoiIn0=",
				}))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result, err := SDKToFuture(c.future, c.futureType, c.service, c.resourceName, c.rgName)
			c.expect(g, result, err)
		})
	}
}

func Test_FutureToSDK(t *testing.T) {
	cases := []struct {
		name   string
		future infrav1.Future
		expect func(*GomegaWithT, azureautorest.FutureAPI, error)
	}{
		{
			name:   "data is empty",
			future: emptyDataFuture,
			expect: func(g *GomegaWithT, f azureautorest.FutureAPI, err error) {
				g.Expect(err.Error()).Should(ContainSubstring("failed to unmarshal future data"))
			},
		},
		{
			name:   "data is not base64 encoded",
			future: decodedDataFuture,
			expect: func(g *GomegaWithT, f azureautorest.FutureAPI, err error) {
				g.Expect(err.Error()).Should(ContainSubstring("failed to base64 decode future data"))
			},
		},
		{
			name:   "base64 data is not a valid future",
			future: invalidFuture,
			expect: func(g *GomegaWithT, f azureautorest.FutureAPI, err error) {
				g.Expect(err.Error()).Should(ContainSubstring("failed to unmarshal future data"))
			},
		},
		{
			name:   "valid future data",
			future: validFuture,
			expect: func(g *GomegaWithT, f azureautorest.FutureAPI, err error) {
				g.Expect(err).Should(BeNil())
				g.Expect(f).Should(BeAssignableToTypeOf(&azureautorest.Future{}))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result, err := FutureToSDK(c.future)
			c.expect(g, result, err)
		})
	}
}
