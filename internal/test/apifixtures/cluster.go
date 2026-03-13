/*
Copyright The Kubernetes Authors.

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

package apifixtures

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// CreateValidClusterWithClusterSubnet returns a valid AzureCluster with a cluster subnet.
func CreateValidClusterWithClusterSubnet() *AzureCluster {
	return &AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: AzureClusterSpec{
			ControlPlaneEnabled: true,
			NetworkSpec:         CreateValidNetworkSpecWithClusterSubnet(),
			AzureClusterClassSpec: AzureClusterClassSpec{
				IdentityRef: &corev1.ObjectReference{
					Kind: "AzureClusterIdentity",
				},
			},
		},
	}
}

// CreateValidCluster returns a valid AzureCluster.
func CreateValidCluster() *AzureCluster {
	return &AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: AzureClusterSpec{
			ControlPlaneEnabled: true,
			NetworkSpec:         CreateValidNetworkSpec(),
			AzureClusterClassSpec: AzureClusterClassSpec{
				IdentityRef: &corev1.ObjectReference{
					Kind: AzureClusterIdentityKind,
				},
			},
		},
	}
}

// CreateClusterNetworkSpec returns a NetworkSpec for a cluster subnet topology.
func CreateClusterNetworkSpec() NetworkSpec {
	return NetworkSpec{
		Vnet: VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets: Subnets{
			{
				SubnetClassSpec: SubnetClassSpec{
					Role: "cluster",
					Name: "cluster-subnet",
				},
			},
		},
		APIServerLB:    CreateValidAPIServerLB(),
		NodeOutboundLB: CreateValidNodeOutboundLB(),
	}
}

// CreateValidNetworkSpecWithClusterSubnet returns a valid NetworkSpec with a cluster subnet.
func CreateValidNetworkSpecWithClusterSubnet() NetworkSpec {
	return NetworkSpec{
		Vnet: VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets: Subnets{
			{
				SubnetClassSpec: SubnetClassSpec{
					Role: "cluster",
					Name: "cluster-subnet",
				},
			},
		},
		APIServerLB:    CreateValidAPIServerLB(),
		NodeOutboundLB: CreateValidNodeOutboundLB(),
	}
}

// CreateValidNetworkSpec returns a valid NetworkSpec.
func CreateValidNetworkSpec() NetworkSpec {
	return NetworkSpec{
		Vnet: VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets:        CreateValidSubnets(),
		APIServerLB:    CreateValidAPIServerLB(),
		NodeOutboundLB: CreateValidNodeOutboundLB(),
	}
}

// CreateValidSubnets returns a valid set of Subnets with control-plane and node roles.
func CreateValidSubnets() Subnets {
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

// CreateValidVnet returns a valid VnetSpec.
func CreateValidVnet() VnetSpec {
	return VnetSpec{
		ResourceGroup: "custom-vnet",
		Name:          "my-vnet",
		VnetClassSpec: VnetClassSpec{
			CIDRBlocks: []string{"10.0.0.0/8"},
		},
	}
}

// CreateValidAPIServerLB returns a valid public API server LoadBalancerSpec.
func CreateValidAPIServerLB() *LoadBalancerSpec {
	return &LoadBalancerSpec{
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

// CreateValidNodeOutboundLB returns a valid node outbound LoadBalancerSpec.
func CreateValidNodeOutboundLB() *LoadBalancerSpec {
	return &LoadBalancerSpec{
		FrontendIPsCount: ptr.To[int32](1),
	}
}

// CreateValidAPIServerInternalLB returns a valid internal API server LoadBalancerSpec.
func CreateValidAPIServerInternalLB() *LoadBalancerSpec {
	return &LoadBalancerSpec{
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
