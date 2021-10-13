/*
Copyright 2021 The Kubernetes Authors.

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

package tele

import (
	"github.com/go-logr/logr"
)

type compositeLogger struct {
	loggers []logr.Logger
}

func (c *compositeLogger) Enabled() bool {
	for _, l := range c.loggers {
		if !l.Enabled() {
			return false
		}
	}
	return true
}

func (c *compositeLogger) iter(fn func(l logr.Logger)) {
	for _, l := range c.loggers {
		fn(l)
	}
}

func (c *compositeLogger) Info(msg string, keysAndValues ...interface{}) {
	c.iter(func(l logr.Logger) {
		l.Info(msg, keysAndValues...)
	})
}

func (c *compositeLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	c.iter(func(l logr.Logger) {
		l.Error(err, msg, keysAndValues...)
	})
}

func (c *compositeLogger) V(level int) logr.Logger {
	return c
}

func (c *compositeLogger) WithValues(keysAndValues ...interface{}) logr.Logger {
	for i, l := range c.loggers {
		c.loggers[i] = l.WithValues(keysAndValues...)
	}
	return c
}

func (c *compositeLogger) WithName(name string) logr.Logger {
	for i, l := range c.loggers {
		c.loggers[i] = l.WithName(name)
	}
	return c
}
