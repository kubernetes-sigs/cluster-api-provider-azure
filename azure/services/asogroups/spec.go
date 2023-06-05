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

package asogroups

import (
	"context"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// GroupSpec defines the specification for a Resource Group.
type GroupSpec struct {
	Name           string
	Namespace      string
	Location       string
	ClusterName    string
	AdditionalTags infrav1.Tags
	Owner          metav1.OwnerReference
}

// ResourceRef implements aso.ResourceSpecGetter.
func (s *GroupSpec) ResourceRef() genruntime.MetaObject {
	return &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
		},
	}
}

// Parameters implements aso.ResourceSpecGetter.
func (s *GroupSpec) Parameters(ctx context.Context, object genruntime.MetaObject) (genruntime.MetaObject, error) {
	if object != nil {
		return nil, nil
	}

	return &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{s.Owner},
		},
		Spec: asoresourcesv1.ResourceGroup_Spec{
			Location: pointer.String(s.Location),
			Tags: infrav1.Build(infrav1.BuildParams{
				ClusterName: s.ClusterName,
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Name:        pointer.String(s.Name),
				Role:        pointer.String(infrav1.CommonRole),
				Additional:  s.AdditionalTags,
			}),
		},
	}, nil
}
