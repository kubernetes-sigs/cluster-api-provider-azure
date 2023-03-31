package azure

import (
	"context"
	"testing"
	"time"

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
