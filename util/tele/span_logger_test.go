/*
Copyright 2024 The Kubernetes Authors.

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

package tele

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
)

func TestSpanLogSinkWithValues(t *testing.T) {
	g := NewGomegaWithT(t)

	var log0 logr.LogSink = &spanLogSink{
		// simulating a slice with cap() > len() where an append() will not create a new array
		vals: make([]interface{}, 0, 4),
	}

	log0 = log0.WithValues("k0", "v0")

	g.Expect(log0.(*spanLogSink).vals).To(HaveExactElements("k0", "v0"))

	log1 := log0.WithValues("k1", "v1")

	g.Expect(log0.(*spanLogSink).vals).To(HaveExactElements("k0", "v0"))
	g.Expect(log1.(*spanLogSink).vals).To(HaveExactElements("k0", "v0", "k1", "v1"))

	log2 := log0.WithValues("k2", "v2")

	g.Expect(log0.(*spanLogSink).vals).To(HaveExactElements("k0", "v0"))
	g.Expect(log1.(*spanLogSink).vals).To(HaveExactElements("k0", "v0", "k1", "v1"))
	g.Expect(log2.(*spanLogSink).vals).To(HaveExactElements("k0", "v0", "k2", "v2"))
}
