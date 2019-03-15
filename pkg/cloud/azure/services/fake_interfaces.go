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

package services

import (
	"context"
	"fmt"

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

// Get returns fake success.
func (s *FakeSuccessService) Get(ctx context.Context) (interface{}, error) {
	return nil, nil
}

// CreateOrUpdate returns fake success.
func (s *FakeSuccessService) CreateOrUpdate(ctx context.Context) error {
	return nil
}

// Delete returns fake success.
func (s *FakeSuccessService) Delete(ctx context.Context) error {
	return nil
}

// Get returns fake failure.
func (s *FakeFailureService) Get(ctx context.Context) (interface{}, error) {
	return nil, fmt.Errorf("Failed to Get service")
}

// CreateOrUpdate returns fake failure.
func (s *FakeFailureService) CreateOrUpdate(ctx context.Context) error {
	return fmt.Errorf("Failed to Create")
}

// Delete returns fake failure.
func (s *FakeFailureService) Delete(ctx context.Context) error {
	return fmt.Errorf("Failed to Delete")
}

// Get returns fake not found.
func (s *FakeNotFoundService) Get(ctx context.Context) (interface{}, error) {
	return nil, autorest.DetailedError{StatusCode: 404}
}

// CreateOrUpdate returns fake not found.
func (s *FakeNotFoundService) CreateOrUpdate(ctx context.Context) error {
	return autorest.DetailedError{StatusCode: 404}
}

// Delete returns fake not found.
func (s *FakeNotFoundService) Delete(ctx context.Context) error {
	return autorest.DetailedError{StatusCode: 404}
}
