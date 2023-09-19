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
// Source: ../roleassignments.go
//
// Generated by this command:
//
//	mockgen -destination roleassignments_mock.go -package mock_roleassignments -source ../roleassignments.go RoleAssignmentScope
//
// Package mock_roleassignments is a generated GoMock package.
package mock_roleassignments

import (
	reflect "reflect"

	azcore "github.com/Azure/azure-sdk-for-go/sdk/azcore"
	autorest "github.com/Azure/go-autorest/autorest"
	gomock "go.uber.org/mock/gomock"
	v1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azure "sigs.k8s.io/cluster-api-provider-azure/azure"
	v1beta10 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// MockRoleAssignmentScope is a mock of RoleAssignmentScope interface.
type MockRoleAssignmentScope struct {
	ctrl     *gomock.Controller
	recorder *MockRoleAssignmentScopeMockRecorder
}

// MockRoleAssignmentScopeMockRecorder is the mock recorder for MockRoleAssignmentScope.
type MockRoleAssignmentScopeMockRecorder struct {
	mock *MockRoleAssignmentScope
}

// NewMockRoleAssignmentScope creates a new mock instance.
func NewMockRoleAssignmentScope(ctrl *gomock.Controller) *MockRoleAssignmentScope {
	mock := &MockRoleAssignmentScope{ctrl: ctrl}
	mock.recorder = &MockRoleAssignmentScopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRoleAssignmentScope) EXPECT() *MockRoleAssignmentScopeMockRecorder {
	return m.recorder
}

// Authorizer mocks base method.
func (m *MockRoleAssignmentScope) Authorizer() autorest.Authorizer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authorizer")
	ret0, _ := ret[0].(autorest.Authorizer)
	return ret0
}

// Authorizer indicates an expected call of Authorizer.
func (mr *MockRoleAssignmentScopeMockRecorder) Authorizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authorizer", reflect.TypeOf((*MockRoleAssignmentScope)(nil).Authorizer))
}

// BaseURI mocks base method.
func (m *MockRoleAssignmentScope) BaseURI() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BaseURI")
	ret0, _ := ret[0].(string)
	return ret0
}

// BaseURI indicates an expected call of BaseURI.
func (mr *MockRoleAssignmentScopeMockRecorder) BaseURI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BaseURI", reflect.TypeOf((*MockRoleAssignmentScope)(nil).BaseURI))
}

// ClientID mocks base method.
func (m *MockRoleAssignmentScope) ClientID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientID indicates an expected call of ClientID.
func (mr *MockRoleAssignmentScopeMockRecorder) ClientID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientID", reflect.TypeOf((*MockRoleAssignmentScope)(nil).ClientID))
}

// ClientSecret mocks base method.
func (m *MockRoleAssignmentScope) ClientSecret() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientSecret")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientSecret indicates an expected call of ClientSecret.
func (mr *MockRoleAssignmentScopeMockRecorder) ClientSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientSecret", reflect.TypeOf((*MockRoleAssignmentScope)(nil).ClientSecret))
}

// CloudEnvironment mocks base method.
func (m *MockRoleAssignmentScope) CloudEnvironment() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudEnvironment")
	ret0, _ := ret[0].(string)
	return ret0
}

// CloudEnvironment indicates an expected call of CloudEnvironment.
func (mr *MockRoleAssignmentScopeMockRecorder) CloudEnvironment() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudEnvironment", reflect.TypeOf((*MockRoleAssignmentScope)(nil).CloudEnvironment))
}

// DeleteLongRunningOperationState mocks base method.
func (m *MockRoleAssignmentScope) DeleteLongRunningOperationState(arg0, arg1, arg2 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeleteLongRunningOperationState", arg0, arg1, arg2)
}

// DeleteLongRunningOperationState indicates an expected call of DeleteLongRunningOperationState.
func (mr *MockRoleAssignmentScopeMockRecorder) DeleteLongRunningOperationState(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteLongRunningOperationState", reflect.TypeOf((*MockRoleAssignmentScope)(nil).DeleteLongRunningOperationState), arg0, arg1, arg2)
}

// GetLongRunningOperationState mocks base method.
func (m *MockRoleAssignmentScope) GetLongRunningOperationState(arg0, arg1, arg2 string) *v1beta1.Future {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLongRunningOperationState", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v1beta1.Future)
	return ret0
}

// GetLongRunningOperationState indicates an expected call of GetLongRunningOperationState.
func (mr *MockRoleAssignmentScopeMockRecorder) GetLongRunningOperationState(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLongRunningOperationState", reflect.TypeOf((*MockRoleAssignmentScope)(nil).GetLongRunningOperationState), arg0, arg1, arg2)
}

// HasSystemAssignedIdentity mocks base method.
func (m *MockRoleAssignmentScope) HasSystemAssignedIdentity() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasSystemAssignedIdentity")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasSystemAssignedIdentity indicates an expected call of HasSystemAssignedIdentity.
func (mr *MockRoleAssignmentScopeMockRecorder) HasSystemAssignedIdentity() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasSystemAssignedIdentity", reflect.TypeOf((*MockRoleAssignmentScope)(nil).HasSystemAssignedIdentity))
}

// HashKey mocks base method.
func (m *MockRoleAssignmentScope) HashKey() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HashKey")
	ret0, _ := ret[0].(string)
	return ret0
}

