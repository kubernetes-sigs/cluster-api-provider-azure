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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

func TestClusterNameValidation(t *testing.T) {
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
			g := NewWithT(t)
			azureCluster := AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
			}

			allErrs := azureCluster.validateClusterName()
			if tc.wantErr {
				g.Expect(allErrs).NotTo(BeNil())
			} else {
				g.Expect(allErrs).To(BeNil())
			}
		})
	}
}

func TestClusterWithPreexistingVnetValid(t *testing.T) {
	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster with pre-existing vnet - valid",
		cluster: createValidCluster(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		_, err := testCase.cluster.validateCluster(nil)
		g.Expect(err).To(BeNil())
	})
}

func TestClusterWithPreexistingVnetInvalid(t *testing.T) {
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
		SubnetClassSpec: SubnetClassSpec{
			Role: "random",
			Name: "random-subnet",
		},
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		_, err := testCase.cluster.validateCluster(nil)
		g.Expect(err).NotTo(BeNil())
	})
}

func TestClusterWithoutPreexistingVnetValid(t *testing.T) {
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
		g := NewWithT(t)
		_, err := testCase.cluster.validateCluster(nil)
		g.Expect(err).To(BeNil())
	})
}

func TestClusterSpecWithPreexistingVnetValid(t *testing.T) {
	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec with pre-existing vnet - valid",
		cluster: createValidCluster(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(errs).To(BeNil())
	})
}

func TestClusterSpecWithPreexistingVnetInvalid(t *testing.T) {
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
		SubnetClassSpec: SubnetClassSpec{
			Role: "random",
			Name: "random-subnet",
		},
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(len(errs)).To(BeNumerically(">", 0))
	})
}

func TestClusterSpecWithoutPreexistingVnetValid(t *testing.T) {
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
		g := NewWithT(t)
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(errs).To(BeNil())
	})
}

func TestClusterSpecWithoutIdentityRefInvalid(t *testing.T) {
	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec without identityRef - invalid",
		cluster: createValidCluster(),
	}

	// invalid because it doesn't specify an identityRef
	testCase.cluster.Spec.IdentityRef = nil

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(len(errs)).To(BeNumerically(">", 0))
	})
}

func TestClusterSpecWithWrongKindInvalid(t *testing.T) {
	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec with wrong kind - invalid",
		cluster: createValidCluster(),
	}

	// invalid because it doesn't specify AzureClusterIdentity as the kind
	testCase.cluster.Spec.IdentityRef.Kind = "bad"

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := testCase.cluster.validateClusterSpec(nil)
		g.Expect(len(errs)).To(BeNumerically(">", 0))
	})
}

func TestNetworkSpecWithPreexistingVnetValid(t *testing.T) {
	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec with pre-existing vnet - valid",
		networkSpec: createValidNetworkSpec(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(BeNil())
	})
}

func TestNetworkSpecWithPreexistingVnetLackRequiredSubnets(t *testing.T) {
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
		g := NewWithT(t)
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets"))
		g.Expect(errs[0].Error()).To(ContainSubstring("required role node not included"))
	})
}

func TestNetworkSpecWithPreexistingVnetInvalidResourceGroup(t *testing.T) {
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
		g := NewWithT(t)
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.vnet.resourceGroup"))
		g.Expect(errs[0].BadValue).To(BeEquivalentTo(testCase.networkSpec.Vnet.ResourceGroup))
	})
}

func TestNetworkSpecWithoutPreexistingVnetValid(t *testing.T) {
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
		g := NewWithT(t)
		errs := validateNetworkSpec(testCase.networkSpec, NetworkSpec{}, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(BeNil())
	})
}

func TestResourceGroupValid(t *testing.T) {
	type test struct {
		name          string
		resourceGroup string
	}

	testCase := test{
		name:          "resourcegroup name - valid",
		resourceGroup: "custom-vnet",
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		err := validateResourceGroup(testCase.resourceGroup,
			field.NewPath("spec").Child("networkSpec").Child("vnet").Child("resourceGroup"))
		g.Expect(err).To(BeNil())
	})
}

func TestResourceGroupInvalid(t *testing.T) {
	type test struct {
		name          string
		resourceGroup string
	}

	testCase := test{
		name:          "resourcegroup name - invalid",
		resourceGroup: "inv@lid-rg",
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		err := validateResourceGroup(testCase.resourceGroup,
			field.NewPath("spec").Child("networkSpec").Child("vnet").Child("resourceGroup"))
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(err.Field).To(Equal("spec.networkSpec.vnet.resourceGroup"))
		g.Expect(err.BadValue).To(BeEquivalentTo(testCase.resourceGroup))
	})
}

