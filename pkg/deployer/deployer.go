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

package deployer

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Deployer satisfies the ProviderDeployer(https://github.com/kubernetes-sigs/cluster-api/blob/master/cmd/clusterctl/clusterdeployer/clusterdeployer.go) interface.
type Deployer struct {
	scopeGetter actuators.ScopeGetter
}

// Params is used to create a new deployer.
type Params struct {
	ScopeGetter actuators.ScopeGetter
}

// New returns a new Deployer.
func New(params Params) *Deployer {
	return &Deployer{
		scopeGetter: params.ScopeGetter,
	}
}

// GetIP returns the IP of a machine, but this is going away.
func (d *Deployer) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	scope, err := d.scopeGetter.GetScope(actuators.ScopeParams{Cluster: cluster})
	if err != nil {
		return "", err
	}

	actuators.CreateOrUpdateNetworkAPIServerIP(scope)

	if scope.ClusterStatus != nil && scope.ClusterStatus.Network.APIServerIP.DNSName != "" {
		return scope.ClusterStatus.Network.APIServerIP.DNSName, nil
	}

	return "", errors.New("error getting dns name from cluster, dns name not set")
}

// GetKubeConfig returns the kubeconfig after the bootstrap process is complete.
func (d *Deployer) GetKubeConfig(cluster *clusterv1.Cluster, _ *clusterv1.Machine) (string, error) {
	scope, err := d.scopeGetter.GetScope(actuators.ScopeParams{Cluster: cluster})
	if err != nil {
		return "", err
	}

	// Poll for cluster to be ready before returning
	if _, err := coreV1Client(scope.ClusterConfig.AdminKubeconfig); err != nil {
		return "", err
	}

	return scope.ClusterConfig.AdminKubeconfig, nil
}

func coreV1Client(kubeconfig string) (corev1.CoreV1Interface, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))

	if err != nil {
		return nil, errors.Wrapf(err, "failed to get client config for cluster")
	}

	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get client config for cluster")
	}

	return corev1.NewForConfig(cfg)
}
