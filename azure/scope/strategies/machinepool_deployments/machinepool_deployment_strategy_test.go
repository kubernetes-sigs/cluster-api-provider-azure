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

package machinepool

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestMachinePoolRollingUpdateStrategy_Type(t *testing.T) {
	g := NewWithT(t)
	strategy := NewMachinePoolDeploymentStrategy(infrav1exp.AzureMachinePoolDeploymentStrategy{
		Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
	})
	g.Expect(strategy.Type()).To(Equal(infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType))
}

func TestMachinePoolRollingUpdateStrategy_Surge(t *testing.T) {
	var (
		two           = intstr.FromInt(2)
		twentyPercent = intstr.FromString("20%")
	)

	tests := []struct {
		name            string
		strategy        Surger
		desiredReplicas int
		want            int
		errStr          string
	}{
		{
			name:     "Strategy is empty",
			strategy: &rollingUpdateStrategy{},
			want:     1,
		},
		{
			name: "MaxSurge is set to 2",
			strategy: &rollingUpdateStrategy{
				MachineRollingUpdateDeployment: infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge: &two,
				},
			},
			want: 2,
		},
		{
			name: "MaxSurge is set to 20% and desiredReplicas is 20",
			strategy: &rollingUpdateStrategy{
				MachineRollingUpdateDeployment: infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge: &twentyPercent,
				},
			},
			desiredReplicas: 20,
			want:            4,
		},
		{
			name: "MaxSurge is set to 20% and desiredReplicas is 21; rounds up",
			strategy: &rollingUpdateStrategy{
				MachineRollingUpdateDeployment: infrav1exp.MachineRollingUpdateDeployment{
					MaxSurge: &twentyPercent,
				},
			},
			desiredReplicas: 21,
			want:            5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, err := tt.strategy.Surge(tt.desiredReplicas)
			if tt.errStr == "" {
				g.Expect(err).To(Succeed())
				g.Expect(got).To(Equal(tt.want))
			} else {
				g.Expect(err).To(MatchError(tt.errStr))
			}
		})
	}
}

func TestMachinePoolScope_maxUnavailable(t *testing.T) {
	var (
		two           = intstr.FromInt(2)
		twentyPercent = intstr.FromString("20%")
	)

	tests := []struct {
		name            string
		strategy        *rollingUpdateStrategy
		desiredReplicas int
		want            int
		errStr          string
	}{
		{
			name:     "Strategy is empty",
			strategy: &rollingUpdateStrategy{},
		},
		{
			name: "MaxUnavailable is nil",
			strategy: &rollingUpdateStrategy{
				MachineRollingUpdateDeployment: infrav1exp.MachineRollingUpdateDeployment{},
			},
			want: 0,
		},
		{
			name: "MaxUnavailable is set to 2",
			strategy: &rollingUpdateStrategy{
				MachineRollingUpdateDeployment: infrav1exp.MachineRollingUpdateDeployment{
					MaxUnavailable: &two,
				},
			},
			want: 2,
		},
		{
			name: "MaxUnavailable is set to 20%",
			strategy: &rollingUpdateStrategy{
				MachineRollingUpdateDeployment: infrav1exp.MachineRollingUpdateDeployment{
					MaxUnavailable: &twentyPercent,
				},
			},
			desiredReplicas: 20,
			want:            4,
		},
		{
			name: "MaxUnavailable is set to 20% and it rounds down",
			strategy: &rollingUpdateStrategy{
				MachineRollingUpdateDeployment: infrav1exp.MachineRollingUpdateDeployment{
					MaxUnavailable: &twentyPercent,
				},
			},
			desiredReplicas: 21,
			want:            4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, err := tt.strategy.maxUnavailable(tt.desiredReplicas)
			if tt.errStr == "" {
				g.Expect(err).To(Succeed())
				g.Expect(got).To(Equal(tt.want))
			} else {
				g.Expect(err).To(MatchError(tt.errStr))
			}
		})
	}
}

