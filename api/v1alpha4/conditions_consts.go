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

package v1alpha4

import clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

// AzureCluster Conditions and Reasons.
const (
	// NetworkInfrastructureReadyCondition reports of current status of cluster infrastructure.
	NetworkInfrastructureReadyCondition clusterv1.ConditionType = "NetworkInfrastructureReady"
	// NamespaceNotAllowedByIdentity used to indicate cluster in a namespace not allowed by identity.
	NamespaceNotAllowedByIdentity = "NamespaceNotAllowedByIdentity"
)

// AzureMachine Conditions and Reasons.
const (
	// VMRunningCondition reports on current status of the Azure VM.
	VMRunningCondition clusterv1.ConditionType = "VMRunning"
	// VMCreatingReason used when the vm creation is in progress.
	VMCreatingReason = "VMCreating"
	// VMUpdatingReason used when the vm updating is in progress.
	VMUpdatingReason = "VMUpdating"
	// VMDeletingReason used when the vm is in a deleting state.
	VMDeletingReason = "VMDeleting"
	// VMProvisionFailedReason used for failures during vm provisioning.
	VMProvisionFailedReason = "VMProvisionFailed"
	// WaitingForClusterInfrastructureReason used when machine is waiting for cluster infrastructure to be ready before proceeding.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	// WaitingForBootstrapDataReason used when machine is waiting for bootstrap data to be ready before proceeding.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
	// BootstrapSucceededCondition reports the result of the execution of the boostrap data on the machine.
	BootstrapSucceededCondition = "BoostrapSucceeded"
	// BootstrapInProgressReason is used to indicate the bootstrap data has not finished executing.
	BootstrapInProgressReason = "BootstrapInProgress"
	// BootstrapFailedReason is used to indicate the bootstrap process ran into an error.
	BootstrapFailedReason = "BootstrapFailed"
)

// AzureMachinePool Conditions and Reasons.
const (
	// ScaleSetRunningCondition reports on current status of the Azure Scale Set.
	ScaleSetRunningCondition clusterv1.ConditionType = "ScaleSetRunning"
	// ScaleSetCreatingReason used when the scale set creation is in progress.
	ScaleSetCreatingReason = "ScaleSetCreating"
	// ScaleSetUpdatingReason used when the scale set updating is in progress.
	ScaleSetUpdatingReason = "ScaleSetUpdating"
	// ScaleSetDeletingReason used when the scale set is in a deleting state.
	ScaleSetDeletingReason = "ScaleSetDeleting"
	// ScaleSetProvisionFailedReason used for failures during scale set provisioning.
	ScaleSetProvisionFailedReason = "ScaleSetProvisionFailed"

	// ScaleSetDesiredReplicasCondition reports on the scaling state of the machine pool.
	ScaleSetDesiredReplicasCondition clusterv1.ConditionType = "ScaleSetDesiredReplicas"
	// ScaleSetScaleUpReason describes the machine pool scaling up.
	ScaleSetScaleUpReason = "ScaleSetScalingUp"
	// ScaleSetScaleDownReason describes the machine pool scaling down.
	ScaleSetScaleDownReason = "ScaleSetScalingDown"

	// ScaleSetModelUpdatedCondition reports on the model state of the pool.
	ScaleSetModelUpdatedCondition clusterv1.ConditionType = "ScaleSetModelUpdated"
	// ScaleSetModelOutOfDateReason describes the machine pool model being out of date.
	ScaleSetModelOutOfDateReason = "ScaleSetModelOutOfDate"
)
