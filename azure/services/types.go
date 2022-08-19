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
	"time"

	"github.com/go-logr/logr"
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
	// Rate limit QPS (Write)
	AzureServiceRateLimitQPSDelete float32 `json:"azureServiceRateLimitQPSDelete,omitempty" yaml:"azureServiceRateLimitQPSWrite,omitempty"`
	// Rate limit Bucket Size
	AzureServiceRateLimitBucketDelete int `json:"azureServiceRateLimitBucketDelete,omitempty" yaml:"azureServiceRateLimitBucketWrite,omitempty"`
}

// ServiceLimiter represents Rate Limiter + Exponential Backoff state.
type ServiceLimiter struct {
	RetryAfter time.Time
	flowcontrol.RateLimiter
}

// TryRequest returns an error if the ServiceLimiter is currently rate limited or under active exponential backoff.
func (s *ServiceLimiter) TryRequest(log logr.Logger) error {
	if !s.TryAccept() {
		log.Error(rateLimitError, "Rate Limited", "qps", s.QPS())
		return rateLimitError
	}
	if s.RetryAfter.After(time.Now()) {
		log.Error(retryAfterError, "In Exponential Backoff", "RetryAfter", s.RetryAfter)
		return retryAfterError
	}
	return nil
}

// StoreRetryAfter detects HTTP 429 responses and stores the RetryAfter value.
func (s *ServiceLimiter) StoreRetryAfter(log logr.Logger, resp *http.Response, err error, defaultRetryAfter time.Duration) {
	if resp != nil {
		if resp.StatusCode == http.StatusTooManyRequests {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter == "" {
				log.Error(err, "HTTP 429 Response with no Retry-After, will use a fallback value", "defaultRetryAfter", defaultRetryAfter)
			} else {
				log.Error(err, "HTTP 429 Response", "Retry-After", retryAfter)
				s.RetryAfter = GetRetryAfterTime(retryAfter, defaultRetryAfter)
			}
		}
	}
}
