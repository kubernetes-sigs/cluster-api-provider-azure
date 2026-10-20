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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	pprofprofile "github.com/google/pprof/profile"
	"github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

const (
	kineProfilerPodName = "collector"
	kineProfilerImage   = "curlimages/curl:8.17.0@sha256:43ebaa53d3806db6b1ce4353b6b26ae638ec1c167ee351524b05690f988bb20d"
	maxProfileBytes     = 64 << 20
)

type kineProfileTarget struct {
	Name      string `json:"name"`
	Component string `json:"component"`
	URL       string `json:"url"`
}

type kineProfilePoint struct {
	Name   string
	Offset time.Duration
}

type kineProfileCapture struct {
	Target      string    `json:"target"`
	Kind        string    `json:"kind"`
	File        string    `json:"file,omitempty"`
	ScheduledAt time.Time `json:"scheduledAt"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	Bytes       int       `json:"bytes"`
	Error       string    `json:"error,omitempty"`
}

type kineProfileManifest struct {
	MeasurementStart *time.Time           `json:"measurementStart,omitempty"`
	Targets          []kineProfileTarget  `json:"targets"`
	Captures         []kineProfileCapture `json:"captures"`
	Error            string               `json:"error,omitempty"`
}

type kineProfileFetcher func(context.Context, kineProfileTarget, string) ([]byte, error)

type kineProfiler struct {
	client    kubernetes.Interface
	config    *rest.Config
	namespace string
	directory string
	cancel    context.CancelFunc
	done      chan struct{}
	err       error
}

func runKinePerformance(ctx context.Context, workload framework.ClusterProxy, reportDir string) (retErr error) {
	ca, err := base64.StdEncoding.DecodeString(os.Getenv("KINE_CA_CERT_B64"))
	if err != nil {
		return fmt.Errorf("decode public Kine profiling CA: %w", err)
	}
	if len(ca) == 0 {
		return errors.New("profiling requires KINE_CA_CERT_B64")
	}
	profiler, err := startKineProfiler(ctx, workload.GetClientSet(), workload.GetRESTConfig(), reportDir, ca)
	if err != nil {
		return err
	}
	defer func() {
		profiler.stop()
		retErr = errors.Join(retErr, profiler.cleanup())
	}()

	command := exec.CommandContext(ctx, "bash", "../../scripts/kine/run-clusterloader.sh")
	command.Env = append(command.Environ(),
		"KUBECONFIG="+workload.GetKubeconfigPath(),
		"CL2_REPORT_DIR="+reportDir,
	)
	command.Stdout = ginkgo.GinkgoWriter
	command.Stderr = ginkgo.GinkgoWriter
	runErr := command.Run()
	profiler.stop()
	snapshotErr := verifyPrometheusSnapshot(reportDir)
	return errors.Join(runErr, profiler.err, snapshotErr)
}

func startKineProfiler(ctx context.Context, client kubernetes.Interface, config *rest.Config, reportDir string, ca []byte) (_ *kineProfiler, retErr error) {
	if err := os.MkdirAll(reportDir, 0o750); err != nil {
		return nil, err
	}
	marker := filepath.Join(reportDir, "profiling-start")
	if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove stale profiling marker: %w", err)
	}
	dir, err := os.MkdirTemp(reportDir, "profiles-")
	if err != nil {
		return nil, err
	}
	setupCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	nodes, err := client.CoreV1().Nodes().List(setupCtx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane"})
	if err != nil {
		return nil, fmt.Errorf("discover API servers for profiling: %w", err)
	}
	targets, err := kineProfileTargets(nodes.Items)
	if err != nil {
		return nil, err
	}
	namespace, err := client.CoreV1().Namespaces().Create(setupCtx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kine-profiles-" + util.RandomString(6)},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create profiling namespace: %w", err)
	}
	p := &kineProfiler{client: client, config: config, namespace: namespace.Name, directory: dir}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, p.cleanup())
		}
	}()
	if err := createKineProfilerResources(setupCtx, client, namespace, ca); err != nil {
		return nil, err
	}
	lastPhase := corev1.PodUnknown
	err = wait.PollUntilContextCancel(setupCtx, time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := client.CoreV1().Pods(namespace.Name).Get(ctx, kineProfilerPodName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		lastPhase = pod.Status.Phase
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("wait for profiling Pod %s/%s (phase %s): %w", namespace.Name, kineProfilerPodName, lastPhase, err)
	}
	profileCtx, cancelProfiles := context.WithCancel(ctx)
	p.cancel = cancelProfiles
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		p.err = collectKineProfiles(profileCtx, marker, dir, targets, []kineProfilePoint{
			{Name: "minute-04", Offset: 4 * time.Minute},
			{Name: "minute-07", Offset: 7 * time.Minute},
		}, p.fetch)
	}()
	return p, nil
}

func createKineProfilerResources(ctx context.Context, client kubernetes.Interface, namespace *corev1.Namespace, ca []byte) error {
	owner := []metav1.OwnerReference{{APIVersion: "v1", Kind: "Namespace", Name: namespace.Name, UID: namespace.UID}}
	if _, err := client.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: namespace.Name, OwnerReferences: owner},
		Rules: []rbacv1.PolicyRule{{
			Verbs: []string{"get"},
			NonResourceURLs: []string{
				"/debug/pprof/profile", "/debug/pprof/heap", "/debug/pprof/goroutine",
			},
		}},
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create profiling-only role: %w", err)
	}
	if _, err := client.CoreV1().ServiceAccounts(namespace.Name).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: kineProfilerPodName},
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create profiling service account: %w", err)
	}
	if _, err := client.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: namespace.Name, OwnerReferences: owner},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: namespace.Name},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: kineProfilerPodName, Namespace: namespace.Name}},
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("bind profiling service account: %w", err)
	}
	if _, err := client.CoreV1().Secrets(namespace.Name).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "kine-ca"},
		Data:       map[string][]byte{"ca.crt": ca},
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create public profiling CA: %w", err)
	}
	if _, err := client.CoreV1().Pods(namespace.Name).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: kineProfilerPodName},
		Spec: corev1.PodSpec{
			ServiceAccountName:            kineProfilerPodName,
			AutomountServiceAccountToken:  ptr.To(true),
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To(int64(1)),
			NodeSelector:                  map[string]string{"kubernetes.io/os": "linux", "kubernetes.io/arch": "amd64"},
			Affinity: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.NodeSelectorOpDoesNotExist}},
				}}},
			}},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true), RunAsUser: ptr.To(int64(100)), RunAsGroup: ptr.To(int64(101)),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name: kineProfilerPodName, Image: kineProfilerImage,
				Command: []string{"/bin/sh", "-c", "exec sleep 86400"},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false), ReadOnlyRootFilesystem: ptr.To(true),
					Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("64Mi")},
					Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("128Mi")},
				},
				VolumeMounts: []corev1.VolumeMount{{Name: "kine-ca", MountPath: "/kine-ca", ReadOnly: true}},
			}},
			Volumes: []corev1.Volume{{
				Name: "kine-ca", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "kine-ca"}},
			}},
		},
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create private profiling helper: %w", err)
	}
	return nil
}

func kineProfileTargets(nodes []corev1.Node) ([]kineProfileTarget, error) {
	if len(nodes) == 0 {
		return nil, errors.New("no control-plane nodes available for profiling")
	}
	targets := []kineProfileTarget{{Name: "kine", Component: "kine", URL: "https://10.250.0.4:8080"}}
	subnet := netip.MustParsePrefix("10.0.0.0/24")
	for _, node := range nodes {
		address := ""
		for _, candidate := range node.Status.Addresses {
			if candidate.Type != corev1.NodeInternalIP {
				continue
			}
			ip, err := netip.ParseAddr(candidate.Address)
			if err == nil && subnet.Contains(ip) {
				address = ip.String()
				break
			}
		}
		if address == "" {
			return nil, fmt.Errorf("node %s has no private address in the Kine control-plane subnet", node.Name)
		}
		targets = append(targets, kineProfileTarget{
			Name: "apiserver-" + node.Name, Component: "apiserver",
			URL: "https://" + net.JoinHostPort(address, "6443"),
		})
	}
	return targets, nil
}

func (p *kineProfiler) stop() {
	if p.cancel != nil {
		p.cancel()
		<-p.done
	}
}

func (p *kineProfiler) cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := p.client.CoreV1().Namespaces().Delete(ctx, p.namespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete profiling namespace %s: %w", p.namespace, err)
	}
	return nil
}

func (p *kineProfiler) fetch(ctx context.Context, target kineProfileTarget, kind string) ([]byte, error) {
	suffix := map[string]string{"cpu": "profile?seconds=30", "heap": "heap?gc=0", "goroutine": "goroutine?debug=2"}[kind]
	if suffix == "" {
		return nil, fmt.Errorf("unknown profile type %q", kind)
	}
	// Keep the token inside the helper and out of argv; never use a TTY for binary profiles.
	const curl = `set -eu
case "$1" in
  apiserver)
    token=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    printf 'Authorization: Bearer %s\n' "$token" |
      curl --fail --silent --show-error --max-time 75 --header @- \
        --cacert /var/run/secrets/kubernetes.io/serviceaccount/ca.crt "$2" ;;
  kine)
    exec curl --fail --silent --show-error --max-time 75 --cacert /kine-ca/ca.crt "$2" ;;
  *) echo "Unsupported profiling target" >&2; exit 1 ;;
