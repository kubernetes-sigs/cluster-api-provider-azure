/*
Copyright The Kubernetes Authors.

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

// Code generated by MockGen. DO NOT EDIT.
// Source: ../interfaces.go

// Package mock_aso is a generated GoMock package.
package mock_aso

import (
	context "context"
	reflect "reflect"

	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	gomock "go.uber.org/mock/gomock"
	v1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azure "sigs.k8s.io/cluster-api-provider-azure/azure"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// MockReconciler is a mock of Reconciler interface.
type MockReconciler[T genruntime.MetaObject] struct {
	ctrl     *gomock.Controller
	recorder *MockReconcilerMockRecorder[T]
}

// MockReconcilerMockRecorder is the mock recorder for MockReconciler.
type MockReconcilerMockRecorder[T genruntime.MetaObject] struct {
	mock *MockReconciler[T]
}

// NewMockReconciler creates a new mock instance.
func NewMockReconciler[T genruntime.MetaObject](ctrl *gomock.Controller) *MockReconciler[T] {
	mock := &MockReconciler[T]{ctrl: ctrl}
	mock.recorder = &MockReconcilerMockRecorder[T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockReconciler[T]) EXPECT() *MockReconcilerMockRecorder[T] {
	return m.recorder
}

// CreateOrUpdateResource mocks base method.
func (m *MockReconciler[T]) CreateOrUpdateResource(ctx context.Context, spec azure.ASOResourceSpecGetter[T], serviceName string) (T, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdateResource", ctx, spec, serviceName)
	ret0, _ := ret[0].(T)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOrUpdateResource indicates an expected call of CreateOrUpdateResource.
func (mr *MockReconcilerMockRecorder[T]) CreateOrUpdateResource(ctx, spec, serviceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdateResource", reflect.TypeOf((*MockReconciler[T])(nil).CreateOrUpdateResource), ctx, spec, serviceName)
}

// DeleteResource mocks base method.
func (m *MockReconciler[T]) DeleteResource(ctx context.Context, spec azure.ASOResourceSpecGetter[T], serviceName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteResource", ctx, spec, serviceName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteResource indicates an expected call of DeleteResource.
func (mr *MockReconcilerMockRecorder[T]) DeleteResource(ctx, spec, serviceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteResource", reflect.TypeOf((*MockReconciler[T])(nil).DeleteResource), ctx, spec, serviceName)
}

// MockTagsGetterSetter is a mock of TagsGetterSetter interface.
type MockTagsGetterSetter[T genruntime.MetaObject] struct {
	ctrl     *gomock.Controller
	recorder *MockTagsGetterSetterMockRecorder[T]
}

// MockTagsGetterSetterMockRecorder is the mock recorder for MockTagsGetterSetter.
type MockTagsGetterSetterMockRecorder[T genruntime.MetaObject] struct {
	mock *MockTagsGetterSetter[T]
}

// NewMockTagsGetterSetter creates a new mock instance.
func NewMockTagsGetterSetter[T genruntime.MetaObject](ctrl *gomock.Controller) *MockTagsGetterSetter[T] {
	mock := &MockTagsGetterSetter[T]{ctrl: ctrl}
	mock.recorder = &MockTagsGetterSetterMockRecorder[T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTagsGetterSetter[T]) EXPECT() *MockTagsGetterSetterMockRecorder[T] {
	return m.recorder
}

// GetActualTags mocks base method.
func (m *MockTagsGetterSetter[T]) GetActualTags(resource T) v1beta1.Tags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActualTags", resource)
	ret0, _ := ret[0].(v1beta1.Tags)
	return ret0
}

// GetActualTags indicates an expected call of GetActualTags.
func (mr *MockTagsGetterSetterMockRecorder[T]) GetActualTags(resource interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActualTags", reflect.TypeOf((*MockTagsGetterSetter[T])(nil).GetActualTags), resource)
}

// GetAdditionalTags mocks base method.
func (m *MockTagsGetterSetter[T]) GetAdditionalTags() v1beta1.Tags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAdditionalTags")
	ret0, _ := ret[0].(v1beta1.Tags)
	return ret0
}

// GetAdditionalTags indicates an expected call of GetAdditionalTags.
func (mr *MockTagsGetterSetterMockRecorder[T]) GetAdditionalTags() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAdditionalTags", reflect.TypeOf((*MockTagsGetterSetter[T])(nil).GetAdditionalTags))
}

// GetDesiredTags mocks base method.
func (m *MockTagsGetterSetter[T]) GetDesiredTags(resource T) v1beta1.Tags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDesiredTags", resource)
	ret0, _ := ret[0].(v1beta1.Tags)
	return ret0
}

// GetDesiredTags indicates an expected call of GetDesiredTags.
func (mr *MockTagsGetterSetterMockRecorder[T]) GetDesiredTags(resource interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDesiredTags", reflect.TypeOf((*MockTagsGetterSetter[T])(nil).GetDesiredTags), resource)
}

// SetTags mocks base method.
func (m *MockTagsGetterSetter[T]) SetTags(resource T, tags v1beta1.Tags) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetTags", resource, tags)
}

// SetTags indicates an expected call of SetTags.
func (mr *MockTagsGetterSetterMockRecorder[T]) SetTags(resource, tags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTags", reflect.TypeOf((*MockTagsGetterSetter[T])(nil).SetTags), resource, tags)
}

// MockScope is a mock of Scope interface.
type MockScope struct {
	ctrl     *gomock.Controller
	recorder *MockScopeMockRecorder
}

// MockScopeMockRecorder is the mock recorder for MockScope.
type MockScopeMockRecorder struct {
	mock *MockScope
}

// NewMockScope creates a new mock instance.
func NewMockScope(ctrl *gomock.Controller) *MockScope {
	mock := &MockScope{ctrl: ctrl}
	mock.recorder = &MockScopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockScope) EXPECT() *MockScopeMockRecorder {
	return m.recorder
}

// ClusterName mocks base method.
func (m *MockScope) ClusterName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClusterName")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClusterName indicates an expected call of ClusterName.
func (mr *MockScopeMockRecorder) ClusterName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClusterName", reflect.TypeOf((*MockScope)(nil).ClusterName))
}

// GetClient mocks base method.
func (m *MockScope) GetClient() client.Client {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClient")
	ret0, _ := ret[0].(client.Client)
	return ret0
}

// GetClient indicates an expected call of GetClient.
func (mr *MockScopeMockRecorder) GetClient() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClient", reflect.TypeOf((*MockScope)(nil).GetClient))
}
