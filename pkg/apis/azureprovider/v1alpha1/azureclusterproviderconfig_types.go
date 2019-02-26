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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureClusterProviderSpec is the Schema for the azureclusterproviderspecs API
// +k8s:openapi-gen=true
type AzureClusterProviderSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ResourceGroup string `json:"resourceGroup"`
	Location      string `json:"location"`

	// CAKeyPair is the key pair for ca certs.
	CAKeyPair KeyPair `json:"caKeyPair,omitempty"`

	//EtcdCAKeyPair is the key pair for etcd.
	EtcdCAKeyPair KeyPair `json:"etcdCAKeyPair,omitempty"`

	// FrontProxyCAKeyPair is the key pair for FrontProxyKeyPair.
	FrontProxyCAKeyPair KeyPair `json:"frontProxyCAKeyPair,omitempty"`

	// SAKeyPair is the service account key pair.
	SAKeyPair KeyPair `json:"saKeyPair,omitempty"`
}

// KeyPair is how operators can supply custom keypairs for kubeadm to use.
type KeyPair struct {
	// base64 encoded cert and key
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

func init() {
	SchemeBuilder.Register(&AzureClusterProviderSpec{})
}

// HasCertAndKey returns whether a keypair contains cert and key of non-zero length.
func (kp *KeyPair) HasCertAndKey() bool {
	return len(kp.Cert) != 0 && len(kp.Key) != 0
}
