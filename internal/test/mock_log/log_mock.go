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
// Source: github.com/go-logr/logr (interfaces: LogSink)
//
// Generated by this command:
//
//	mockgen -destination log_mock.go -package mock_log github.com/go-logr/logr LogSink
//
// Package mock_log is a generated GoMock package.
package mock_log

import (
	reflect "reflect"

	logr "github.com/go-logr/logr"
	gomock "go.uber.org/mock/gomock"
)

// MockLogSink is a mock of LogSink interface.
type MockLogSink struct {
	ctrl     *gomock.Controller
	recorder *MockLogSinkMockRecorder
}

// MockLogSinkMockRecorder is the mock recorder for MockLogSink.
type MockLogSinkMockRecorder struct {
	mock *MockLogSink
}

// NewMockLogSink creates a new mock instance.
func NewMockLogSink(ctrl *gomock.Controller) *MockLogSink {
	mock := &MockLogSink{ctrl: ctrl}
	mock.recorder = &MockLogSinkMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLogSink) EXPECT() *MockLogSinkMockRecorder {
	return m.recorder
}

// Enabled mocks base method.
func (m *MockLogSink) Enabled(arg0 int) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enabled", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Enabled indicates an expected call of Enabled.
func (mr *MockLogSinkMockRecorder) Enabled(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enabled", reflect.TypeOf((*MockLogSink)(nil).Enabled), arg0)
}

// Error mocks base method.
func (m *MockLogSink) Error(arg0 error, arg1 string, arg2 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Error", varargs...)
}

// Error indicates an expected call of Error.
func (mr *MockLogSinkMockRecorder) Error(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockLogSink)(nil).Error), varargs...)
}

// Info mocks base method.
func (m *MockLogSink) Info(arg0 int, arg1 string, arg2 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Info", varargs...)
}

// Info indicates an expected call of Info.
func (mr *MockLogSinkMockRecorder) Info(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockLogSink)(nil).Info), varargs...)
}

// Init mocks base method.
func (m *MockLogSink) Init(arg0 logr.RuntimeInfo) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Init", arg0)
}

// Init indicates an expected call of Init.
func (mr *MockLogSinkMockRecorder) Init(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockLogSink)(nil).Init), arg0)
}

// WithName mocks base method.
func (m *MockLogSink) WithName(arg0 string) logr.LogSink {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithName", arg0)
	ret0, _ := ret[0].(logr.LogSink)
	return ret0
}

// WithName indicates an expected call of WithName.
func (mr *MockLogSinkMockRecorder) WithName(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithName", reflect.TypeOf((*MockLogSink)(nil).WithName), arg0)
}

// WithValues mocks base method.
func (m *MockLogSink) WithValues(arg0 ...any) logr.LogSink {
	m.ctrl.T.Helper()
	varargs := []any{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "WithValues", varargs...)
	ret0, _ := ret[0].(logr.LogSink)
	return ret0
}

// WithValues indicates an expected call of WithValues.
func (mr *MockLogSinkMockRecorder) WithValues(arg0 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithValues", reflect.TypeOf((*MockLogSink)(nil).WithValues), arg0...)
}
