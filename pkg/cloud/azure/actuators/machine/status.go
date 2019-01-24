/*
Copyright 2018 The Kubernetes Authors.
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

package machine

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resources"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/util"
)

// holds the machine status under an annotation.
// TODO: implement MachineStatus once the API is stable

// Status is an instance of the MachineType custom resource.
type Status *clusterv1.Machine

// AnnotationKey represents the key value of a Kubernetes annotation.
type AnnotationKey string

const (
	// Name is the annotation key for the machine type's name.
	Name AnnotationKey = "azure-name"
	// ResourceGroup is the annotation key for the machine's resource group.
	ResourceGroup AnnotationKey = "azure-rg"
	// InstanceStatus is the annotation key for the machine's instance status.
	InstanceStatus AnnotationKey = "instance-status"
)

func (azure *AzureClient) status(m *clusterv1.Machine) (Status, error) {
	if azure.client == nil {
		return nil, nil
	}
	currentMachine, err := util.GetMachineIfExists(azure.client, m.ObjectMeta.Namespace, m.ObjectMeta.Name)
	if err != nil {
		return nil, err
	}

	if currentMachine == nil {
		return nil, nil
	}
	return azure.machineStatus(currentMachine)
}

func (azure *AzureClient) updateStatus(machine *clusterv1.Machine) error {
	if azure.client == nil {
		return nil
	}
	currentMachine, err := util.GetMachineIfExists(azure.client, machine.ObjectMeta.Namespace, machine.ObjectMeta.Name)
	if err != nil {
		return err
	}

	if currentMachine == nil {
		return fmt.Errorf("machine %v has been deleted. can not update status for machine", machine.ObjectMeta.Name)
	}

	m, err := azure.setMachineStatus(currentMachine, Status(machine))
	if err != nil {
		return err
	}
	return azure.client.Update(context.Background(), m)
}

func (azure *AzureClient) setMachineStatus(machine *clusterv1.Machine, status Status) (*clusterv1.Machine, error) {
	status.ObjectMeta.Annotations[string(InstanceStatus)] = ""

	serializer := json.NewSerializer(json.DefaultMetaFactory, azure.scheme, azure.scheme, false)
	b := []byte{}
	buff := bytes.NewBuffer(b)
	err := serializer.Encode((*clusterv1.Machine)(status), buff)
	if err != nil {
		return nil, fmt.Errorf("encoding failure: %v", err)
	}

	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}
	machine.ObjectMeta.Annotations[string(InstanceStatus)] = buff.String()
	return machine, nil
}

func (azure *AzureClient) updateAnnotations(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if azure.client == nil {
		return nil
	}
	clusterConfig, err := clusterProviderFromProviderSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return fmt.Errorf("error loading cluster provider config: %v", err)
	}

	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}
	// store the name of the azure VM
	machine.ObjectMeta.Annotations[string(Name)] = resources.GetVMName(machine)
	machine.ObjectMeta.Annotations[string(ResourceGroup)] = clusterConfig.ResourceGroup

	err = azure.client.Update(context.Background(), machine)
	if err != nil {
		return err
	}
	return azure.updateStatus(machine)
}

func (azure *AzureClient) machineStatus(machine *clusterv1.Machine) (Status, error) {
	if machine.ObjectMeta.Annotations == nil {
		return nil, nil
	}

	annotation := machine.ObjectMeta.Annotations[string(InstanceStatus)]
	if annotation == "" {
		return nil, nil
	}

	serializer := json.NewSerializer(json.DefaultMetaFactory, azure.scheme, azure.scheme, false)
	var status clusterv1.Machine
	gvk := clusterv1.SchemeGroupVersion.WithKind("Machine")
	_, _, err := serializer.Decode([]byte(annotation), &gvk, &status)
	if err != nil {
		return nil, fmt.Errorf("decoding failure: %v", err)
	}
	return Status(&status), nil
}
