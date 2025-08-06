//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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

package storageclass

import (
	"context"
	"log"
	"time"

	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

/*
// In-tree azuredisk csi driver sc example
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: managedhdd
provisioner: kubernetes.io/azure-disk
volumeBindingMode: WaitForFirstConsumer
parameters:
  storageaccounttype: Standard_LRS
  kind: Managed
*/

/*
// External azuredisk csi driver sc example
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: managed-csi
provisioner: disk.csi.azure.com
parameters:
  skuName: StandardSSD_LRS  # alias: storageaccounttype, available values: Standard_LRS, Premium_LRS, StandardSSD_LRS, UltraSSD_LRS
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
*/

const (
	scOperationTimeout             = 30 * time.Second
	scOperationSleepBetweenRetries = 3 * time.Second
	AzureDiskProvisioner           = "kubernetes.io/azure-disk"
	OotAzureDiskProvisioner        = "disk.csi.azure.com"
)

// Builder provides a helper interface for building storage class manifest.
type Builder struct {
	sc *storagev1.StorageClass
}

// Create creates a storage class builder manifest.
func Create(scName string) *Builder {
	scBuilder := &Builder{
		sc: &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: AzureDiskProvisioner,
			Parameters: map[string]string{
				"storageaccounttype": "Standard_LRS",
				"kind":               "managed",
			},
		},
	}
	return scBuilder
}

// WithWaitForFirstConsumer sets volume binding on first consumer.
func (d *Builder) WithWaitForFirstConsumer() *Builder {
	volumeBinding := storagev1.VolumeBindingWaitForFirstConsumer
	d.sc.VolumeBindingMode = &volumeBinding
	return d
}

// WithOotProvisionerName sets the correct external azure disk csi driver provisioner name.
func (d *Builder) WithOotProvisionerName() *Builder {
	d.sc.Provisioner = OotAzureDiskProvisioner
	return d
}

// WithOotParameters sets the sc parameters for out of tree csi drivers.
func (d *Builder) WithOotParameters() *Builder {
	d.sc.Parameters = map[string]string{
		"skuName": "StandardSSD_LRS",
	}
	return d
}

// DeployStorageClass creates a storage class on the k8s cluster.
func (d *Builder) DeployStorageClass(ctx context.Context, clientset *kubernetes.Clientset) {
	Eventually(func(g Gomega) {
		_, err := clientset.StorageV1().StorageClasses().Create(ctx, d.sc, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Error trying to deploy storage class %s in namespace %s:%s\n", d.sc.Name, d.sc.ObjectMeta.Namespace, err.Error())
		}
		g.Expect(err).To(HaveOccurred())
	}, scOperationTimeout, scOperationSleepBetweenRetries).Should(Succeed())
}
