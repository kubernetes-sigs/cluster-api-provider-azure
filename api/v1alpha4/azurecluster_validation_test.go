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

package v1alpha4

import (
	"testing"

	"k8s.io/utils/pointer"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestClusterNameValidation(t *testing.T) {
	g := NewWithT(t)
	tests := []struct {
		name        string
		clusterName string
		wantErr     bool
	}{
		{
			name:        "cluster name more than 44 characters",
			clusterName: "vegkebfadbczdtevzjiyookobkdgfofjxmlquonomzoes",
			wantErr:     true,
		},
		{
			name:        "cluster name with letters",
			clusterName: "cluster",
			wantErr:     false,
		},
		{
			name:        "cluster name with upper case letters",
			clusterName: "clusterName",
			wantErr:     true,
		},
		{
			name:        "cluster name with hyphen",
			clusterName: "test-cluster",
			wantErr:     false,
		},
		{
			name:        "cluster name with letters and numbers",
			clusterName: "clustername1",
			wantErr:     false,
		},
		{
			name:        "cluster name with special characters",
			clusterName: "cluster$?name",
			wantErr:     true,
		},
		{
			name:        "cluster name starting with underscore",
			clusterName: "_clustername",
			wantErr:     true,
		},
		{
			name:        "cluster name starting with number",
			clusterName: "1clustername",
			wantErr:     false,
		},
		{
			name:        "cluster name with underscore",
			clusterName: "cluster_name",
			wantErr:     true,
		},
		{
			name:        "cluster name with period",
			clusterName: "cluster.name",
			wantErr:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			azureCluster := AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
			}

			allErrs := azureCluster.validateClusterName()
			if tc.wantErr {
				g.Expect(allErrs).ToNot(BeNil())
			} else {
				g.Expect(allErrs).To(BeNil())
			}
		})
	}
}

func TestClusterWithPreexistingVnetValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster with pre-existing vnet - valid",
		cluster: createValidCluster(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := testCase.cluster.validateCluster(nil)
		g.Expect(err).To(BeNil())
	})
}

func TestClusterWithPreexistingVnetInvalid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster with pre-existing vnet - invalid",
		cluster: createValidCluster(),
	}

	// invalid because it doesn't specify a controlplane subnet
	testCase.cluster.Spec.NetworkSpec.Subnets[0] = SubnetSpec{
		Name: "random-subnet",
		Role: "random",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := testCase.cluster.validateCluster(nil)
		g.Expect(err).ToNot(BeNil())
	})
}

func TestClusterWithoutPreexistingVnetValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster without pre-existing vnet - valid",
		cluster: createValidCluster(),
	}

	// When ResourceGroup is an empty string, the cluster doesn't
	// have a pre-existing vnet.
	testCase.cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""

	t.Run(testCase.name, func(t *testing.T) {
		err := testCase.cluster.validateCluster(nil)
		g.Expect(err).To(BeNil())
	})
}

func TestClusterSpecWithPreexistingVnetValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec with pre-existing vnet - valid",
		cluster: createValidCluster(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(errs).To(BeNil())
	})
}

func TestClusterSpecWithPreexistingVnetInvalid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec with pre-existing vnet - invalid",
		cluster: createValidCluster(),
	}

	// invalid because it doesn't specify a controlplane subnet
	testCase.cluster.Spec.NetworkSpec.Subnets[0] = SubnetSpec{
		Name: "random-subnet",
		Role: "random",
	}

	t.Run(testCase.name, func(t *testing.T) {
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(len(errs)).To(BeNumerically(">", 0))
	})
}

func TestClusterSpecWithoutPreexistingVnetValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec without pre-existing vnet - valid",
		cluster: createValidCluster(),
	}

	// When ResourceGroup is an empty string, the cluster doesn't
	// have a pre-existing vnet.
	testCase.cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""

	t.Run(testCase.name, func(t *testing.T) {
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(errs).To(BeNil())
	})
}

func TestNetworkSpecWithPreexistingVnetValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec with pre-existing vnet - valid",
		networkSpec: createValidNetworkSpec(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(BeNil())
	})
}

func TestNetworkSpecWithPreexistingVnetLackRequiredSubnets(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec with pre-existing vnet - lack required subnets",
		networkSpec: createValidNetworkSpec(),
	}

	// invalid because it doesn't specify a node subnet
	testCase.networkSpec.Subnets = testCase.networkSpec.Subnets[:1]

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets"))
		g.Expect(errs[0].Error()).To(ContainSubstring("required role node not included"))
	})
}

