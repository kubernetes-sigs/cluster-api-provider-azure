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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// CreateValidClusterWithClusterSubnet returns a valid AzureCluster with a cluster subnet.
func CreateValidClusterWithClusterSubnet() *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: infrav1.AzureClusterSpec{
			ControlPlaneEnabled: true,
			NetworkSpec:         CreateValidNetworkSpecWithClusterSubnet(),
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				IdentityRef: &corev1.ObjectReference{
					Kind: "AzureClusterIdentity",
				},
			},
		},
	}
}

// CreateValidCluster returns a valid AzureCluster.
func CreateValidCluster() *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: infrav1.AzureClusterSpec{
			ControlPlaneEnabled: true,
			NetworkSpec:         CreateValidNetworkSpec(),
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				IdentityRef: &corev1.ObjectReference{
					Kind: infrav1.AzureClusterIdentityKind,
				},
			},
		},
	}
}

// CreateClusterNetworkSpec returns a NetworkSpec for a cluster subnet topology.
func CreateClusterNetworkSpec() infrav1.NetworkSpec {
	return infrav1.NetworkSpec{
		Vnet: infrav1.VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets: infrav1.Subnets{
			{
				SubnetClassSpec: infrav1.SubnetClassSpec{
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
func CreateValidNetworkSpecWithClusterSubnet() infrav1.NetworkSpec {
	return infrav1.NetworkSpec{
		Vnet: infrav1.VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets: infrav1.Subnets{
			{
				SubnetClassSpec: infrav1.SubnetClassSpec{
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
func CreateValidNetworkSpec() infrav1.NetworkSpec {
	return infrav1.NetworkSpec{
		Vnet: infrav1.VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets:        CreateValidSubnets(),
		APIServerLB:    CreateValidAPIServerLB(),
		NodeOutboundLB: CreateValidNodeOutboundLB(),
	}
}

// CreateValidSubnets returns a valid set of Subnets with control-plane and node roles.
func CreateValidSubnets() infrav1.Subnets {
	return infrav1.Subnets{
		{
			SubnetClassSpec: infrav1.SubnetClassSpec{
				Role: "control-plane",
				Name: "control-plane-subnet",
			},
		},
		{
			SubnetClassSpec: infrav1.SubnetClassSpec{
				Role: "node",
				Name: "node-subnet",
			},
		},
	}
}

// CreateValidVnet returns a valid VnetSpec.
func CreateValidVnet() infrav1.VnetSpec {
	return infrav1.VnetSpec{
		ResourceGroup: "custom-vnet",
		Name:          "my-vnet",
		VnetClassSpec: infrav1.VnetClassSpec{
			CIDRBlocks: []string{"10.0.0.0/8"},
		},
	}
}

// CreateValidAPIServerLB returns a valid public API server LoadBalancerSpec.
func CreateValidAPIServerLB() *infrav1.LoadBalancerSpec {
	return &infrav1.LoadBalancerSpec{
		Name: "my-lb",
		FrontendIPs: []infrav1.FrontendIP{
			{
				Name: "ip-config",
				PublicIP: &infrav1.PublicIPSpec{
					Name:    "public-ip",
					DNSName: "myfqdn.azure.com",
				},
			},
		},
		LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
			SKU:  infrav1.SKUStandard,
			Type: infrav1.Public,
		},
	}
}

// CreateValidNodeOutboundLB returns a valid node outbound LoadBalancerSpec.
func CreateValidNodeOutboundLB() *infrav1.LoadBalancerSpec {
	return &infrav1.LoadBalancerSpec{
		FrontendIPsCount: ptr.To[int32](1),
	}
}

// CreateValidAPIServerInternalLB returns a valid internal API server LoadBalancerSpec.
func CreateValidAPIServerInternalLB() *infrav1.LoadBalancerSpec {
	return &infrav1.LoadBalancerSpec{
		Name: "my-lb",
		FrontendIPs: []infrav1.FrontendIP{
			{
				Name: "ip-config-private",
				FrontendIPClass: infrav1.FrontendIPClass{
					PrivateIPAddress: "10.10.1.1",
				},
			},
		},
		LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
			SKU:  infrav1.SKUStandard,
			Type: infrav1.Internal,
		},
	}
}
