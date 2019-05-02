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

package config

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
)

// GetVMStartupScript returns startup script based on role
func GetVMStartupScript(machine *actuators.MachineScope, bootstrapToken string) (string, error) {
	var startupScript string

	if !machine.Scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
		return "", errors.New("failed to run controlplane, missing CACertificate")
	}

	if machine.Scope.Network().APIServerIP.DNSName == "" {
		return "", errors.New("failed to run controlplane, dns name not available")
	}

	dnsName := machine.Scope.Network().APIServerIP.DNSName

	caCertHash := ""

	if len(machine.Scope.ClusterConfig.DiscoveryHashes) > 0 {
		caCertHash = machine.Scope.ClusterConfig.DiscoveryHashes[0]
	}

	if caCertHash == "" {
		return "", errors.New("failed to run controlplane, missing discovery hashes")
	}

	// apply values based on the role of the machine
	switch machine.Role() {
	case v1alpha1.ControlPlane:
		// TODO: Check for existence of control plane subnet & ensure NSG is attached to subnet

		var err error

		if bootstrapToken != "" {
			klog.V(2).Infof("Allowing machine %s to join control plane for cluster %s", machine.Name(), machine.Scope.Name())

			startupScript, err = JoinControlPlane(&ContolPlaneJoinInput{
				CACert:              string(machine.Scope.ClusterConfig.CAKeyPair.Cert),
				CAKey:               string(machine.Scope.ClusterConfig.CAKeyPair.Key),
				CACertHash:          caCertHash,
				EtcdCACert:          string(machine.Scope.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:           string(machine.Scope.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:    string(machine.Scope.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:     string(machine.Scope.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:              string(machine.Scope.ClusterConfig.SAKeyPair.Cert),
				SaKey:               string(machine.Scope.ClusterConfig.SAKeyPair.Key),
				BootstrapToken:      bootstrapToken,
				LBAddress:           dnsName,
				KubernetesVersion:   "1.13.4",
				CloudProviderConfig: getAzureCloudProviderConfig(machine),
			})
			if err != nil {
				return "", err
			}
		} else {
			klog.V(2).Infof("Machine %s is the first controlplane machine for cluster %s", machine.Name(), machine.Scope.Name())
			if !machine.Scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
				return "", errors.New("failed to run controlplane, missing CAPrivateKey")
			}

			startupScript, err = NewControlPlane(&ControlPlaneInput{
				CACert:              string(machine.Scope.ClusterConfig.CAKeyPair.Cert),
				CAKey:               string(machine.Scope.ClusterConfig.CAKeyPair.Key),
				EtcdCACert:          string(machine.Scope.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:           string(machine.Scope.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:    string(machine.Scope.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:     string(machine.Scope.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:              string(machine.Scope.ClusterConfig.SAKeyPair.Cert),
				SaKey:               string(machine.Scope.ClusterConfig.SAKeyPair.Key),
				LBAddress:           dnsName,
				InternalLBAddress:   azure.DefaultInternalLBIPAddress,
				ClusterName:         machine.Scope.Name(),
				PodSubnet:           machine.Scope.Cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
				ServiceSubnet:       machine.Scope.Cluster.Spec.ClusterNetwork.Services.CIDRBlocks[0],
				ServiceDomain:       machine.Scope.Cluster.Spec.ClusterNetwork.ServiceDomain,
				KubernetesVersion:   "1.13.4",
				CloudProviderConfig: getAzureCloudProviderConfig(machine),
			})

			if err != nil {
				return "", err
			}
		}

	case v1alpha1.Node:
		// TODO: Check for existence of node subnet & ensure NSG is attached to subnet
		var err error
		startupScript, err = NewNode(&NodeInput{
			CACertHash:          caCertHash,
			BootstrapToken:      bootstrapToken,
			InternalLBAddress:   azure.DefaultInternalLBIPAddress,
			KubernetesVersion:   "1.13.4",
			CloudProviderConfig: getAzureCloudProviderConfig(machine),
		})

		if err != nil {
			return "", err
		}

	default:
		return "", errors.Errorf("Unknown node role %s", machine.Role())
	}
	return startupScript, nil
}

// getAzureCloudProviderConfig gets azure provider config for control plane and kubelet
func getAzureCloudProviderConfig(machine *actuators.MachineScope) string {
	return fmt.Sprintf(`{
"cloud":"AzurePublicCloud",
"tenantId": "%[1]s",
"subscriptionId": "%[2]s",
"aadClientId": "%[3]s",
"aadClientSecret": "%[4]s",
"resourceGroup": "%[5]s",
"location": "%[6]s",
"vmType": "standard",
"subnetName": "%[7]s",
"securityGroupName": "%[8]s",
"vnetName": "%[9]s",
"vnetResourceGroup": "%[5]s",
"routeTableName": "%[10]s",
"primaryAvailabilitySetName": "",
"primaryScaleSetName": "",
"cloudProviderBackoff": true,
"cloudProviderBackoffRetries": 6,
"cloudProviderBackoffExponent": 1.5,
"cloudProviderBackoffDuration": 5,
"cloudProviderBackoffJitter": 1.0,
"cloudProviderRatelimit": true,
"cloudProviderRateLimitQPS": 3.0,
"cloudProviderRateLimitBucket": 10,
"useManagedIdentityExtension": false,
"userAssignedIdentityID": "",
"useInstanceMetadata": true,
"loadBalancerSku": "Standard",
"excludeMasterFromStandardLB": true,
"providerVaultName": "",
"maximumLoadBalancerRuleCount": 250,
"providerKeyName": "k8s",
"providerKeyVersion": ""
}`,
		os.Getenv("AZURE_TENANT_ID"),
		os.Getenv("AZURE_SUBSCRIPTION_ID"),
		os.Getenv("AZURE_CLIENT_ID"),
		os.Getenv("AZURE_CLIENT_SECRET"),
		machine.Scope.ClusterConfig.ResourceGroup,
		machine.Scope.ClusterConfig.Location,
		azure.GenerateNodeSubnetName(machine.Scope.Cluster.Name),
		azure.GenerateNodeSecurityGroupName(machine.Scope.Cluster.Name),
		azure.GenerateVnetName(machine.Scope.Cluster.Name),
		azure.GenerateNodeRouteTableName(machine.Scope.Cluster.Name),
	)
}
