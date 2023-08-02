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
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

func TestARMClientOptions(t *testing.T) {
	g := NewWithT(t)

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
		t.Run(tc.name, func(t *testing.T) {
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

func TestPerCallPolicies(t *testing.T) {
	g := NewWithT(t)

	corrID := "test-1234abcd-5678efgh"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Header.Get("User-Agent")).To(ContainSubstring("cluster-api-provider-azure/"))
		g.Expect(r.Header.Get(string(tele.CorrIDKeyVal))).To(Equal(corrID))
		fmt.Fprintf(w, "Hello, %s", r.Proto)
	}))
	defer server.Close()

	opts, err := ARMClientOptions("")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.PerCallPolicies).To(HaveLen(2))
	ctx := context.WithValue(context.Background(), tele.CorrIDKeyVal, tele.CorrID(corrID))
	req, err := runtime.NewRequest(ctx, http.MethodGet, server.URL)
	g.Expect(err).NotTo(HaveOccurred())
	pipeline := defaultTestPipeline(opts.PerCallPolicies)
	resp, err := pipeline.Do(req)
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

func defaultTestPipeline(policies []policy.Policy) runtime.Pipeline {
	return runtime.NewPipeline(
		"testmodule",
		"v0.1.0",
		runtime.PipelineOptions{},
		&policy.ClientOptions{PerCallPolicies: policies},
	)
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

func TestMSCorrelationIDSendDecorator(t *testing.T) {
	g := NewWithT(t)
	const corrID tele.CorrID = "TestMSCorrelationIDSendDecoratorCorrID"
	ctx := context.WithValue(context.Background(), tele.CorrIDKeyVal, corrID)

	// create a fake server so that the sender can send to
	// somewhere
	var wg sync.WaitGroup
	receivedReqs := []*http.Request{}
	wg.Add(1)
	originHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReqs = append(receivedReqs, r)
		wg.Done()
	})

	testSrv := httptest.NewServer(originHandler)
	defer testSrv.Close()

	// create a sender that sends to the fake server, then
	// decorate the sender with the msCorrelationIDSendDecorator
	origSender := autorest.SenderFunc(func(r *http.Request) (*http.Response, error) {
		// preserve the incoming headers to the fake server, so that
		// we can test that the fake server received the right
		// correlation ID header.
		req, err := http.NewRequest(http.MethodGet, testSrv.URL, http.NoBody)
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)
		req.Header = r.Header
		return testSrv.Client().Do(req)
	})
	newSender := autorest.DecorateSender(origSender, msCorrelationIDSendDecorator)

	// create a new HTTP request and send it via the new decorated sender
	req, err := http.NewRequest(http.MethodGet, "/abc", http.NoBody)
	g.Expect(err).NotTo(HaveOccurred())

	req = req.WithContext(ctx)
	rsp, err := newSender.Do(req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rsp.Body.Close()).To(Succeed())
	wg.Wait()
	g.Expect(len(receivedReqs)).To(Equal(1))
	receivedReq := receivedReqs[0]
	g.Expect(
		receivedReq.Header.Get(string(tele.CorrIDKeyVal)),
	).To(Equal(string(corrID)))
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
			name:            "Linux OS, Public Cloud, arm64 CPU Architecture",
			osType:          LinuxOS,
			cloud:           PublicCloudName,
			vmName:          "test-vm",
			cpuArchitecture: "arm64",
			expectedVersion: "1.1.1",
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
