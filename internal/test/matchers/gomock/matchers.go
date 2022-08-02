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

package gomock

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega/types"
)

type (
	cmpMatcher struct {
		x    interface{}
		diff string
	}

	errStrEq struct {
		expected string
		actual   string
	}

	contextMatcher struct {
		actual interface{}
	}

	LogMatcher interface {
		types.GomegaMatcher
		WithLevel(int) LogMatcher
		WithLogFunc(string) LogMatcher
	}

	customMatcher struct {
		state    map[string]interface{}
		matcher  func(x interface{}, state map[string]interface{}) bool
		stringer func(state map[string]interface{}) string
	}
)

// DiffEq will verify cmp.Diff(expected, actual) == "" using github.com/google/go-cmp/cmp.
func DiffEq(x interface{}) gomock.Matcher {
	return &cmpMatcher{
		x: x,
	}
}

func (c *cmpMatcher) Matches(x interface{}) bool {
	c.diff = cmp.Diff(x, c.x)
	return c.diff == ""
}

func (c *cmpMatcher) String() string {
	want := fmt.Sprintf("is equal to %v", c.x)
	if c.diff != "" {
		want = fmt.Sprintf("%s, but difference is %s", want, c.diff)
	}
	return want
}

// ErrStrEq will verify the string matches error.Error().
func ErrStrEq(expected string) gomock.Matcher {
	return &errStrEq{
		expected: expected,
	}
}

func (e *errStrEq) Matches(y interface{}) bool {
	err, ok := y.(error)
	if !ok {
		return false
	}
	e.actual = err.Error()
	return e.expected == e.actual
}

func (e *errStrEq) String() string {
	return fmt.Sprintf("error.Error() %q, but got %q", e.expected, e.actual)
}

func AContext() gomock.Matcher {
	return &contextMatcher{}
}

func (e *contextMatcher) Matches(y interface{}) bool {
	_, ok := y.(context.Context)
	e.actual = y
	return ok
}

func (e *contextMatcher) String() string {
	return fmt.Sprintf("expected a context.Context, but got %T", e.actual)
}

// CustomMatcher creates a matcher from two funcs rather than having to make a new struct and implement Matcher.
func CustomMatcher(matcher func(x interface{}, state map[string]interface{}) bool, stringer func(state map[string]interface{}) string) gomock.Matcher {
	return &customMatcher{
		state:    make(map[string]interface{}),
		matcher:  matcher,
		stringer: stringer,
	}
}

func (c customMatcher) Matches(x interface{}) bool {
	return c.matcher(x, c.state)
}

func (c customMatcher) String() string {
	return c.stringer(c.state)
}
