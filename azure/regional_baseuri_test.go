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

package azure

import (
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
)

func TestWithRegionalBaseURI(t *testing.T) {
	cases := []struct {
		Name              string
		AuthorizerFactory func(authMock *mock_azure.MockAuthorizer) Authorizer
		Region            string
		Result            string
	}{
		{
			Name: "with a region",
			AuthorizerFactory: func(authMock *mock_azure.MockAuthorizer) Authorizer {
				authMock.EXPECT().BaseURI().Return("http://foo.bar").AnyTimes()
				return authMock
			},
			Region: "bazz",
			Result: "http://bazz.foo.bar",
		},
		{
			Name: "with no region",
			AuthorizerFactory: func(authMock *mock_azure.MockAuthorizer) Authorizer {
				authMock.EXPECT().BaseURI().Return("http://foo.bar").AnyTimes()
				return authMock
			},
			Result: "http://foo.bar",
		},
		{
			Name: "with a region and path",
			AuthorizerFactory: func(authMock *mock_azure.MockAuthorizer) Authorizer {
				authMock.EXPECT().BaseURI().Return("http://foo.bar/something/id").AnyTimes()
				return authMock
			},
			Region: "bazz",
			Result: "http://bazz.foo.bar/something/id",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			authMock := mock_azure.NewMockAuthorizer(mockCtrl)
			regionalAuth, err := WithRegionalBaseURI(c.AuthorizerFactory(authMock), c.Region)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(regionalAuth.BaseURI()).To(Equal(c.Result))
		})
	}
}
