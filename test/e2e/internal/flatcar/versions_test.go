/*
Copyright The Kubernetes Authors.

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

package flatcar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		requested string
		files     []string
		want      string
		wantError string
	}{
		{
			name:      "requested version available",
			requested: "1.35.10",
			files:     []string{"kubernetes-v1.35.9-x86-64.raw", "kubernetes-v1.35.10-x86-64.raw"},
			want:      "v1.35.10",
		},
		{
			name:      "missing version falls back",
			requested: "v1.35.10",
			files:     []string{"kubernetes-v1.35.8-x86-64.raw", "kubernetes-v1.35.9-x86-64.raw"},
			want:      "v1.35.9",
		},
		{
			name:      "highest version selected semantically regardless of order",
			requested: "v1.35.11",
			files:     []string{"kubernetes-v1.35.9-x86-64.raw", "kubernetes-v1.35.10-x86-64.raw", "kubernetes-v1.35.8-x86-64.raw"},
			want:      "v1.35.10",
		},
		{
			name:      "only stable x86-64 versions in the requested minor and patch bound",
			requested: "v1.35.10",
			files: []string{
				"kubernetes-v1.35.8-x86-64.raw", "kubernetes-v2.35.0-x86-64.raw",
				"kubernetes-v1.36.0-x86-64.raw", "kubernetes-v1.34.20-x86-64.raw",
				"kubernetes-v1.35.11-x86-64.raw", "kubernetes-v1.35.10-alpha.1-x86-64.raw",
				"kubernetes-v1.35.9+build.1-x86-64.raw", "kubernetes-v1.35.10-arm64.raw",
				"kubernetes-v1.35.9-x86-64.raw.sig", "containerd-2.0.0-x86-64.raw",
			},
			want: "v1.35.8",
		},
		{
			name:      "binary checksum marker",
			requested: "v1.35.10",
			files:     []string{"*kubernetes-v1.35.9-x86-64.raw"},
			want:      "v1.35.9",
		},
		{
			name:      "no matching version",
			requested: "v1.35.10",
			files:     []string{"kubernetes-v1.34.9-x86-64.raw", "kubernetes-v1.35.10-arm64.raw"},
			wantError: "no published Flatcar x86-64 extension for 1.35 at or below v1.35.10",
		},
		{
			name:      "empty manifest",
			requested: "v1.35.10",
			wantError: "no published Flatcar x86-64 extension",
		},
		{
			name:      "unresolved requested version",
			requested: "stable-1.35",
			wantError: "invalid resolved Kubernetes version",
		},
		{
			name:      "prerelease requested version",
			requested: "v1.35.10-rc.1",
			wantError: "invalid resolved Kubernetes version",
		},
		{
			name:      "requested version with build metadata",
			requested: "v1.35.10+build.1",
			wantError: "invalid resolved Kubernetes version",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.String() != "https://extensions.flatcar.org/extensions/kubernetes/SHA256SUMS" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
				}
				return response(http.StatusOK, strings.NewReader(manifest(tt.files...))), nil
			})}
			version, err := resolve(t.Context(), tt.requested, client)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil || version != tt.want {
				t.Fatalf("version = %q, error = %v, want %q", version, err, tt.want)
			}
		})
	}
}

func TestResolveManifestErrors(t *testing.T) {
	t.Parallel()
	valid := manifest("kubernetes-v1.35.9-x86-64.raw")
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "not found", status: http.StatusNotFound, wantError: "server returned 404"},
		{name: "server error", status: http.StatusServiceUnavailable, wantError: "server returned 503"},
		{name: "malformed entry", status: http.StatusOK, body: valid + "broken\n", wantError: "invalid Flatcar SHA256SUMS entry"},
		{name: "invalid checksum", status: http.StatusOK, body: valid + "bad kubernetes-v1.35.10-x86-64.raw\n", wantError: "invalid Flatcar SHA256SUMS checksum"},
		{name: "invalid version", status: http.StatusOK, body: valid + manifest("kubernetes-vbroken-x86-64.raw"), wantError: "invalid Flatcar extension version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(tt.status, strings.NewReader(tt.body)), nil
			})}
			version, err := resolve(t.Context(), "v1.35.10", client)
			if version != "" || err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("version = %q, error = %v, want %q", version, err, tt.wantError)
			}
		})
	}
}

func TestResolveReadError(t *testing.T) {
	t.Parallel()
	want := errors.New("interrupted response")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := io.MultiReader(strings.NewReader(manifest("kubernetes-v1.35.9-x86-64.raw")), errorReader{want})
		return response(http.StatusOK, body), nil
	})}
	version, err := resolve(t.Context(), "v1.35.10", client)
	if version != "" || !errors.Is(err, want) {
		t.Fatalf("version = %q, error = %v, want %v", version, err, want)
	}
}

func TestResolveCanceledRequest(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	})}
	_, err := resolve(ctx, "v1.35.10", client)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

func manifest(files ...string) string {
	var body strings.Builder
	for _, file := range files {
		fmt.Fprintf(&body, "%s  %s\n", strings.Repeat("a", 64), file)
	}
	return body.String()
}

func response(status int, body io.Reader) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(body),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
