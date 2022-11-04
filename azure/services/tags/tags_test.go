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
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/tags/mock_tags"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileTags(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder)
		expectedError string
	}{
		{
			name:          "create tags for managed resources",
			expectedError: "",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.ClusterName().AnyTimes().Return("test-cluster")
				gomock.InOrder(
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
					}),
					m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{Properties: &resources.Tags{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
							"externalSystemTag": to.StringPtr("randomValue"),
						},
					}}, nil),
					s.AnnotationJSON("my-annotation"),
					m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", resources.TagsPatchResource{
						Operation: "Merge",
						Properties: &resources.Tags{
							Tags: map[string]*string{
								"foo":   to.StringPtr("bar"),
								"thing": to.StringPtr("stuff"),
							},
						},
					}),
					s.UpdateAnnotationJSON("my-annotation", map[string]interface{}{"foo": "bar", "thing": "stuff"}),
					m.GetAtScope(gomockinternal.AContext(), "/sub/123/other/scope").Return(resources.TagsResource{Properties: &resources.Tags{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
							"externalSystem2Tag": to.StringPtr("randomValue2"),
						},
					}}, nil),
					s.AnnotationJSON("my-annotation-2"),
					m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/other/scope", resources.TagsPatchResource{
						Operation: "Merge",
						Properties: &resources.Tags{
							Tags: map[string]*string{
								"tag1": to.StringPtr("value1"),
							},
						},
					}),
					s.UpdateAnnotationJSON("my-annotation-2", map[string]interface{}{"tag1": "value1"}),
				)
			},
		},
		{
			name:          "do not create tags for unmanaged resources",
			expectedError: "",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.ClusterName().AnyTimes().Return("test-cluster")
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
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{}, nil)
			},
		},
		{
			name:          "delete removed tags",
			expectedError: "",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.ClusterName().AnyTimes().Return("test-cluster")
				gomock.InOrder(
					s.TagsSpecs().Return([]azure.TagsSpec{
						{
							Scope: "/sub/123/fake/scope",
							Tags: map[string]string{
								"foo": "bar",
							},
							Annotation: "my-annotation",
						},
					}),
					m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{Properties: &resources.Tags{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
							"foo":   to.StringPtr("bar"),
							"thing": to.StringPtr("stuff"),
						},
					}}, nil),
					s.AnnotationJSON("my-annotation").Return(map[string]interface{}{"foo": "bar", "thing": "stuff"}, nil),
					m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", resources.TagsPatchResource{
						Operation: "Delete",
						Properties: &resources.Tags{
							Tags: map[string]*string{
								"thing": to.StringPtr("stuff"),
							},
						},
					}),
					s.UpdateAnnotationJSON("my-annotation", map[string]interface{}{"foo": "bar"}),
				)
			},
		},
		{
			name:          "error getting existing tags",
			expectedError: "failed to get existing tags: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.ClusterName().AnyTimes().Return("test-cluster")
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
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error"))
			},
		},
		{
			name:          "error updating tags",
			expectedError: "cannot update tags: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.ClusterName().AnyTimes().Return("test-cluster")
				s.TagsSpecs().Return([]azure.TagsSpec{
					{
						Scope: "/sub/123/fake/scope",
						Tags: map[string]string{
							"key": "value",
						},
						Annotation: "my-annotation",
					},
				})
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{Properties: &resources.Tags{
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
					},
				}}, nil)
				s.AnnotationJSON("my-annotation")
				m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", resources.TagsPatchResource{
					Operation: "Merge",
					Properties: &resources.Tags{
						Tags: map[string]*string{
							"key": to.StringPtr("value"),
						},
					},
				}).Return(resources.TagsResource{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error"))
			},
		},
		{
			name:          "tags unchanged",
			expectedError: "",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				s.ClusterName().AnyTimes().Return("test-cluster")
				s.TagsSpecs().Return([]azure.TagsSpec{
					{
						Scope: "/sub/123/fake/scope",
						Tags: map[string]string{
							"key": "value",
						},
						Annotation: "my-annotation",
					},
				})
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(resources.TagsResource{Properties: &resources.Tags{
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
						"key": to.StringPtr("value"),
					},
				}}, nil)
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
		lastAppliedTags          map[string]interface{}
		desiredTags              map[string]string
		currentTags              map[string]*string
		expectedResult           bool
		expectedCreatedOrUpdated map[string]string
		expectedDeleted          map[string]string
		expectedNewAnnotations   map[string]interface{}
	}{
		"tags are the same": {
			lastAppliedTags: map[string]interface{}{
				"foo": "hello",
			},
			desiredTags: map[string]string{
				"foo": "hello",
			},
			currentTags: map[string]*string{
				"foo": to.StringPtr("hello"),
			},
			expectedResult:           false,
			expectedCreatedOrUpdated: map[string]string{},
			expectedDeleted:          map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "hello",
			},
		}, "tag value changed": {
			lastAppliedTags: map[string]interface{}{
				"foo": "hello",
			},
			desiredTags: map[string]string{
				"foo": "goodbye",
			},
			currentTags: map[string]*string{
				"foo": to.StringPtr("hello"),
			},
			expectedResult: true,
			expectedCreatedOrUpdated: map[string]string{
				"foo": "goodbye",
			},
			expectedDeleted: map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "goodbye",
			},
		}, "tag deleted": {
			lastAppliedTags: map[string]interface{}{
				"foo": "hello",
			},
			desiredTags: map[string]string{},
			currentTags: map[string]*string{
				"foo": to.StringPtr("hello"),
			},
			expectedResult:           true,
			expectedCreatedOrUpdated: map[string]string{},
			expectedDeleted: map[string]string{
				"foo": "hello",
			},
			expectedNewAnnotations: map[string]interface{}{},
		}, "tag created": {
			lastAppliedTags: map[string]interface{}{
				"foo": "hello",
			},
			desiredTags: map[string]string{
				"foo": "hello",
				"bar": "welcome",
			},
			currentTags: map[string]*string{
				"foo": to.StringPtr("hello"),
			},
			expectedResult: true,
			expectedCreatedOrUpdated: map[string]string{
				"bar": "welcome",
			},
			expectedDeleted: map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "hello",
				"bar": "welcome",
			},
		}, "tag deleted and another created": {
			lastAppliedTags: map[string]interface{}{
				"foo": "hello",
			},
			desiredTags: map[string]string{
				"bar": "welcome",
			},
			currentTags: map[string]*string{
				"foo": to.StringPtr("hello"),
			},
			expectedResult: true,
			expectedCreatedOrUpdated: map[string]string{
				"bar": "welcome",
			},
			expectedDeleted: map[string]string{
				"foo": "hello",
			},
			expectedNewAnnotations: map[string]interface{}{
				"bar": "welcome",
			},
		},
		"current tags removed by external entity": {
			lastAppliedTags: map[string]interface{}{
				"foo": "hello",
				"bar": "welcome",
			},
			desiredTags: map[string]string{
				"foo": "hello",
				"bar": "welcome",
			},
			currentTags: map[string]*string{
				"foo": to.StringPtr("hello"),
			},
			expectedResult: true,
			expectedCreatedOrUpdated: map[string]string{
				"bar": "welcome",
			},
			expectedDeleted: map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "hello",
				"bar": "welcome",
			},
		},
		"current tags modified by external entity": {
			lastAppliedTags: map[string]interface{}{
				"foo": "hello",
				"bar": "welcome",
			},
			desiredTags: map[string]string{
				"foo": "hello",
				"bar": "welcome",
			},
			currentTags: map[string]*string{
				"foo": to.StringPtr("hello"),
				"bar": to.StringPtr("random"),
			},
			expectedResult: true,
			expectedCreatedOrUpdated: map[string]string{
				"bar": "welcome",
			},
			expectedDeleted: map[string]string{},
			expectedNewAnnotations: map[string]interface{}{
				"foo": "hello",
				"bar": "welcome",
			},
		}}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			changed, createdOrUpdated, deleted, newAnnotation := tagsChanged(test.lastAppliedTags, test.desiredTags, test.currentTags)
			g.Expect(changed).To(Equal(test.expectedResult))
			g.Expect(createdOrUpdated).To(Equal(test.expectedCreatedOrUpdated))
			g.Expect(deleted).To(Equal(test.expectedDeleted))
			g.Expect(newAnnotation).To(Equal(test.expectedNewAnnotations))
		})
	}
}
