/*
Copyright 2025 The Kubernetes Authors.

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

package networksecuritygroups

import (
	"context"
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func TestList(t *testing.T) {
	g := NewWithT(t)

	ctx := t.Context()
	mockClient := &MockClient{}

	// Test successful list
	expectedNSGs := []v1api20201101.NetworkSecurityGroup{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nsg1",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nsg2",
				Namespace: "default",
			},
		},
	}

	mockClient.nsgList = &v1api20201101.NetworkSecurityGroupList{
		Items: expectedNSGs,
	}

	result, err := list(ctx, mockClient)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(2))
	g.Expect(*result[0]).To(Equal(expectedNSGs[0]))
	g.Expect(*result[1]).To(Equal(expectedNSGs[1]))
}

func TestServiceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(serviceName).To(Equal("networksecuritygroups"))
}

func TestNetworkSecurityGroupsCondition(t *testing.T) {
	g := NewWithT(t)
	g.Expect(string(NetworkSecurityGroupsCondition)).To(Equal("NetworkSecurityGroupsReady"))
}

// MockClient implements client.Client for testing
type MockClient struct {
	client.Client
	nsgList   *v1api20201101.NetworkSecurityGroupList
	listError error
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.listError != nil {
		return m.listError
	}

	if nsgList, ok := list.(*v1api20201101.NetworkSecurityGroupList); ok {
		if m.nsgList != nil {
			*nsgList = *m.nsgList
		}
		return nil
	}

	return nil
}

// MockNetworkSecurityGroupScope implements NetworkSecurityGroupScope for testing
type MockNetworkSecurityGroupScope struct {
	specs []azure.ASOResourceSpecGetter[*v1api20201101.NetworkSecurityGroup]
}

func (m *MockNetworkSecurityGroupScope) NetworkSecurityGroupSpecs() []azure.ASOResourceSpecGetter[*v1api20201101.NetworkSecurityGroup] {
	if m.specs == nil {
		return []azure.ASOResourceSpecGetter[*v1api20201101.NetworkSecurityGroup]{
			&NSGSpec{
				Name:          "test-nsg",
				Location:      "eastus",
				ClusterName:   "test-cluster",
				ResourceGroup: "test-rg",
				SecurityRules: []infrav1.SecurityRule{
					{
						Name:             "allow_ssh",
						Description:      "Allow SSH",
						Priority:         1000,
						Protocol:         infrav1.SecurityGroupProtocolTCP,
						Direction:        infrav1.SecurityRuleDirectionInbound,
						Source:           ptr.To("*"),
						SourcePorts:      ptr.To("*"),
						Destination:      ptr.To("*"),
						DestinationPorts: ptr.To("22"),
						Action:           infrav1.SecurityRuleActionAllow,
					},
				},
				AdditionalTags: infrav1.Tags{
					"environment": "test",
				},
			},
		}
	}
	return m.specs
}
