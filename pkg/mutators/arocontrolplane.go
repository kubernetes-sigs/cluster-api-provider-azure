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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
)

// SetAROClusterDefaults propagates values defined by Cluster API to an ARO AROCluster.
func SetAROClusterDefaults(_ client.Client, aroControlPlane *controlv1.AROControlPlane, cluster *clusterv1.Cluster) ResourcesMutator {
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

		if err := setAROClusterKubernetesVersion(ctx, aroControlPlane, aroClusterPath, aroCluster); err != nil {
			return err
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

func setAROClusterKubernetesVersion(ctx context.Context, aroControlPlane *controlv1.AROControlPlane, aroClusterPath string, aroCluster *unstructured.Unstructured) error {
	_, log, done := tele.StartSpanWithLogger(ctx, "mutators.setAROClusterKubernetesVersion")
	defer done()

	capzK8sVersion := strings.TrimPrefix(aroControlPlane.Spec.Version, "v")
	if capzK8sVersion == "" {
		// When the CAPI contract field isn't set, any value for version in the embedded ARO resource may be specified.
		return nil
	}

	k8sVersionPath := []string{"spec", "kubernetesVersion"}
	userK8sVersion, k8sVersionFound, err := unstructured.NestedString(aroCluster.UnstructuredContent(), k8sVersionPath...)
	if err != nil {
		return err
	}
	setK8sVersion := mutation{
		location: aroClusterPath + "." + strings.Join(k8sVersionPath, "."),
		val:      capzK8sVersion,
		reason:   "because spec.version is set to " + aroControlPlane.Spec.Version,
	}
	if k8sVersionFound && userK8sVersion != capzK8sVersion {
		return Incompatible{
			mutation: setK8sVersion,
			userVal:  userK8sVersion,
		}
	}
	logMutation(log, setK8sVersion)
	return unstructured.SetNestedField(aroCluster.UnstructuredContent(), capzK8sVersion, k8sVersionPath...)
}

func setAROClusterServiceCIDR(ctx context.Context, cluster *clusterv1.Cluster, aroClusterPath string, aroCluster *unstructured.Unstructured) error {
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

func setAROClusterPodCIDR(ctx context.Context, cluster *clusterv1.Cluster, aroClusterPath string, aroCluster *unstructured.Unstructured) error {
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

func setAROClusterCredentials(ctx context.Context, cluster *clusterv1.Cluster, aroClusterPath string, aroCluster *unstructured.Unstructured) error {
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
