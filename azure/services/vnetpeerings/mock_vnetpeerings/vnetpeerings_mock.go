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
// Source: ../vnetpeerings.go

// Package mock_vnetpeerings is a generated GoMock package.
package mock_vnetpeerings

import (
	reflect "reflect"

	autorest "github.com/Azure/go-autorest/autorest"
	gomock "github.com/golang/mock/gomock"
	v1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azure "sigs.k8s.io/cluster-api-provider-azure/azure"
	v1beta10 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// MockVnetPeeringScope is a mock of VnetPeeringScope interface.
type MockVnetPeeringScope struct {
	ctrl     *gomock.Controller
	recorder *MockVnetPeeringScopeMockRecorder
}

// MockVnetPeeringScopeMockRecorder is the mock recorder for MockVnetPeeringScope.
type MockVnetPeeringScopeMockRecorder struct {
	mock *MockVnetPeeringScope
}

// NewMockVnetPeeringScope creates a new mock instance.
func NewMockVnetPeeringScope(ctrl *gomock.Controller) *MockVnetPeeringScope {
	mock := &MockVnetPeeringScope{ctrl: ctrl}
	mock.recorder = &MockVnetPeeringScopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVnetPeeringScope) EXPECT() *MockVnetPeeringScopeMockRecorder {
	return m.recorder
}

// Authorizer mocks base method.
func (m *MockVnetPeeringScope) Authorizer() autorest.Authorizer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authorizer")
	ret0, _ := ret[0].(autorest.Authorizer)
	return ret0
}

// Authorizer indicates an expected call of Authorizer.
func (mr *MockVnetPeeringScopeMockRecorder) Authorizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authorizer", reflect.TypeOf((*MockVnetPeeringScope)(nil).Authorizer))
}

// AzureClusterIdentity mocks base method.
func (m *MockVnetPeeringScope) AzureClusterIdentity() *v1beta1.AzureClusterIdentity {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AzureClusterIdentity")
	ret0, _ := ret[0].(*v1beta1.AzureClusterIdentity)
	return ret0
}

// AzureClusterIdentity indicates an expected call of AzureClusterIdentity.
func (mr *MockVnetPeeringScopeMockRecorder) AzureClusterIdentity() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AzureClusterIdentity", reflect.TypeOf((*MockVnetPeeringScope)(nil).AzureClusterIdentity))
}

// BaseURI mocks base method.
func (m *MockVnetPeeringScope) BaseURI() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BaseURI")
	ret0, _ := ret[0].(string)
	return ret0
}

// BaseURI indicates an expected call of BaseURI.
func (mr *MockVnetPeeringScopeMockRecorder) BaseURI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BaseURI", reflect.TypeOf((*MockVnetPeeringScope)(nil).BaseURI))
}

// ClientID mocks base method.
func (m *MockVnetPeeringScope) ClientID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientID indicates an expected call of ClientID.
func (mr *MockVnetPeeringScopeMockRecorder) ClientID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientID", reflect.TypeOf((*MockVnetPeeringScope)(nil).ClientID))
}

// ClientSecret mocks base method.
func (m *MockVnetPeeringScope) ClientSecret() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientSecret")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientSecret indicates an expected call of ClientSecret.
func (mr *MockVnetPeeringScopeMockRecorder) ClientSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientSecret", reflect.TypeOf((*MockVnetPeeringScope)(nil).ClientSecret))
}

// CloudEnvironment mocks base method.
func (m *MockVnetPeeringScope) CloudEnvironment() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudEnvironment")
	ret0, _ := ret[0].(string)
	return ret0
}

// CloudEnvironment indicates an expected call of CloudEnvironment.
func (mr *MockVnetPeeringScopeMockRecorder) CloudEnvironment() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudEnvironment", reflect.TypeOf((*MockVnetPeeringScope)(nil).CloudEnvironment))
}

// DeleteLongRunningOperationState mocks base method.
func (m *MockVnetPeeringScope) DeleteLongRunningOperationState(arg0, arg1, arg2 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeleteLongRunningOperationState", arg0, arg1, arg2)
}

// DeleteLongRunningOperationState indicates an expected call of DeleteLongRunningOperationState.
func (mr *MockVnetPeeringScopeMockRecorder) DeleteLongRunningOperationState(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteLongRunningOperationState", reflect.TypeOf((*MockVnetPeeringScope)(nil).DeleteLongRunningOperationState), arg0, arg1, arg2)
}

