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
// Source: ../routetables.go

// Package mock_routetables is a generated GoMock package.
package mock_routetables

import (
	reflect "reflect"

	autorest "github.com/Azure/go-autorest/autorest"
	logr "github.com/go-logr/logr"
	gomock "github.com/golang/mock/gomock"
	v1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	azure "sigs.k8s.io/cluster-api-provider-azure/azure"
)

// MockRouteTableScope is a mock of RouteTableScope interface.
type MockRouteTableScope struct {
	ctrl     *gomock.Controller
	recorder *MockRouteTableScopeMockRecorder
}

// MockRouteTableScopeMockRecorder is the mock recorder for MockRouteTableScope.
type MockRouteTableScopeMockRecorder struct {
	mock *MockRouteTableScope
}

// NewMockRouteTableScope creates a new mock instance.
func NewMockRouteTableScope(ctrl *gomock.Controller) *MockRouteTableScope {
	mock := &MockRouteTableScope{ctrl: ctrl}
	mock.recorder = &MockRouteTableScopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRouteTableScope) EXPECT() *MockRouteTableScopeMockRecorder {
	return m.recorder
}

// APIServerLBName mocks base method.
func (m *MockRouteTableScope) APIServerLBName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "APIServerLBName")
	ret0, _ := ret[0].(string)
	return ret0
}

// APIServerLBName indicates an expected call of APIServerLBName.
func (mr *MockRouteTableScopeMockRecorder) APIServerLBName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "APIServerLBName", reflect.TypeOf((*MockRouteTableScope)(nil).APIServerLBName))
}

// APIServerLBPoolName mocks base method.
func (m *MockRouteTableScope) APIServerLBPoolName(arg0 string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "APIServerLBPoolName", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// APIServerLBPoolName indicates an expected call of APIServerLBPoolName.
func (mr *MockRouteTableScopeMockRecorder) APIServerLBPoolName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "APIServerLBPoolName", reflect.TypeOf((*MockRouteTableScope)(nil).APIServerLBPoolName), arg0)
}

// AdditionalTags mocks base method.
func (m *MockRouteTableScope) AdditionalTags() v1alpha4.Tags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AdditionalTags")
	ret0, _ := ret[0].(v1alpha4.Tags)
	return ret0
}

// AdditionalTags indicates an expected call of AdditionalTags.
func (mr *MockRouteTableScopeMockRecorder) AdditionalTags() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AdditionalTags", reflect.TypeOf((*MockRouteTableScope)(nil).AdditionalTags))
}

// Authorizer mocks base method.
func (m *MockRouteTableScope) Authorizer() autorest.Authorizer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authorizer")
	ret0, _ := ret[0].(autorest.Authorizer)
	return ret0
}

// Authorizer indicates an expected call of Authorizer.
func (mr *MockRouteTableScopeMockRecorder) Authorizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authorizer", reflect.TypeOf((*MockRouteTableScope)(nil).Authorizer))
}

// AvailabilitySetEnabled mocks base method.
func (m *MockRouteTableScope) AvailabilitySetEnabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AvailabilitySetEnabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// AvailabilitySetEnabled indicates an expected call of AvailabilitySetEnabled.
func (mr *MockRouteTableScopeMockRecorder) AvailabilitySetEnabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AvailabilitySetEnabled", reflect.TypeOf((*MockRouteTableScope)(nil).AvailabilitySetEnabled))
}

// BaseURI mocks base method.
func (m *MockRouteTableScope) BaseURI() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BaseURI")
	ret0, _ := ret[0].(string)
	return ret0
}

// BaseURI indicates an expected call of BaseURI.
func (mr *MockRouteTableScopeMockRecorder) BaseURI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BaseURI", reflect.TypeOf((*MockRouteTableScope)(nil).BaseURI))
}

// ClientID mocks base method.
func (m *MockRouteTableScope) ClientID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientID indicates an expected call of ClientID.
func (mr *MockRouteTableScopeMockRecorder) ClientID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientID", reflect.TypeOf((*MockRouteTableScope)(nil).ClientID))
}

// ClientSecret mocks base method.
func (m *MockRouteTableScope) ClientSecret() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientSecret")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientSecret indicates an expected call of ClientSecret.
func (mr *MockRouteTableScopeMockRecorder) ClientSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientSecret", reflect.TypeOf((*MockRouteTableScope)(nil).ClientSecret))
}

// CloudEnvironment mocks base method.
func (m *MockRouteTableScope) CloudEnvironment() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudEnvironment")
	ret0, _ := ret[0].(string)
	return ret0
}

// CloudEnvironment indicates an expected call of CloudEnvironment.
func (mr *MockRouteTableScopeMockRecorder) CloudEnvironment() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudEnvironment", reflect.TypeOf((*MockRouteTableScope)(nil).CloudEnvironment))
}

