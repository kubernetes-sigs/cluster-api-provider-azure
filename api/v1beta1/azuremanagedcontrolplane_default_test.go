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
	"k8s.io/utils/pointer"
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
				BalanceSimilarNodeGroups:      (*BalanceSimilarNodeGroups)(pointer.String(string(BalanceSimilarNodeGroupsFalse))),
				Expander:                      (*Expander)(pointer.String(string(ExpanderRandom))),
				MaxEmptyBulkDelete:            pointer.String("10"),
				MaxGracefulTerminationSec:     pointer.String("600"),
				MaxNodeProvisionTime:          pointer.String("15m"),
				MaxTotalUnreadyPercentage:     pointer.String("45"),
				NewPodScaleUpDelay:            pointer.String("0s"),
				OkTotalUnreadyCount:           pointer.String("3"),
				ScanInterval:                  pointer.String("10s"),
				ScaleDownDelayAfterAdd:        pointer.String("10m"),
				ScaleDownDelayAfterDelete:     pointer.String("10s"),
				ScaleDownDelayAfterFailure:    pointer.String("3m"),
				ScaleDownUnneededTime:         pointer.String("10m"),
				ScaleDownUnreadyTime:          pointer.String("20m"),
				ScaleDownUtilizationThreshold: pointer.String("0.5"),
				SkipNodesWithLocalStorage:     (*SkipNodesWithLocalStorage)(pointer.String(string(SkipNodesWithLocalStorageFalse))),
				SkipNodesWithSystemPods:       (*SkipNodesWithSystemPods)(pointer.String(string(SkipNodesWithSystemPodsTrue))),
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
				BalanceSimilarNodeGroups:      (*BalanceSimilarNodeGroups)(pointer.String(string(BalanceSimilarNodeGroupsTrue))),
				Expander:                      (*Expander)(pointer.String(string(ExpanderLeastWaste))),
				MaxEmptyBulkDelete:            pointer.String("5"),
				MaxGracefulTerminationSec:     pointer.String("300"),
				MaxNodeProvisionTime:          pointer.String("10m"),
				MaxTotalUnreadyPercentage:     pointer.String("30"),
				NewPodScaleUpDelay:            pointer.String("30s"),
				OkTotalUnreadyCount:           pointer.String("5"),
				ScanInterval:                  pointer.String("20s"),
				ScaleDownDelayAfterAdd:        pointer.String("5m"),
				ScaleDownDelayAfterDelete:     pointer.String("1m"),
				ScaleDownDelayAfterFailure:    pointer.String("2m"),
				ScaleDownUnneededTime:         pointer.String("5m"),
				ScaleDownUnreadyTime:          pointer.String("10m"),
				ScaleDownUtilizationThreshold: pointer.String("0.4"),
				SkipNodesWithLocalStorage:     (*SkipNodesWithLocalStorage)(pointer.String(string(SkipNodesWithLocalStorageTrue))),
				SkipNodesWithSystemPods:       (*SkipNodesWithSystemPods)(pointer.String(string(SkipNodesWithSystemPodsFalse))),
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
