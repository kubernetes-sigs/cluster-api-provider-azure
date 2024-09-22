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

package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/env"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	testEnv       *env.TestEnvironment
	testEnvCancel func()
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	testEnv = env.NewTestEnvironment()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = log.IntoContext(ctx, logr.New(testEnv.Log))

	Expect(NewAzureMachinePoolReconciler(testEnv, testEnv.GetEventRecorderFor("azuremachinepool-reconciler"),
		reconciler.Timeouts{}, "", "", testEnv.CredentialCache).SetupWithManager(ctx, testEnv.Manager, controllers.Options{Options: controller.Options{MaxConcurrentReconciles: 1}})).To(Succeed())

	Expect(NewAzureMachinePoolMachineController(testEnv, testEnv.GetEventRecorderFor("azuremachinepoolmachine-reconciler"),
		reconciler.Timeouts{}, "", testEnv.CredentialCache).SetupWithManager(ctx, testEnv.Manager, controllers.Options{Options: controller.Options{MaxConcurrentReconciles: 1}})).To(Succeed())

	// +kubebuilder:scaffold:scheme

	mgrCtx, mgrCancel := context.WithCancel(context.Background())
	testEnvCancel = mgrCancel

	By("starting the manager")
	go func() {
		defer GinkgoRecover()
		Expect(testEnv.StartManager(mgrCtx)).To(Succeed())
	}()

	Eventually(func() bool {
		nodes := &corev1.NodeList{}
		if err := testEnv.Client.List(ctx, nodes); err != nil {
			return false
		}
		return true
	}).Should(BeTrue())
})

var _ = AfterSuite(func() {
	testEnvCancel()
	if testEnv != nil {
		By("tearing down the test environment")
		Expect(testEnv.Stop()).To(Succeed())
	}
})
