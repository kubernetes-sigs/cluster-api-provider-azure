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
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	"sigs.k8s.io/cluster-api/controllers/remote"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	drain "sigs.k8s.io/cluster-api/third_party/kubernetes-drain"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	// MachinePoolMachineScopeName is the sourceName, or more specifically the UserAgent, of client used in cordon and drain.
	MachinePoolMachineScopeName = "azuremachinepoolmachine-scope"
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

// CordonAndDrain will cordon and drain the Kubernetes node associated with this AzureMachinePoolMachine.
func (s *MachinePoolMachineScope) CordonAndDrain(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.CordonAndDrain")
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

	switch {
	case err != nil && !apierrors.IsNotFound(err):
		// failed due to an unexpected error
		return errors.Wrap(err, "failed to find node")
	case err != nil && apierrors.IsNotFound(err):
		// node was not found due to 404 when finding by ObjectReference
		return nil
	case node == nil:
		// node was not found due to not finding a nodes with the ProviderID
		return nil
	}

	// Drain node before deletion and issue a patch in order to make this operation visible to the users.
	if s.isNodeDrainAllowed() {
		patchHelper, err := patch.NewHelper(s.AzureMachinePoolMachine, s.client)
		if err != nil {
			return errors.Wrap(err, "failed to build a patchHelper when draining node")
		}

		s.V(4).Info("Draining node", "node", node.Name)
		// The DrainingSucceededCondition never exists before the node is drained for the first time,
		// so its transition time can be used to record the first time draining.
		// This `if` condition prevents the transition time to be changed more than once.
		if conditions.Get(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition) == nil {
			conditions.MarkFalse(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition, clusterv1.DrainingReason, clusterv1.ConditionSeverityInfo, "Draining the node before deletion")
		}

		if err := patchHelper.Patch(ctx, s.AzureMachinePoolMachine); err != nil {
			return errors.Wrap(err, "failed to patch AzureMachinePoolMachine")
		}

		if err := s.drainNode(ctx, node); err != nil {
			// Check for condition existence. If the condition exists, it may have a different severity or message, which
			// would cause the last transition time to be updated. The last transition time is used to determine how
			// long to wait to timeout the node drain operation. If we were to keep updating the last transition time,
			// a drain operation may never timeout.
			if conditions.Get(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition) == nil {
				conditions.MarkFalse(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition, clusterv1.DrainingFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			}
			return err
		}

		conditions.MarkTrue(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition)
	}

	return nil
}

func (s *MachinePoolMachineScope) drainNode(ctx context.Context, node *corev1.Node) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolMachineScope.drainNode")
	defer span.End()

	restConfig, err := remote.RESTConfig(ctx, MachinePoolMachineScopeName, s.client, client.ObjectKey{
		Name:      s.ClusterName(),
		Namespace: s.AzureMachinePoolMachine.Namespace,
	})

	if err != nil {
		s.Error(err, "Error creating a remote client while deleting Machine, won't retry")
		return nil
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		s.Error(err, "Error creating a remote client while deleting Machine, won't retry")
		return nil
	}

	drainer := &drain.Helper{
		Client:              kubeClient,
		Force:               true,
		IgnoreAllDaemonSets: true,
		DeleteLocalData:     true,
		GracePeriodSeconds:  -1,
		// If a pod is not evicted in 20 seconds, retry the eviction next time the
		// machine gets reconciled again (to allow other machines to be reconciled).
		Timeout: 20 * time.Second,
		OnPodDeletedOrEvicted: func(pod *corev1.Pod, usingEviction bool) {
			verbStr := "Deleted"
			if usingEviction {
				verbStr = "Evicted"
			}
			s.V(4).Info(fmt.Sprintf("%s pod from Node", verbStr),
				"pod", fmt.Sprintf("%s/%s", pod.Name, pod.Namespace))
		},
		Out:    writer{klog.Info},
		ErrOut: writer{klog.Error},
		DryRun: false,
	}

	if noderefutil.IsNodeUnreachable(node) {
		// When the node is unreachable and some pods are not evicted for as long as this timeout, we ignore them.
		drainer.SkipWaitForDeleteTimeoutSeconds = 60 * 5 // 5 minutes
	}

	if err := drain.RunCordonOrUncordon(ctx, drainer, node, true); err != nil {
		// Machine will be re-reconciled after a cordon failure.
		return azure.WithTransientError(errors.Errorf("unable to cordon node %s: %v", node.Name, err), 20*time.Second)
	}

	if err := drain.RunNodeDrain(ctx, drainer, node.Name); err != nil {
		// Machine will be re-reconciled after a drain failure.
		return azure.WithTransientError(errors.Wrap(err, "Drain failed, retry in 20s"), 20*time.Second)
	}

	s.V(4).Info("Drain successful")
	return nil
}

// isNodeDrainAllowed checks to see the node is excluded from draining or if the NodeDrainTimeout has expired.
func (s *MachinePoolMachineScope) isNodeDrainAllowed() bool {
	if _, exists := s.AzureMachinePoolMachine.ObjectMeta.Annotations[clusterv1.ExcludeNodeDrainingAnnotation]; exists {
		return false
	}

	if s.nodeDrainTimeoutExceeded() {
		return false
	}

	return true
}

// nodeDrainTimeoutExceeded will check to see if the AzureMachinePool's NodeDrainTimeout is exceeded for the
// AzureMachinePoolMachine.
func (s *MachinePoolMachineScope) nodeDrainTimeoutExceeded() bool {
	// if the NodeDrainTineout type is not set by user
	pool := s.AzureMachinePool
	if pool == nil || pool.Spec.NodeDrainTimeout == nil || pool.Spec.NodeDrainTimeout.Seconds() <= 0 {
		return false
	}

	// if the draining succeeded condition does not exist
	if conditions.Get(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition) == nil {
		return false
	}

	now := time.Now()
	firstTimeDrain := conditions.GetLastTransitionTime(s.AzureMachinePoolMachine, clusterv1.DrainingSucceededCondition)
	diff := now.Sub(firstTimeDrain.Time)
	return diff.Seconds() >= s.AzureMachinePool.Spec.NodeDrainTimeout.Seconds()
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

	return remote.NewClusterClient(ctx, MachinePoolMachineScopeName, c, cluster)
}

// writer implements io.Writer interface as a pass-through for klog.
type writer struct {
	logFunc func(args ...interface{})
}

// Write passes string(p) into writer's logFunc and always returns len(p).
func (w writer) Write(p []byte) (n int, err error) {
	w.logFunc(string(p))
	return len(p), nil
}
