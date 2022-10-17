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

func TestValidateBoolPtrImmutable(t *testing.T) {
	g := NewWithT(t)
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		path           *field.Path
		input1         *bool
		input2         *bool
		expectedOutput *field.Error
	}{
		{
			name:   "nil",
			path:   testPath,
			input1: nil,
			input2: nil,
		},
		{
			name:   "no change",
			path:   testPath,
			input1: nil,
			input2: nil,
		},
		{
			name:           "can't unset",
			path:           testPath,
			input1:         to.BoolPtr(true),
			input2:         nil,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			path:           testPath,
			input1:         nil,
			input2:         to.BoolPtr(true),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			path:           testPath,
			input1:         to.BoolPtr(true),
			input2:         to.BoolPtr(false),
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBoolPtrImmutable(tc.path, tc.input1, tc.input2)
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

func TestValidateStringImmutable(t *testing.T) {
	g := NewWithT(t)
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		path           *field.Path
		input1         string
		input2         string
		expectedOutput *field.Error
	}{
		{
			name:   "empty string",
			path:   testPath,
			input1: "",
			input2: "",
		},
		{
			name:   "no change",
			path:   testPath,
			input1: "",
			input2: "",
		},
		{
			name:           "can't unset",
			path:           testPath,
			input1:         "foo",
			input2:         "",
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			path:           testPath,
			input1:         "",
			input2:         "foo",
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			path:           testPath,
			input1:         "foo",
			input2:         "bar",
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStringImmutable(tc.path, tc.input1, tc.input2)
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

func TestValidateStringPtrImmutable(t *testing.T) {
	g := NewWithT(t)
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		path           *field.Path
		input1         *string
		input2         *string
		expectedOutput *field.Error
	}{
		{
			name:   "nil",
			path:   testPath,
			input1: nil,
			input2: nil,
		},
		{
			name:   "no change",
			path:   testPath,
			input1: to.StringPtr("foo"),
			input2: to.StringPtr("foo"),
		},
		{
			name:           "can't unset",
			path:           testPath,
			input1:         to.StringPtr("foo"),
			input2:         nil,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			path:           testPath,
			input1:         nil,
			input2:         to.StringPtr("foo"),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			path:           testPath,
			input1:         to.StringPtr("foo"),
			input2:         to.StringPtr("bar"),
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStringPtrImmutable(tc.path, tc.input1, tc.input2)
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

func TestValidateInt32PtrImmutable(t *testing.T) {
	g := NewWithT(t)
	testPath := field.NewPath("Spec", "Foo")

	tests := []struct {
		name           string
		path           *field.Path
		input1         *int32
		input2         *int32
		expectedOutput *field.Error
	}{
		{
			name:   "nil",
			path:   testPath,
			input1: nil,
			input2: nil,
		},
		{
			name:   "no change",
			path:   testPath,
			input1: to.Int32Ptr(5),
			input2: to.Int32Ptr(5),
		},
		{
			name:           "can't unset",
			path:           testPath,
			input1:         to.Int32Ptr(5),
			input2:         nil,
			expectedOutput: field.Invalid(testPath, nil, unsetMessage),
		},
		{
			name:           "can't set from empty",
			path:           testPath,
			input1:         nil,
			input2:         to.Int32Ptr(5),
			expectedOutput: field.Invalid(testPath, nil, setMessage),
		},
		{
			name:           "can't change",
			path:           testPath,
			input1:         to.Int32Ptr(5),
			input2:         to.Int32Ptr(6),
			expectedOutput: field.Invalid(testPath, nil, immutableMessage),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInt32PtrImmutable(tc.path, tc.input1, tc.input2)
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
	g := NewWithT(t)

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
			ret := EnsureStringSlicesAreEquivalent(tc.input1, tc.input2)
			g.Expect(ret).To(Equal(tc.expectedOutput))
		})
	}
}
