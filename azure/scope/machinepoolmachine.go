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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	utilkubeconfig "sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// MachinePoolMachineScopeParams defines the input parameters used to create a new MachinePoolScope.
	MachinePoolMachineScopeParams struct {
		AzureMachinePool        *infrav1exp.AzureMachinePool
		AzureMachinePoolMachine *infrav1exp.AzureMachinePoolMachine
		Client                  client.Client
		ClusterScope            azure.ClusterScoper
		Logger                  logr.Logger
		MachinePool             *capiv1exp.MachinePool
	}

	// MachinePoolMachineScope defines a scope defined around a machine pool machine.
	MachinePoolMachineScope struct {
		azure.ClusterScoper
		logr.Logger
		AzureMachinePoolMachine *infrav1exp.AzureMachinePoolMachine
		AzureMachinePool        *infrav1exp.AzureMachinePool
		MachinePool             *capiv1exp.MachinePool
		client                  client.Client
		patchHelper             *patch.Helper
		instance                *infrav1exp.VMSSVM
	}
)

// NewMachinePoolMachineScope creates a new MachinePoolMachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachinePoolMachineScope(params MachinePoolMachineScopeParams) (*MachinePoolMachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machine pool is required when creating a MachinePoolScope")
	}
	if params.AzureMachinePool == nil {
		return nil, errors.New("azure machine pool is required when creating a MachinePoolScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
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
		client:                  params.Client,
		patchHelper:             helper,
	}, nil
}

// Name is the name of the Machine Pool Machine
func (s *MachinePoolMachineScope) Name() string {
	return s.AzureMachinePoolMachine.Name
}

// InstanceID is the unique ID of the machine within the Machine Pool
func (s *MachinePoolMachineScope) InstanceID() string {
	return s.AzureMachinePoolMachine.Status.InstanceID
}

// ScaleSetName is the name of the VMSS
func (s *MachinePoolMachineScope) ScaleSetName() string {
	return s.AzureMachinePool.Name
}

// GetLongRunningOperationState gets a future representing the current state of a long running operation if one exists
func (s *MachinePoolMachineScope) GetLongRunningOperationState() *infrav1.Future {
	return s.AzureMachinePoolMachine.Status.LongRunningOperationState
}

// SetLongRunningOperationState sets a future representing the current state of a long running operation
func (s *MachinePoolMachineScope) SetLongRunningOperationState(future *infrav1.Future) {
	s.AzureMachinePoolMachine.Status.LongRunningOperationState = future
}

// SetVMSSVM update the scope with the current state of the VMSS VM
func (s *MachinePoolMachineScope) SetVMSSVM(instance *infrav1exp.VMSSVM) {
	s.instance = instance
}

// Close updates the state of MachinePoolMachine
func (s *MachinePoolMachineScope) Close(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.Close")
	defer span.End()

	if err := s.updateState(ctx); err != nil {
		return errors.Wrap(err, "failed to update state")
	}

	return s.patchHelper.Patch(ctx, s.AzureMachinePoolMachine)
}

// ShouldDrain returns a bool indicating the controller should attempt drain the node before deleting
func (s *MachinePoolMachineScope) ShouldDrain() bool {
	return true
}

// Drain safely drains workloads from the machine's K8s node so that the machine can be replaced
func (s *MachinePoolMachineScope) Drain(ctx context.Context) error {
	_, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.Close")
	defer span.End()

	//patchHelper, err := patch.NewHelper(s.AzureMachinePoolMachine, s.client)
	//if err != nil {
	//	return ctrl.Result{}, err
	//}
	//
	//getNodeStatusByProviderID(ctx)
	//
	//s.Info("Draining node", "node", m.Status.NodeRef.Name)
	//// The DrainingSucceededCondition never exists before the node is drained for the first time,
	//// so its transition time can be used to record the first time draining.
	//// This `if` condition prevents the transition time to be changed more than once.
	//if conditions.Get(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition) == nil {
	//	conditions.MarkFalse(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition, clusterv1.DrainingReason, clusterv1.ConditionSeverityInfo, "Draining the node before deletion")
	//}

	return nil
}

func (s *MachinePoolMachineScope) updateState(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.Get")
	defer span.End()

	node, err := s.getNode(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to to get node ref")
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
		s.AzureMachinePoolMachine.Status.LatestModelApplied = s.instance.LatestModelApplied
		s.AzureMachinePoolMachine.Status.ProvisioningState = &s.instance.State
	}

	return nil
}

func (s *MachinePoolMachineScope) getNode(ctx context.Context) (*corev1.Node, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.getNode")
	defer span.End()

	workloadClient, err := s.getWorkloadClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the workload cluster client")
	}

	if s.AzureMachinePoolMachine.Status.NodeRef == nil {
		return getNodeByProviderID(ctx, workloadClient, s.AzureMachinePoolMachine.Spec.ProviderID)
	}

	var node corev1.Node
	err = workloadClient.Get(ctx, client.ObjectKey{
		Namespace: s.AzureMachinePoolMachine.Status.NodeRef.Namespace,
		Name:      s.AzureMachinePoolMachine.Status.NodeRef.Name,
	}, &node)

	return &node, err
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

func (s *MachinePoolMachineScope) getWorkloadClient(ctx context.Context) (client.Client, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.getWorkloadClient")
	defer span.End()

	obj := client.ObjectKey{
		Namespace: s.MachinePool.Namespace,
		Name:      s.ClusterName(),
	}
	dataBytes, err := utilkubeconfig.FromSecret(ctx, s.client, obj)
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
