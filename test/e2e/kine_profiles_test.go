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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	pprofprofile "github.com/google/pprof/profile"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func testKineProfile(t *testing.T, kind string) []byte {
	t.Helper()
	if kind == "goroutine" {
		return []byte("goroutine 1 [chan receive]:\nexample.wait()\n")
	}
	sampleType, unit := "cpu", "nanoseconds"
	if kind == "heap" {
		sampleType, unit = "inuse_space", "bytes"
	}
	profile := &pprofprofile.Profile{
		SampleType: []*pprofprofile.ValueType{{Type: sampleType, Unit: unit}},
		Sample:     []*pprofprofile.Sample{{Value: []int64{1}}},
	}
	var output bytes.Buffer
	if err := profile.Write(&output); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestKineProfilerResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "profiles-test", UID: "profile-owner"}}
	ca := []byte("public-ca")
	if err := createKineProfilerResources(t.Context(), client, ns, ca); err != nil {
		t.Fatal(err)
	}
	role, err := client.RbacV1().ClusterRoles().Get(t.Context(), ns.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(role.Rules) != 1 || !slices.Equal(role.Rules[0].Verbs, []string{"get"}) ||
		!slices.Equal(role.Rules[0].NonResourceURLs, []string{"/debug/pprof/profile", "/debug/pprof/heap", "/debug/pprof/goroutine"}) ||
		len(role.Rules[0].Resources) != 0 || len(role.OwnerReferences) != 1 || role.OwnerReferences[0].UID != ns.UID {
		t.Fatal("profiling role must be owned by the temporary namespace and grant only the required GET URLs")
	}
	binding, err := client.RbacV1().ClusterRoleBindings().Get(t.Context(), ns.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(binding.Subjects) != 1 || binding.Subjects[0].Kind != "ServiceAccount" ||
		binding.Subjects[0].Name != kineProfilerPodName || binding.Subjects[0].Namespace != ns.Name ||
		binding.RoleRef.Name != role.Name || len(binding.OwnerReferences) != 1 || binding.OwnerReferences[0].UID != ns.UID {
		t.Fatal("profiling access must be scoped to the helper service account, never anonymous users")
	}
	secret, err := client.CoreV1().Secrets(ns.Name).Get(t.Context(), "kine-ca", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(secret.Data) != 1 || !bytes.Equal(secret.Data["ca.crt"], ca) {
		t.Fatal("the helper must receive only the public Kine CA")
	}
	pod, err := client.CoreV1().Pods(ns.Name).Get(t.Context(), kineProfilerPodName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pod.Spec.ServiceAccountName != kineProfilerPodName || pod.Spec.HostNetwork ||
		pod.Spec.SecurityContext == nil || !*pod.Spec.SecurityContext.RunAsNonRoot ||
		pod.Spec.NodeSelector["kubernetes.io/arch"] != "amd64" || len(pod.Spec.Containers) != 1 {
		t.Fatal("the helper must use the restricted account and an unprivileged worker Pod")
	}
	affinity := pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if len(affinity.NodeSelectorTerms) != 1 || len(affinity.NodeSelectorTerms[0].MatchExpressions) != 1 ||
		affinity.NodeSelectorTerms[0].MatchExpressions[0].Key != "node-role.kubernetes.io/control-plane" ||
		affinity.NodeSelectorTerms[0].MatchExpressions[0].Operator != corev1.NodeSelectorOpDoesNotExist {
		t.Fatal("the helper must run on workers permitted by the metrics NSG rule")
	}
	container := pod.Spec.Containers[0]
	if !strings.Contains(container.Image, "@sha256:") || *container.SecurityContext.AllowPrivilegeEscalation ||
		!*container.SecurityContext.ReadOnlyRootFilesystem ||
		!slices.Equal(container.SecurityContext.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Fatal("the helper image and security context must remain constrained")
	}
}

func TestKineProfileTargets(t *testing.T) {
	makeNode := func(name, address string) corev1.Node {
		return corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "203.0.113.1"},
				{Type: corev1.NodeInternalIP, Address: address},
			}},
		}
	}
	targets, err := kineProfileTargets([]corev1.Node{makeNode("cp-1", "10.0.0.4"), makeNode("cp-2", "10.0.0.5")})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 || targets[1].URL != "https://10.0.0.4:6443" || targets[2].URL != "https://10.0.0.5:6443" {
		t.Fatalf("must profile each API server directly rather than sampling a load balancer: %+v", targets)
	}
	for _, address := range []string{"203.0.113.2", "10.0.1.4", "not-an-address"} {
		if _, err := kineProfileTargets([]corev1.Node{makeNode("cp", address)}); err == nil {
			t.Fatalf("must not send the profiling token outside the control-plane subnet: %s", address)
		}
	}
	if _, err := kineProfileTargets(nil); err == nil {
		t.Fatal("missing control-plane nodes must be reported")
	}
}

