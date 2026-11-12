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

// Package flatcar selects published Kubernetes system extensions for Flatcar.
package flatcar

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/blang/semver"
)

// Resolve selects the newest published x86-64 extension in the requested minor at or below requestedVersion.
func Resolve(ctx context.Context, requestedVersion string) (string, error) {
	return resolve(ctx, requestedVersion, http.DefaultClient)
}

func resolve(ctx context.Context, requestedVersion string, client *http.Client) (string, error) {
	requested, err := semver.Parse(strings.TrimPrefix(requestedVersion, "v"))
	if err != nil || len(requested.Pre) != 0 || len(requested.Build) != 0 {
		return "", fmt.Errorf("invalid resolved Kubernetes version %q", requestedVersion)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://extensions.flatcar.org/extensions/kubernetes/SHA256SUMS", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("create request for Flatcar extensions: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get Flatcar extensions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get Flatcar extensions: server returned %s", resp.Status)
	}

	var selected semver.Version
	found := false
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return "", fmt.Errorf("invalid Flatcar SHA256SUMS entry %q", scanner.Text())
		}
		checksum, err := hex.DecodeString(fields[0])
		if err != nil || len(checksum) != sha256.Size {
			return "", fmt.Errorf("invalid Flatcar SHA256SUMS checksum for %q", fields[1])
		}
		filename := strings.TrimPrefix(fields[1], "*")
		version, ok := strings.CutPrefix(filename, "kubernetes-v")
		if !ok {
			continue
		}
		version, ok = strings.CutSuffix(version, "-x86-64.raw")
		if !ok {
			continue
		}
		candidate, err := semver.Parse(version)
		if err != nil {
			return "", fmt.Errorf("invalid Flatcar extension version %q: %w", version, err)
		}
		if len(candidate.Pre) != 0 || len(candidate.Build) != 0 {
			continue
		}
		if candidate.Major != requested.Major || candidate.Minor != requested.Minor || candidate.GT(requested) {
			continue
		}
		if !found || candidate.GT(selected) {
			selected = candidate
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read Flatcar extensions: %w", err)
	}
	if !found {
		return "", fmt.Errorf("no published Flatcar x86-64 extension for %d.%d at or below %s",
			requested.Major, requested.Minor, requestedVersion)
	}
	return "v" + selected.String(), nil
}
