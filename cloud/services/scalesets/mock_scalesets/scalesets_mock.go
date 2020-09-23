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
// Source: ../service.go

// Package mock_scalesets is a generated GoMock package.
package mock_scalesets

import (
	context "context"
	autorest "github.com/Azure/go-autorest/autorest"
	logr "github.com/go-logr/logr"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
	v1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// MockScaleSetScope is a mock of ScaleSetScope interface.
type MockScaleSetScope struct {
	ctrl     *gomock.Controller
	recorder *MockScaleSetScopeMockRecorder
}

// MockScaleSetScopeMockRecorder is the mock recorder for MockScaleSetScope.
type MockScaleSetScopeMockRecorder struct {
	mock *MockScaleSetScope
}

// NewMockScaleSetScope creates a new mock instance.
func NewMockScaleSetScope(ctrl *gomock.Controller) *MockScaleSetScope {
	mock := &MockScaleSetScope{ctrl: ctrl}
	mock.recorder = &MockScaleSetScopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockScaleSetScope) EXPECT() *MockScaleSetScopeMockRecorder {
	return m.recorder
}

// SubscriptionID mocks base method.
func (m *MockScaleSetScope) SubscriptionID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscriptionID")
	ret0, _ := ret[0].(string)
	return ret0
}

// SubscriptionID indicates an expected call of SubscriptionID.
func (mr *MockScaleSetScopeMockRecorder) SubscriptionID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscriptionID", reflect.TypeOf((*MockScaleSetScope)(nil).SubscriptionID))
}

// ClientID mocks base method.
func (m *MockScaleSetScope) ClientID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientID indicates an expected call of ClientID.
func (mr *MockScaleSetScopeMockRecorder) ClientID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientID", reflect.TypeOf((*MockScaleSetScope)(nil).ClientID))
}

// ClientSecret mocks base method.
func (m *MockScaleSetScope) ClientSecret() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientSecret")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientSecret indicates an expected call of ClientSecret.
func (mr *MockScaleSetScopeMockRecorder) ClientSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientSecret", reflect.TypeOf((*MockScaleSetScope)(nil).ClientSecret))
}

// CloudEnvironment mocks base method.
func (m *MockScaleSetScope) CloudEnvironment() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudEnvironment")
	ret0, _ := ret[0].(string)
	return ret0
}

// CloudEnvironment indicates an expected call of CloudEnvironment.
func (mr *MockScaleSetScopeMockRecorder) CloudEnvironment() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudEnvironment", reflect.TypeOf((*MockScaleSetScope)(nil).CloudEnvironment))
}

// TenantID mocks base method.
func (m *MockScaleSetScope) TenantID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TenantID")
	ret0, _ := ret[0].(string)
	return ret0
}

// TenantID indicates an expected call of TenantID.
func (mr *MockScaleSetScopeMockRecorder) TenantID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TenantID", reflect.TypeOf((*MockScaleSetScope)(nil).TenantID))
}

// BaseURI mocks base method.
func (m *MockScaleSetScope) BaseURI() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BaseURI")
	ret0, _ := ret[0].(string)
	return ret0
}

// BaseURI indicates an expected call of BaseURI.
func (mr *MockScaleSetScopeMockRecorder) BaseURI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BaseURI", reflect.TypeOf((*MockScaleSetScope)(nil).BaseURI))
}

// Authorizer mocks base method.
func (m *MockScaleSetScope) Authorizer() autorest.Authorizer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authorizer")
	ret0, _ := ret[0].(autorest.Authorizer)
	return ret0
}

// Authorizer indicates an expected call of Authorizer.
func (mr *MockScaleSetScopeMockRecorder) Authorizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authorizer", reflect.TypeOf((*MockScaleSetScope)(nil).Authorizer))
}

// ResourceGroup mocks base method.
func (m *MockScaleSetScope) ResourceGroup() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResourceGroup")
	ret0, _ := ret[0].(string)
	return ret0
}

// ResourceGroup indicates an expected call of ResourceGroup.
func (mr *MockScaleSetScopeMockRecorder) ResourceGroup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResourceGroup", reflect.TypeOf((*MockScaleSetScope)(nil).ResourceGroup))
}

