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

package v1alpha3

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// DecodeRawExtension will decode a runtime.RawExtension into a specific runtime object based on the schema
func DecodeRawExtension(in *runtime.RawExtension, out runtime.Object) error {
	scheme, err := SchemeBuilder.Build()
	if err != nil {
		return errors.Wrap(err, "Error building schema")
	}

	codecs := serializer.NewCodecFactory(scheme)
	deserializer := codecs.UniversalDeserializer()

	return runtime.DecodeInto(deserializer, in.Raw, out)
}
