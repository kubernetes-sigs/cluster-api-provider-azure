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

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
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
			input1: ptr.To(true),
			input2: ptr.To(true),
		},
		{
			name:           "can't unset",
			input1:         ptr.To(true),
			input2:         nil,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			input1:         nil,
			input2:         ptr.To(true),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         ptr.To(true),
			input2:         ptr.To(false),
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
			input1: ptr.To("foo"),
			input2: ptr.To("foo"),
		},
		{
			name:           "can't unset",
			input1:         ptr.To("foo"),
			input2:         nil,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			input1:         nil,
			input2:         ptr.To("foo"),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         ptr.To("foo"),
			input2:         ptr.To("bar"),
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

func TestEnsureStringSlicesAreEquivalent(t *testing.T) {
	tests := []struct {
		name           string
		input1         []string
		input2         []string
		expectedOutput bool
	}{
		{
			name:           "nil",
			input1:         nil,
			input2:         nil,
			expectedOutput: true,
		},
		{
			name:           "no change",
			input1:         []string{"foo", "bar"},
			input2:         []string{"foo", "bar"},
			expectedOutput: true,
		},
		{
			name:           "different",
			input1:         []string{"foo", "bar"},
			input2:         []string{"foo", "foo"},
			expectedOutput: false,
		},
		{
			name:           "different order, but equal",
			input1:         []string{"1", "2"},
			input2:         []string{"2", "1"},
			expectedOutput: true,
		},
		{
			name:           "different lengths",
			input1:         []string{"foo"},
			input2:         []string{"foo", "foo"},
			expectedOutput: false,
		},
		{
			name:           "different",
			input1:         []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
			input2:         []string{"1", "2", "3", "4", "5", "7", "8", "9"},
			expectedOutput: false,
		},
		{
			name:           "another different variant",
			input1:         []string{"a", "a", "b"},
			input2:         []string{"a", "b", "b"},
			expectedOutput: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ret := EnsureStringSlicesAreEquivalent(tc.input1, tc.input2)
			g.Expect(ret).To(Equal(tc.expectedOutput))
		})
	}
}

func TestValidateZeroTransitionPtr(t *testing.T) {
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
			input1: ptr.To(true),
			input2: ptr.To(true),
		},
		{
			name:   "can unset",
			input1: ptr.To(true),
			input2: nil,
		},
		{
			name:           "can't set from empty",
			input1:         nil,
			input2:         ptr.To(true),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         ptr.To(true),
			input2:         ptr.To(false),
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateZeroTransition(testPath, tc.input1, tc.input2)
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

func TestValidateZeroTransitionString(t *testing.T) {
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
			name:   "can unset",
			input1: "foo",
			input2: "",
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
			err := ValidateZeroTransition(testPath, tc.input1, tc.input2)
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

func TestValidateZeroTransitionStringPtr(t *testing.T) {
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
			input1: ptr.To("foo"),
			input2: ptr.To("foo"),
		},
		{
			name:   "can unset",
			input1: ptr.To("foo"),
			input2: nil,
		},
		{
			name:           "can't set from empty",
			input1:         nil,
			input2:         ptr.To("foo"),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			input1:         ptr.To("foo"),
			input2:         ptr.To("bar"),
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateZeroTransition(testPath, tc.input1, tc.input2)
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

func TestValidateZeroTransitionInt32(t *testing.T) {
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
			name:   "can unset",
			input1: 5,
			input2: 0,
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
			err := ValidateZeroTransition(testPath, tc.input1, tc.input2)
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