esac`
	request := p.client.CoreV1().RESTClient().Post().Namespace(p.namespace).Resource("pods").
		Name(kineProfilerPodName).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: kineProfilerPodName,
			Command:   []string{"/bin/sh", "-c", curl, "profile", target.Component, target.URL + "/debug/pprof/" + suffix},
			Stdout:    true, Stderr: true,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(p.config, "POST", request.URL())
	if err != nil {
		return nil, err
	}
	stdout := &profileBuffer{limit: maxProfileBytes}
	stderr := &profileBuffer{limit: 4096}
	streamErr := executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: stdout, Stderr: stderr})
	if err := errors.Join(streamErr, stdout.Err(), stderr.Err()); err != nil {
		return nil, fmt.Errorf("fetch %s %s profile: %w; %s", target.Name, kind, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

type profileBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	limit  int
	err    error
}

func (b *profileBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(data) > b.limit-b.buffer.Len() {
		b.err = errors.New("profile output exceeds its size limit")
		return 0, b.err
	}
	return b.buffer.Write(data)
}

func (b *profileBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.buffer.Bytes())
}

func (b *profileBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *profileBuffer) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

func collectKineProfiles(ctx context.Context, marker, directory string, targets []kineProfileTarget, points []kineProfilePoint, fetch kineProfileFetcher) (retErr error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	manifest := kineProfileManifest{Targets: targets, Captures: []kineProfileCapture{}}
	save := func() error {
		sort.Slice(manifest.Captures, func(i, j int) bool {
			return manifest.Captures[i].ScheduledAt.Before(manifest.Captures[j].ScheduledAt) ||
				(manifest.Captures[i].ScheduledAt.Equal(manifest.Captures[j].ScheduledAt) &&
					manifest.Captures[i].Target+manifest.Captures[i].Kind < manifest.Captures[j].Target+manifest.Captures[j].Kind)
		})
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		if err := root.WriteFile("captures.json.tmp", data, 0o600); err != nil {
			return err
		}
		return root.Rename("captures.json.tmp", "captures.json")
	}
	defer func() {
		if retErr != nil {
			manifest.Error = retErr.Error()
		}
		retErr = errors.Join(retErr, save())
	}()
	var start time.Time
	if err := wait.PollUntilContextCancel(ctx, 100*time.Millisecond, true, func(context.Context) (bool, error) {
		info, err := os.Stat(marker)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, errors.New("profiling measurement-start marker is not a regular file")
		}
		start = info.ModTime().UTC()
		return true, nil
	}); err != nil {
		return fmt.Errorf("profiling measurement-start marker was not observed: %w", err)
	}
	manifest.MeasurementStart = &start
	var captureErrors []error
	for _, point := range points {
		scheduled := start.Add(point.Offset)
		timer := time.NewTimer(time.Until(scheduled))
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(append(captureErrors, fmt.Errorf("benchmark ended before %s profiling: %w", point.Name, ctx.Err()))...)
		case <-timer.C:
		}
		if err := root.MkdirAll(point.Name, 0o700); err != nil {
			return errors.Join(append(captureErrors, err)...)
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, target := range targets {
			for _, kind := range []string{"cpu", "heap", "goroutine"} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					capture := kineProfileCapture{Target: target.Name, Kind: kind, ScheduledAt: scheduled, StartedAt: time.Now().UTC()}
					requestCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
					defer cancel()
					data, err := fetch(requestCtx, target, kind)
					if err == nil {
						err = validateKineProfile(kind, data)
					}
					if err == nil {
						extension := ".pprof"
						if kind == "goroutine" {
							extension = ".txt"
						}
						file := filepath.Join(point.Name, target.Name+"-"+kind+extension)
						err = root.WriteFile(file, data, 0o600)
						if err == nil {
							capture.File, capture.Bytes = file, len(data)
						}
					}
					capture.FinishedAt = time.Now().UTC()
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						capture.Error = err.Error()
						captureErrors = append(captureErrors, fmt.Errorf("%s %s %s: %w", point.Name, target.Name, kind, err))
					}
					manifest.Captures = append(manifest.Captures, capture)
				}()
			}
		}
		wg.Wait()
		if err := save(); err != nil {
			captureErrors = append(captureErrors, err)
		}
	}
	return errors.Join(captureErrors...)
}

func validateKineProfile(kind string, data []byte) error {
	if kind == "goroutine" {
		if !bytes.HasPrefix(data, []byte("goroutine ")) {
			return errors.New("response is not a goroutine stack dump")
		}
		return nil
	}
	profile, err := pprofprofile.ParseData(data)
	if err != nil {
		return fmt.Errorf("response is not a valid %s pprof profile: %w", kind, err)
	}
	expected := "cpu"
	if kind == "heap" {
		expected = "inuse_space"
	}
	for _, sampleType := range profile.SampleType {
		if sampleType.Type == expected {
			return nil
		}
	}
	return fmt.Errorf("%s profile has no %s samples", kind, expected)
}
