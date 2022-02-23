//go:build e2e
// +build e2e

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

package e2e

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
)

// This code was inspired from kubernetes/kubernetes, specifically https://github.com/oomichi/kubernetes/blob/master/test/e2e/framework/log.go

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func logf(level string, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf prints info logs with a timestamp and formatting.
func Logf(format string, args ...interface{}) {
	logf("INFO", format, args...)
}

// LogWarningf prints warning logs with a timestamp and formatting.
func LogWarningf(format string, args ...interface{}) {
	logf("WARNING", format, args...)
}

// Log prints info logs with a timestamp.
func Log(message string) {
	logf("INFO", message)
}

// Log prints warning logs with a timestamp.
func LogWarning(message string) {
	logf("WARNING", message)
}
