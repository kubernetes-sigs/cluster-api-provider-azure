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

package rke2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
)

func TestResolve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		requested     string
		images        []string
		published     []string
		expected      string
		expectedTags  []string
		expectedError string
	}{
		{
			name:         "requested release available",
			requested:    "1.35.10",
			images:       []string{"1.35.9", "1.35.10"},
			published:    []string{"v1.35.10+rke2r1"},
			expected:     "v1.35.10",
			expectedTags: []string{"v1.35.10+rke2r1"},
		},
		{
			name:         "missing release falls back in descending semantic order",
			requested:    "v1.35.10",
			images:       []string{"1.35.8", "1.35.10", "1.35.9"},
			published:    []string{"v1.35.8+rke2r1", "v1.35.9+rke2r1"},
			expected:     "v1.35.9",
			expectedTags: []string{"v1.35.10+rke2r1", "v1.35.9+rke2r1"},
		},
		{
			name:         "release without a gallery image is not queried",
			requested:    "v1.35.10",
			images:       []string{"1.35.8", "1.35.10"},
			published:    []string{"v1.35.9+rke2r1", "v1.35.8+rke2r1"},
			expected:     "v1.35.8",
			expectedTags: []string{"v1.35.10+rke2r1", "v1.35.8+rke2r1"},
		},
		{
			name:      "only stable images within the requested minor and patch bound",
			requested: "v1.35.10",
			images: []string{
				"2.35.0", "1.36.0", "1.34.20", "1.35.11", "1.35.10-alpha.1", "1.35.10+build.1", "1.35.9",
			},
			published:    []string{"v1.35.9+rke2r1"},
			expected:     "v1.35.9",
			expectedTags: []string{"v1.35.9+rke2r1"},
		},
		{
			name:          "no eligible gallery image",
			requested:     "v1.35.10",
			images:        []string{"1.36.0", "1.35.11", "1.34.20"},
			expectedError: "no published RKE2 rke2r1 release with a matching gallery image for 1.35 at or below v1.35.10",
		},
		{
			name:          "eligible images exhausted",
			requested:     "v1.35.10",
			images:        []string{"1.35.9", "1.35.10"},
			expectedTags:  []string{"v1.35.10+rke2r1", "v1.35.9+rke2r1"},
			expectedError: "no published RKE2 rke2r1 release",
		},
		{
			name:          "unresolved suite version fails",
			requested:     "stable-1.35",
			expectedError: `invalid resolved Kubernetes version "stable-1.35"`,
		},
		{
			name:          "prerelease suite version fails",
			requested:     "v1.35.10-rc.1",
			expectedError: "invalid resolved Kubernetes version",
		},
		{
			name:          "suite version with RKE2 suffix fails",
			requested:     "v1.35.10+rke2r1",
			expectedError: "invalid resolved Kubernetes version",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var requestedTags []string
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				tag := strings.TrimPrefix(req.URL.Path, "/repos/rancher/rke2/releases/tags/")
				requestedTags = append(requestedTags, tag)
				if req.URL.Scheme != "https" || req.URL.Host != "api.github.com" || req.Method != http.MethodGet {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
				}
				expectedPath := "/repos/rancher/rke2/releases/tags/" + strings.ReplaceAll(tag, "+", "%2B")
				if req.URL.EscapedPath() != expectedPath {
					t.Errorf("escaped path = %q, want %q", req.URL.EscapedPath(), expectedPath)
				}
				for _, published := range tt.published {
					if tag == published {
						return response(http.StatusOK, publishedRelease(tag)), nil
					}
				}
				return response(http.StatusNotFound, `{"message":"Not Found"}`), nil
			})}
			version, err := resolve(t.Context(), tt.requested, galleryVersions(tt.images), client)
			if tt.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Fatalf("error = %v, want %q", err, tt.expectedError)
				}
			} else if err != nil || version != tt.expected {
				t.Fatalf("version = %q, error = %v, want %q", version, err, tt.expected)
			}
			if !reflect.DeepEqual(requestedTags, tt.expectedTags) {
				t.Errorf("queried tags = %v, want %v", requestedTags, tt.expectedTags)
			}
		})
	}
}

