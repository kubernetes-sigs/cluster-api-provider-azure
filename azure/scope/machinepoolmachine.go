/*
Copyright 2021 The Kubernetes Authors.

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
	"reflect"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	utilkubeconfig "sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	nodeGetter interface {
		GetNodeByProviderID(ctx context.Context, providerID string) (*corev1.Node, error)
		GetNodeByObjectReference(ctx context.Context, nodeRef corev1.ObjectReference) (*corev1.Node, error)
	}

	workloadClusterProxy struct {
		Client  client.Client
		Cluster client.ObjectKey
	}

	// MachinePoolMachineScopeParams defines the input parameters used to create a new MachinePoolScope.
	MachinePoolMachineScopeParams struct {
		AzureMachinePool        *infrav1exp.AzureMachinePool
		AzureMachinePoolMachine *infrav1exp.AzureMachinePoolMachine
		Client                  client.Client
		ClusterScope            azure.ClusterScoper
		Logger                  logr.Logger
		MachinePool             *capiv1exp.MachinePool

		// workloadNodeGetter is only used for testing purposes and provides a way for mocking requests to the workload cluster
		workloadNodeGetter nodeGetter
	}

	// MachinePoolMachineScope defines a scope defined around a machine pool machine.
	MachinePoolMachineScope struct {
		azure.ClusterScoper
		logr.Logger
		AzureMachinePoolMachine *infrav1exp.AzureMachinePoolMachine
		AzureMachinePool        *infrav1exp.AzureMachinePool
		MachinePool             *capiv1exp.MachinePool
		MachinePoolScope        *MachinePoolScope
		client                  client.Client
		patchHelper             *patch.Helper
		instance                *azure.VMSSVM

		// workloadNodeGetter is only used for testing purposes and provides a way for mocking requests to the workload cluster
		workloadNodeGetter nodeGetter
	}
)

// NewMachinePoolMachineScope creates a new MachinePoolMachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachinePoolMachineScope(params MachinePoolMachineScopeParams) (*MachinePoolMachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}

	if params.ClusterScope == nil {
		return nil, errors.New("cluster scope is required when creating a MachinePoolScope")
	}

	if params.MachinePool == nil {
		return nil, errors.New("machine pool is required when creating a MachinePoolScope")
	}

	if params.AzureMachinePool == nil {
		return nil, errors.New("azure machine pool is required when creating a MachinePoolScope")
	}

	if params.AzureMachinePoolMachine == nil {
		return nil, errors.New("azure machine pool machine is required when creating a MachinePoolScope")
	}

	if params.workloadNodeGetter == nil {
		params.workloadNodeGetter = newWorkloadClusterProxy(
			params.Client,
			client.ObjectKey{
				Namespace: params.MachinePool.Namespace,
				Name:      params.ClusterScope.ClusterName(),
			},
		)
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	mpScope, err := NewMachinePoolScope(MachinePoolScopeParams{
		Client:           params.Client,
		Logger:           params.Logger,
		MachinePool:      params.MachinePool,
		AzureMachinePool: params.AzureMachinePool,
		ClusterScope:     params.ClusterScope,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to build machine pool scope")
	}

	helper, err := patch.NewHelper(params.AzureMachinePoolMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &MachinePoolMachineScope{
		AzureMachinePool:        params.AzureMachinePool,
		AzureMachinePoolMachine: params.AzureMachinePoolMachine,
		ClusterScoper:           params.ClusterScope,
		Logger:                  params.Logger,
		MachinePool:             params.MachinePool,
		MachinePoolScope:        mpScope,
		client:                  params.Client,
		patchHelper:             helper,
		workloadNodeGetter:      params.workloadNodeGetter,
	}, nil
}

// Name is the name of the Machine Pool Machine.
func (s *MachinePoolMachineScope) Name() string {
	return s.AzureMachinePoolMachine.Name
}

// InstanceID is the unique ID of the machine within the Machine Pool.
func (s *MachinePoolMachineScope) InstanceID() string {
	return s.AzureMachinePoolMachine.Spec.InstanceID
}

// ScaleSetName is the name of the VMSS.
func (s *MachinePoolMachineScope) ScaleSetName() string {
	return s.MachinePoolScope.Name()
}

// GetLongRunningOperationState gets a future representing the current state of a long-running operation if one exists.
func (s *MachinePoolMachineScope) GetLongRunningOperationState() *infrav1.Future {
	return s.AzureMachinePoolMachine.Status.LongRunningOperationState
}

// SetLongRunningOperationState sets a future representing the current state of a long-running operation.
func (s *MachinePoolMachineScope) SetLongRunningOperationState(future *infrav1.Future) {
	s.AzureMachinePoolMachine.Status.LongRunningOperationState = future
}

// SetVMSSVM update the scope with the current state of the VMSS VM.
func (s *MachinePoolMachineScope) SetVMSSVM(instance *azure.VMSSVM) {
	s.instance = instance
}

// ProvisioningState returns the AzureMachinePoolMachine provisioning state.
func (s *MachinePoolMachineScope) ProvisioningState() infrav1.ProvisioningState {
	if s.AzureMachinePoolMachine.Status.ProvisioningState != nil {
		return *s.AzureMachinePoolMachine.Status.ProvisioningState
	}
	return ""
}

// IsReady indicates the machine has successfully provisioned and has a node ref associated.
func (s *MachinePoolMachineScope) IsReady() bool {
	state := s.AzureMachinePoolMachine.Status.ProvisioningState
	return s.AzureMachinePoolMachine.Status.Ready && state != nil && *state == infrav1.Succeeded
}

// SetFailureMessage sets the AzureMachinePoolMachine status failure message.
func (s *MachinePoolMachineScope) SetFailureMessage(v error) {
	s.AzureMachinePool.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the AzureMachinePoolMachine status failure reason.
func (s *MachinePoolMachineScope) SetFailureReason(v capierrors.MachineStatusError) {
	s.AzureMachinePool.Status.FailureReason = &v
}

// ProviderID returns the AzureMachinePool ID by parsing Spec.FakeProviderID.
func (s *MachinePoolMachineScope) ProviderID() string {
	return s.AzureMachinePoolMachine.Spec.ProviderID
}

// Close updates the state of MachinePoolMachine.
func (s *MachinePoolMachineScope) Close(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.Close")
	defer span.End()

	return s.patchHelper.Patch(ctx, s.AzureMachinePoolMachine)
}

// UpdateStatus updates the node reference for the machine and other status fields. This func should be called at the
// end of a reconcile request and after updating the scope with the most recent Azure data.
func (s *MachinePoolMachineScope) UpdateStatus(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.Get")
	defer span.End()

	var (
		nodeRef = s.AzureMachinePoolMachine.Status.NodeRef
		node    *corev1.Node
		err     error
	)
	if nodeRef == nil || nodeRef.Name == "" {
		node, err = s.workloadNodeGetter.GetNodeByProviderID(ctx, s.ProviderID())
	} else {
		node, err = s.workloadNodeGetter.GetNodeByObjectReference(ctx, *nodeRef)
	}

	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to to get node by providerID or object reference")
	}

	if node != nil {
		s.AzureMachinePoolMachine.Status.NodeRef = &corev1.ObjectReference{
			Kind:       node.Kind,
			Namespace:  node.Namespace,
			Name:       node.Name,
			UID:        node.UID,
			APIVersion: node.APIVersion,
		}

		s.AzureMachinePoolMachine.Status.Ready = noderefutil.IsNodeReady(node)
		s.AzureMachinePoolMachine.Status.Version = node.Status.NodeInfo.KubeletVersion
	}

	if s.instance != nil {
		hasLatestModel, err := s.hasLatestModelApplied()
		if err != nil {
			return errors.Wrap(err, "failed to determine if the VMSS instance has the latest model")
		}

		s.AzureMachinePoolMachine.Status.LatestModelApplied = hasLatestModel
		s.AzureMachinePoolMachine.Status.ProvisioningState = &s.instance.State
	}

	return nil
}

func (s *MachinePoolMachineScope) hasLatestModelApplied() (bool, error) {
	if s.instance == nil {
		return false, errors.New("instance must not be nil")
	}

	image, err := s.MachinePoolScope.GetVMImage()
	if err != nil {
		return false, errors.Wrap(err, "unable to build vm image information from MachinePoolScope")
	}

	// this should never happen as GetVMImage should only return nil when err != nil. Just in case.
	if image == nil {
		return false, errors.New("machinepoolscope image must not be nil")
	}

	// if the images match, then the VM is of the same model
	return reflect.DeepEqual(s.instance.Image, *image), nil
}

func newWorkloadClusterProxy(c client.Client, cluster client.ObjectKey) *workloadClusterProxy {
	return &workloadClusterProxy{
		Client:  c,
		Cluster: cluster,
	}
}

// GetNodeByObjectReference will fetch a *corev1.Node via a node object reference.
func (np *workloadClusterProxy) GetNodeByObjectReference(ctx context.Context, nodeRef corev1.ObjectReference) (*corev1.Node, error) {
	workloadClient, err := getWorkloadClient(ctx, np.Client, np.Cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the workload cluster client")
	}

	var node corev1.Node
	err = workloadClient.Get(ctx, client.ObjectKey{
		Namespace: nodeRef.Namespace,
		Name:      nodeRef.Name,
	}, &node)

	return &node, err
}

// GetNodeByProviderID will fetch a node from the workload cluster by it's providerID.
func (np *workloadClusterProxy) GetNodeByProviderID(ctx context.Context, providerID string) (*corev1.Node, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.getNode")
	defer span.End()

	workloadClient, err := getWorkloadClient(ctx, np.Client, np.Cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the workload cluster client")
	}

	return getNodeByProviderID(ctx, workloadClient, providerID)
}

func getNodeByProviderID(ctx context.Context, workloadClient client.Client, providerID string) (*corev1.Node, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.getNodeRefForProviderID")
	defer span.End()

	nodeList := corev1.NodeList{}
	for {
		if err := workloadClient.List(ctx, &nodeList, client.Continue(nodeList.Continue)); err != nil {
			return nil, errors.Wrapf(err, "failed to List nodes")
		}

		for _, node := range nodeList.Items {
			if node.Spec.ProviderID == providerID {
				return &node, nil
			}
		}

		if nodeList.Continue == "" {
			break
		}
	}

	return nil, nil
}

func getWorkloadClient(ctx context.Context, c client.Client, cluster client.ObjectKey) (client.Client, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.getWorkloadClient")
	defer span.End()

	obj := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}
	dataBytes, err := utilkubeconfig.FromSecret(ctx, c, obj)
	if err != nil {
		return nil, errors.Wrapf(err, "\"%s-kubeconfig\" not found in namespace %q", obj.Name, obj.Namespace)
	}

	config, err := clientcmd.Load(dataBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load \"%s-kubeconfig\" in namespace %q", obj.Name, obj.Namespace)
	}

	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed transform config \"%s-kubeconfig\" in namespace %q", obj.Name, obj.Namespace)
	}

	return client.New(restConfig, client.Options{})
}
