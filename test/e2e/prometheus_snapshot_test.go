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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func verifyPrometheusSnapshot(reportDir string) error {
	file, err := os.OpenInRoot(reportDir, "prometheus_snapshot.tar.gz")
	if err != nil {
		return fmt.Errorf("open Prometheus snapshot: %w", err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read Prometheus snapshot: %w", err)
	}
	defer reader.Close()
	archive := tar.NewReader(reader)
	hasData := false
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid Prometheus snapshot archive: %w", err)
		}
		if header.Typeflag == tar.TypeReg && header.Size > 0 {
			hasData = true
		}
	}
	// Allow tar block padding, but require a valid gzip trailer and bounded trailing data.
	if _, err := io.CopyN(io.Discard, reader, 1<<20); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("Prometheus snapshot has excessive data after the tar archive")
		}
		return fmt.Errorf("incomplete Prometheus snapshot: %w", err)
	}
	if !hasData {
		return errors.New("Prometheus snapshot contains no data files")
	}
	return nil
}

func TestVerifyPrometheusSnapshot(t *testing.T) {
	makeArchive := func(withData bool) []byte {
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
	valid := makeArchive(true)
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
		{"no-data", makeArchive(false), false},
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