func TestNetworkSpecWithPreexistingVnetInvalidResourceGroup(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec with pre-existing vnet - invalid resource group",
		networkSpec: createValidNetworkSpec(),
	}

	testCase.networkSpec.Vnet.ResourceGroup = "invalid-name###"

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.vnet.resourceGroup"))
		g.Expect(errs[0].BadValue).To(BeEquivalentTo(testCase.networkSpec.Vnet.ResourceGroup))
	})
}

func TestNetworkSpecWithoutPreexistingVnetValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec without pre-existing vnet - valid",
		networkSpec: createValidNetworkSpec(),
	}

	testCase.networkSpec.Vnet.ResourceGroup = ""

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(BeNil())
	})
}

func TestResourceGroupValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name          string
		resourceGroup string
	}

	testCase := test{
		name:          "resourcegroup name - valid",
		resourceGroup: "custom-vnet",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateResourceGroup(testCase.resourceGroup,
			field.NewPath("spec").Child("networkSpec").Child("vnet").Child("resourceGroup"))
		g.Expect(err).To(BeNil())
	})
}

func TestResourceGroupInvalid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name          string
		resourceGroup string
	}

	testCase := test{
		name:          "resourcegroup name - invalid",
		resourceGroup: "inv@lid-rg",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateResourceGroup(testCase.resourceGroup,
			field.NewPath("spec").Child("networkSpec").Child("vnet").Child("resourceGroup"))
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(err.Field).To(Equal("spec.networkSpec.vnet.resourceGroup"))
		g.Expect(err.BadValue).To(BeEquivalentTo(testCase.resourceGroup))
	})
}

func TestSubnetsValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - valid",
		subnets: createValidSubnets(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateSubnets(testCase.subnets,
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(BeNil())
	})
}

func TestSubnetsInvalidSubnetName(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - invalid subnet name",
		subnets: createValidSubnets(),
	}

	testCase.subnets[0].Name = "invalid-subnet-name-due-to-bracket)"

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateSubnets(testCase.subnets,
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets[0].name"))
		g.Expect(errs[0].BadValue).To(BeEquivalentTo("invalid-subnet-name-due-to-bracket)"))
	})
}

func TestSubnetsInvalidLackRequiredSubnet(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - lack required subnet",
		subnets: createValidSubnets(),
	}

	testCase.subnets[0].Role = "random-role"

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateSubnets(testCase.subnets,
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets"))
		g.Expect(errs[0].Detail).To(ContainSubstring("required role control-plane not included"))
	})
}

func TestSubnetNamesNotUnique(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - names not unique",
		subnets: createValidSubnets(),
	}

	testCase.subnets[0].Name = "subnet-name"
	testCase.subnets[1].Name = "subnet-name"

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateSubnets(testCase.subnets,
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeDuplicate))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets"))
	})
}

func TestSubnetNameValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name       string
		subnetName string
	}

	testCase := test{
		name:       "subnet name - valid",
		subnetName: "control-plane-subnet",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateSubnetName(testCase.subnetName,
			field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("name"))
		g.Expect(err).To(BeNil())
	})
}

func TestSubnetNameInvalid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name       string
		subnetName string
	}

	testCase := test{
		name:       "subnet name - invalid",
		subnetName: "inv@lid-subnet-name",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateSubnetName(testCase.subnetName,
			field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("name"))
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(err.Field).To(Equal("spec.networkSpec.subnets[0].name"))
		g.Expect(err.BadValue).To(BeEquivalentTo(testCase.subnetName))
	})
}