// ClusterName mocks base method.
func (m *MockRouteTableScope) ClusterName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClusterName")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClusterName indicates an expected call of ClusterName.
func (mr *MockRouteTableScopeMockRecorder) ClusterName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClusterName", reflect.TypeOf((*MockRouteTableScope)(nil).ClusterName))
}

// ControlPlaneRouteTable mocks base method.
func (m *MockRouteTableScope) ControlPlaneRouteTable() v1alpha4.RouteTable {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControlPlaneRouteTable")
	ret0, _ := ret[0].(v1alpha4.RouteTable)
	return ret0
}

// ControlPlaneRouteTable indicates an expected call of ControlPlaneRouteTable.
func (mr *MockRouteTableScopeMockRecorder) ControlPlaneRouteTable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControlPlaneRouteTable", reflect.TypeOf((*MockRouteTableScope)(nil).ControlPlaneRouteTable))
}

// ControlPlaneSubnet mocks base method.
func (m *MockRouteTableScope) ControlPlaneSubnet() (string, v1alpha4.SubnetSpec) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControlPlaneSubnet")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(v1alpha4.SubnetSpec)
	return ret0, ret1
}

// ControlPlaneSubnet indicates an expected call of ControlPlaneSubnet.
func (mr *MockRouteTableScopeMockRecorder) ControlPlaneSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControlPlaneSubnet", reflect.TypeOf((*MockRouteTableScope)(nil).ControlPlaneSubnet))
}

// Enabled mocks base method.
func (m *MockRouteTableScope) Enabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Enabled indicates an expected call of Enabled.
func (mr *MockRouteTableScopeMockRecorder) Enabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enabled", reflect.TypeOf((*MockRouteTableScope)(nil).Enabled))
}

// Error mocks base method.
func (m *MockRouteTableScope) Error(err error, msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{err, msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Error", varargs...)
}

// Error indicates an expected call of Error.
func (mr *MockRouteTableScopeMockRecorder) Error(err, msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{err, msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockRouteTableScope)(nil).Error), varargs...)
}

// HashKey mocks base method.
func (m *MockRouteTableScope) HashKey() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HashKey")
	ret0, _ := ret[0].(string)
	return ret0
}

// HashKey indicates an expected call of HashKey.
func (mr *MockRouteTableScopeMockRecorder) HashKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HashKey", reflect.TypeOf((*MockRouteTableScope)(nil).HashKey))
}

// Info mocks base method.
func (m *MockRouteTableScope) Info(msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Info", varargs...)
}

// Info indicates an expected call of Info.
func (mr *MockRouteTableScopeMockRecorder) Info(msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockRouteTableScope)(nil).Info), varargs...)
}

// IsAPIServerPrivate mocks base method.
func (m *MockRouteTableScope) IsAPIServerPrivate() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsAPIServerPrivate")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsAPIServerPrivate indicates an expected call of IsAPIServerPrivate.
func (mr *MockRouteTableScopeMockRecorder) IsAPIServerPrivate() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsAPIServerPrivate", reflect.TypeOf((*MockRouteTableScope)(nil).IsAPIServerPrivate))
}

// IsIPv6Enabled mocks base method.
func (m *MockRouteTableScope) IsIPv6Enabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsIPv6Enabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsIPv6Enabled indicates an expected call of IsIPv6Enabled.
func (mr *MockRouteTableScopeMockRecorder) IsIPv6Enabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsIPv6Enabled", reflect.TypeOf((*MockRouteTableScope)(nil).IsIPv6Enabled))
}

// IsVnetManaged mocks base method.
func (m *MockRouteTableScope) IsVnetManaged() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsVnetManaged")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsVnetManaged indicates an expected call of IsVnetManaged.
func (mr *MockRouteTableScopeMockRecorder) IsVnetManaged() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsVnetManaged", reflect.TypeOf((*MockRouteTableScope)(nil).IsVnetManaged))
}

// Location mocks base method.
func (m *MockRouteTableScope) Location() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Location")
	ret0, _ := ret[0].(string)
	return ret0
}

// Location indicates an expected call of Location.
func (mr *MockRouteTableScopeMockRecorder) Location() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Location", reflect.TypeOf((*MockRouteTableScope)(nil).Location))
}

// NodeRouteTable mocks base method.
func (m *MockRouteTableScope) NodeRouteTable() v1alpha4.RouteTable {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeRouteTable")
	ret0, _ := ret[0].(v1alpha4.RouteTable)
	return ret0
}

// NodeRouteTable indicates an expected call of NodeRouteTable.
func (mr *MockRouteTableScopeMockRecorder) NodeRouteTable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeRouteTable", reflect.TypeOf((*MockRouteTableScope)(nil).NodeRouteTable))
}

// NodeSubnet mocks base method.
func (m *MockRouteTableScope) NodeSubnet() (string, v1alpha4.SubnetSpec) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeSubnet")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(v1alpha4.SubnetSpec)
	return ret0, ret1
}