// ClusterName mocks base method.
func (m *MockScaleSetScope) ClusterName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClusterName")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClusterName indicates an expected call of ClusterName.
func (mr *MockScaleSetScopeMockRecorder) ClusterName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClusterName", reflect.TypeOf((*MockScaleSetScope)(nil).ClusterName))
}

// Location mocks base method.
func (m *MockScaleSetScope) Location() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Location")
	ret0, _ := ret[0].(string)
	return ret0
}

// Location indicates an expected call of Location.
func (mr *MockScaleSetScopeMockRecorder) Location() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Location", reflect.TypeOf((*MockScaleSetScope)(nil).Location))
}

// AdditionalTags mocks base method.
func (m *MockScaleSetScope) AdditionalTags() v1alpha3.Tags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AdditionalTags")
	ret0, _ := ret[0].(v1alpha3.Tags)
	return ret0
}

// AdditionalTags indicates an expected call of AdditionalTags.
func (mr *MockScaleSetScopeMockRecorder) AdditionalTags() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AdditionalTags", reflect.TypeOf((*MockScaleSetScope)(nil).AdditionalTags))
}

// Vnet mocks base method.
func (m *MockScaleSetScope) Vnet() *v1alpha3.VnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Vnet")
	ret0, _ := ret[0].(*v1alpha3.VnetSpec)
	return ret0
}

// Vnet indicates an expected call of Vnet.
func (mr *MockScaleSetScopeMockRecorder) Vnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Vnet", reflect.TypeOf((*MockScaleSetScope)(nil).Vnet))
}

// IsVnetManaged mocks base method.
func (m *MockScaleSetScope) IsVnetManaged() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsVnetManaged")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsVnetManaged indicates an expected call of IsVnetManaged.
func (mr *MockScaleSetScopeMockRecorder) IsVnetManaged() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsVnetManaged", reflect.TypeOf((*MockScaleSetScope)(nil).IsVnetManaged))
}

// NodeSubnet mocks base method.
func (m *MockScaleSetScope) NodeSubnet() *v1alpha3.SubnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeSubnet")
	ret0, _ := ret[0].(*v1alpha3.SubnetSpec)
	return ret0
}

// NodeSubnet indicates an expected call of NodeSubnet.
func (mr *MockScaleSetScopeMockRecorder) NodeSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeSubnet", reflect.TypeOf((*MockScaleSetScope)(nil).NodeSubnet))
}

// ControlPlaneSubnet mocks base method.
func (m *MockScaleSetScope) ControlPlaneSubnet() *v1alpha3.SubnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControlPlaneSubnet")
	ret0, _ := ret[0].(*v1alpha3.SubnetSpec)
	return ret0
}

// ControlPlaneSubnet indicates an expected call of ControlPlaneSubnet.
func (mr *MockScaleSetScopeMockRecorder) ControlPlaneSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControlPlaneSubnet", reflect.TypeOf((*MockScaleSetScope)(nil).ControlPlaneSubnet))
}

// RouteTable mocks base method.
func (m *MockScaleSetScope) RouteTable() *v1alpha3.RouteTable {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RouteTable")
	ret0, _ := ret[0].(*v1alpha3.RouteTable)
	return ret0
}

// RouteTable indicates an expected call of RouteTable.
func (mr *MockScaleSetScopeMockRecorder) RouteTable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RouteTable", reflect.TypeOf((*MockScaleSetScope)(nil).RouteTable))
}

// IsIPv6Enabled mocks base method.
func (m *MockScaleSetScope) IsIPv6Enabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsIPv6Enabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsIPv6Enabled indicates an expected call of IsIPv6Enabled.
func (mr *MockScaleSetScopeMockRecorder) IsIPv6Enabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsIPv6Enabled", reflect.TypeOf((*MockScaleSetScope)(nil).IsIPv6Enabled))
}

// APIServerLBName mocks base method.
func (m *MockScaleSetScope) APIServerLBName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "APIServerLBName")
	ret0, _ := ret[0].(string)
	return ret0
}

// APIServerLBName indicates an expected call of APIServerLBName.
func (mr *MockScaleSetScopeMockRecorder) APIServerLBName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "APIServerLBName", reflect.TypeOf((*MockScaleSetScope)(nil).APIServerLBName))
}

// Info mocks base method.
func (m *MockScaleSetScope) Info(msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Info", varargs...)
}

// Info indicates an expected call of Info.
func (mr *MockScaleSetScopeMockRecorder) Info(msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockScaleSetScope)(nil).Info), varargs...)
}

