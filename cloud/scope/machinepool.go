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
	"encoding/base64"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	// MachinePoolScopeParams defines the input parameters used to create a new MachinePoolScope.
	MachinePoolScopeParams struct {
		AzureClients
		Client           client.Client
		Logger           logr.Logger
		Cluster          *capiv1.Cluster
		MachinePool      *capiv1exp.MachinePool
		AzureCluster     *infrav1.AzureCluster
		AzureMachinePool *infrav1exp.AzureMachinePool
	}

	// MachinePoolScope defines a scope defined around a machine pool and its cluster.
	MachinePoolScope struct {
		logr.Logger
		AzureClients
		client           client.Client
		patchHelper      *patch.Helper
		Cluster          *capiv1.Cluster
		MachinePool      *capiv1exp.MachinePool
		AzureCluster     *infrav1.AzureCluster
		AzureMachinePool *infrav1exp.AzureMachinePool
	}
)

// NewMachinePoolScope creates a new MachinePoolScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machine pool is required when creating a MachinePoolScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("cluster is required when creating a MachinePoolScope")
	}
	if params.AzureCluster == nil {
		return nil, errors.New("azure cluster is required when creating a MachinePoolScope")
	}
	if params.AzureMachinePool == nil {
		return nil, errors.New("azure machine pool is required when creating a MachinePoolScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	if err := params.AzureClients.setCredentials(params.AzureCluster.Spec.SubscriptionID); err != nil {
		return nil, errors.Wrap(err, "failed to create Azure session")
	}

	helper, err := patch.NewHelper(params.AzureMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &MachinePoolScope{
		client:           params.Client,
		Cluster:          params.Cluster,
		MachinePool:      params.MachinePool,
		AzureCluster:     params.AzureCluster,
		AzureMachinePool: params.AzureMachinePool,
		AzureClients:     params.AzureClients,
		Logger:           params.Logger,
		patchHelper:      helper,
	}, nil
}

func (m *MachinePoolScope) Name() string {
	return m.AzureMachinePool.Name
}

// GetID returns the AzureMachinePool ID by parsing Spec.ProviderID.
func (m *MachinePoolScope) GetID() *string {
	parsed, err := noderefutil.NewProviderID(m.AzureMachinePool.Spec.ProviderID)
	if err != nil {
		return nil
	}
	return pointer.StringPtr(parsed.ID())
}

// SetReady sets the AzureMachinePool Ready Status
func (m *MachinePoolScope) SetReady() {
	m.AzureMachinePool.Status.Ready = true
}

// SetFailureMessage sets the AzureMachinePool status failure message.
func (m *MachinePoolScope) SetFailureMessage(v error) {
	m.AzureMachinePool.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the AzureMachinePool status failure reason.
func (m *MachinePoolScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.AzureMachinePool.Status.FailureReason = &v
}

// AdditionalTags merges AdditionalTags from the scope's AzureCluster and AzureMachinePool. If the same key is present in both,
// the value from AzureMachinePool takes precedence.
func (m *MachinePoolScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	tags.Merge(m.AzureCluster.Spec.AdditionalTags)
	tags.Merge(m.AzureMachinePool.Spec.AdditionalTags)
	return tags
}

// SetAnnotation sets a key value annotation on the AzureMachinePool.
func (m *MachinePoolScope) SetAnnotation(key, value string) {
	if m.AzureMachinePool.Annotations == nil {
		m.AzureMachinePool.Annotations = map[string]string{}
	}
	m.AzureMachinePool.Annotations[key] = value
}

// PatchObject persists the machine spec and status.
func (m *MachinePoolScope) PatchObject() error {
	// TODO[dj]: any function we are building where we are not adding context to the signature is welcoming unbound execution times. This needs to be addressed.
	return m.patchHelper.Patch(context.TODO(), m.AzureMachinePool)
}

func (m *MachinePoolScope) AzureMachineTemplate(ctx context.Context) (*infrav1.AzureMachineTemplate, error) {
	ref := m.MachinePool.Spec.Template.Spec.InfrastructureRef
	return getAzureMachineTemplate(ctx, m.client, ref.Name, ref.Namespace)
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachinePoolScope) Close() error {
	return m.patchHelper.Patch(context.TODO(), m.AzureMachinePool)
}

func getAzureMachineTemplate(ctx context.Context, c client.Client, name, namespace string) (*infrav1.AzureMachineTemplate, error) {
	m := &infrav1.AzureMachineTemplate{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachinePoolScope) GetBootstrapData() (string, error) {
	dataSecretName := m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName
	if dataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine Spec's bootstrap.dataSecretName is nil")
	}
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.AzureMachinePool.Namespace, Name: *dataSecretName}
	if err := m.client.Get(context.TODO(), key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for AzureMachinePool %s/%s", m.AzureMachinePool.Namespace, m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return base64.StdEncoding.EncodeToString(value), nil
}