func TestResolveUnavailableReleases(t *testing.T) {
	t.Parallel()
	release := publishedRelease("v1.35.10+rke2r1")
	tests := []struct {
		name string
		body string
	}{
		{name: "draft", body: strings.Replace(release, `"draft":false`, `"draft":true`, 1)},
		{name: "prerelease", body: strings.Replace(release, `"prerelease":false`, `"prerelease":true`, 1)},
		{name: "unpublished", body: strings.Replace(release, `"2026-01-01T00:00:00Z"`, `null`, 1)},
		{name: "missing checksum", body: strings.Replace(release, "sha256sum-amd64.txt", "sha256sum-arm64.txt", 1)},
		{name: "missing tarball", body: strings.Replace(release, "rke2.linux-amd64.tar.gz", "rke2.linux-arm64.tar.gz", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var requestedTags []string
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				tag := strings.TrimPrefix(req.URL.Path, "/repos/rancher/rke2/releases/tags/")
				requestedTags = append(requestedTags, tag)
				if tag == "v1.35.10+rke2r1" {
					return response(http.StatusOK, tt.body), nil
				}
				return response(http.StatusOK, publishedRelease(tag)), nil
			})}
			version, err := resolve(t.Context(), "v1.35.10", galleryVersions([]string{"1.35.10", "1.35.9"}), client)
			if err != nil || version != "v1.35.9" {
				t.Fatalf("version = %q, error = %v, want v1.35.9", version, err)
			}
			if !reflect.DeepEqual(requestedTags, []string{"v1.35.10+rke2r1", "v1.35.9+rke2r1"}) {
				t.Errorf("queried tags = %v", requestedTags)
			}
		})
	}
}

func TestResolveFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		status        int
		body          string
		expectedError string
	}{
		{name: "authentication", status: http.StatusUnauthorized, expectedError: "401 Unauthorized"},
		{name: "forbidden or rate limited", status: http.StatusForbidden, expectedError: "403 Forbidden"},
		{name: "rate limited", status: http.StatusTooManyRequests, expectedError: "429 Too Many Requests"},
		{name: "server error", status: http.StatusInternalServerError, expectedError: "500 Internal Server Error"},
		{name: "invalid JSON", status: http.StatusOK, body: `{`, expectedError: "decode RKE2 release"},
		{name: "trailing garbage", status: http.StatusOK, body: publishedRelease("v1.35.10+rke2r1") + "bad", expectedError: "decode RKE2 release"},
		{name: "missing metadata", status: http.StatusOK, body: `{}`, expectedError: "invalid metadata"},
		{name: "null metadata", status: http.StatusOK, body: `null`, expectedError: "invalid metadata"},
		{name: "wrong tag", status: http.StatusOK, body: publishedRelease("v1.35.9+rke2r1"), expectedError: "invalid metadata"},
		{
			name:          "invalid publication time",
			status:        http.StatusOK,
			body:          strings.Replace(publishedRelease("v1.35.10+rke2r1"), "2026-01-01T00:00:00Z", "bad", 1),
			expectedError: "decode RKE2 release",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				calls++
				return response(tt.status, tt.body), nil
			})}
			version, err := resolve(t.Context(), "v1.35.10", galleryVersions([]string{"1.35.10", "1.35.9"}), client)
			if err == nil || !strings.Contains(err.Error(), tt.expectedError) || !strings.Contains(err.Error(), "v1.35.10+rke2r1") {
				t.Fatalf("version = %q, error = %v, want error containing %q and requested tag", version, err, tt.expectedError)
			}
			if calls != 1 {
				t.Errorf("HTTP calls = %d, want 1", calls)
			}
		})
	}
}

func TestResolveRequestErrors(t *testing.T) {
	t.Parallel()
	t.Run("network failure", func(t *testing.T) {
		t.Parallel()
		networkErr := errors.New("network unavailable")
		client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, networkErr
		})}
		_, err := resolve(t.Context(), "v1.35.10", galleryVersions([]string{"1.35.10"}), client)
		if !errors.Is(err, networkErr) {
			t.Fatalf("error = %v, want network error", err)
		}
	})
	for _, reason := range []string{"cancellation", "deadline"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			wantErr := context.Canceled
			if reason == "deadline" {
				var deadlineCancel context.CancelFunc
				ctx, deadlineCancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer deadlineCancel()
				wantErr = context.DeadlineExceeded
			}
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				cancel()
				<-req.Context().Done()
				return nil, req.Context().Err()
			})}
			_, err := resolve(ctx, "v1.35.10", galleryVersions([]string{"1.35.10", "1.35.9"}), client)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
			if calls != 1 {
				t.Errorf("HTTP calls = %d, want 1", calls)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func publishedRelease(tag string) string {
	return fmt.Sprintf(`{"tag_name":%q,"draft":false,"prerelease":false,"published_at":"2026-01-01T00:00:00Z","assets":[{"name":"sha256sum-amd64.txt"},{"name":"rke2.linux-amd64.tar.gz"}]}`, tag)
}

func galleryVersions(versions []string) map[string]semver.Version {
	images := make(map[string]semver.Version, len(versions))
	for _, version := range versions {
		images[version] = semver.MustParse(version)
	}
	return images
}
