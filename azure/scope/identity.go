/*
Copyright 2020 The Kubernetes Authors.

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

package scope

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/identity"
	"sigs.k8s.io/cluster-api-provider-azure/util/system"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterctl "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CredentialsProvider defines the behavior for azure identity based credential providers.
type CredentialsProvider interface {
	GetAuthorizer(ctx context.Context, resourceManagerEndpoint string) (autorest.Authorizer, error)
}

// AzureCredentialsProvider represents a credential provider with azure cluster identity.
type AzureCredentialsProvider struct {
	Client   client.Client
	Identity *infrav1.AzureClusterIdentity
}

// AzureClusterCredentialsProvider wraps AzureCredentialsProvider with AzureCluster.
type AzureClusterCredentialsProvider struct {
	AzureCredentialsProvider
	AzureCluster *infrav1.AzureCluster
}

// ManagedControlPlaneCredentialsProvider wraps AzureCredentialsProvider with AzureManagedControlPlane.
type ManagedControlPlaneCredentialsProvider struct {
	AzureCredentialsProvider
	AzureManagedControlPlane *infrav1exp.AzureManagedControlPlane
}

var _ CredentialsProvider = (*AzureClusterCredentialsProvider)(nil)
var _ CredentialsProvider = (*ManagedControlPlaneCredentialsProvider)(nil)

// NewAzureClusterCredentialsProvider creates a new AzureClusterCredentialsProvider from the supplied inputs.
func NewAzureClusterCredentialsProvider(ctx context.Context, kubeClient client.Client, azureCluster *infrav1.AzureCluster) (*AzureClusterCredentialsProvider, error) {
	if azureCluster.Spec.IdentityRef == nil {
		return nil, errors.New("failed to generate new AzureClusterCredentialsProvider from empty identityName")
	}

	ref := azureCluster.Spec.IdentityRef
	// if the namespace isn't specified then assume it's in the same namespace as the AzureCluster
	namespace := ref.Namespace
	if namespace == "" {
		namespace = azureCluster.Namespace
	}
	identity := &infrav1.AzureClusterIdentity{}
	key := client.ObjectKey{Name: ref.Name, Namespace: namespace}
	if err := kubeClient.Get(ctx, key, identity); err != nil {
		return nil, errors.Errorf("failed to retrieve AzureClusterIdentity external object %q/%q: %v", key.Namespace, key.Name, err)
	}

	if identity.Spec.Type != infrav1.ServicePrincipal {
		return nil, errors.New("AzureClusterIdentity is not of type Service Principal")
	}

	return &AzureClusterCredentialsProvider{
		AzureCredentialsProvider{
			Client:   kubeClient,
			Identity: identity,
		},
		azureCluster,
	}, nil
}

// GetAuthorizer returns an Azure authorizer based on the provided azure identity. It delegates to AzureCredentialsProvider with AzureCluster metadata.
func (p *AzureClusterCredentialsProvider) GetAuthorizer(ctx context.Context, resourceManagerEndpoint string) (autorest.Authorizer, error) {
	return p.AzureCredentialsProvider.GetAuthorizer(ctx, resourceManagerEndpoint, p.AzureCluster.ObjectMeta)
}

// NewManagedControlPlaneCredentialsProvider creates a new ManagedControlPlaneCredentialsProvider from the supplied inputs.
func NewManagedControlPlaneCredentialsProvider(ctx context.Context, kubeClient client.Client, managedControlPlane *infrav1exp.AzureManagedControlPlane) (*ManagedControlPlaneCredentialsProvider, error) {
	if managedControlPlane.Spec.IdentityRef == nil {
		return nil, errors.New("failed to generate new ManagedControlPlaneCredentialsProvider from empty identityName")
	}

	ref := managedControlPlane.Spec.IdentityRef
	// if the namespace isn't specified then assume it's in the same namespace as the AzureManagedControlPlane
	namespace := ref.Namespace
	if namespace == "" {
		namespace = managedControlPlane.Namespace
	}
	identity := &infrav1.AzureClusterIdentity{}
	key := client.ObjectKey{Name: ref.Name, Namespace: namespace}
	if err := kubeClient.Get(ctx, key, identity); err != nil {
		return nil, errors.Errorf("failed to retrieve AzureClusterIdentity external object %q/%q: %v", key.Namespace, key.Name, err)
	}

	if identity.Spec.Type != infrav1.ServicePrincipal {
		return nil, errors.New("AzureClusterIdentity is not of type Service Principal")
	}

	return &ManagedControlPlaneCredentialsProvider{
		AzureCredentialsProvider{
			Client:   kubeClient,
			Identity: identity,
		},
		managedControlPlane,
	}, nil
}

// GetAuthorizer returns an Azure authorizer based on the provided azure identity. It delegates to AzureCredentialsProvider with AzureManagedControlPlane metadata.
func (p *ManagedControlPlaneCredentialsProvider) GetAuthorizer(ctx context.Context, resourceManagerEndpoint string) (autorest.Authorizer, error) {
	return p.AzureCredentialsProvider.GetAuthorizer(ctx, resourceManagerEndpoint, p.AzureManagedControlPlane.ObjectMeta)
}

// GetAuthorizer returns an Azure authorizer based on the provided azure identity and cluster metadata.
func (p *AzureCredentialsProvider) GetAuthorizer(ctx context.Context, resourceManagerEndpoint string, clusterMeta metav1.ObjectMeta) (autorest.Authorizer, error) {
	azureIdentityType, err := getAzureIdentityType(p.Identity)
	if err != nil {
		return nil, err
	}

	// AzureIdentity and AzureIdentityBinding will no longer have an OwnerRef starting from capz release v0.5.0 because of the following:
	// In Kubenetes v1.20+, if the garbage collector detects an invalid cross-namespace ownerReference, or a cluster-scoped dependent with
	// an ownerReference referencing a namespaced kind, a warning Event with a reason of OwnerRefInvalidNamespace and an involvedObject
	// of the invalid dependent is reported. You can check for that kind of Event by running kubectl get events -A --field-selector=reason=OwnerRefInvalidNamespace.

	copiedIdentity := &aadpodv1.AzureIdentity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureIdentity",
			APIVersion: "aadpodidentity.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      identity.GetAzureIdentityName(clusterMeta.Name, clusterMeta.Namespace, p.Identity.Name),
			Namespace: system.GetManagerNamespace(),
			Annotations: map[string]string{
				aadpodv1.BehaviorKey: "namespaced",
			},
			Labels: map[string]string{
				clusterv1.ClusterLabelName:                  clusterMeta.Name,
				infrav1.ClusterLabelNamespace:               clusterMeta.Namespace,
				clusterctl.ClusterctlMoveHierarchyLabelName: "true",
			},
		},
		Spec: aadpodv1.AzureIdentitySpec{
			Type:           azureIdentityType,
			TenantID:       p.Identity.Spec.TenantID,
			ClientID:       p.Identity.Spec.ClientID,
			ClientPassword: p.Identity.Spec.ClientSecret,
			ResourceID:     p.Identity.Spec.ResourceID,
		},
	}
	err = p.Client.Create(ctx, copiedIdentity)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, errors.Errorf("failed to create copied AzureIdentity %s in %s: %v", copiedIdentity.Name, system.GetManagerNamespace(), err)
	}

	azureIdentityBinding := &aadpodv1.AzureIdentityBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureIdentityBinding",
			APIVersion: "aadpodidentity.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-binding", copiedIdentity.Name),
			Namespace: copiedIdentity.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName:                  clusterMeta.Name,
				infrav1.ClusterLabelNamespace:               clusterMeta.Namespace,
				clusterctl.ClusterctlMoveHierarchyLabelName: "true",
			},
		},
		Spec: aadpodv1.AzureIdentityBindingSpec{
			AzureIdentity: copiedIdentity.Name,
			Selector:      infrav1.AzureIdentityBindingSelector, //should be same as selector added on controller
		},
	}
	err = p.Client.Create(ctx, azureIdentityBinding)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, errors.Errorf("failed to create AzureIdentityBinding %s in %s: %v", copiedIdentity.Name, system.GetManagerNamespace(), err)
	}

	var spt *adal.ServicePrincipalToken
	msiEndpoint, err := adal.GetMSIVMEndpoint()
	if err != nil {
		return nil, errors.Errorf("failed to get MSI endpoint: %v", err)
	}
	if p.Identity.Spec.Type == infrav1.ServicePrincipal {
		spt, err = adal.NewServicePrincipalTokenFromMSIWithUserAssignedID(msiEndpoint, resourceManagerEndpoint, p.Identity.Spec.ClientID)
		if err != nil {
			return nil, errors.Errorf("failed to get token from service principal identity: %v", err)
		}
	} else if p.Identity.Spec.Type == infrav1.UserAssignedMSI {
		return nil, errors.Errorf("UserAssignedMSI not supported: %v", err)
	}

	return autorest.NewBearerAuthorizer(spt), nil
}

func getAzureIdentityType(identity *infrav1.AzureClusterIdentity) (aadpodv1.IdentityType, error) {
	switch identity.Spec.Type {
	case infrav1.ServicePrincipal:
		return aadpodv1.ServicePrincipal, nil
	case infrav1.UserAssignedMSI:
		return aadpodv1.UserAssignedMSI, nil
	}

	return 0, errors.New("AzureIdentity does not have a vaild type")
}

// IsClusterNamespaceAllowed indicates if the cluster namespace is allowed.
func IsClusterNamespaceAllowed(ctx context.Context, k8sClient client.Client, allowedNamespaces *infrav1.AllowedNamespaces, namespace string) bool {
	if allowedNamespaces == nil {
		return false
	}

	// empty value matches with all namespaces
	if reflect.DeepEqual(*allowedNamespaces, infrav1.AllowedNamespaces{}) {
		return true
	}

	for _, v := range allowedNamespaces.NamespaceList {
		if v == namespace {
			return true
		}
	}

	// Check if clusterNamespace is in the namespaces selected by the identity's allowedNamespaces selector.
	namespaces := &corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(allowedNamespaces.Selector)
	if err != nil {
		return false
	}

	// If a Selector has a nil or empty selector, it should match nothing.
	if selector.Empty() {
		return false
	}

	if err := k8sClient.List(ctx, namespaces, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return false
	}

	for _, n := range namespaces.Items {
		if n.Name == namespace {
			return true
		}
	}

	return false
}