// NodeSubnet indicates an expected call of NodeSubnet.
func (mr *MockRouteTableScopeMockRecorder) NodeSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeSubnet", reflect.TypeOf((*MockRouteTableScope)(nil).NodeSubnet))
}

// OutboundLBName mocks base method.
func (m *MockRouteTableScope) OutboundLBName(arg0 string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OutboundLBName", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// OutboundLBName indicates an expected call of OutboundLBName.
func (mr *MockRouteTableScopeMockRecorder) OutboundLBName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OutboundLBName", reflect.TypeOf((*MockRouteTableScope)(nil).OutboundLBName), arg0)
}

// OutboundPoolName mocks base method.
func (m *MockRouteTableScope) OutboundPoolName(arg0 string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OutboundPoolName", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// OutboundPoolName indicates an expected call of OutboundPoolName.
func (mr *MockRouteTableScopeMockRecorder) OutboundPoolName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OutboundPoolName", reflect.TypeOf((*MockRouteTableScope)(nil).OutboundPoolName), arg0)
}

// ResourceGroup mocks base method.
func (m *MockRouteTableScope) ResourceGroup() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResourceGroup")
	ret0, _ := ret[0].(string)
	return ret0
}

// ResourceGroup indicates an expected call of ResourceGroup.
func (mr *MockRouteTableScopeMockRecorder) ResourceGroup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResourceGroup", reflect.TypeOf((*MockRouteTableScope)(nil).ResourceGroup))
}

// RouteTableSpecs mocks base method.
func (m *MockRouteTableScope) RouteTableSpecs() []azure.RouteTableSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RouteTableSpecs")
	ret0, _ := ret[0].([]azure.RouteTableSpec)
	return ret0
}

// RouteTableSpecs indicates an expected call of RouteTableSpecs.
func (mr *MockRouteTableScopeMockRecorder) RouteTableSpecs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RouteTableSpecs", reflect.TypeOf((*MockRouteTableScope)(nil).RouteTableSpecs))
}

// SetSubnet mocks base method.
func (m *MockRouteTableScope) SetSubnet(arg0 string, arg1 v1alpha4.SubnetSpec) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetSubnet", arg0, arg1)
}

// SetSubnet indicates an expected call of SetSubnet.
func (mr *MockRouteTableScopeMockRecorder) SetSubnet(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSubnet", reflect.TypeOf((*MockRouteTableScope)(nil).SetSubnet), arg0, arg1)
}

// Subnet mocks base method.
func (m *MockRouteTableScope) Subnet(arg0 string) v1alpha4.SubnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subnet", arg0)
	ret0, _ := ret[0].(v1alpha4.SubnetSpec)
	return ret0
}

// Subnet indicates an expected call of Subnet.
func (mr *MockRouteTableScopeMockRecorder) Subnet(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subnet", reflect.TypeOf((*MockRouteTableScope)(nil).Subnet), arg0)
}

// SubscriptionID mocks base method.
func (m *MockRouteTableScope) SubscriptionID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscriptionID")
	ret0, _ := ret[0].(string)
	return ret0
}

// SubscriptionID indicates an expected call of SubscriptionID.
func (mr *MockRouteTableScopeMockRecorder) SubscriptionID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscriptionID", reflect.TypeOf((*MockRouteTableScope)(nil).SubscriptionID))
}

// TenantID mocks base method.
func (m *MockRouteTableScope) TenantID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TenantID")
	ret0, _ := ret[0].(string)
	return ret0
}

// TenantID indicates an expected call of TenantID.
func (mr *MockRouteTableScopeMockRecorder) TenantID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TenantID", reflect.TypeOf((*MockRouteTableScope)(nil).TenantID))
}

// V mocks base method.
func (m *MockRouteTableScope) V(level int) logr.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "V", level)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// V indicates an expected call of V.
func (mr *MockRouteTableScopeMockRecorder) V(level interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "V", reflect.TypeOf((*MockRouteTableScope)(nil).V), level)
}

// Vnet mocks base method.
func (m *MockRouteTableScope) Vnet() *v1alpha4.VnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Vnet")
	ret0, _ := ret[0].(*v1alpha4.VnetSpec)
	return ret0
}

// Vnet indicates an expected call of Vnet.
func (mr *MockRouteTableScopeMockRecorder) Vnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Vnet", reflect.TypeOf((*MockRouteTableScope)(nil).Vnet))
}

// WithName mocks base method.
func (m *MockRouteTableScope) WithName(name string) logr.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithName", name)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// WithName indicates an expected call of WithName.
func (mr *MockRouteTableScopeMockRecorder) WithName(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithName", reflect.TypeOf((*MockRouteTableScope)(nil).WithName), name)
}

// WithValues mocks base method.
func (m *MockRouteTableScope) WithValues(keysAndValues ...interface{}) logr.Logger {
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
func (mr *MockRouteTableScopeMockRecorder) WithValues(keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithValues", reflect.TypeOf((*MockRouteTableScope)(nil).WithValues), keysAndValues...)
}
