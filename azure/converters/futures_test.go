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
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
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

func TestPollerToFuture(t *testing.T) {
	cases := []struct {
		name        string
		futureType  string
		statusCode  int
		expectedErr string
	}{
		{
			name:       "valid DELETE poller",
			futureType: infrav1.DeleteFuture,
			statusCode: http.StatusAccepted,
		},
		{
			name:        "invalid DELETE poller",
			futureType:  infrav1.DeleteFuture,
			statusCode:  http.StatusNoContent,
			expectedErr: "failed to get resume token",
		},
		{
			name:       "valid PUT poller",
			futureType: infrav1.PutFuture,
			statusCode: http.StatusAccepted,
		},
		{
			name:        "invalid PUT poller",
			futureType:  infrav1.PutFuture,
			statusCode:  http.StatusNoContent,
			expectedErr: "failed to get resume token",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			poller := fakePoller[MockPolled](g, c.statusCode)
			future, err := PollerToFuture(poller, c.futureType, "test-service", "test-resource", "test-group")
			if c.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(c.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(future).NotTo(BeNil())
				g.Expect(future.Data).NotTo(BeNil())
				token, err := base64.URLEncoding.DecodeString(future.Data)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(token)).NotTo(BeEmpty())
				// The following assertion should pass, but the token's format could change.
				// g.Expect(string(token)).To(MatchJSON(`{"type":"MockPolled","token":{"type":"body","pollURL":"/","state":"InProgress"}}`))
			}
		})
	}
}

func TestFutureToResumeToken(t *testing.T) {
	cases := []struct {
		name   string
		future infrav1.Future
		expect func(*GomegaWithT, string, error)
	}{
		{
			name:   "data is empty",
			future: emptyDataFuture,
			expect: func(g *GomegaWithT, token string, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring("failed to unmarshal future data"))
			},
		},
		{
			name:   "data is not base64-encoded",
			future: decodedDataFuture,
			expect: func(g *GomegaWithT, token string, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring("failed to decode future data"))
			},
		},
		{
			name:   "data does not contain a valid resume token",
			future: invalidFuture,
			expect: func(g *GomegaWithT, token string, err error) {
				// "The token's format should be considered opaque and is subject to change."
				// This validates decoding the unit test data, but actual SDKv2 tokens won't look like this.
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(ContainSubstring("fake b64 future data"))
			},
		},
		{
			name:   "data contains a valid resume token",
			future: validFuture,
			expect: func(g *GomegaWithT, token string, err error) {
				// "The token's format should be considered opaque and is subject to change."
				// This validates decoding the unit test data, but actual SDKv2 tokens won't look like this.
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(MatchJSON(`{"method":"DELETE","pollingMethod":"Location","lroState":"InProgress"}`))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			token, err := FutureToResumeToken(c.future)
			c.expect(g, token, err)
		})
	}
}

type MockPolled struct{}

func (m *MockPolled) Done() bool { return true }

func fakePoller[T any](g *GomegaWithT, statusCode int) *runtime.Poller[T] {
	response := &http.Response{
		Body: io.NopCloser(strings.NewReader("")),
		Request: &http.Request{
			Method: http.MethodPut,
			URL:    &url.URL{Path: "/"},
		},
		StatusCode: statusCode,
	}
	pipeline := runtime.NewPipeline("testmodule", "v0.1.0", runtime.PipelineOptions{}, nil)
	poller, err := runtime.NewPoller[T](response, pipeline, nil)
	g.Expect(err).NotTo(HaveOccurred())
	return poller
}