func TestMachinePoolRollingUpdateStrategy_SelectMachinesToDelete(t *testing.T) {
	var (
		one              = intstr.FromInt(1)
		two              = intstr.FromInt(2)
		fortyFivePercent = intstr.FromString("45%")
		thirtyPercent    = intstr.FromString("30%")
		succeeded        = infrav1.Succeeded
		baseTime         = time.Now().Add(-24 * time.Hour).Truncate(time.Microsecond)
		deleteTime       = metav1.NewTime(time.Now())
	)

	tests := []struct {
		name            string
		strategy        DeleteSelector
		input           map[string]infrav1exp.AzureMachinePoolMachine
		desiredReplicas int32
		want            types.GomegaMatcher
		errStr          string
	}{
		{
			name:            "should not select machines to delete if less than desired replica count",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{}),
			desiredReplicas: 1,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
			},
			want: Equal([]infrav1exp.AzureMachinePoolMachine{}),
		},
		{
			name:            "if over-provisioned, select a machine with an out-of-date model",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			},
			want: Equal([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			}),
		},
		{
			name:            "if over-provisioned, select a machine with an out-of-date model when using Random Delete Policy",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.RandomDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			},
			want: Equal([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			}),
		},
		{
			name:            "if over-provisioned, select the oldest machine",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			}),
		},
		{
			name:            "if over-provisioned and has delete machine annotation, select machines those first and then by oldest",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour)), HasDeleteMachineAnnotation: true}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour)), HasDeleteMachineAnnotation: true}),
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			}),
		},
		{
			name:            "if over-provisioned, select machines ordered by creation date",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
			}),
		},
		{
			name:            "if over-provisioned and has delete machine annotation, prioritize those machines first over creation date",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour)), HasDeleteMachineAnnotation: true}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour)), HasDeleteMachineAnnotation: true}),
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			}),
		},
		{
			name:            "if over-provisioned, select machines ordered by newest first",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.NewestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
			}),
		},
		{
			name:            "if over-provisioned and has delete machine annotation, select those machines first followed by newest",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.NewestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour)), HasDeleteMachineAnnotation: true}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour)), HasDeleteMachineAnnotation: true}),
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
			}),
		},
		{
			name:            "if over-provisioned but with an equivalent number marked for deletion, nothing to do; this is the case where Azure has not yet caught up to capz",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, DeletionTime: &deleteTime, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, DeletionTime: &deleteTime, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: BeEmpty(),
		},
		{
			name:            "if Azure is deleting 2 machines, but we have already marked their AzureMachinePoolMachine equivalents for deletion, nothing to do; this is the case where capz has not yet caught up to Azure",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: infrav1.Deleting, DeletionTime: &deleteTime, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: infrav1.Deleting, DeletionTime: &deleteTime, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: BeEmpty(),
		},
		{
			name:            "if Azure is deleting 2 machines, we want to delete their AzureMachinePoolMachine equivalents",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: infrav1.Deleting, DeletionTime: nil, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: infrav1.Deleting, DeletionTime: nil, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: infrav1.Deleting, DeletionTime: nil, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: infrav1.Deleting, DeletionTime: nil, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
			}),
		},
		{
			name:            "if Azure is deleting 1 machine, pick another candidate for deletion",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{DeletePolicy: infrav1exp.OldestDeletePolicyType}),
			desiredReplicas: 2,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(4 * time.Hour))}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(3 * time.Hour))}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
				"bar": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, DeletionTime: &deleteTime, CreationTime: metav1.NewTime(baseTime.Add(1 * time.Hour))}),
			},
			want: gomega.DiffEq([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded, CreationTime: metav1.NewTime(baseTime.Add(2 * time.Hour))}),
			}),
		},
		{
			name:            "if maxUnavailable is 1, and 1 is not the latest model, delete it.",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{MaxUnavailable: &one}),
			desiredReplicas: 3,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			},
			want: Equal([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			}),
		},
		{
			name:            "if maxUnavailable is 1, and all are the latest model, delete nothing.",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{MaxUnavailable: &one}),
			desiredReplicas: 3,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
			},
			want: BeEmpty(),
		},
		{
			name:            "if maxUnavailable is 2, and there are 2 with the latest model == false, delete 2.",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{MaxUnavailable: &two}),
			desiredReplicas: 3,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			},
			want: Equal([]infrav1exp.AzureMachinePoolMachine{
				makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
				makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			}),
		},
		{
			name:            "if maxUnavailable is 45%, and there are 2 with the latest model == false, delete 1.",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{MaxUnavailable: &fortyFivePercent}),
			desiredReplicas: 3,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			},
			want: HaveLen(1),
		},
		{
			name:            "if maxUnavailable is 30%, and there are 2 with the latest model == false, delete 0.",
			strategy:        makeRollingUpdateStrategy(infrav1exp.MachineRollingUpdateDeployment{MaxUnavailable: &thirtyPercent}),
			desiredReplicas: 3,
			input: map[string]infrav1exp.AzureMachinePoolMachine{
				"foo": makeAMPM(ampmOptions{Ready: true, LatestModel: true, ProvisioningState: succeeded}),
				"bin": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
				"baz": makeAMPM(ampmOptions{Ready: true, LatestModel: false, ProvisioningState: succeeded}),
			},
			want: BeEmpty(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, err := tt.strategy.SelectMachinesToDelete(context.Background(), tt.desiredReplicas, tt.input)
			if tt.errStr == "" {
				g.Expect(err).To(Succeed())
				g.Expect(got).To(tt.want)
			} else {
				g.Expect(err).To(MatchError(tt.errStr))
			}
		})
	}
}

func makeRollingUpdateStrategy(rolling infrav1exp.MachineRollingUpdateDeployment) *rollingUpdateStrategy {
	return &rollingUpdateStrategy{
		MachineRollingUpdateDeployment: rolling,
	}
}

type ampmOptions struct {
	Ready                      bool
	LatestModel                bool
	ProvisioningState          infrav1.ProvisioningState
	CreationTime               metav1.Time
	DeletionTime               *metav1.Time
	HasDeleteMachineAnnotation bool
}

func makeAMPM(opts ampmOptions) infrav1exp.AzureMachinePoolMachine {
	ampm := infrav1exp.AzureMachinePoolMachine{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: opts.CreationTime,
			DeletionTimestamp: opts.DeletionTime,
			Annotations:       map[string]string{},
		},
		Status: infrav1exp.AzureMachinePoolMachineStatus{
			Ready:              opts.Ready,
			LatestModelApplied: opts.LatestModel,
			ProvisioningState:  &opts.ProvisioningState,
		},
	}

	if opts.HasDeleteMachineAnnotation {
		ampm.Annotations[clusterv1.DeleteMachineAnnotation] = "true"
	}

	return ampm
}
