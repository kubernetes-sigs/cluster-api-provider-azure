//go:build e2e
// +build e2e

/*
Copyright 2026 The Kubernetes Authors.

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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func testPrometheusSnapshot(t *testing.T, withData bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	compressed := gzip.NewWriter(&buf)
	archive := tar.NewWriter(compressed)
	if withData {
		data := []byte("sample TSDB data")
		if err := archive.WriteHeader(&tar.Header{Name: "snapshot/block/index", Mode: 0o600, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestVerifyPrometheusSnapshot(t *testing.T) {
	valid := testPrometheusSnapshot(t, true)
	for _, tc := range []struct {
		name string
		data []byte
		ok   bool
	}{
		{"complete", valid, true},
		{"missing", nil, false},
		{"empty", []byte{}, false},
		{"corrupt", []byte("not a snapshot"), false},
		{"truncated", valid[:len(valid)-8], false},
		{"no-data", testPrometheusSnapshot(t, false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.data != nil {
				if err := os.WriteFile(filepath.Join(dir, "prometheus_snapshot.tar.gz"), tc.data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := verifyPrometheusSnapshot(dir); (err == nil) != tc.ok {
				t.Fatalf("unexpected snapshot validation result: %v", err)
			}
		})
	}
}
