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
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetAKSKubernetesVersion gets the kubernetes version for AKS clusters as specified by the environment variable defined by versionVar.
func GetAKSKubernetesVersion(ctx context.Context, e2eConfig *clusterctl.E2EConfig, versionVar string) (string, error) {
	e2eAKSVersion := e2eConfig.GetVariable(versionVar)

	location := e2eConfig.GetVariable(AzureLocation)

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	var maxVersion string
	switch e2eAKSVersion {
	case "latest":
		maxVersion, err = GetLatestStableAKSKubernetesVersion(ctx, subscriptionID, location)
		Expect(err).NotTo(HaveOccurred())
	case "latest-1":
		maxVersion, err = GetNextLatestStableAKSKubernetesVersion(ctx, subscriptionID, location)
		Expect(err).NotTo(HaveOccurred())
	default:
		maxVersion, err = GetWorkingAKSKubernetesVersion(ctx, subscriptionID, location, e2eAKSVersion)
		Expect(err).NotTo(HaveOccurred())
	}

	return maxVersion, nil
}

// byClusterOptions returns a set of ListOptions that will identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel: name,
		},
	}
}

// GetWorkingAKSKubernetesVersion returns an available Kubernetes version of AKS given a desired semver version, if possible.
// If the desired version is available, we return it.
// If the desired version is not available, we check for any available patch version using desired version's Major.Minor semver release.
// If no versions are available in the desired version's Major.Minor semver release, we return an error.
func GetWorkingAKSKubernetesVersion(ctx context.Context, subscriptionID, location, version string) (string, error) {
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return "", errors.Wrap(err, "failed to get settings from environment")
	}
	authorizer, err := azureutil.GetAuthorizer(settings)
	if err != nil {
		return "", errors.Wrap(err, "failed to create an Authorizer")
	}
	containerServiceClient := containerservice.NewContainerServicesClient(subscriptionID)
	containerServiceClient.Authorizer = authorizer
	result, err := containerServiceClient.ListOrchestrators(ctx, location, ManagedClustersResourceType)
	if err != nil {
		return "", errors.Wrap(err, "failed to list Orchestrators")
	}

	var latestStableVersionDesired bool
	// We're not doing much input validation here,
	// we assume that if the prefix is 'stable-' that the remainder of the string is in the format <Major>.<Minor>
	if isStableVersion, _ := validateStableReleaseString(version); isStableVersion {
		latestStableVersionDesired = true
		// Form a fully valid semver version @ the initial patch release (".0")
		version = fmt.Sprintf("%s.0", version[7:])
	}

	// semver comparisons below require a "v" prefix
	if version[:1] != "v" {
		version = fmt.Sprintf("v%s", version)
	}
	// Create a var of the patch ".0" equivalent of the inputted version
	baseVersion := fmt.Sprintf("%s.0", semver.MajorMinor(version))
	maxVersion := fmt.Sprintf("%s.0", semver.MajorMinor(version))
	var foundWorkingVersion bool
	for _, o := range *result.Orchestrators {
		orchVersion := *o.OrchestratorVersion
		// semver comparisons require a "v" prefix
		if orchVersion[:1] != "v" {
			orchVersion = fmt.Sprintf("v%s", *o.OrchestratorVersion)
		}
		// if the inputted version matches with an available AKS version we can return immediately
		if orchVersion == version && !latestStableVersionDesired {
			return version, nil
		}

		// or, keep track of the highest aks version for a given major.minor
		if semver.MajorMinor(orchVersion) == semver.MajorMinor(maxVersion) && semver.Compare(orchVersion, maxVersion) >= 0 {
			maxVersion = orchVersion
			foundWorkingVersion = true
		}
	}

	// This means there is no version supported by AKS for this major.minor
	if !foundWorkingVersion {
		return "", errors.New(fmt.Sprintf("No AKS versions found for %s", semver.MajorMinor(baseVersion)))
	}

	return maxVersion, nil
}

// GetLatestStableAKSKubernetesVersion returns the latest stable available Kubernetes version of AKS.
func GetLatestStableAKSKubernetesVersion(ctx context.Context, subscriptionID, location string) (string, error) {
	return getLatestStableAKSKubernetesVersionOffset(ctx, subscriptionID, location, 0)
}

// GetNextLatestStableAKSKubernetesVersion returns the stable available
// Kubernetes version of AKS immediately preceding the latest.
func GetNextLatestStableAKSKubernetesVersion(ctx context.Context, subscriptionID, location string) (string, error) {
	return getLatestStableAKSKubernetesVersionOffset(ctx, subscriptionID, location, 1)
}

func getLatestStableAKSKubernetesVersionOffset(ctx context.Context, subscriptionID, location string, offset int) (string, error) {
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return "", errors.Wrap(err, "failed to get settings from environment")
	}
	authorizer, err := azureutil.GetAuthorizer(settings)
	if err != nil {
		return "", errors.Wrap(err, "failed to create an Authorizer")
	}
	containerServiceClient := containerservice.NewContainerServicesClient(subscriptionID)
	containerServiceClient.Authorizer = authorizer
	result, err := containerServiceClient.ListOrchestrators(ctx, location, ManagedClustersResourceType)
	if err != nil {
		return "", errors.Wrap(err, "failed to list Orchestrators")
	}

	var orchestratorversions []string
	var foundWorkingVersion bool
	var orchVersion string
	var maxVersion string

	for _, o := range *result.Orchestrators {
		orchVersion = *o.OrchestratorVersion
		// semver comparisons require a "v" prefix
		if orchVersion[:1] != "v" && o.IsPreview == nil {
			orchVersion = fmt.Sprintf("v%s", *o.OrchestratorVersion)
		}
		orchestratorversions = append(orchestratorversions, orchVersion)
	}
	semver.Sort(orchestratorversions)
	maxVersion = orchestratorversions[len(orchestratorversions)-1-offset]
	if semver.IsValid(maxVersion) {
		foundWorkingVersion = true
	}
	if !foundWorkingVersion {
		return "", errors.New("latest stable AKS version not found")
	}
	return maxVersion, nil
}
