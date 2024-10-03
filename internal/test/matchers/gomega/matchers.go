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

package gomega

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/types"

	"sigs.k8s.io/cluster-api-provider-azure/internal/test/record"
)

type (
	logEntryMatcher struct {
		level   *int
		logFunc *string
		values  []interface{}
	}

	// LogMatcher is a Gomega matcher for logs.
	LogMatcher interface {
		types.GomegaMatcher
		WithLevel(int) LogMatcher
		WithLogFunc(string) LogMatcher
	}

	cmpMatcher struct {
		x    interface{}
		diff string
	}
)

// DiffEq will verify cmp.Diff(expected, actual) == "" using github.com/google/go-cmp/cmp.
func DiffEq(x interface{}) types.GomegaMatcher {
	return &cmpMatcher{
		x: x,
	}
}

// Match returns whether the actual value matches the expected value.
func (c *cmpMatcher) Match(actual interface{}) (bool, error) {
	c.diff = cmp.Diff(actual, c.x)
	return c.diff == "", nil
}

// FailWithMessage returns the matcher's diff as the failure message.
func (c *cmpMatcher) FailureMessage(_ interface{}) string {
	return c.diff
}

// NegatedFailureMessage return the matcher's diff as the negated failure message.
func (c *cmpMatcher) NegatedFailureMessage(_ interface{}) string {
	return c.diff
}

// LogContains verifies that LogEntry matches the specified values.
func LogContains(values ...interface{}) LogMatcher {
	return &logEntryMatcher{
		values: values,
	}
}

// WithLevel sets the log level to that specified.
func (l *logEntryMatcher) WithLevel(level int) LogMatcher {
	l.level = &level
	return l
}

// WithLogFunc sets the log function to that specified.
func (l *logEntryMatcher) WithLogFunc(logFunc string) LogMatcher {
	l.logFunc = &logFunc
	return l
}

// Match returns whether the actual value matches the expected value.
func (l *logEntryMatcher) Match(actual interface{}) (bool, error) {
	logEntry, ok := actual.(record.LogEntry)
	if !ok {
		return false, fmt.Errorf("LogContains matcher expects an record.LogEntry")
	}
	return len(l.validate(logEntry)) == 0, nil
}

// FailureMessage returns the specified value as a failure message.
func (l *logEntryMatcher) FailureMessage(actual interface{}) string {
	return failMessage(l.validate(actual))
}

// NegatedFailureMessage returns the specified value as a negated failure message.
func (l *logEntryMatcher) NegatedFailureMessage(actual interface{}) string {
	return failMessage(l.validate(actual))
}

func (l *logEntryMatcher) validate(actual interface{}) []error {
	logEntry, ok := actual.(record.LogEntry)
	if !ok {
		return []error{fmt.Errorf("expected record.LogEntry, but got %T", actual)}
	}

	var errs []error
	containsValues := matchers.ContainElementsMatcher{Elements: l.values}
	ok, err := containsValues.Match(logEntry.Values)
	if err != nil || !ok {
		errs = append(errs, fmt.Errorf("actual log values %q didn't match expected %q", logEntry.Values, l.values))
	}

	if l.logFunc != nil && *l.logFunc != logEntry.LogFunc {
		errs = append(errs, fmt.Errorf("actual log Func %q didn't match expected %q", logEntry.LogFunc, *l.logFunc))
	}

	if l.level != nil && *l.level != logEntry.Level {
		errs = append(errs, fmt.Errorf("actual log level %q didn't match expected %q", logEntry.Level, *l.level))
	}

	return errs
}

func failMessage(errs []error) string {
	errMsgs := make([]string, len(errs))
	for i, err := range errs {
		errMsgs[i] = err.Error()
	}
	return fmt.Sprintf("LogEntry errors: %s", strings.Join(errMsgs, ", "))
}
