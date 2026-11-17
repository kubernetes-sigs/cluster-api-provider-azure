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

// Package rke2 selects published RKE2 releases with matching gallery images.
package rke2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/blang/semver"
)

// Resolve selects the newest published rke2r1 release in the requested minor at or below requestedVersion.
// The returned Kubernetes version has a v prefix and no RKE2 suffix.
func Resolve(ctx context.Context, requestedVersion string, imageVersions map[string]semver.Version) (string, error) {
	return resolve(ctx, requestedVersion, imageVersions, http.DefaultClient)
}

func resolve(ctx context.Context, requestedVersion string, imageVersions map[string]semver.Version, client *http.Client) (string, error) {
	requested, err := semver.Parse(strings.TrimPrefix(requestedVersion, "v"))
	if err != nil || len(requested.Pre) != 0 || len(requested.Build) != 0 {
		return "", fmt.Errorf("invalid resolved Kubernetes version %q", requestedVersion)
	}

	candidates := make(semver.Versions, 0, len(imageVersions))
	for _, version := range imageVersions {
		if len(version.Pre) != 0 || len(version.Build) != 0 {
			continue
		}
		if version.Major == requested.Major && version.Minor == requested.Minor && version.LE(requested) {
			candidates = append(candidates, version)
		}
	}
	semver.Sort(candidates)

	for i := len(candidates) - 1; i >= 0; i-- {
		version := "v" + candidates[i].String()
		available, err := releaseAvailable(ctx, client, version+"+rke2r1")
		if err != nil {
			return "", err
		}
		if available {
			return version, nil
		}
	}
	return "", fmt.Errorf("no published RKE2 rke2r1 release with a matching gallery image for %d.%d at or below %s",
		requested.Major, requested.Minor, requestedVersion)
}

func releaseAvailable(ctx context.Context, client *http.Client, tag string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/rancher/rke2/releases/tags/"+url.QueryEscape(tag), http.NoBody)
	if err != nil {
		return false, fmt.Errorf("create request for RKE2 release %s: %w", tag, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("get RKE2 release %s: %w", tag, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("get RKE2 release %s: GitHub returned %s", tag, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read RKE2 release %s: %w", tag, err)
	}
	var release struct {
		TagName     string     `json:"tag_name"`
		Draft       *bool      `json:"draft"`
		Prerelease  *bool      `json:"prerelease"`
		PublishedAt *time.Time `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return false, fmt.Errorf("decode RKE2 release %s: %w", tag, err)
	}
	if release.TagName != tag || release.Draft == nil || release.Prerelease == nil {
		return false, fmt.Errorf("invalid metadata for RKE2 release %s", tag)
	}
	if *release.Draft || *release.Prerelease || release.PublishedAt == nil {
		return false, nil
	}
	var hasChecksum, hasTarball bool
	for _, asset := range release.Assets {
		switch asset.Name {
		case "sha256sum-amd64.txt":
			hasChecksum = true
		case "rke2.linux-amd64.tar.gz":
			hasTarball = true
		}
	}
	return hasChecksum && hasTarball, nil
}