func TestIngressRules(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name      string
		validRule IngressRule
		wantErr   bool
	}{
		{
			name: "ingressRule - valid priority",
			validRule: IngressRule{
				Name:        "allow_apiserver",
				Description: "Allow K8s API Server",
				Priority:    101,
			},
			wantErr: false,
		},
		{
			name: "ingressRule - invalid low priority",
			validRule: IngressRule{
				Name:        "allow_apiserver",
				Description: "Allow K8s API Server",
				Priority:    99,
			},
			wantErr: true,
		},
		{
			name: "ingressRule - invalid high priority",
			validRule: IngressRule{
				Name:        "allow_apiserver",
				Description: "Allow K8s API Server",
				Priority:    5000,
			},
			wantErr: true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := validateIngressRule(
				testCase.validRule,
				field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("securityGroup").Child("ingressRules").Index(0),
			)
			if testCase.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestValidateAPIServerLB(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name        string
		lb          LoadBalancerSpec
		old         LoadBalancerSpec
		cpCIDRS     []string
		wantErr     bool
		expectedErr field.Error
	}{
		{
			name: "invalid SKU",
			lb: LoadBalancerSpec{
				Name: "my-awesome-lb",
				SKU:  "Awesome",
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-config",
					},
				},
				Type: Public,
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueNotSupported",
				Field:    "apiServerLB.sku",
				BadValue: "Awesome",
				Detail:   "supported values: \"Standard\"",
			},
		},
		{
			name: "invalid Type",
			lb: LoadBalancerSpec{
				Type: "Foo",
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueNotSupported",
				Field:    "apiServerLB.type",
				BadValue: "Foo",
				Detail:   "supported values: \"Public\", \"Internal\"",
			},
		},
		{
			name: "invalid Name",
			lb: LoadBalancerSpec{
				Name: "***",
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "apiServerLB.name",
				BadValue: "***",
				Detail:   "name of load balancer doesn't match regex ^[-\\w\\._]+$",
			},
		},
		{
			name: "too many IP configs",
			lb: LoadBalancerSpec{
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-1",
					},
					{
						Name: "ip-2",
					},
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:  "FieldValueInvalid",
				Field: "apiServerLB.frontendIPConfigs",
				BadValue: []FrontendIP{
					{
						Name: "ip-1",
					},
					{
						Name: "ip-2",
					},
				},
				Detail: "API Server Load balancer should have 1 Frontend IP configuration",
			},
		},
		{
			name: "public LB with private IP",
			lb: LoadBalancerSpec{
				Type: Public,
				FrontendIPs: []FrontendIP{
					{
						Name:             "ip-1",
						PrivateIPAddress: "10.0.0.4",
					},
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:   "FieldValueForbidden",
				Field:  "apiServerLB.frontendIPConfigs[0].privateIP",
				Detail: "Public Load Balancers cannot have a Private IP",
			},
		},
		{
			name: "internal LB with public IP",
			lb: LoadBalancerSpec{
				Type: Internal,
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-1",
						PublicIP: &PublicIPSpec{
							Name: "my-invalid-ip",
						},
					},
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:   "FieldValueForbidden",
				Field:  "apiServerLB.frontendIPConfigs[0].publicIP",
				Detail: "Internal Load Balancers cannot have a Public IP",
			},
		},
		{
			name: "internal LB with invalid private IP",
			lb: LoadBalancerSpec{
				Type: Internal,
				FrontendIPs: []FrontendIP{
					{
						Name:             "ip-1",
						PrivateIPAddress: "NAIP",
					},
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "apiServerLB.frontendIPConfigs[0].privateIP",
				BadValue: "NAIP",
				Detail:   "Internal LB IP address isn't a valid IPv4 or IPv6 address",
			},
		},
		{
			name: "internal LB with out of range private IP",
			lb: LoadBalancerSpec{
				Type: Internal,
				FrontendIPs: []FrontendIP{
					{
						Name:             "ip-1",
						PrivateIPAddress: "20.1.2.3",
					},
				},
			},
			cpCIDRS: []string{"10.0.0.0/24", "10.1.0.0/24"},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "apiServerLB.frontendIPConfigs[0].privateIP",
				BadValue: "20.1.2.3",
				Detail:   "Internal LB IP address needs to be in control plane subnet range ([10.0.0.0/24 10.1.0.0/24])",
			},
		},
		{
			name: "internal LB with in range private IP",
			lb: LoadBalancerSpec{
				Type: Internal,
				SKU:  SKUStandard,
				Name: "my-private-lb",
				FrontendIPs: []FrontendIP{
					{
						Name:             "ip-1",
						PrivateIPAddress: "10.1.0.3",
					},
				},
			},
			cpCIDRS: []string{"10.0.0.0/24", "10.1.0.0/24"},
			wantErr: false,
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateAPIServerLB(test.lb, test.old, test.cpCIDRS, field.NewPath("apiServerLB"))
			if test.wantErr {
				g.Expect(err).NotTo(HaveLen(0))
				found := false
				for _, actual := range err {
					if actual.Error() == test.expectedErr.Error() {
						found = true
					}
				}
				g.Expect(found).To(BeTrue())
			} else {
				g.Expect(err).To(HaveLen(0))
			}
		})
	}
}

