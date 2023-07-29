/*
Copyright 2023 The Kubernetes Authors.

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

package aso

import (
	"encoding/json"
	"testing"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso/mock_aso"
)

func TestReconcileTags(t *testing.T) {
	tests := []struct {
		name               string
		lastAppliedTags    infrav1.Tags
		existingTags       infrav1.Tags
		additionalTagsSpec infrav1.Tags
		tagsFromParams     infrav1.Tags
		expectedTags       infrav1.Tags
	}{
		{
			name: "tag update",
			lastAppliedTags: infrav1.Tags{
				"oldAdditionalTag": "oldAdditionalVal",
			},
			existingTags: infrav1.Tags{
				"nonAdditionalTag": "nonAdditionalVal",
			},
			additionalTagsSpec: infrav1.Tags{
				"additionalTag": "additionalVal",
			},
			tagsFromParams: infrav1.Tags{
				"paramTag": "paramVal",
			},
			expectedTags: infrav1.Tags{
				"additionalTag":    "additionalVal",
				"nonAdditionalTag": "nonAdditionalVal",
				"paramTag":         "paramVal",
			},
		},
		{
			name: "no tag update needed",
			lastAppliedTags: infrav1.Tags{
				"additionalTag": "additionalVal",
			},
			additionalTagsSpec: infrav1.Tags{
				"additionalTag": "additionalVal",
			},
			expectedTags: infrav1.Tags{
				"additionalTag": "additionalVal",
			},
		},
		{
			name:               "no additional tags",
			lastAppliedTags:    nil,
			additionalTagsSpec: nil,
			expectedTags:       nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			tag := mock_aso.NewMockTagsGetterSetter(mockCtrl)

			lastAppliedTagsJSON, err := json.Marshal(test.lastAppliedTags)
			g.Expect(err).NotTo(HaveOccurred())

			existing := &asoresourcesv1.ResourceGroup{}
			if test.lastAppliedTags != nil {
				existing.SetAnnotations(map[string]string{
					tagsLastAppliedAnnotation: string(lastAppliedTagsJSON),
				})
			}
			tag.EXPECT().GetActualTags(existing).Return(test.existingTags)
			tag.EXPECT().GetAdditionalTags().Return(test.additionalTagsSpec)

			parameters := &asoresourcesv1.ResourceGroup{}
			tag.EXPECT().GetDesiredTags(parameters).Return(test.tagsFromParams)
			tag.EXPECT().SetTags(parameters, test.expectedTags)

			err = reconcileTags(tag, existing, parameters)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(parameters.GetAnnotations()).To(HaveKey(tagsLastAppliedAnnotation))
		})
	}

	t.Run("error unmarshaling last applied tags", func(t *testing.T) {
		g := NewWithT(t)

		mockCtrl := gomock.NewController(t)
		tag := mock_aso.NewMockTagsGetterSetter(mockCtrl)

		existing := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					tagsLastAppliedAnnotation: "{",
				},
			},
		}

		err := reconcileTags(tag, existing, nil)
		g.Expect(err).To(HaveOccurred())
	})
}
