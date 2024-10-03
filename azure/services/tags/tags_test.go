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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/tags/mock_tags"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func internalError() *azcore.ResponseError {
	return &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
}

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
					m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(armresources.TagsResource{Properties: &armresources.Tags{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": ptr.To("owned"),
							"externalSystemTag": ptr.To("randomValue"),
						},
					}}, nil),
					s.AnnotationJSON("my-annotation"),
					m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", armresources.TagsPatchResource{
						Operation: ptr.To(armresources.TagsPatchOperationMerge),
						Properties: &armresources.Tags{
							Tags: map[string]*string{
								"foo":   ptr.To("bar"),
								"thing": ptr.To("stuff"),
							},
						},
					}),
					s.UpdateAnnotationJSON("my-annotation", map[string]interface{}{"foo": "bar", "thing": "stuff"}),
					m.GetAtScope(gomockinternal.AContext(), "/sub/123/other/scope").Return(armresources.TagsResource{Properties: &armresources.Tags{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": ptr.To("owned"),
							"externalSystem2Tag": ptr.To("randomValue2"),
						},
					}}, nil),
					s.AnnotationJSON("my-annotation-2"),
					m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/other/scope", armresources.TagsPatchResource{
						Operation: ptr.To(armresources.TagsPatchOperationMerge),
						Properties: &armresources.Tags{
							Tags: map[string]*string{
								"tag1": ptr.To("value1"),
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
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(armresources.TagsResource{}, nil)
			},
		},
		{
			name:          "create tags for managed resource without \"owned\" tag",
			expectedError: "",
			expect: func(s *mock_tags.MockTagScopeMockRecorder, m *mock_tags.MockclientMockRecorder) {
				annotation := azure.ManagedClusterTagsLastAppliedAnnotation
				gomock.InOrder(
					s.ClusterName().AnyTimes().Return("test-cluster"),
					s.TagsSpecs().Return([]azure.TagsSpec{
						{
							Scope: "/sub/123/fake/scope",
							Tags: map[string]string{
								"foo":   "bar",
								"thing": "stuff",
							},
							Annotation: annotation,
						},
					}),
					m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(armresources.TagsResource{}, nil),
					s.AnnotationJSON(annotation),
					m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", armresources.TagsPatchResource{
						Operation: ptr.To(armresources.TagsPatchOperationMerge),
						Properties: &armresources.Tags{
							Tags: map[string]*string{
								"foo":   ptr.To("bar"),
								"thing": ptr.To("stuff"),
							},
						},
					}),
					s.UpdateAnnotationJSON(annotation, map[string]interface{}{"foo": "bar", "thing": "stuff"}),
				)
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
					m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(armresources.TagsResource{Properties: &armresources.Tags{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": ptr.To("owned"),
							"foo":   ptr.To("bar"),
							"thing": ptr.To("stuff"),
						},
					}}, nil),
					s.AnnotationJSON("my-annotation").Return(map[string]interface{}{"foo": "bar", "thing": "stuff"}, nil),
					m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", armresources.TagsPatchResource{
						Operation: ptr.To(armresources.TagsPatchOperationDelete),
						Properties: &armresources.Tags{
							Tags: map[string]*string{
								"thing": ptr.To("stuff"),
							},
						},
					}),
					s.UpdateAnnotationJSON("my-annotation", map[string]interface{}{"foo": "bar"}),
				)
			},
		},
		{
			name:          "error getting existing tags",
			expectedError: "failed to get existing tags:.*#: Internal Server Error: StatusCode=500",
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
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(armresources.TagsResource{}, internalError())
			},
		},
		{
			name:          "error updating tags",
			expectedError: "cannot update tags:.*#: Internal Server Error: StatusCode=500",
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
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(armresources.TagsResource{Properties: &armresources.Tags{
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": ptr.To("owned"),
					},
				}}, nil)
				s.AnnotationJSON("my-annotation")
				m.UpdateAtScope(gomockinternal.AContext(), "/sub/123/fake/scope", armresources.TagsPatchResource{
					Operation: ptr.To(armresources.TagsPatchOperationMerge),
					Properties: &armresources.Tags{
						Tags: map[string]*string{
							"key": ptr.To("value"),
						},
					},
				}).Return(armresources.TagsResource{}, internalError())
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
				m.GetAtScope(gomockinternal.AContext(), "/sub/123/fake/scope").Return(armresources.TagsResource{Properties: &armresources.Tags{
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": ptr.To("owned"),
						"key": ptr.To("value"),
					},
				}}, nil)
				s.AnnotationJSON("my-annotation").Return(map[string]interface{}{"key": "value"}, nil)
				s.UpdateAnnotationJSON("my-annotation", map[string]interface{}{"key": "value"})
			},
		},
	}

	for _, tc := range testcases {
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
				g.Expect(strings.ReplaceAll(err.Error(), "\n", "")).To(MatchRegexp(tc.expectedError))
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
				"foo": ptr.To("hello"),
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
				"foo": ptr.To("hello"),
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
				"foo": ptr.To("hello"),
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
				"foo": ptr.To("hello"),
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
				"foo": ptr.To("hello"),
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
				"foo": ptr.To("hello"),
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
				"foo": ptr.To("hello"),
				"bar": ptr.To("random"),
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
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			changed, createdOrUpdated, deleted, newAnnotation := TagsChanged(test.lastAppliedTags, test.desiredTags, test.currentTags)
			g.Expect(changed).To(Equal(test.expectedResult))
			g.Expect(createdOrUpdated).To(Equal(test.expectedCreatedOrUpdated))
			g.Expect(deleted).To(Equal(test.expectedDeleted))
			g.Expect(newAnnotation).To(Equal(test.expectedNewAnnotations))
		})
	}
}
