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
	"net/http"
	"strconv"
	"time"

	"k8s.io/client-go/util/flowcontrol"
)

// RateLimitConfig indicates the rate limit config options.
type RateLimitConfig struct {
	// Enable rate limiting
	AzureServiceRateLimit bool `json:"azureServiceRateLimit,omitempty" yaml:"azureServiceRateLimit,omitempty"`
	// Rate limit QPS (Read)
	AzureServiceRateLimitQPS float32 `json:"azureServiceRateLimitQPS,omitempty" yaml:"azureServiceRateLimitQPS,omitempty"`
	// Rate limit Bucket Size
	AzureServiceRateLimitBucket int `json:"azureServiceRateLimitBucket,omitempty" yaml:"azureServiceRateLimitBucket,omitempty"`
	// Rate limit QPS (Write)
	AzureServiceRateLimitQPSWrite float32 `json:"azureServiceRateLimitQPSWrite,omitempty" yaml:"azureServiceRateLimitQPSWrite,omitempty"`
	// Rate limit Bucket Size
	AzureServiceRateLimitBucketWrite int `json:"azureServiceRateLimitBucketWrite,omitempty" yaml:"azureServiceRateLimitBucketWrite,omitempty"`
}

type RateLimiter struct {
	Reader flowcontrol.RateLimiter
	Writer flowcontrol.RateLimiter
}

type RetryAfter struct {
	Reader time.Time
	Writer time.Time
}

// Standard defaults for Azure service rate limits
const AzureServiceRateLimitEnabled bool = true

// NewRateLimiter creates new read and write flowcontrol.RateLimiter from RateLimitConfig.
func NewRateLimiter(config *RateLimitConfig) (flowcontrol.RateLimiter, flowcontrol.RateLimiter) {
	readLimiter := flowcontrol.NewFakeAlwaysRateLimiter()
	writeLimiter := flowcontrol.NewFakeAlwaysRateLimiter()

	if config != nil && config.AzureServiceRateLimit {
		readLimiter = flowcontrol.NewTokenBucketRateLimiter(
			config.AzureServiceRateLimitQPS,
			config.AzureServiceRateLimitBucket)

		writeLimiter = flowcontrol.NewTokenBucketRateLimiter(
			config.AzureServiceRateLimitQPSWrite,
			config.AzureServiceRateLimitBucketWrite)
	}

	return readLimiter, writeLimiter
}

func GetRetryAfterTime(resp *http.Response) time.Time {
	ra := resp.Header.Get("Retry-After")
	if ra != "" {
		var dur time.Duration
		if retryAfter, _ := strconv.Atoi(ra); retryAfter > 0 {
			dur = time.Duration(retryAfter) * time.Second
		} else if t, err := time.Parse(time.RFC1123, ra); err == nil {
			dur = time.Until(t)
		}
		return time.Now().Add(dur)
	}
	return time.Now()
}