func TestValidateNodeOutboundLB(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name        string
		lb          *LoadBalancerSpec
		old         *LoadBalancerSpec
		apiServerLB LoadBalancerSpec
		wantErr     bool
		expectedErr field.Error
	}{
		{
			name:        "no lb for public clusters",
			lb:          nil,
			apiServerLB: LoadBalancerSpec{Type: Public},
			wantErr:     true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "nodeOutboundLB",
				BadValue: nil,
				Detail:   "Node outbound load balancer cannot be nil for public clusters.",
			},
		},
		{
			name:        "no lb allowed for internal clusters",
			lb:          nil,
			apiServerLB: LoadBalancerSpec{Type: Internal},
			wantErr:     false,
		},
		{
			name: "invalid ID update",
			lb: &LoadBalancerSpec{
				ID: "some-id",
			},
			old: &LoadBalancerSpec{
				ID: "old-id",
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "nodeOutboundLB.id",
				BadValue: "some-id",
				Detail:   "Node outbound load balancer ID should not be modified after AzureCluster creation.",
			},
		},
		{
			name: "invalid Name update",
			lb: &LoadBalancerSpec{
				Name: "some-name",
			},
			old: &LoadBalancerSpec{
				Name: "old-name",
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "nodeOutboundLB.name",
				BadValue: "some-name",
				Detail:   "Node outbound load balancer Name should not be modified after AzureCluster creation.",
			},
		},
		{
			name: "invalid SKU update",
			lb: &LoadBalancerSpec{
				SKU: "some-sku",
			},
			old: &LoadBalancerSpec{
				SKU: "old-sku",
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "nodeOutboundLB.sku",
				BadValue: "some-sku",
				Detail:   "Node outbound load balancer SKU should not be modified after AzureCluster creation.",
			},
		},
		{
			name: "invalid FrontendIps update",
			lb: &LoadBalancerSpec{
				FrontendIPs: []FrontendIP{{
					Name: "some-frontend-ip",
				}},
			},
			old: &LoadBalancerSpec{
				FrontendIPs: []FrontendIP{{
					Name: "old-frontend-ip",
				}},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:  "FieldValueInvalid",
				Field: "nodeOutboundLB.frontendIPs[0]",
				BadValue: FrontendIP{
					Name: "some-frontend-ip",
				},
				Detail: "Node outbound load balancer FrontendIPs is not allowed to be modified after AzureCluster creation.",
			},
		},
		{
			name: "FrontendIps can update when frontendIpsCount changes",
			lb: &LoadBalancerSpec{
				FrontendIPs: []FrontendIP{{
					Name: "some-frontend-ip-1",
				}, {
					Name: "some-frontend-ip-2",
				}},
				FrontendIPsCount: pointer.Int32Ptr(2),
			},
			old: &LoadBalancerSpec{
				FrontendIPs: []FrontendIP{{
					Name: "old-frontend-ip",
				}},
			},
			wantErr: false,
		},
		{
			name: "frontend ips count exceeds max value",
			lb: &LoadBalancerSpec{
				FrontendIPsCount: pointer.Int32Ptr(100),
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "nodeOutboundLB.frontendIPsCount",
				BadValue: 100,
				Detail:   "Max front end ips allowed is 16",
			},
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateNodeOutboundLB(test.lb, test.old, test.apiServerLB, field.NewPath("nodeOutboundLB"))
			if test.wantErr {
				g.Expect(err).NotTo(HaveLen(0))
				found := false
				for _, actual := range err {
					if actual.Error() == test.expectedErr.Error() {
						found = true
					}
				}
				g.Expect(found).To(BeTrue())
			} else {
				g.Expect(err).To(HaveLen(0))
			}
		})
	}
}

func createValidCluster() *AzureCluster {
	return &AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: AzureClusterSpec{
			NetworkSpec: createValidNetworkSpec(),
		},
	}
}

func createValidNetworkSpec() NetworkSpec {
	return NetworkSpec{
		Vnet: VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets:        createValidSubnets(),
		APIServerLB:    createValidAPIServerLB(),
		NodeOutboundLB: createValidNodeOutboundLB(),
	}
}

func createValidSubnets() Subnets {
	return Subnets{
		{
			Name: "control-plane-subnet",
			Role: "control-plane",
		},
		{
			Name: "node-subnet",
			Role: "node",
		},
	}
}

func createValidAPIServerLB() LoadBalancerSpec {
	return LoadBalancerSpec{
		Name: "my-lb",
		SKU:  SKUStandard,
		FrontendIPs: []FrontendIP{
			{
				Name: "ip-config",
				PublicIP: &PublicIPSpec{
					Name:    "public-ip",
					DNSName: "myfqdn.azure.com",
				},
			},
		},
		Type: Public,
	}
}

func createValidNodeOutboundLB() *LoadBalancerSpec {
	return &LoadBalancerSpec{
		FrontendIPsCount: pointer.Int32Ptr(1),
	}
}
