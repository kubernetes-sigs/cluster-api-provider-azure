/*
Copyright 2023 The Kubernetes Authors.

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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

func TestAzureManagedControlPlane_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		m *AzureManagedControlPlane
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{m: createAzureManagedControlPlaneWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{m: createAzureManagedControlPlaneWithSSHPublicKey("")}

	err := publicKeyExistTest.m.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(*publicKeyExistTest.m.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.m.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(*publicKeyNotExistTest.m.Spec.SSHPublicKey).NotTo(BeEmpty())
}

func createAzureManagedControlPlaneWithSSHPublicKey(sshPublicKey string) *AzureManagedControlPlane {
	return hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey)
}

func hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey string) *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			SSHPublicKey: &sshPublicKey,
		},
	}
}

func TestSetDefaultAutoScalerProfile(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		amcp *AzureManagedControlPlane
	}

	defaultAMP := &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			AutoScalerProfile: &AutoScalerProfile{
				BalanceSimilarNodeGroups:      (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsFalse))),
				Expander:                      (*Expander)(ptr.To(string(ExpanderRandom))),
				MaxEmptyBulkDelete:            ptr.To("10"),
				MaxGracefulTerminationSec:     ptr.To("600"),
				MaxNodeProvisionTime:          ptr.To("15m"),
				MaxTotalUnreadyPercentage:     ptr.To("45"),
				NewPodScaleUpDelay:            ptr.To("0s"),
				OkTotalUnreadyCount:           ptr.To("3"),
				ScanInterval:                  ptr.To("10s"),
				ScaleDownDelayAfterAdd:        ptr.To("10m"),
				ScaleDownDelayAfterDelete:     ptr.To("10s"),
				ScaleDownDelayAfterFailure:    ptr.To("3m"),
				ScaleDownUnneededTime:         ptr.To("10m"),
				ScaleDownUnreadyTime:          ptr.To("20m"),
				ScaleDownUtilizationThreshold: ptr.To("0.5"),
				SkipNodesWithLocalStorage:     (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageFalse))),
				SkipNodesWithSystemPods:       (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsTrue))),
			},
		},
	}

	allFieldsAreNilTest := test{amcp: &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			AutoScalerProfile: &AutoScalerProfile{},
		},
	}}

	allFieldsAreNilTest.amcp.setDefaultAutoScalerProfile()

	g.Expect(allFieldsAreNilTest.amcp.Spec.AutoScalerProfile).To(Equal(defaultAMP.Spec.AutoScalerProfile))

	expectedNotNil := &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			AutoScalerProfile: &AutoScalerProfile{
				BalanceSimilarNodeGroups:      (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsTrue))),
				Expander:                      (*Expander)(ptr.To(string(ExpanderLeastWaste))),
				MaxEmptyBulkDelete:            ptr.To("5"),
				MaxGracefulTerminationSec:     ptr.To("300"),
				MaxNodeProvisionTime:          ptr.To("10m"),
				MaxTotalUnreadyPercentage:     ptr.To("30"),
				NewPodScaleUpDelay:            ptr.To("30s"),
				OkTotalUnreadyCount:           ptr.To("5"),
				ScanInterval:                  ptr.To("20s"),
				ScaleDownDelayAfterAdd:        ptr.To("5m"),
				ScaleDownDelayAfterDelete:     ptr.To("1m"),
				ScaleDownDelayAfterFailure:    ptr.To("2m"),
				ScaleDownUnneededTime:         ptr.To("5m"),
				ScaleDownUnreadyTime:          ptr.To("10m"),
				ScaleDownUtilizationThreshold: ptr.To("0.4"),
				SkipNodesWithLocalStorage:     (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageTrue))),
				SkipNodesWithSystemPods:       (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsFalse))),
			},
		},
	}

	allFieldsAreNotNilTest := test{amcp: &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			AutoScalerProfile: expectedNotNil.Spec.AutoScalerProfile,
		},
	}}

	allFieldsAreNotNilTest.amcp.setDefaultAutoScalerProfile()

	g.Expect(allFieldsAreNotNilTest.amcp.Spec.AutoScalerProfile).To(Equal(expectedNotNil.Spec.AutoScalerProfile))
}