// HashKey indicates an expected call of HashKey.
func (mr *MockRoleAssignmentScopeMockRecorder) HashKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HashKey", reflect.TypeOf((*MockRoleAssignmentScope)(nil).HashKey))
}

// Name mocks base method.
func (m *MockRoleAssignmentScope) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockRoleAssignmentScopeMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockRoleAssignmentScope)(nil).Name))
}

// ResourceGroup mocks base method.
func (m *MockRoleAssignmentScope) ResourceGroup() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResourceGroup")
	ret0, _ := ret[0].(string)
	return ret0
}

// ResourceGroup indicates an expected call of ResourceGroup.
func (mr *MockRoleAssignmentScopeMockRecorder) ResourceGroup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResourceGroup", reflect.TypeOf((*MockRoleAssignmentScope)(nil).ResourceGroup))
}

// RoleAssignmentResourceType mocks base method.
func (m *MockRoleAssignmentScope) RoleAssignmentResourceType() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RoleAssignmentResourceType")
	ret0, _ := ret[0].(string)
	return ret0
}

// RoleAssignmentResourceType indicates an expected call of RoleAssignmentResourceType.
func (mr *MockRoleAssignmentScopeMockRecorder) RoleAssignmentResourceType() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RoleAssignmentResourceType", reflect.TypeOf((*MockRoleAssignmentScope)(nil).RoleAssignmentResourceType))
}

// RoleAssignmentSpecs mocks base method.
func (m *MockRoleAssignmentScope) RoleAssignmentSpecs(principalID *string) []azure.ResourceSpecGetter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RoleAssignmentSpecs", principalID)
	ret0, _ := ret[0].([]azure.ResourceSpecGetter)
	return ret0
}

// RoleAssignmentSpecs indicates an expected call of RoleAssignmentSpecs.
func (mr *MockRoleAssignmentScopeMockRecorder) RoleAssignmentSpecs(principalID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RoleAssignmentSpecs", reflect.TypeOf((*MockRoleAssignmentScope)(nil).RoleAssignmentSpecs), principalID)
}

// SetLongRunningOperationState mocks base method.
func (m *MockRoleAssignmentScope) SetLongRunningOperationState(arg0 *v1beta1.Future) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetLongRunningOperationState", arg0)
}

// SetLongRunningOperationState indicates an expected call of SetLongRunningOperationState.
func (mr *MockRoleAssignmentScopeMockRecorder) SetLongRunningOperationState(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetLongRunningOperationState", reflect.TypeOf((*MockRoleAssignmentScope)(nil).SetLongRunningOperationState), arg0)
}

// SubscriptionID mocks base method.
func (m *MockRoleAssignmentScope) SubscriptionID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscriptionID")
	ret0, _ := ret[0].(string)
	return ret0
}

// SubscriptionID indicates an expected call of SubscriptionID.
func (mr *MockRoleAssignmentScopeMockRecorder) SubscriptionID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscriptionID", reflect.TypeOf((*MockRoleAssignmentScope)(nil).SubscriptionID))
}

// TenantID mocks base method.
func (m *MockRoleAssignmentScope) TenantID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TenantID")
	ret0, _ := ret[0].(string)
	return ret0
}

// TenantID indicates an expected call of TenantID.
func (mr *MockRoleAssignmentScopeMockRecorder) TenantID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TenantID", reflect.TypeOf((*MockRoleAssignmentScope)(nil).TenantID))
}

// Token mocks base method.
func (m *MockRoleAssignmentScope) Token() azcore.TokenCredential {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Token")
	ret0, _ := ret[0].(azcore.TokenCredential)
	return ret0
}

// Token indicates an expected call of Token.
func (mr *MockRoleAssignmentScopeMockRecorder) Token() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Token", reflect.TypeOf((*MockRoleAssignmentScope)(nil).Token))
}

// UpdateDeleteStatus mocks base method.
func (m *MockRoleAssignmentScope) UpdateDeleteStatus(arg0 v1beta10.ConditionType, arg1 string, arg2 error) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateDeleteStatus", arg0, arg1, arg2)
}

// UpdateDeleteStatus indicates an expected call of UpdateDeleteStatus.
func (mr *MockRoleAssignmentScopeMockRecorder) UpdateDeleteStatus(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDeleteStatus", reflect.TypeOf((*MockRoleAssignmentScope)(nil).UpdateDeleteStatus), arg0, arg1, arg2)
}

// UpdatePatchStatus mocks base method.
func (m *MockRoleAssignmentScope) UpdatePatchStatus(arg0 v1beta10.ConditionType, arg1 string, arg2 error) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdatePatchStatus", arg0, arg1, arg2)
}

// UpdatePatchStatus indicates an expected call of UpdatePatchStatus.
func (mr *MockRoleAssignmentScopeMockRecorder) UpdatePatchStatus(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePatchStatus", reflect.TypeOf((*MockRoleAssignmentScope)(nil).UpdatePatchStatus), arg0, arg1, arg2)
}

// UpdatePutStatus mocks base method.
func (m *MockRoleAssignmentScope) UpdatePutStatus(arg0 v1beta10.ConditionType, arg1 string, arg2 error) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdatePutStatus", arg0, arg1, arg2)
}

// UpdatePutStatus indicates an expected call of UpdatePutStatus.
func (mr *MockRoleAssignmentScopeMockRecorder) UpdatePutStatus(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePutStatus", reflect.TypeOf((*MockRoleAssignmentScope)(nil).UpdatePutStatus), arg0, arg1, arg2)
}