func TestValidateVnetCIDR(t *testing.T) {
	tests := []struct {
		name           string
		vnetCidrBlocks []string
		wantErr        bool
		expectedErr    field.Error
	}{
		{
			name:           "valid subnet cidr",
			vnetCidrBlocks: []string{"10.0.0.0/8"},
			wantErr:        false,
		},
		{
			name:           "invalid subnet cidr not in the right format",
			vnetCidrBlocks: []string{"10.0.0.0/8", "foo/bar"},
			wantErr:        true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "vnet.cidrBlocks",
				BadValue: "foo/bar",
				Detail:   "invalid CIDR format",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateVnetCIDR(testCase.vnetCidrBlocks, field.NewPath("vnet.cidrBlocks"))
			if testCase.wantErr {
				g.Expect(err).To(ContainElement(MatchError(testCase.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestSubnetsValid(t *testing.T) {
	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - valid",
		subnets: createValidSubnets(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := validateSubnets(testCase.subnets, createValidVnet(),
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(BeNil())
	})
}

func TestSubnetsInvalidSubnetName(t *testing.T) {
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
		g := NewWithT(t)
		errs := validateSubnets(testCase.subnets, createValidVnet(),
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets[0].name"))
		g.Expect(errs[0].BadValue).To(BeEquivalentTo("invalid-subnet-name-due-to-bracket)"))
	})
}

func TestSubnetsInvalidLackRequiredSubnet(t *testing.T) {
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
		g := NewWithT(t)
		errs := validateSubnets(testCase.subnets, createValidVnet(),
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets"))
		g.Expect(errs[0].Detail).To(ContainSubstring("required role control-plane not included"))
	})
}

func TestSubnetNamesNotUnique(t *testing.T) {
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
		g := NewWithT(t)
		errs := validateSubnets(testCase.subnets, createValidVnet(),
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeDuplicate))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets"))
	})
}

func TestSubnetNameValid(t *testing.T) {
	type test struct {
		name       string
		subnetName string
	}

	testCase := test{
		name:       "subnet name - valid",
		subnetName: "control-plane-subnet",
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		err := validateSubnetName(testCase.subnetName,
			field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("name"))
		g.Expect(err).To(BeNil())
	})
}

func TestSubnetNameInvalid(t *testing.T) {
	type test struct {
		name       string
		subnetName string
	}

	testCase := test{
		name:       "subnet name - invalid",
		subnetName: "inv@lid-subnet-name",
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		err := validateSubnetName(testCase.subnetName,
			field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("name"))
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(err.Field).To(Equal("spec.networkSpec.subnets[0].name"))
		g.Expect(err.BadValue).To(BeEquivalentTo(testCase.subnetName))
	})
}

