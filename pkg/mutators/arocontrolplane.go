/*
Copyright 2024 The Kubernetes Authors.

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

package mutators

import (
	"context"
	"fmt"
	"strings"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

var (
	// ErrNoAROClusterDefined describes an AROControlPlane without a AROCluster.
	ErrNoAROClusterDefined = fmt.Errorf("no %s AROCluster defined in AROControlPlane spec.resources", infrav1exp.GroupVersion.Group)

	// ErrNoHcpOpenShiftClusterDefined describes an AROControlPlane without a HcpOpenShiftCluster.
	ErrNoHcpOpenShiftClusterDefined = fmt.Errorf("no %s HcpOpenShiftCluster defined in AROControlPlane spec.resources", asoredhatopenshiftv1.GroupVersion.Group)
)

// SetAROClusterDefaults propagates values defined by Cluster API to an ARO AROCluster.
func SetAROClusterDefaults(_ client.Client, _ *controlv1.AROControlPlane, cluster *clusterv1beta1.Cluster) ResourcesMutator {
	return func(ctx context.Context, us []*unstructured.Unstructured) error {
		ctx, _, done := tele.StartSpanWithLogger(ctx, "mutators.SetAROClusterDefaults")
		defer done()

		var aroCluster *unstructured.Unstructured
		var aroClusterPath string
		for i, u := range us {
			if u.GroupVersionKind().Group == infrav1exp.GroupVersion.Group &&
				u.GroupVersionKind().Kind == "AROCluster" {
				aroCluster = u
				aroClusterPath = fmt.Sprintf("spec.resources[%d]", i)
				break
			}
		}
		if aroCluster == nil {
			return reconcile.TerminalError(ErrNoAROClusterDefined)
		}

		if err := setAROClusterServiceCIDR(ctx, cluster, aroClusterPath, aroCluster); err != nil {
			return err
		}

		if err := setAROClusterPodCIDR(ctx, cluster, aroClusterPath, aroCluster); err != nil {
			return err
		}

		if err := setAROClusterCredentials(ctx, cluster, aroClusterPath, aroCluster); err != nil { //nolint:nolintlint // leave it as is
			return err
		}

		return nil
	}
}

func setAROClusterServiceCIDR(ctx context.Context, cluster *clusterv1beta1.Cluster, aroClusterPath string, aroCluster *unstructured.Unstructured) error {
	_, log, done := tele.StartSpanWithLogger(ctx, "mutators.setAROClusterServiceCIDR")
	defer done()

	if cluster.Spec.ClusterNetwork == nil ||
		cluster.Spec.ClusterNetwork.Services == nil ||
		len(cluster.Spec.ClusterNetwork.Services.CIDRBlocks) == 0 {
		return nil
	}

	capiCIDR := cluster.Spec.ClusterNetwork.Services.CIDRBlocks[0]

	// AROCluster.v1api20210501.containerservice.azure.com does not contain the plural serviceCidrs field.
	svcCIDRPath := []string{"spec", "networkProfile", "serviceCidr"}
	userSvcCIDR, found, err := unstructured.NestedString(aroCluster.UnstructuredContent(), svcCIDRPath...)
	if err != nil {
		return err
	}
	setSvcCIDR := mutation{
		location: aroClusterPath + "." + strings.Join(svcCIDRPath, "."),
		val:      capiCIDR,
		reason:   fmt.Sprintf("because spec.clusterNetwork.services.cidrBlocks[0] in Cluster %s/%s is set to %s", cluster.Namespace, cluster.Name, capiCIDR),
	}
	if found && userSvcCIDR != capiCIDR {
		return Incompatible{
			mutation: setSvcCIDR,
			userVal:  userSvcCIDR,
		}
	}
	logMutation(log, setSvcCIDR)
	return unstructured.SetNestedField(aroCluster.UnstructuredContent(), capiCIDR, svcCIDRPath...)
}

func setAROClusterPodCIDR(ctx context.Context, cluster *clusterv1beta1.Cluster, aroClusterPath string, aroCluster *unstructured.Unstructured) error {
	_, log, done := tele.StartSpanWithLogger(ctx, "mutators.setAROClusterPodCIDR")
	defer done()

	if cluster.Spec.ClusterNetwork == nil ||
		cluster.Spec.ClusterNetwork.Pods == nil ||
		len(cluster.Spec.ClusterNetwork.Pods.CIDRBlocks) == 0 {
		return nil
	}

	capiCIDR := cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0]

	// AROCluster.v1api20210501.containerservice.azure.com does not contain the plural podCidrs field.
	podCIDRPath := []string{"spec", "networkProfile", "podCidr"}
	userPodCIDR, found, err := unstructured.NestedString(aroCluster.UnstructuredContent(), podCIDRPath...)
	if err != nil {
		return err
	}
	setPodCIDR := mutation{
		location: aroClusterPath + "." + strings.Join(podCIDRPath, "."),
		val:      capiCIDR,
		reason:   fmt.Sprintf("because spec.clusterNetwork.pods.cidrBlocks[0] in Cluster %s/%s is set to %s", cluster.Namespace, cluster.Name, capiCIDR),
	}
	if found && userPodCIDR != capiCIDR {
		return Incompatible{
			mutation: setPodCIDR,
			userVal:  userPodCIDR,
		}
	}
	logMutation(log, setPodCIDR)
	return unstructured.SetNestedField(aroCluster.UnstructuredContent(), capiCIDR, podCIDRPath...)
}

func setAROClusterCredentials(ctx context.Context, cluster *clusterv1beta1.Cluster, aroClusterPath string, aroCluster *unstructured.Unstructured) error {
	_, log, done := tele.StartSpanWithLogger(ctx, "mutators.setAROClusterCredentials")
	defer done()

	// CAPZ only cares that some set of credentials is created by ARO, but not where. CAPZ will propagate
	// whatever is defined in the ARO resource to the <cluster>-kubeconfig secret as expected by CAPI.

	_, hasUserCreds, err := unstructured.NestedMap(aroCluster.UnstructuredContent(), "spec", "operatorSpec", "secrets", "userCredentials")
	if err != nil {
		return err
	}
	if hasUserCreds {
		return nil
	}

	_, hasAdminCreds, err := unstructured.NestedMap(aroCluster.UnstructuredContent(), "spec", "operatorSpec", "secrets", "adminCredentials")
	if err != nil {
		return err
	}
	if hasAdminCreds {
		return nil
	}

	secrets := map[string]interface{}{
		"adminCredentials": map[string]interface{}{
			"name": cluster.Name + "-" + string(secret.Kubeconfig),
			"key":  secret.KubeconfigDataName,
		},
	}

	setCreds := mutation{
		location: aroClusterPath + ".spec.operatorSpec.secrets",
		val:      secrets,
		reason:   "because no userCredentials or adminCredentials are defined",
	}
	logMutation(log, setCreds)
	return unstructured.SetNestedMap(aroCluster.UnstructuredContent(), secrets, "spec", "operatorSpec", "secrets")
}

// VaultInfoProvider provides vault encryption key information.
type VaultInfoProvider interface {
	GetVaultInfo() (vaultName, keyName, keyVersion *string)
}

// SetHcpOpenShiftClusterDefaults sets defaults for HcpOpenShiftCluster resources.
// This mutator automatically sets owner references for HcpOpenShiftClustersExternalAuth resources.
func SetHcpOpenShiftClusterDefaults(_ client.Client, _ *controlv1.AROControlPlane, cluster *clusterv1.Cluster) ResourcesMutator {
	return func(ctx context.Context, us []*unstructured.Unstructured) error {
		ctx, log, done := tele.StartSpanWithLogger(ctx, "mutators.SetHcpOpenShiftClusterDefaults")
		defer done()

		// Find the HcpOpenShiftCluster
		var hcpCluster *unstructured.Unstructured
		var hcpClusterPath string
		var hcpClusterName string
		for i, u := range us {
			if u.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
				u.GroupVersionKind().Kind == "HcpOpenShiftCluster" {
				hcpCluster = u
				hcpClusterName = u.GetName()
				hcpClusterPath = fmt.Sprintf("spec.resources[%d]", i)
				break
			}
		}
		if hcpCluster == nil {
			return reconcile.TerminalError(ErrNoHcpOpenShiftClusterDefined)
		}

		// Set kubeconfig secret if not defined
		if err := setHcpOpenShiftClusterCredentials(ctx, cluster, hcpClusterPath, hcpCluster, log); err != nil {
			return err
		}

		// Set owner references for HcpOpenShiftClustersExternalAuth resources
		for i, u := range us {
			if u.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
				u.GroupVersionKind().Kind == "HcpOpenShiftClustersExternalAuth" {
				if err := setExternalAuthOwner(ctx, u, hcpClusterName, fmt.Sprintf("spec.resources[%d]", i), log); err != nil {
					return err
				}
			}
		}

		return nil
	}
}

// SetHcpOpenShiftClusterEncryptionKey sets the ETCD encryption key version if KMS encryption is configured.
// This mutator should run after the KeyVault service has created the encryption key.
func SetHcpOpenShiftClusterEncryptionKey(vaultInfoProvider VaultInfoProvider) ResourcesMutator {
	return func(ctx context.Context, us []*unstructured.Unstructured) error {
		_, log, done := tele.StartSpanWithLogger(ctx, "mutators.SetHcpOpenShiftClusterEncryptionKey")
		defer done()

		// Find the HcpOpenShiftCluster
		var hcpCluster *unstructured.Unstructured
		var hcpClusterPath string
		for i, u := range us {
			if u.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
				u.GroupVersionKind().Kind == "HcpOpenShiftCluster" {
				hcpCluster = u
				hcpClusterPath = fmt.Sprintf("spec.resources[%d]", i)
				break
			}
		}
		if hcpCluster == nil {
			// No HcpOpenShiftCluster found, nothing to do
			return nil
		}

		// Check if ETCD encryption with KMS is configured
		etcdPath := []string{"spec", "properties", "etcd", "dataEncryption"}
		etcdEncryption, hasEtcdEncryption, err := unstructured.NestedMap(hcpCluster.UnstructuredContent(), etcdPath...)
		if err != nil {
			return err
		}

		if !hasEtcdEncryption {
			// No ETCD encryption configured, nothing to do
			return nil
		}

		// Check if KMS is configured
		kmPath := []string{"customerManaged", "kms"}
		_, hasKMS, err := unstructured.NestedFieldNoCopy(etcdEncryption, kmPath...)
		if err != nil {
			return err
		}

		if !hasKMS {
			// KMS not configured, nothing to do
			return nil
		}

		// Get vault info from provider
		vaultName, keyName, keyVersion := vaultInfoProvider.GetVaultInfo()
		if vaultName == nil || keyName == nil || keyVersion == nil {
			log.V(2).Info("vault info not available yet, skipping encryption key version injection")
			// Vault info not available yet - key creation might still be in progress
			// Return nil to allow reconciliation to continue and retry later
			return nil
		}

		// Check if activeKey.version is already set
		activeKeyVersionPath := []string{"spec", "properties", "etcd", "dataEncryption", "customerManaged", "kms", "activeKey", "version"}
		existingVersion, versionExists, err := unstructured.NestedString(hcpCluster.UnstructuredContent(), activeKeyVersionPath...)
		if err != nil {
			return err
		}

		// If version is already set and matches, nothing to do
		if versionExists && existingVersion == *keyVersion {
			return nil
		}

		// If version is set but doesn't match, this is a conflict
		if versionExists && existingVersion != *keyVersion {
			setVersion := mutation{
				location: hcpClusterPath + "." + strings.Join(activeKeyVersionPath, "."),
				val:      *keyVersion,
				reason:   fmt.Sprintf("because KeyVault encryption key version was created/updated to %s", *keyVersion),
			}
			return Incompatible{
				mutation: setVersion,
				userVal:  existingVersion,
			}
		}

		// Set the encryption key version
		setVersion := mutation{
			location: hcpClusterPath + "." + strings.Join(activeKeyVersionPath, "."),
			val:      *keyVersion,
			reason:   fmt.Sprintf("because KeyVault encryption key was created with version %s", *keyVersion),
		}
		logMutation(log, setVersion)
		return unstructured.SetNestedField(hcpCluster.UnstructuredContent(), *keyVersion, activeKeyVersionPath...)
	}
}

func setHcpOpenShiftClusterCredentials(_ context.Context, cluster *clusterv1.Cluster, hcpClusterPath string, hcpCluster *unstructured.Unstructured, log logr.Logger) error {
	// Check if credentials are already defined
	_, hasUserCreds, err := unstructured.NestedMap(hcpCluster.UnstructuredContent(), "spec", "operatorSpec", "secrets", "userCredentials")
	if err != nil {
		return err
	}
	if hasUserCreds {
		return nil
	}

	_, hasAdminCreds, err := unstructured.NestedMap(hcpCluster.UnstructuredContent(), "spec", "operatorSpec", "secrets", "adminCredentials")
	if err != nil {
		return err
	}
	if hasAdminCreds {
		return nil
	}

	// Set default admin credentials
	secrets := map[string]interface{}{
		"adminCredentials": map[string]interface{}{
			"name": cluster.Name + "-" + string(secret.Kubeconfig),
			"key":  secret.KubeconfigDataName,
		},
	}

	setCreds := mutation{
		location: hcpClusterPath + ".spec.operatorSpec.secrets",
		val:      secrets,
		reason:   "because no userCredentials or adminCredentials are defined",
	}
	logMutation(log, setCreds)
	return unstructured.SetNestedMap(hcpCluster.UnstructuredContent(), secrets, "spec", "operatorSpec", "secrets")
}

func setExternalAuthOwner(_ context.Context, externalAuth *unstructured.Unstructured, hcpClusterName, externalAuthPath string, log logr.Logger) error {
	// Check if owner is already set
	ownerMap, hasOwner, err := unstructured.NestedMap(externalAuth.UnstructuredContent(), "spec", "owner")
	if err != nil {
		return err
	}

	// If owner is already set, validate it references the HcpOpenShiftCluster
	if hasOwner {
		ownerName, _, _ := unstructured.NestedString(ownerMap, "name")
		if ownerName != "" && ownerName != hcpClusterName {
			return Incompatible{
				mutation: mutation{
					location: externalAuthPath + ".spec.owner.name",
					val:      hcpClusterName,
					reason:   "because HcpOpenShiftClustersExternalAuth must reference the HcpOpenShiftCluster",
				},
				userVal: ownerName,
			}
		}
		// Owner already set correctly
		return nil
	}

	// Set the owner reference
	owner := map[string]interface{}{
		"name": hcpClusterName,
	}

	setOwner := mutation{
		location: externalAuthPath + ".spec.owner",
		val:      owner,
		reason:   fmt.Sprintf("because HcpOpenShiftClustersExternalAuth must reference HcpOpenShiftCluster %s", hcpClusterName),
	}
	logMutation(log, setOwner)
	return unstructured.SetNestedMap(externalAuth.UnstructuredContent(), owner, "spec", "owner")
}
