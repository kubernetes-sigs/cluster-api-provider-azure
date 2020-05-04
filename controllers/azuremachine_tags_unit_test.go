/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestTagsChanged(t *testing.T) {
	g := NewWithT(t)

	var tests = map[string]struct {
		annotation             map[string]interface{}
		src                    map[string]string
		expectedResult         bool
		expectedCreated        map[string]string
		expectedDeleted        map[string]string
		expectedNewAnnotations map[string]interface{}
	}{
		"tags are the same": {
			annotation: map[string]interface{}{
				"foo": "hello",
			},
			src: map[string]string{
				"foo": "hello",
			},
			expectedResult:  false,
			expectedCreated: map[string]string{},
			expectedDeleted: map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "hello",
			},
		}, "tag value changed": {
			annotation: map[string]interface{}{
				"foo": "hello",
			},
			src: map[string]string{
				"foo": "goodbye",
			},
			expectedResult: true,
			expectedCreated: map[string]string{
				"foo": "goodbye",
			},
			expectedDeleted: map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "goodbye",
			},
		}, "tag deleted": {
			annotation: map[string]interface{}{
				"foo": "hello",
			},
			src:             map[string]string{},
			expectedResult:  true,
			expectedCreated: map[string]string{},
			expectedDeleted: map[string]string{
				"foo": "hello",
			},
			expectedNewAnnotations: map[string]interface{}{},
		}, "tag created": {
			annotation: map[string]interface{}{
				"foo": "hello",
			},
			src: map[string]string{
				"foo": "hello",
				"bar": "welcome",
			},
			expectedResult: true,
			expectedCreated: map[string]string{
				"bar": "welcome",
			},
			expectedDeleted: map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "hello",
				"bar": "welcome",
			},
		}, "tag deleted and another created": {
			annotation: map[string]interface{}{
				"foo": "hello",
			},
			src: map[string]string{
				"bar": "welcome",
			},
			expectedResult: true,
			expectedCreated: map[string]string{
				"bar": "welcome",
			},
			expectedDeleted: map[string]string{
				"foo": "hello",
			},
			expectedNewAnnotations: map[string]interface{}{
				"bar": "welcome",
			},
		}}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			changed, created, deleted, newAnnotation := TagsChanged(test.annotation, test.src)
			g.Expect(changed).To(Equal(test.expectedResult))
			g.Expect(created).To(Equal(test.expectedCreated))
			g.Expect(deleted).To(Equal(test.expectedDeleted))
			g.Expect(newAnnotation).To(Equal(test.expectedNewAnnotations))
		})
	}
}