// GetLongRunningOperationState mocks base method.
func (m *MockVnetPeeringScope) GetLongRunningOperationState(arg0, arg1, arg2 string) *v1beta1.Future {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLongRunningOperationState", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v1beta1.Future)
	return ret0
}

// GetLongRunningOperationState indicates an expected call of GetLongRunningOperationState.
func (mr *MockVnetPeeringScopeMockRecorder) GetLongRunningOperationState(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLongRunningOperationState", reflect.TypeOf((*MockVnetPeeringScope)(nil).GetLongRunningOperationState), arg0, arg1, arg2)
}

// HashKey mocks base method.
func (m *MockVnetPeeringScope) HashKey() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HashKey")
	ret0, _ := ret[0].(string)
	return ret0
}

// HashKey indicates an expected call of HashKey.
func (mr *MockVnetPeeringScopeMockRecorder) HashKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HashKey", reflect.TypeOf((*MockVnetPeeringScope)(nil).HashKey))
}

// SetLongRunningOperationState mocks base method.
func (m *MockVnetPeeringScope) SetLongRunningOperationState(arg0 *v1beta1.Future) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetLongRunningOperationState", arg0)
}

// SetLongRunningOperationState indicates an expected call of SetLongRunningOperationState.
func (mr *MockVnetPeeringScopeMockRecorder) SetLongRunningOperationState(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetLongRunningOperationState", reflect.TypeOf((*MockVnetPeeringScope)(nil).SetLongRunningOperationState), arg0)
}

// SubscriptionID mocks base method.
func (m *MockVnetPeeringScope) SubscriptionID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscriptionID")
	ret0, _ := ret[0].(string)
	return ret0
}

// SubscriptionID indicates an expected call of SubscriptionID.
func (mr *MockVnetPeeringScopeMockRecorder) SubscriptionID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscriptionID", reflect.TypeOf((*MockVnetPeeringScope)(nil).SubscriptionID))
}

// TenantID mocks base method.
func (m *MockVnetPeeringScope) TenantID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TenantID")
	ret0, _ := ret[0].(string)
	return ret0
}

// TenantID indicates an expected call of TenantID.
func (mr *MockVnetPeeringScopeMockRecorder) TenantID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TenantID", reflect.TypeOf((*MockVnetPeeringScope)(nil).TenantID))
}

// UpdateDeleteStatus mocks base method.
func (m *MockVnetPeeringScope) UpdateDeleteStatus(arg0 v1beta10.ConditionType, arg1 string, arg2 error) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateDeleteStatus", arg0, arg1, arg2)
}

// UpdateDeleteStatus indicates an expected call of UpdateDeleteStatus.
func (mr *MockVnetPeeringScopeMockRecorder) UpdateDeleteStatus(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDeleteStatus", reflect.TypeOf((*MockVnetPeeringScope)(nil).UpdateDeleteStatus), arg0, arg1, arg2)
}

// UpdatePatchStatus mocks base method.
func (m *MockVnetPeeringScope) UpdatePatchStatus(arg0 v1beta10.ConditionType, arg1 string, arg2 error) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdatePatchStatus", arg0, arg1, arg2)
}

// UpdatePatchStatus indicates an expected call of UpdatePatchStatus.
func (mr *MockVnetPeeringScopeMockRecorder) UpdatePatchStatus(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePatchStatus", reflect.TypeOf((*MockVnetPeeringScope)(nil).UpdatePatchStatus), arg0, arg1, arg2)
}

// UpdatePutStatus mocks base method.
func (m *MockVnetPeeringScope) UpdatePutStatus(arg0 v1beta10.ConditionType, arg1 string, arg2 error) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdatePutStatus", arg0, arg1, arg2)
}

// UpdatePutStatus indicates an expected call of UpdatePutStatus.
func (mr *MockVnetPeeringScopeMockRecorder) UpdatePutStatus(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePutStatus", reflect.TypeOf((*MockVnetPeeringScope)(nil).UpdatePutStatus), arg0, arg1, arg2)
}

// VnetPeeringSpecs mocks base method.
func (m *MockVnetPeeringScope) VnetPeeringSpecs() []azure.ResourceSpecGetter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VnetPeeringSpecs")
	ret0, _ := ret[0].([]azure.ResourceSpecGetter)
	return ret0
}

// VnetPeeringSpecs indicates an expected call of VnetPeeringSpecs.
func (mr *MockVnetPeeringScopeMockRecorder) VnetPeeringSpecs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VnetPeeringSpecs", reflect.TypeOf((*MockVnetPeeringScope)(nil).VnetPeeringSpecs))
}
