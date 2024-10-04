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

package futures

import (
	"testing"

	. "github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestSet(t *testing.T) {
	testService := "test-service"
	a := fakeFuture("a", testService)
	b := fakeFuture("b", testService)
	newA := a
	newA.Data = "new"

	tests := []struct {
		name   string
		to     Setter
		future *infrav1.Future
		want   infrav1.Futures
	}{
		{
			name:   "Set adds a future",
			to:     setterWithFutures(infrav1.Futures{}),
			future: &a,
			want:   infrav1.Futures{a},
		},
		{
			name:   "Set adds more futures",
			to:     setterWithFutures(infrav1.Futures{a}),
			future: &b,
			want:   infrav1.Futures{a, b},
		},
		{
			name:   "Set does not duplicate existing future",
			to:     setterWithFutures(infrav1.Futures{a, b}),
			future: &a,
			want:   infrav1.Futures{a, b},
		},
		{
			name:   "Set updates an existing future",
			to:     setterWithFutures(infrav1.Futures{a, b}),
			future: &newA,
			want:   infrav1.Futures{newA, b},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			Set(tt.to, tt.future)

			g.Expect(tt.to.GetFutures()).To(Equal(tt.want))
		})
	}
}

func TestDelete(t *testing.T) {
	testService := "test-service"
	a := fakeFuture("a", testService)
	b := fakeFuture("b", testService)
	c := fakeFuture("c", testService)
	d := fakeFuture("d", testService)

	tests := []struct {
		name   string
		to     Setter
		future string
		want   infrav1.Futures
	}{
		{
			name:   "Delete removes a future",
			to:     setterWithFutures(infrav1.Futures{a, b, c, d}),
			future: "b",
			want:   infrav1.Futures{a, c, d},
		},
		{
			name:   "Delete does nothing if the future does not exist",
			to:     setterWithFutures(infrav1.Futures{a}),
			future: "b",
			want:   infrav1.Futures{a},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			Delete(tt.to, tt.future, testService, fakeFutureType)

			g.Expect(tt.to.GetFutures()).To(Equal(tt.want))
		})
	}
}

func setterWithFutures(futures infrav1.Futures) Setter {
	obj := &infrav1.AzureCluster{}
	obj.SetFutures(futures)
	return obj
}
