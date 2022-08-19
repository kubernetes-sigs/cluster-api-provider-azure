/*
Copyright 2022 The Kubernetes Authors.

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

package services

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/util/flowcontrol"
)

// Standard defaults for Azure service rate limits.
const (
	// AzureServiceRateLimitEnabled sets the default "Enabled" value for a rate limiter.
	AzureServiceRateLimitEnabled = true
	// AzureServiceRateLimitQPS is the default QPS permitted by a rate limiter.
	AzureServiceRateLimitQPS = 1.0
	// AzureServiceRateLimitBucket is the default maximum number of requests to hold in a queue when rate limiting is active.
	AzureServiceRateLimitBucket = 5
)

var rateLimitError error = errors.Errorf("actively rate limited")

// NewRateLimiters creates new read, write, and delete flowcontrol.RateLimiters from RateLimitConfig.
func NewRateLimiters(config *RateLimitConfig) (reader flowcontrol.RateLimiter, writer flowcontrol.RateLimiter, deleter flowcontrol.RateLimiter) {
	reader = flowcontrol.NewFakeAlwaysRateLimiter()
	writer = flowcontrol.NewFakeAlwaysRateLimiter()
	deleter = flowcontrol.NewFakeAlwaysRateLimiter()

	if config != nil && config.AzureServiceRateLimit {
		reader = flowcontrol.NewTokenBucketRateLimiter(
			config.AzureServiceRateLimitQPS,
			config.AzureServiceRateLimitBucket)

		writer = flowcontrol.NewTokenBucketRateLimiter(
			config.AzureServiceRateLimitQPSWrite,
			config.AzureServiceRateLimitBucketWrite)

		deleter = flowcontrol.NewTokenBucketRateLimiter(
			config.AzureServiceRateLimitQPSDelete,
			config.AzureServiceRateLimitBucketDelete)
	}

	return reader, writer, deleter
}
