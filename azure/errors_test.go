/*
Copyright 2019 The Kubernetes Authors.

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

package azure

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/pkg/errors"
)

func TestIsContextDeadlineExceededOrCanceled(t *testing.T) {
	tests := []struct {
		name string
		want bool
		err  error
	}{
		{
			name: "Context deadline exceeded error",
			err: func() error {
				ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-7*time.Hour))
				defer cancel()
				return ctx.Err()
			}(),
			want: true,
		},
		{
			name: "Context canceled exceeded error",
			err: func() error {
				ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(1*time.Hour))
				cancel()
				return ctx.Err()
			}(),
			want: true,
		},
		{
			name: "Nil error",
			err:  nil,
			want: false,
		},
		{
			name: "Error other than context deadline exceeded or canceled error",
			err:  errors.New("dummy error"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsContextDeadlineExceededOrCanceledError(tt.err); got != tt.want {
				t.Errorf("IsContextDeadlineExceededOrCanceled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "transient ReconcileError",
			err:  WithTransientError(errors.New("boom"), time.Minute),
			want: true,
		},
		{
			name: "terminal ReconcileError",
			err:  WithTerminalError(errors.New("boom")),
			want: false,
		},
		{
			name: "terminal ReconcileError wrapping a retryable status is not transient",
			err:  WithTerminalError(&azcore.ResponseError{StatusCode: http.StatusTooManyRequests}),
			want: false,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "wrapped context deadline exceeded",
			err:  errors.Wrapf(context.DeadlineExceeded, "failed to create or update resource"),
			want: true,
		},
		{
			name: "throttling response error (429)",
			err:  &azcore.ResponseError{StatusCode: http.StatusTooManyRequests},
			want: true,
		},
		{
			name: "service unavailable response error (503)",
			err:  &azcore.ResponseError{StatusCode: http.StatusServiceUnavailable},
			want: true,
		},
		{
			name: "retryable status only on the raw response (StatusCode unset)",
			err:  &azcore.ResponseError{RawResponse: &http.Response{StatusCode: http.StatusInternalServerError}},
			want: true,
		},
		{
			name: "non-retryable status only on the raw response (StatusCode unset)",
			err:  &azcore.ResponseError{RawResponse: &http.Response{StatusCode: http.StatusBadRequest}},
			want: false,
		},
		{
			name: "wrapped throttling response error",
			err:  errors.Wrapf(&azcore.ResponseError{StatusCode: http.StatusTooManyRequests}, "failed to create or update resource"),
			want: true,
		},
		{
			name: "bad request response error (400) is terminal",
			err:  &azcore.ResponseError{StatusCode: http.StatusBadRequest},
			want: false,
		},
		{
			// Synthetic classification case: classification is by HTTP status code
			// only, so a ResponseError with a non-retryable status (200 here) and a
			// populated ErrorCode is not classified transient.
			name: "non-retryable status with an ErrorCode is not transient",
			err:  errors.Wrapf(&azcore.ResponseError{StatusCode: http.StatusOK, ErrorCode: "OSProvisioningTimedOut"}, "wrapped"),
			want: false,
		},
		{
			name: "not found response error (404) is terminal",
			err:  &azcore.ResponseError{StatusCode: http.StatusNotFound},
			want: false,
		},
		{
			name: "network timeout is transient",
			err:  &net.DNSError{Err: "i/o timeout", Name: "management.azure.com", IsTimeout: true},
			want: true,
		},
		{
			name: "plain error is terminal",
			err:  errors.New("OSProvisioningTimedOut"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsTransientError(tt.err); got != tt.want {
				t.Errorf("IsTransientError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceNotFound(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		success bool
	}{
		{
			name:    "Not Found response error",
			err:     &azcore.ResponseError{StatusCode: http.StatusNotFound},
			success: true,
		},
		{
			name:    "Conflict response error",
			err:     &azcore.ResponseError{StatusCode: http.StatusConflict},
			success: false,
		},
		{
			name:    "Not Found generic error",
			err:     errors.New("404: Not Found"),
			success: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ResourceNotFound(tc.err); got != tc.success {
				t.Errorf("ResourceNotFound() = %v, want %v", got, tc.success)
			}
		})
	}
}
