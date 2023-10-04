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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// TestARMClientOptions tests the `ARMClientOptions()` factory function.
func TestARMClientOptions(t *testing.T) {
	tests := []struct {
		name          string
		cloudName     string
		expectedCloud cloud.Configuration
		expectError   bool
	}{
		{
			name:          "should return default client options if cloudName is empty",
			cloudName:     "",
			expectedCloud: cloud.Configuration{},
		},
		{
			name:          "should return Azure public cloud client options",
			cloudName:     PublicCloudName,
			expectedCloud: cloud.AzurePublic,
		},
		{
			name:          "should return Azure China cloud client options",
			cloudName:     ChinaCloudName,
			expectedCloud: cloud.AzureChina,
		},
		{
			name:          "should return Azure government cloud client options",
			cloudName:     USGovernmentCloudName,
			expectedCloud: cloud.AzureGovernment,
		},
		{
			name:        "should return error if cloudName is unrecognized",
			cloudName:   "AzureUnrecognizedCloud",
			expectError: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			opts, err := ARMClientOptions(tc.cloudName)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Cloud).To(Equal(tc.expectedCloud))
			g.Expect(opts.Retry.MaxRetries).To(BeNumerically("==", -1))
			g.Expect(opts.PerCallPolicies).To(HaveLen(2))
		})
	}
}

// TestPerCallPolicies tests the per-call policies returned by `ARMClientOptions()`.
func TestPerCallPolicies(t *testing.T) {
	g := NewWithT(t)

	corrID := "test-1234abcd-5678efgh"
	// This server will check that the correlation ID and user-agent are set correctly.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Header.Get("User-Agent")).To(ContainSubstring("cluster-api-provider-azure/"))
		g.Expect(r.Header.Get(string(tele.CorrIDKeyVal))).To(Equal(corrID))
		fmt.Fprintf(w, "Hello, %s", r.Proto)
	}))
	defer server.Close()

	// Call the factory function and ensure it has both PerCallPolicies.
	opts, err := ARMClientOptions("")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.PerCallPolicies).To(HaveLen(2))
	g.Expect(opts.PerCallPolicies).To(ContainElement(BeAssignableToTypeOf(correlationIDPolicy{})))
	g.Expect(opts.PerCallPolicies).To(ContainElement(BeAssignableToTypeOf(userAgentPolicy{})))

	// Create a request with a correlation ID.
	ctx := context.WithValue(context.Background(), tele.CorrIDKeyVal, tele.CorrID(corrID))
	req, err := runtime.NewRequest(ctx, http.MethodGet, server.URL)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a pipeline and send the request, where it will be checked by the server.
	pipeline := defaultTestPipeline(opts.PerCallPolicies)
	resp, err := pipeline.Do(req)
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

func TestCustomPutPatchHeaderPolicy(t *testing.T) {
	testHeaders := map[string]string{
		"X-Test-Header":  "test-value",
		"X-Test-Header2": "test-value2",
	}
	testcases := []struct {
		name     string
		method   string
		headers  map[string]string
		expected map[string]string
	}{
		{
			name:     "should add custom headers to PUT request",
			method:   http.MethodPut,
			headers:  testHeaders,
			expected: testHeaders,
		},
		{
			name:     "should add custom headers to PATCH request",
			method:   http.MethodPatch,
			headers:  testHeaders,
			expected: testHeaders,
		},
		{
			name:   "should skip empty custom headers for PUT request",
			method: http.MethodPut,
		},
		{
			name:   "should skip empty custom headers for PATCH request",
			method: http.MethodPatch,
		},
		{
			name:   "should skip empty custom headers for GET request",
			method: http.MethodGet,
		},
		{
			name:    "should not add custom headers to GET request",
			method:  http.MethodGet,
			headers: testHeaders,
		},
		{
			name:    "should not add custom headers to POST request",
			method:  http.MethodPost,
			headers: testHeaders,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			// This server will check that custom headers are set correctly.
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.expected {
					g.Expect(r.Header.Get(k)).To(Equal(v))
				}
				fmt.Fprintf(w, "Hello, %s", r.Proto)
			}))
			defer server.Close()

			// Create options with a custom PUT/PATCH header per-call policy
			getterMock := mock_azure.NewMockResourceSpecGetterWithHeaders(mockCtrl)
			getterMock.EXPECT().CustomHeaders().Return(tc.headers).AnyTimes()
			opts, err := ARMClientOptions("", CustomPutPatchHeaderPolicy{Headers: tc.headers})
			g.Expect(err).NotTo(HaveOccurred())

			// Create a request
			req, err := runtime.NewRequest(context.Background(), tc.method, server.URL)
			g.Expect(err).NotTo(HaveOccurred())

			// Create a pipeline and send the request to the test server for validation.
			pipeline := defaultTestPipeline(opts.PerCallPolicies)
			resp, err := pipeline.Do(req)
			g.Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	}
}

func defaultTestPipeline(policies []policy.Policy) runtime.Pipeline {
	return runtime.NewPipeline(
		"testmodule",
		"v0.1.0",
		runtime.PipelineOptions{},
		&policy.ClientOptions{PerCallPolicies: policies},
	)
}

func TestGetBootstrappingVMExtension(t *testing.T) {
	testCases := []struct {
		name            string
		osType          string
		cloud           string
		vmName          string
		cpuArchitecture string
		expectedVersion string
		expectNil       bool
	}{
		{
			name:            "Linux OS, Public Cloud, x64 CPU Architecture",
			osType:          LinuxOS,
			cloud:           PublicCloudName,
			vmName:          "test-vm",
			cpuArchitecture: "x64",
			expectedVersion: "1.0",
		},
		{
			name:            "Linux OS, Public Cloud, ARM64 CPU Architecture",
			osType:          LinuxOS,
			cloud:           PublicCloudName,
			vmName:          "test-vm",
			cpuArchitecture: "Arm64",
			expectedVersion: "1.1",
		},
		{
			name:            "Windows OS, Public Cloud",
			osType:          WindowsOS,
			cloud:           PublicCloudName,
			vmName:          "test-vm",
			cpuArchitecture: "x64",
			expectedVersion: "1.0",
		},
		{
			name:            "Invalid OS Type",
			osType:          "invalid",
			cloud:           PublicCloudName,
			vmName:          "test-vm",
			cpuArchitecture: "x64",
			expectedVersion: "1.0",
			expectNil:       true,
		},
		{
			name:            "Invalid Cloud",
			osType:          LinuxOS,
			cloud:           "invalid",
			vmName:          "test-vm",
			cpuArchitecture: "x64",
			expectedVersion: "1.0",
			expectNil:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			actualExtension := GetBootstrappingVMExtension(tc.osType, tc.cloud, tc.vmName, tc.cpuArchitecture)
			if tc.expectNil {
				g.Expect(actualExtension).To(BeNil())
			} else {
				g.Expect(actualExtension.Version).To(Equal(tc.expectedVersion))
			}
		})
	}
}
