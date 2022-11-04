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

package webhook

import (
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateImmutableBoolPtr(t *testing.T) {
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		input1         *bool
		input2         *bool
		expectedOutput *field.Error
	}{
		{
			name:   "nil",
			input1: nil,
			input2: nil,
		},
		{
			name:   "no change",
			input1: to.BoolPtr(true),
			input2: to.BoolPtr(true),
		},
		{
			name:           "can't unset",
			input1:         to.BoolPtr(true),
			input2:         nil,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			input1:         nil,
			input2:         to.BoolPtr(true),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         to.BoolPtr(true),
			input2:         to.BoolPtr(false),
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateImmutable(testPath, tc.input1, tc.input2)
			if tc.expectedOutput != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Detail).To(Equal(tc.expectedOutput.Detail))
				g.Expect(err.Type).To(Equal(tc.expectedOutput.Type))
				g.Expect(err.Field).To(Equal(tc.expectedOutput.Field))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestValidateImmutableString(t *testing.T) {
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		input1         string
		input2         string
		expectedOutput *field.Error
	}{
		{
			name:   "empty string",
			input1: "",
			input2: "",
		},
		{
			name:   "no change",
			input1: "foo",
			input2: "foo",
		},
		{
			name:           "can't unset",
			input1:         "foo",
			input2:         "",
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			input1:         "",
			input2:         "foo",
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         "foo",
			input2:         "bar",
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateImmutable(testPath, tc.input1, tc.input2)
			if tc.expectedOutput != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Detail).To(Equal(tc.expectedOutput.Detail))
				g.Expect(err.Type).To(Equal(tc.expectedOutput.Type))
				g.Expect(err.Field).To(Equal(tc.expectedOutput.Field))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestValidateImmutableStringPtr(t *testing.T) {
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		input1         *string
		input2         *string
		expectedOutput *field.Error
	}{
		{
			name:   "nil",
			input1: nil,
			input2: nil,
		},
		{
			name:   "no change",
			input1: to.StringPtr("foo"),
			input2: to.StringPtr("foo"),
		},
		{
			name:           "can't unset",
			input1:         to.StringPtr("foo"),
			input2:         nil,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			input1:         nil,
			input2:         to.StringPtr("foo"),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         to.StringPtr("foo"),
			input2:         to.StringPtr("bar"),
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateImmutable(testPath, tc.input1, tc.input2)
			if tc.expectedOutput != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Detail).To(Equal(tc.expectedOutput.Detail))
				g.Expect(err.Type).To(Equal(tc.expectedOutput.Type))
				g.Expect(err.Field).To(Equal(tc.expectedOutput.Field))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestValidateImmutableInt32(t *testing.T) {
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		input1         int32
		input2         int32
		expectedOutput *field.Error
	}{
		{
			name:   "unset",
			input1: 0,
			input2: 0,
		},
		{
			name:   "no change",
			input1: 5,
			input2: 5,
		},
		{
			name:           "can't unset",
			input1:         5,
			input2:         0,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			input1:         0,
			input2:         5,
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         5,
			input2:         6,
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateImmutable(testPath, tc.input1, tc.input2)
			if tc.expectedOutput != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Detail).To(Equal(tc.expectedOutput.Detail))
				g.Expect(err.Type).To(Equal(tc.expectedOutput.Type))
				g.Expect(err.Field).To(Equal(tc.expectedOutput.Field))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
