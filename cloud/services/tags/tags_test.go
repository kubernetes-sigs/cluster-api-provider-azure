/*
Copyright 2020 The Kubernetes Authors.

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

package tags

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-10-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/tags/mock_tags"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileTags(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder)
		expectedError string
	}{
		{
			name:          "create tags",
			expectedError: "",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.TagsSpecs().Return([]azure.TagsSpec{
					{
						Scope: "/sub/123/fake/scope",
						Tags: map[string]string{
							"foo":   "bar",
							"thing": "stuff",
						},
						Annotation: "my-annotation",
					},
					{
						Scope: "/sub/123/other/scope",
						Tags: map[string]string{
							"tag1": "value1",
						},
						Annotation: "my-annotation-2",
					},
				})
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{}, nil)
				s.AnnotationJSON("my-annotation")
				m.CreateOrUpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", resources.TagsResource{
					Properties: &resources.Tags{
						Tags: map[string]*string{
							"foo":   to.StringPtr("bar"),
							"thing": to.StringPtr("stuff"),
						},
					},
				})
				s.UpdateAnnotationJSON("my-annotation", map[string]interface{}{"foo": "bar", "thing": "stuff"})
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/other/scope").Return(resources.TagsResource{}, nil)
				s.AnnotationJSON("my-annotation-2")
				m.CreateOrUpdateAtScope(gomockinternal.AContext(), "/sub/123/other/scope", resources.TagsResource{
					Properties: &resources.Tags{
						Tags: map[string]*string{
							"tag1": to.StringPtr("value1"),
						},
					},
				})
				s.UpdateAnnotationJSON("my-annotation-2", map[string]interface{}{"tag1": "value1"})
			},
		},
		{
			name:          "error getting existing tags",
			expectedError: "failed to get existing tags: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.TagsSpecs().Return([]azure.TagsSpec{
					{
						Scope: "/sub/123/fake/scope",
						Tags: map[string]string{
							"foo":   "bar",
							"thing": "stuff",
						},
						Annotation: "my-annotation",
					},
				})
				s.AnnotationJSON("my-annotation")
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "error updating tags",
			expectedError: "cannot update tags: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.TagsSpecs().Return([]azure.TagsSpec{
					{
						Scope: "/sub/123/fake/scope",
						Tags: map[string]string{
							"key": "value",
						},
						Annotation: "my-annotation",
					},
				})
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{}, nil)
				s.AnnotationJSON("my-annotation")
				m.CreateOrUpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", resources.TagsResource{
					Properties: &resources.Tags{
						Tags: map[string]*string{
							"key": to.StringPtr("value"),
						},
					},
				}).Return(resources.TagsResource{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "tags unchanged",
			expectedError: "",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.TagsSpecs().Return([]azure.TagsSpec{
					{
						Scope: "/sub/123/fake/scope",
						Tags: map[string]string{
							"key": "value",
						},
						Annotation: "my-annotation",
					},
				})
				s.AnnotationJSON("my-annotation").Return(map[string]interface{}{"key": "value"}, nil)
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_tags.NewMockTagScope(mockCtrl)
			clientMock := mock_tags.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

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
			changed, created, deleted, newAnnotation := tagsChanged(test.annotation, test.src)
			g.Expect(changed).To(Equal(test.expectedResult))
			g.Expect(created).To(Equal(test.expectedCreated))
			g.Expect(deleted).To(Equal(test.expectedDeleted))
			g.Expect(newAnnotation).To(Equal(test.expectedNewAnnotations))
		})
	}
}
