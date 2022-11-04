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
