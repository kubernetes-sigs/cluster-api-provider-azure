package azure

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/go-autorest/autorest"
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
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-7*time.Hour))
				defer cancel()
				return ctx.Err()
			}(),
			want: true,
		},
		{
			name: "Context canceled exceeded error",
			err: func() error {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Hour))
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

func TestResourceNotFound(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		success bool
	}{
		{
			name:    "Not Found detailed error",
			err:     autorest.DetailedError{StatusCode: http.StatusNotFound},
			success: true,
		},
		{
			name:    "Conflict detailed error",
			err:     autorest.DetailedError{StatusCode: http.StatusConflict},
			success: false,
		},
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ResourceNotFound(tc.err); got != tc.success {
				t.Errorf("ResourceNotFound() = %v, want %v", got, tc.success)
			}
		})
	}
}
