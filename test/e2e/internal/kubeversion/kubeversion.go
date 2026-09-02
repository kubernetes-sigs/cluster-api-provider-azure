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

// Package kubeversion resolves the Kubernetes version expressions used by the e2e tests
// ("latest", "latest-1", "stable-<Major>.<Minor>", or an explicit version) against the
// versions an environment actually offers. It holds no build tag so that these pure
// helpers are exercised by the default `go test ./...` run.
package kubeversion

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
)

var stableReleaseFormat = regexp.MustCompile(`^stable-(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// ValidateStableReleaseString validates the string format that declares "get the latest stable release for this <Major>.<Minor>"
// it should be called wherever we process a stable version string expression like "stable-1.22".
func ValidateStableReleaseString(stableVersion string) (isStable bool, matches []string) {
	matches = stableReleaseFormat.FindStringSubmatch(stableVersion)
	return len(matches) > 0, matches
}

// Common returns the versions found in both a and b, canonicalized and sorted in ascending
// semver order. Prerelease, duplicate, and unparsable versions are dropped.
func Common(a, b []string) []string {
	inA := make(map[string]struct{}, len(a))
	for _, version := range normalize(a) {
		inA[version] = struct{}{}
	}

	common := make([]string, 0, len(inA))
	for _, version := range normalize(b) {
		if _, ok := inA[version]; ok {
			common = append(common, version)
		}
	}
	return common
}

// Select resolves requestedVersion against versions, which must be canonical, stable, and sorted
// in ascending semver order as returned by Common. An explicit version that versions doesn't offer
// falls back to the newest version sharing its major.minor.
func Select(requestedVersion string, versions []string) (string, error) {
	switch requestedVersion {
	case "latest":
		return atOffset(versions, 0)
	case "latest-1":
		return atOffset(versions, 1)
	}

	if isStableVersion, match := ValidateStableReleaseString(requestedVersion); isStableVersion {
		return latestForMajorMinor(versions, fmt.Sprintf("v%s.%s", match[1], match[2]))
	}

	canonicalRequestedVersion := canonical(requestedVersion)
	if canonicalRequestedVersion == "" {
		return "", errors.Errorf("invalid Kubernetes version %q", requestedVersion)
	}
	for _, version := range versions {
		if version == canonicalRequestedVersion {
			return version, nil
		}
	}

	return latestForMajorMinor(versions, semver.MajorMinor(canonicalRequestedVersion))
}

// LatestStable returns the stable version at offset positions before the newest in versions.
// versions need not be canonical, sorted, or free of prereleases.
func LatestStable(versions []string, offset int) (string, error) {
	return atOffset(normalize(versions), offset)
}

// atOffset returns the version at offset positions before the end of an ascending-sorted versions.
func atOffset(versions []string, offset int) (string, error) {
	if len(versions) <= offset {
		if offset == 0 {
			return "", errors.New("no stable Kubernetes versions available")
		}
		return "", errors.Errorf("fewer than %d stable Kubernetes versions available", offset+1)
	}
	return versions[len(versions)-1-offset], nil
}

// latestForMajorMinor returns the newest version in an ascending-sorted versions matching majorMinor.
func latestForMajorMinor(versions []string, majorMinor string) (string, error) {
	for i := len(versions) - 1; i >= 0; i-- {
		if semver.MajorMinor(versions[i]) == majorMinor {
			return versions[i], nil
		}
	}
	return "", errors.Errorf("no Kubernetes version available for %s", majorMinor)
}

// normalize returns the canonical, stable versions in versions, sorted in ascending semver order.
func normalize(versions []string) []string {
	set := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		if version = canonical(version); version != "" && semver.Prerelease(version) == "" {
			set[version] = struct{}{}
		}
	}

	normalized := make([]string, 0, len(set))
	for version := range set {
		normalized = append(normalized, version)
	}
	semver.Sort(normalized)
	return normalized
}

// canonical returns the canonical "v"-prefixed semver form of version, or "" if it isn't valid semver.
func canonical(version string) string {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return semver.Canonical(version)
}
