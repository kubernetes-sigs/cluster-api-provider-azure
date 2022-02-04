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

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
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
		{
			k8sVersion:     "v1.20.8",
			expectedResult: "k8s-1dot20dot8-windows-2019",
			expectedError:  false,
			os:             "windows",
			osVersion:      "2019",
		},
		{
			k8sVersion:     "v1.21.2",
			expectedResult: "k8s-1dot21dot2-windows-2019",
			expectedError:  false,
			os:             "windows",
			osVersion:      "2019",
		},
		{
			k8sVersion:     "v1.20.8",
			expectedResult: "k8s-1dot20dot8-ubuntu-2004",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "2004",
		},
		{
			k8sVersion:     "v1.21.2",
			expectedResult: "k8s-1dot21dot2-ubuntu-2004",
			expectedError:  false,
			os:             "ubuntu",
			osVersion:      "2004",
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

func TestGetDefaultUbuntuImage(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		k8sVersion  string
		expectedSKU string
	}{
		{
			k8sVersion:  "v1.15.6",
			expectedSKU: "k8s-1dot15dot6-ubuntu-1804",
		},
		{
			k8sVersion:  "v1.17.11",
			expectedSKU: "k8s-1dot17dot11-ubuntu-1804",
		},
		{
			k8sVersion:  "v1.18.19",
			expectedSKU: "k8s-1dot18dot19-ubuntu-1804",
		},
		{
			k8sVersion:  "v1.18.20",
			expectedSKU: "k8s-1dot18dot20-ubuntu-2004",
		},
		{
			k8sVersion:  "v1.19.11",
			expectedSKU: "k8s-1dot19dot11-ubuntu-1804",
		},
		{
			k8sVersion:  "v1.19.12",
			expectedSKU: "k8s-1dot19dot12-ubuntu-2004",
		},
		{
			k8sVersion:  "v1.21.1",
			expectedSKU: "k8s-1dot21dot1-ubuntu-1804",
		},
		{
			k8sVersion:  "v1.21.2",
			expectedSKU: "k8s-1dot21dot2-ubuntu-2004",
		},
		{
			k8sVersion:  "v1.22.0",
			expectedSKU: "k8s-1dot22dot0-ubuntu-2004",
		},
		{
			k8sVersion:  "v1.23.6",
			expectedSKU: "k8s-1dot23dot6-ubuntu-2004",
		},
	}

	for _, test := range tests {
		t.Run(test.k8sVersion, func(t *testing.T) {
			image, err := GetDefaultUbuntuImage(test.k8sVersion)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(image.Marketplace.SKU).To(Equal(test.expectedSKU))
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
		req, err := http.NewRequest("GET", testSrv.URL, nil)
		if err != nil {
			return nil, err
		}
		req.Header = r.Header
		return testSrv.Client().Do(req)
	})
	newSender := autorest.DecorateSender(origSender, msCorrelationIDSendDecorator)

	// create a new HTTP request and send it via the new decorated sender
	req, err := http.NewRequest("GET", "/abc", nil)
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
