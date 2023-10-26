/*
Copyright 2020 The Kubernetes Authors.

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

package env

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	logger                 = record.NewLogger(record.WithThreshold(ptr.To(1)), record.WithWriter(ginkgo.GinkgoWriter))
	scheme                 = runtime.NewScheme()
	env                    *envtest.Environment
	clusterAPIVersionRegex = regexp.MustCompile(`^(\W)sigs\.k8s\.io/cluster-api v(.+)`)
)

func init() {
	// Calculate the scheme.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(expv1.AddToScheme(scheme))
	utilruntime.Must(infrav1.AddToScheme(scheme))
	utilruntime.Must(infrav1exp.AddToScheme(scheme))

	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0) //nolint:dogsled // Ignore "declaration has 3 blank identifiers" check.
	root := path.Join(path.Dir(filename), "..", "..", "..")

	crdPaths := []string{
		filepath.Join(root, "config", "crd", "bases"),
	}

	if capiPath := getFilePathToCAPICRDs(root); capiPath != "" {
		crdPaths = append(crdPaths, capiPath)
	}

	// Create the test environment.
	env = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     crdPaths,
	}
}

type (
	// TestEnvironment encapsulates a Kubernetes local test environment.
	TestEnvironment struct {
		manager.Manager
		client.Client
		Config      *rest.Config
		Log         logr.LogSink
		LogRecorder *record.Logger
		doneMgr     chan struct{}
	}
)

// NewTestEnvironment creates a new environment spinning up a local api-server.
//
// This function should be called only once for each package you're running tests within,
// usually the environment is initialized in a suite_test.go file within a `BeforeSuite` ginkgo block.
func NewTestEnvironment() *TestEnvironment {
	if _, err := env.Start(); err != nil {
		panic(err)
	}

	mgr, err := manager.New(env.Config, manager.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Fatalf("Failed to start testenv manager: %v", err)
	}

	return &TestEnvironment{
		Manager:     mgr,
		Client:      mgr.GetClient(),
		Config:      mgr.GetConfig(),
		LogRecorder: logger,
		Log:         logger,
		doneMgr:     make(chan struct{}),
	}
}

// StartManager starts the test environment manager and blocks until the context is canceled.
func (t *TestEnvironment) StartManager(ctx context.Context) error {
	return t.Manager.Start(ctx)
}

// Stop stops the test environment.
func (t *TestEnvironment) Stop() error {
	return env.Stop()
}

func getFilePathToCAPICRDs(root string) string {
	modBits, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	var clusterAPIVersion string
	for _, line := range strings.Split(string(modBits), "\n") {
		matches := clusterAPIVersionRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			clusterAPIVersion = matches[2]
		}
	}

	if clusterAPIVersion == "" {
		return ""
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@v%s", clusterAPIVersion), "config", "crd", "bases")
}

func envOr(envKey, defaultValue string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	return defaultValue
}
