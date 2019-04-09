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

package azure

import (
	"context"
	"errors"
	"reflect"

	"github.com/Azure/go-autorest/autorest"
)

// FakeSuccessService generic service which always returns success.
type FakeSuccessService struct {
}

// FakeFailureService generic service which always returns failure.
type FakeFailureService struct {
}

// FakeNotFoundService generic service which always returns not found
type FakeNotFoundService struct {
}

// FakeCachedService updates the cache with name whenefver createorupdate is called
type FakeCachedService struct {
	Cache *map[string]int
}

// Get returns fake success.
func (s *FakeSuccessService) Get(ctx context.Context, spec Spec) (interface{}, error) {
	return nil, nil
}

// CreateOrUpdate returns fake success.
func (s *FakeSuccessService) CreateOrUpdate(ctx context.Context, spec Spec) error {
	return nil
}

// Delete returns fake success.
func (s *FakeSuccessService) Delete(ctx context.Context, spec Spec) error {
	return nil
}

// FakeStruct fakes return for Get
type FakeStruct struct {
}

// Get returns fake failure.
func (s *FakeFailureService) Get(ctx context.Context, spec Spec) (interface{}, error) {
	return FakeStruct{}, errors.New("Failed to Get service")
}

// CreateOrUpdate returns fake failure.
func (s *FakeFailureService) CreateOrUpdate(ctx context.Context, spec Spec) error {
	return errors.New("Failed to Create")
}

// Delete returns fake failure.
func (s *FakeFailureService) Delete(ctx context.Context, spec Spec) error {
	return errors.New("Failed to Delete")
}

// Get returns fake not found.
func (s *FakeNotFoundService) Get(ctx context.Context, spec Spec) (interface{}, error) {
	return nil, autorest.DetailedError{StatusCode: 404}
}

// CreateOrUpdate returns fake not found.
func (s *FakeNotFoundService) CreateOrUpdate(ctx context.Context, spec Spec) error {
	return autorest.DetailedError{StatusCode: 404}
}

// Delete returns fake not found.
func (s *FakeNotFoundService) Delete(ctx context.Context, spec Spec) error {
	return autorest.DetailedError{StatusCode: 404}
}

// Get returns fake success.
func (s *FakeCachedService) Get(ctx context.Context, spec Spec) (interface{}, error) {
	return nil, nil
}

// CreateOrUpdate returns fake success.
func (s *FakeCachedService) CreateOrUpdate(ctx context.Context, spec Spec) error {
	if spec == nil {
		return nil
	}
	v := reflect.ValueOf(spec).Elem()
	(*s.Cache)[v.FieldByName("Name").String()]++
	return nil
}

// Delete returns fake success.
func (s *FakeCachedService) Delete(ctx context.Context, spec Spec) error {
	return nil
}