func TestCollectKineProfiles(t *testing.T) {
	fixtures := map[string][]byte{
		"cpu": testKineProfile(t, "cpu"), "heap": testKineProfile(t, "heap"), "goroutine": testKineProfile(t, "goroutine"),
	}
	for _, failure := range []string{"none", "http", "invalid-profile"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "profiling-start")
			if err := os.WriteFile(marker, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(marker)
			if err != nil {
				t.Fatal(err)
			}
			fetch := func(_ context.Context, target kineProfileTarget, kind string) ([]byte, error) {
				if target.Name == "kine" && kind == "cpu" {
					switch failure {
					case "http":
						return nil, errors.New("HTTP request failed")
					case "invalid-profile":
						return []byte("<html>upstream error</html>"), nil
					}
				}
				return fixtures[kind], nil
			}
			err = collectKineProfiles(t.Context(), marker, dir,
				[]kineProfileTarget{{Name: "kine"}, {Name: "apiserver-cp"}},
				[]kineProfilePoint{{Name: "before", Offset: 0}, {Name: "after", Offset: time.Millisecond}}, fetch)
			if (err == nil) != (failure == "none") {
				t.Fatalf("unexpected collection result: %v", err)
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			data, err := root.ReadFile("captures.json")
			if err != nil {
				t.Fatal(err)
			}
			var manifest kineProfileManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.MeasurementStart == nil || !manifest.MeasurementStart.Equal(info.ModTime()) || len(manifest.Captures) != 12 {
				t.Fatalf("profiles must be anchored to the measurement marker and include both phases/targets: %+v", manifest)
			}
			failed := 0
			for _, capture := range manifest.Captures {
				if capture.StartedAt.Before(capture.ScheduledAt) || capture.FinishedAt.Before(capture.StartedAt) {
					t.Fatalf("invalid capture timing: %+v", capture)
				}
				if capture.Error != "" {
					failed++
					if capture.File != "" {
						t.Fatal("failed responses must not be published as valid profiles")
					}
					continue
				}
				profile, err := root.ReadFile(capture.File)
				if err != nil {
					t.Fatal(err)
				}
				if len(profile) != capture.Bytes || validateKineProfile(capture.Kind, profile) != nil {
					t.Fatalf("invalid persisted profile: %+v", capture)
				}
			}
			wantFailures := 0
			if failure != "none" {
				wantFailures = 2
			}
			if failed != wantFailures || (manifest.Error == "") != (failure == "none") {
				t.Fatalf("partial capture failures must be retained without skipping other profiles: %+v", manifest)
			}
		})
	}
}

func TestKineProfileCancellation(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	err := collectKineProfiles(ctx, filepath.Join(dir, "missing-marker"), dir, nil, nil,
		func(context.Context, kineProfileTarget, string) ([]byte, error) {
			called = true
			return nil, nil
		})
	if err == nil || called {
		t.Fatal("an early benchmark exit must report missing profiles without waiting for the scheduled times")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	data, err := root.ReadFile("captures.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest kineProfileManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Error == "" || manifest.MeasurementStart != nil {
		t.Fatal("the artifact must explain that no measurement-start marker was observed")
	}
}

func TestKineProfileCancellationDuringCapture(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "profiling-start")
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{}, 3)
	done := make(chan error, 1)
	go func() {
		done <- collectKineProfiles(ctx, marker, dir, []kineProfileTarget{{Name: "kine"}},
			[]kineProfilePoint{{Name: "first"}, {Name: "later", Offset: time.Hour}},
			func(ctx context.Context, _ kineProfileTarget, _ string) ([]byte, error) {
				started <- struct{}{}
				<-ctx.Done()
				return nil, ctx.Err()
			})
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("profile capture did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("capture cancellation must be surfaced: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("profiling must stop promptly rather than wait for the next capture point")
	}
}

func TestKineProfileValidation(t *testing.T) {
	if err := validateKineProfile("cpu", testKineProfile(t, "heap")); err == nil {
		t.Fatal("a heap profile must not be accepted as a CPU profile")
	}
	if err := validateKineProfile("goroutine", []byte("Forbidden")); err == nil {
		t.Fatal("an HTTP error body must not be accepted as a stack dump")
	}
	buffer := &profileBuffer{limit: 8}
	if _, err := io.Copy(buffer, io.LimitReader(strings.NewReader("123456789"), 9)); err == nil || buffer.Err() == nil {
		t.Fatal("streaming output must enforce its size limit")
	}
	if len(buffer.Bytes()) > 8 {
		t.Fatal("profile buffer exceeded its bound")
	}
}
