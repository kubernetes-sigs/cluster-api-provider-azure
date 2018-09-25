/*
Copyright 2017 The Kubernetes Authors.

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

package providerconfig

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzureMachineProviderConfig struct {
	metav1.TypeMeta `json:",inline"`
	Roles           []MachineRole `json:"MachineRoles,omitempty"`
	Location        string        `json:"location"`
	VMSize          string        `json:"vmSize"`
	Image           Image         `json:"image"`
	OSDisk          OSDisk        `json:"osDisk"`
	SSHPublicKey    string        `json:"sshPublicKey"`
	SSHPrivateKey   string        `json:"sshPrivateKey"`
}
type MachineRole string

const (
	Master MachineRole = "Master"
	Node   MachineRole = "Node"
)

type Image struct {
	Publisher string `json:"publisher"`
	Offer     string `json:"offer"`
	SKU       string `json:"sku"`
	Version   string `json:"version"`
}

type OSDisk struct {
	OSType      string      `json:"osType"`
	ManagedDisk ManagedDisk `json:"managedDisk"`
	DiskSizeGB  int         `json:"diskSizeGB"`
}

type ManagedDisk struct {
	StorageAccountType string `json:"storageAccountType"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzureClusterProviderConfig struct {
	metav1.TypeMeta `json:",inline"`

	ResourceGroup string `json:"resourceGroup"`
	Location      string `json:"location"`
}