func TestValidateSubnetCIDR(t *testing.T) {
	tests := []struct {
		name             string
		vnetCidrBlocks   []string
		subnetCidrBlocks []string
		wantErr          bool
		expectedErr      field.Error
	}{
		{
			name:             "valid subnet cidr",
			vnetCidrBlocks:   []string{"10.0.0.0/8"},
			subnetCidrBlocks: []string{"10.1.0.0/16", "10.0.0.0/16"},
			wantErr:          false,
		},
		{
			name:             "invalid subnet cidr not in the right format",
			vnetCidrBlocks:   []string{"10.0.0.0/8"},
			subnetCidrBlocks: []string{"10.1.0.0/16", "10.0.0.0/16", "foo/bar"},
			wantErr:          true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "subnets.cidrBlocks",
				BadValue: "foo/bar",
				Detail:   "invalid CIDR format",
			},
		},
		{
			name:             "subnet cidr not in vnet range",
			vnetCidrBlocks:   []string{"10.0.0.0/8"},
			subnetCidrBlocks: []string{"10.1.0.0/16", "10.0.0.0/16", "11.1.0.0/16"},
			wantErr:          true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "subnets.cidrBlocks",
				BadValue: "11.1.0.0/16",
				Detail:   "subnet CIDR not in vnet address space: [10.0.0.0/8]",
			},
		},
		{
			name:             "subnet cidr in at least one vnet's range in case of multiple vnet cidr blocks",
			vnetCidrBlocks:   []string{"10.0.0.0/8", "11.0.0.0/8"},
			subnetCidrBlocks: []string{"10.1.0.0/16", "10.0.0.0/16", "11.1.0.0/16"},
			wantErr:          false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateSubnetCIDR(testCase.subnetCidrBlocks, testCase.vnetCidrBlocks, field.NewPath("subnets.cidrBlocks"))
			if testCase.wantErr {
				// Searches for expected error in list of thrown errors
				g.Expect(err).To(ContainElement(MatchError(testCase.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestValidateSecurityRule(t *testing.T) {
	tests := []struct {
		name      string
		validRule SecurityRule
		wantErr   bool
	}{
		{
			name: "security rule - valid priority",
			validRule: SecurityRule{
				Name:        "allow_apiserver",
				Description: "Allow K8s API Server",
				Priority:    101,
			},
			wantErr: false,
		},
		{
			name: "security rule - invalid low priority",
			validRule: SecurityRule{
				Name:        "allow_apiserver",
				Description: "Allow K8s API Server",
				Priority:    99,
			},
			wantErr: true,
		},
		{
			name: "security rule - invalid high priority",
			validRule: SecurityRule{
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
			g := NewWithT(t)
			err := validateSecurityRule(
				testCase.validRule,
				field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("securityGroup").Child("securityRules").Index(0),
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
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-config",
					},
				},
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					SKU:  "Awesome",
					Type: Public,
				},
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
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: "Foo",
				},
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
				Detail: "API Server Load balancer should have 1 Frontend IP",
			},
		},
		{
			name: "public LB with private IP",
			lb: LoadBalancerSpec{
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-1",
						FrontendIPClass: FrontendIPClass{
							PrivateIPAddress: "10.0.0.4",
						},
					},
				},
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Public,
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
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-1",
						PublicIP: &PublicIPSpec{
							Name: "my-invalid-ip",
						},
					},
				},
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
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
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-1",
						FrontendIPClass: FrontendIPClass{
							PrivateIPAddress: "NAIP",
						},
					},
				},
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
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
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-1",
						FrontendIPClass: FrontendIPClass{
							PrivateIPAddress: "20.1.2.3",
						},
					},
				},
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
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
				FrontendIPs: []FrontendIP{
					{
						Name: "ip-1",
						FrontendIPClass: FrontendIPClass{
							PrivateIPAddress: "10.1.0.3",
						},
					},
				},
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
					SKU:  SKUStandard,
				},
				Name: "my-private-lb",
			},
			cpCIDRS: []string{"10.0.0.0/24", "10.1.0.0/24"},
			wantErr: false,
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			err := validateAPIServerLB(test.lb, test.old, test.cpCIDRS, field.NewPath("apiServerLB"))
			if test.wantErr {
				g.Expect(err).To(ContainElement(MatchError(test.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}
func TestPrivateDNSZoneName(t *testing.T) {
	testcases := []struct {
		name        string
		network     NetworkSpec
		wantErr     bool
		expectedErr field.Error
	}{
		{
			name: "testInvalidPrivateDNSZoneName",
			network: NetworkSpec{
				NetworkClassSpec: NetworkClassSpec{
					PrivateDNSZoneName: "wrong@d_ns.io",
				},
				APIServerLB: createValidAPIServerInternalLB(),
			},
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "spec.networkSpec.privateDNSZoneName",
				BadValue: "wrong@d_ns.io",
				Detail:   "PrivateDNSZoneName can only contain alphanumeric characters, underscores and dashes, must end with an alphanumeric character",
			},
			wantErr: true,
		},
		{
			name: "testValidPrivateDNSZoneName",
			network: NetworkSpec{
				NetworkClassSpec: NetworkClassSpec{
					PrivateDNSZoneName: "good.dns.io",
				},
				APIServerLB: createValidAPIServerInternalLB(),
			},
			wantErr: false,
		},
		{
			name: "testValidPrivateDNSZoneNameWithUnderscore",
			network: NetworkSpec{
				NetworkClassSpec: NetworkClassSpec{
					PrivateDNSZoneName: "_good.__dns.io",
				},
				APIServerLB: createValidAPIServerInternalLB(),
			},
			wantErr: false,
		},
		{
			name: "testBadAPIServerLBType",
			network: NetworkSpec{
				NetworkClassSpec: NetworkClassSpec{
					PrivateDNSZoneName: "good.dns.io",
				},
				APIServerLB: LoadBalancerSpec{
					Name: "my-lb",
					LoadBalancerClassSpec: LoadBalancerClassSpec{
						Type: Public,
					},
				},
			},
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "spec.networkSpec.privateDNSZoneName",
				BadValue: "Public",
				Detail:   "PrivateDNSZoneName is available only if APIServerLB.Type is Internal",
			},
			wantErr: true,
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			err := validatePrivateDNSZoneName(test.network.PrivateDNSZoneName, test.network.APIServerLB.Type, field.NewPath("spec", "networkSpec", "privateDNSZoneName"))
			if test.wantErr {
				g.Expect(err).To(ContainElement(MatchError(test.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestValidateNodeOutboundLB(t *testing.T) {
	testcases := []struct {
		name        string
		lb          *LoadBalancerSpec
		old         *LoadBalancerSpec
		apiServerLB LoadBalancerSpec
		wantErr     bool
		expectedErr field.Error
	}{
		{
			name: "no lb for public clusters",
			lb:   nil,
			apiServerLB: LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Public,
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueRequired",
				Field:    "nodeOutboundLB",
				BadValue: nil,
				Detail:   "Node outbound load balancer cannot be nil for public clusters.",
			},
		},
		{
			name: "no lb allowed for internal clusters",
			lb:   nil,
			apiServerLB: LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
				},
			},
			wantErr: false,
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
				Type:     "FieldValueForbidden",
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
				Type:     "FieldValueForbidden",
				Field:    "nodeOutboundLB.name",
				BadValue: "some-name",
				Detail:   "Node outbound load balancer Name should not be modified after AzureCluster creation.",
			},
		},
		{
			name: "invalid SKU update",
			lb: &LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					SKU: "some-sku",
				},
			},
			old: &LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					SKU: "old-sku",
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueForbidden",
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
				Type:  "FieldValueForbidden",
				Field: "nodeOutboundLB.frontendIPs[0]",
				BadValue: FrontendIP{
					Name: "some-frontend-ip",
				},
				Detail: "Node outbound load balancer FrontendIPs cannot be modified after AzureCluster creation.",
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
				FrontendIPsCount: ptr.To[int32](2),
			},
			old: &LoadBalancerSpec{
				FrontendIPs: []FrontendIP{{
					Name: "old-frontend-ip",
				}},
				LoadBalancerClassSpec: LoadBalancerClassSpec{},
			},
			wantErr: false,
		},
		{
			name: "frontend ips count exceeds max value",
			lb: &LoadBalancerSpec{
				FrontendIPsCount: ptr.To[int32](100),
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
			g := NewWithT(t)
			err := validateNodeOutboundLB(test.lb, test.old, test.apiServerLB, field.NewPath("nodeOutboundLB"))
			if test.wantErr {
				g.Expect(err).To(ContainElement(MatchError(test.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestValidateControlPlaneNodeOutboundLB(t *testing.T) {
	testcases := []struct {
		name        string
		lb          *LoadBalancerSpec
		old         *LoadBalancerSpec
		apiServerLB LoadBalancerSpec
		wantErr     bool
		expectedErr field.Error
	}{
		{
			name: "cp outbound lb cannot be set for public clusters",
			lb:   &LoadBalancerSpec{Name: "foo"},
			apiServerLB: LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Public,
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueForbidden",
				Field:    "controlPlaneOutboundLB",
				BadValue: LoadBalancerSpec{Name: "foo"},
				Detail:   "Control plane outbound load balancer cannot be set for public clusters.",
			},
		},
		{
			name: "cp outbound lb can be set for private clusters",
			lb:   &LoadBalancerSpec{Name: "foo"},
			apiServerLB: LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
				},
			},
			wantErr: false,
		},
		{
			name: "cp outbound lb can be nil for private clusters",
			lb:   nil,
			apiServerLB: LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
				},
			},
			wantErr: false,
		},
		{
			name: "frontend ips count exceeds max value",
			lb: &LoadBalancerSpec{
				FrontendIPsCount: ptr.To[int32](100),
			},
			apiServerLB: LoadBalancerSpec{
				LoadBalancerClassSpec: LoadBalancerClassSpec{
					Type: Internal,
				},
			},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "controlPlaneOutboundLB.frontendIPsCount",
				BadValue: 100,
				Detail:   "Max front end ips allowed is 16",
			},
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			err := validateControlPlaneOutboundLB(test.lb, test.apiServerLB, field.NewPath("controlPlaneOutboundLB"))
			if test.wantErr {
				g.Expect(err).To(ContainElement(MatchError(test.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestValidateCloudProviderConfigOverrides(t *testing.T) {
	tests := []struct {
		name        string
		oldConfig   *CloudProviderConfigOverrides
		newConfig   *CloudProviderConfigOverrides
		wantErr     bool
		expectedErr field.Error
	}{
		{
			name:    "both old and new config nil",
			wantErr: false,
		},
		{
			name: "both old and new config are same",
			oldConfig: &CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
				Name:   "foo",
				Config: RateLimitConfig{CloudProviderRateLimitBucket: 10, CloudProviderRateLimit: true},
			}}},
			newConfig: &CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
				Name:   "foo",
				Config: RateLimitConfig{CloudProviderRateLimitBucket: 10, CloudProviderRateLimit: true},
			}}},
			wantErr: false,
		},
		{
			name: "old and new config are not same",
			oldConfig: &CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
				Name:   "foo",
				Config: RateLimitConfig{CloudProviderRateLimitBucket: 10, CloudProviderRateLimit: true},
			}}},
			newConfig: &CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
				Name:   "foo",
				Config: RateLimitConfig{CloudProviderRateLimitBucket: 11, CloudProviderRateLimit: true},
			}}},
			wantErr: true,
			expectedErr: field.Error{
				Type:  "FieldValueInvalid",
				Field: "spec.cloudProviderConfigOverrides",
				BadValue: CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
					Name:   "foo",
					Config: RateLimitConfig{CloudProviderRateLimitBucket: 11, CloudProviderRateLimit: true},
				}}},
				Detail: "cannot change cloudProviderConfigOverrides cluster creation",
			},
		},
		{
			name: "new config is nil",
			oldConfig: &CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
				Name:   "foo",
				Config: RateLimitConfig{CloudProviderRateLimitBucket: 10, CloudProviderRateLimit: true},
			}}},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "spec.cloudProviderConfigOverrides",
				BadValue: nil,
				Detail:   "cannot change cloudProviderConfigOverrides cluster creation",
			},
		},
		{
			name: "old config is nil",
			newConfig: &CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
				Name:   "foo",
				Config: RateLimitConfig{CloudProviderRateLimitBucket: 10, CloudProviderRateLimit: true},
			}}},
			wantErr: true,
			expectedErr: field.Error{
				Type:  "FieldValueInvalid",
				Field: "spec.cloudProviderConfigOverrides",
				BadValue: &CloudProviderConfigOverrides{RateLimits: []RateLimitSpec{{
					Name:   "foo",
					Config: RateLimitConfig{CloudProviderRateLimitBucket: 10, CloudProviderRateLimit: true},
				}}},
				Detail: "cannot change cloudProviderConfigOverrides cluster creation",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateCloudProviderConfigOverrides(testCase.oldConfig, testCase.newConfig, field.NewPath("spec.cloudProviderConfigOverrides"))
			if testCase.wantErr {
				g.Expect(err).To(ContainElement(MatchError(testCase.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
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
			AzureClusterClassSpec: AzureClusterClassSpec{
				IdentityRef: &corev1.ObjectReference{
					Kind: "AzureClusterIdentity",
				},
			},
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
			SubnetClassSpec: SubnetClassSpec{
				Role: "control-plane",
				Name: "control-plane-subnet",
			},
		},
		{
			SubnetClassSpec: SubnetClassSpec{
				Role: "node",
				Name: "node-subnet",
			},
		},
	}
}

func createValidVnet() VnetSpec {
	return VnetSpec{
		ResourceGroup: "custom-vnet",
		Name:          "my-vnet",
		VnetClassSpec: VnetClassSpec{
			CIDRBlocks: []string{DefaultVnetCIDR},
		},
	}
}

func createValidAPIServerLB() LoadBalancerSpec {
	return LoadBalancerSpec{
		Name: "my-lb",
		FrontendIPs: []FrontendIP{
			{
				Name: "ip-config",
				PublicIP: &PublicIPSpec{
					Name:    "public-ip",
					DNSName: "myfqdn.azure.com",
				},
			},
		},
		LoadBalancerClassSpec: LoadBalancerClassSpec{
			SKU:  SKUStandard,
			Type: Public,
		},
	}
}

func createValidNodeOutboundLB() *LoadBalancerSpec {
	return &LoadBalancerSpec{
		FrontendIPsCount: ptr.To[int32](1),
	}
}

func createValidAPIServerInternalLB() LoadBalancerSpec {
	return LoadBalancerSpec{
		Name: "my-lb",
		FrontendIPs: []FrontendIP{
			{
				Name: "ip-config-private",
				FrontendIPClass: FrontendIPClass{
					PrivateIPAddress: "10.10.1.1",
				},
			},
		},
		LoadBalancerClassSpec: LoadBalancerClassSpec{
			SKU:  SKUStandard,
			Type: Internal,
		},
	}
}

func TestValidateServiceEndpoints(t *testing.T) {
	tests := []struct {
		name             string
		serviceEndpoints ServiceEndpoints
		wantErr          bool
		expectedErr      field.Error
	}{
		{
			name: "valid service endpoint",
			serviceEndpoints: []ServiceEndpointSpec{{
				Service:   "Microsoft.Foo",
				Locations: []string{"*", "eastus2"},
			}},
			wantErr: false,
		},
		{
			name: "invalid service endpoint name doesn't start with Microsoft",
			serviceEndpoints: []ServiceEndpointSpec{{
				Service:   "Foo",
				Locations: []string{"*"},
			}},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "subnets[0].serviceEndpoints[0].service",
				BadValue: "Foo",
				Detail:   "service name of endpoint service doesn't match regex ^Microsoft\\.[a-zA-Z]{1,42}[a-zA-Z0-9]{0,42}$",
			},
		},
		{
			name: "invalid service endpoint name contains invalid characters",
			serviceEndpoints: []ServiceEndpointSpec{{
				Service:   "Microsoft.Foo",
				Locations: []string{"*"},
			}, {
				Service:   "Microsoft.Foo-Bar",
				Locations: []string{"*"},
			}},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "subnets[0].serviceEndpoints[1].service",
				BadValue: "Microsoft.Foo-Bar",
				Detail:   "service name of endpoint service doesn't match regex ^Microsoft\\.[a-zA-Z]{1,42}[a-zA-Z0-9]{0,42}$",
			},
		},
		{
			name: "invalid service endpoint location contains invalid characters",
			serviceEndpoints: []ServiceEndpointSpec{{
				Service:   "Microsoft.Foo",
				Locations: []string{"*"},
			}, {
				Service:   "Microsoft.Bar",
				Locations: []string{"foo", "foo-bar"},
			}},
			wantErr: true,
			expectedErr: field.Error{
				Type:     "FieldValueInvalid",
				Field:    "subnets[0].serviceEndpoints[1].locations[1]",
				BadValue: "foo-bar",
				Detail:   "location doesn't match regex ^([a-z]{1,42}\\d{0,5}|[*])$",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateServiceEndpoints(testCase.serviceEndpoints, field.NewPath("subnets[0].serviceEndpoints"))
			if testCase.wantErr {
				// Searches for expected error in list of thrown errors
				g.Expect(err).To(ContainElement(MatchError(testCase.expectedErr.Error())))
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestServiceEndpointsLackRequiredFieldService(t *testing.T) {
	type test struct {
		name             string
		serviceEndpoints ServiceEndpoints
	}

	testCase := test{
		name: "service endpoint missing service name",
		serviceEndpoints: []ServiceEndpointSpec{{
			Locations: []string{"*"},
		}},
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := validateServiceEndpoints(testCase.serviceEndpoints, field.NewPath("subnets[0].serviceEndpoints"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
		g.Expect(errs[0].Field).To(Equal("subnets[0].serviceEndpoints[0].service"))
		g.Expect(errs[0].Error()).To(ContainSubstring("service is required for all service endpoints"))
	})
}

func TestServiceEndpointsLackRequiredFieldLocations(t *testing.T) {
	type test struct {
		name             string
		serviceEndpoints ServiceEndpoints
	}

	testCase := test{
		name: "service endpoint missing locations",
		serviceEndpoints: []ServiceEndpointSpec{{
			Service: "Microsoft.Foo",
		}},
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		errs := validateServiceEndpoints(testCase.serviceEndpoints, field.NewPath("subnets[0].serviceEndpoints"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
		g.Expect(errs[0].Field).To(Equal("subnets[0].serviceEndpoints[0].locations"))
		g.Expect(errs[0].Error()).To(ContainSubstring("locations are required for all service endpoints"))
	})
}

func TestClusterWithExtendedLocationInvalid(t *testing.T) {
	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec with extended location but not enable EdgeZone feature gate flag",
		cluster: createValidCluster(),
	}

	testCase.cluster.Spec.ExtendedLocation = &ExtendedLocationSpec{
		Name: "rr4",
		Type: "EdgeZone",
	}

	t.Run(testCase.name, func(t *testing.T) {
		g := NewWithT(t)
		err := testCase.cluster.validateClusterSpec(nil)
		g.Expect(err).NotTo(BeNil())
	})
}
