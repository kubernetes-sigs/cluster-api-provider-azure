//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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

package e2e

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestIsDualStackCluster(t *testing.T) {
	cases := []struct {
		Name        string
		ClusterCIDR string
		Expected    bool
	}{
		{
			Name:        "dual stack cluster-cidr",
			ClusterCIDR: "10.244.0.0/16,2001:1234:5678:9a40::/58",
			Expected:    true,
		},
		{
			Name:        "ipv4 cluster-cidr",
			ClusterCIDR: "10.244.0.0/16",
			Expected:    false,
		},
		{
			Name:        "ipv6 cluster-cidr",
			ClusterCIDR: "2001:1234:5678:9a40::/58",
			Expected:    false,
		},
		{
			Name:        "bogon",
			ClusterCIDR: "Hello, World!",
			Expected:    false,
		},
		{
			Name:        "empty string",
			ClusterCIDR: "",
			Expected:    false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(isDualStackCluster(c.ClusterCIDR)).To(Equal(c.Expected))
		})
	}
}

func TestIsIPv6Cluster(t *testing.T) {
	cases := []struct {
		Name        string
		ClusterCIDR string
		Expected    bool
	}{
		{
			Name:        "dual stack cluster-cidr",
			ClusterCIDR: "10.244.0.0/16,2001:1234:5678:9a40::/58",
			Expected:    false,
		},
		{
			Name:        "ipv4 cluster-cidr",
			ClusterCIDR: "10.244.0.0/16",
			Expected:    false,
		},
		{
			Name:        "ipv6 cluster-cidr",
			ClusterCIDR: "2001:1234:5678:9a40::/58",
			Expected:    true,
		},
		{
			Name:        "bogon",
			ClusterCIDR: "Hello, World!",
			Expected:    false,
		},
		{
			Name:        "empty string",
			ClusterCIDR: "",
			Expected:    false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(isIPv6Cluster(c.ClusterCIDR)).To(Equal(c.Expected))
		})
	}
}