// Enabled mocks base method.
func (m *MockScaleSetScope) Enabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Enabled indicates an expected call of Enabled.
func (mr *MockScaleSetScopeMockRecorder) Enabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enabled", reflect.TypeOf((*MockScaleSetScope)(nil).Enabled))
}

// Error mocks base method.
func (m *MockScaleSetScope) Error(err error, msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{err, msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Error", varargs...)
}

// Error indicates an expected call of Error.
func (mr *MockScaleSetScopeMockRecorder) Error(err, msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{err, msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockScaleSetScope)(nil).Error), varargs...)
}

// V mocks base method.
func (m *MockScaleSetScope) V(level int) logr.InfoLogger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "V", level)
	ret0, _ := ret[0].(logr.InfoLogger)
	return ret0
}

// V indicates an expected call of V.
func (mr *MockScaleSetScopeMockRecorder) V(level interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "V", reflect.TypeOf((*MockScaleSetScope)(nil).V), level)
}

// WithValues mocks base method.
func (m *MockScaleSetScope) WithValues(keysAndValues ...interface{}) logr.Logger {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "WithValues", varargs...)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// WithValues indicates an expected call of WithValues.
func (mr *MockScaleSetScopeMockRecorder) WithValues(keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithValues", reflect.TypeOf((*MockScaleSetScope)(nil).WithValues), keysAndValues...)
}

// WithName mocks base method.
func (m *MockScaleSetScope) WithName(name string) logr.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithName", name)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// WithName indicates an expected call of WithName.
func (mr *MockScaleSetScopeMockRecorder) WithName(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithName", reflect.TypeOf((*MockScaleSetScope)(nil).WithName), name)
}

// ScaleSetSpec mocks base method.
func (m *MockScaleSetScope) ScaleSetSpec() azure.ScaleSetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ScaleSetSpec")
	ret0, _ := ret[0].(azure.ScaleSetSpec)
	return ret0
}

// ScaleSetSpec indicates an expected call of ScaleSetSpec.
func (mr *MockScaleSetScopeMockRecorder) ScaleSetSpec() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScaleSetSpec", reflect.TypeOf((*MockScaleSetScope)(nil).ScaleSetSpec))
}

// GetBootstrapData mocks base method.
func (m *MockScaleSetScope) GetBootstrapData(ctx context.Context) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBootstrapData", ctx)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBootstrapData indicates an expected call of GetBootstrapData.
func (mr *MockScaleSetScopeMockRecorder) GetBootstrapData(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBootstrapData", reflect.TypeOf((*MockScaleSetScope)(nil).GetBootstrapData), ctx)
}

// GetVMImage mocks base method.
func (m *MockScaleSetScope) GetVMImage() (*v1alpha3.Image, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVMImage")
	ret0, _ := ret[0].(*v1alpha3.Image)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVMImage indicates an expected call of GetVMImage.
func (mr *MockScaleSetScopeMockRecorder) GetVMImage() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVMImage", reflect.TypeOf((*MockScaleSetScope)(nil).GetVMImage))
}

// SetAnnotation mocks base method.
func (m *MockScaleSetScope) SetAnnotation(arg0, arg1 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetAnnotation", arg0, arg1)
}

// SetAnnotation indicates an expected call of SetAnnotation.
func (mr *MockScaleSetScopeMockRecorder) SetAnnotation(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAnnotation", reflect.TypeOf((*MockScaleSetScope)(nil).SetAnnotation), arg0, arg1)
}

// SetProviderID mocks base method.
func (m *MockScaleSetScope) SetProviderID(arg0 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetProviderID", arg0)
}

// SetProviderID indicates an expected call of SetProviderID.
func (mr *MockScaleSetScopeMockRecorder) SetProviderID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProviderID", reflect.TypeOf((*MockScaleSetScope)(nil).SetProviderID), arg0)
}

// SetProvisioningState mocks base method.
func (m *MockScaleSetScope) SetProvisioningState(arg0 v1alpha3.VMState) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetProvisioningState", arg0)
}

// SetProvisioningState indicates an expected call of SetProvisioningState.
func (mr *MockScaleSetScopeMockRecorder) SetProvisioningState(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProvisioningState", reflect.TypeOf((*MockScaleSetScope)(nil).SetProvisioningState), arg0)
}
