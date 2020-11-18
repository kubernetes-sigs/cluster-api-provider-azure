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

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterctl "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AzureCredentialsProvider provides
type AzureCredentialsProvider struct {
	Client       client.Client
	AzureCluster *infrav1.AzureCluster
	Identity     *aadpodv1.AzureIdentity
}

// NewAzureCredentialsProvider creates a new AzureCredentialsProvider from the supplied inputs.
func NewAzureCredentialsProvider(ctx context.Context, kubeClient client.Client, azureCluster *infrav1.AzureCluster, identityName, namespace string) (*AzureCredentialsProvider, error) {
	if identityName == "" {
		return nil, errors.New("failed to generate new AzureCredentialsProvider from empty identityName")
	}

	azureIdentity := &aadpodv1.AzureIdentity{}
	err := kubeClient.Get(ctx, client.ObjectKey{Name: identityName, Namespace: namespace}, azureIdentity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get AzureIdentity")
	}
	if azureIdentity.Spec.Type != aadpodv1.ServicePrincipal {
		return nil, errors.New("AzureIdentity is not of type Service Principal")
	}

	return &AzureCredentialsProvider{
		Client:       kubeClient,
		AzureCluster: azureCluster,
		Identity:     azureIdentity,
	}, nil
}

// GetAuthorizer returns Azure authorizer based on the provided azure identity
func (p *AzureCredentialsProvider) GetAuthorizer(ctx context.Context, resourceManagerEndpoint string) (autorest.Authorizer, error) {
	copiedIdentity := &aadpodv1.AzureIdentity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureIdentity",
			APIVersion: "aadpodidentity.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", p.AzureCluster.Namespace, p.Identity.Name),
			Namespace: infrav1.ControllerNamespace,
			Annotations: map[string]string{
				aadpodv1.BehaviorKey: "namespaced",
			},
			Labels: map[string]string{
				clusterv1.ClusterLabelName:         p.AzureCluster.Name,
				infrav1.ClusterLabelNamespace:      p.AzureCluster.Namespace,
				clusterctl.ClusterctlMoveLabelName: "true",
			},
			OwnerReferences: p.AzureCluster.OwnerReferences,
		},
		Spec: p.Identity.Spec,
	}
	err := p.Client.Create(ctx, copiedIdentity)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, errors.Wrapf(err, "failed to create copied AzureIdentity %s in %s", copiedIdentity.Name, infrav1.ControllerNamespace)
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
				clusterv1.ClusterLabelName:         p.AzureCluster.Name,
				infrav1.ClusterLabelNamespace:      p.AzureCluster.Namespace,
				clusterctl.ClusterctlMoveLabelName: "true",
			},
			OwnerReferences: p.AzureCluster.OwnerReferences,
		},
		Spec: aadpodv1.AzureIdentityBindingSpec{
			AzureIdentity: copiedIdentity.Name,
			Selector:      infrav1.AzureIdentityBindingSelector, //should be same as selector added on controller
		},
	}
	err = p.Client.Create(ctx, azureIdentityBinding)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, errors.Wrapf(err, "failed to create AzureIdentityBinding %s in %s", copiedIdentity.Name, infrav1.ControllerNamespace)
	}

	var spt *adal.ServicePrincipalToken
	msiEndpoint, err := adal.GetMSIVMEndpoint()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get MSI endpoint")
	}
	if p.Identity.Spec.Type == aadpodv1.ServicePrincipal {
		spt, err = adal.NewServicePrincipalTokenFromMSIWithUserAssignedID(msiEndpoint, resourceManagerEndpoint, p.Identity.Spec.ClientID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get token from service principal identity")
		}
	} else if p.Identity.Spec.Type == aadpodv1.UserAssignedMSI {
		return nil, errors.Wrap(err, "UserAssignedMSI not supported")
	}

	return autorest.NewBearerAuthorizer(spt), nil
}
