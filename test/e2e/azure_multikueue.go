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
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultKueueVersion  = "0.18.2"
	defaultJobSetVersion = "0.12.0"

	kueueHelmChart       = "oci://registry.k8s.io/kueue/charts/kueue"
	kueueHelmReleaseName = "kueue"
	kueueNamespace       = "kueue-system"
	kueueControllerName  = "kueue-controller-manager"

	jobSetHelmChart       = "oci://registry.k8s.io/jobset/charts/jobset"
	jobSetHelmReleaseName = "jobset"
	jobSetNamespace       = "jobset-system"
	jobSetControllerName  = "jobset-controller"

	multiKueueServiceAccountName = "multikueue-sa"
	multiKueueKubeconfigKey      = "kubeconfig"
	multiKueueControllerName     = "kueue.x-k8s.io/multikueue"
	multiKueueQueueLabel         = "kueue.x-k8s.io/queue-name"
	multiKueuePrebuiltWorkload   = "kueue.x-k8s.io/prebuilt-workload-name"
	multiKueueWorkerCount        = 3
	multiKueueDefaultStressJobs  = 8

	multiKueueLocalQueueName = "user-queue"
	multiKueueJobImage       = "registry.k8s.io/e2e-test-images/agnhost:2.53"
	multiKueueJobCommand     = "/bin/sh"
	multiKueueJobArgs        = "sleep 120"

	multiKueueAKSFlavorVariable          = "MULTIKUEUE_AKS_FLAVOR"
	multiKueueKueueVersionVariable       = "KUEUE_VERSION"
	multiKueueJobSetVersionVariable      = "JOBSET_VERSION"
	multiKueueWorkerClusterCountVariable = "MULTIKUEUE_WORKER_CLUSTER_COUNT"
	multiKueueStressJobCountVariable     = "MULTIKUEUE_STRESS_JOB_COUNT"
)

var (
	kueueResourceFlavorGVR = schema.GroupVersionResource{Group: "kueue.x-k8s.io", Version: "v1beta2", Resource: "resourceflavors"}
	kueueClusterQueueGVR   = schema.GroupVersionResource{Group: "kueue.x-k8s.io", Version: "v1beta2", Resource: "clusterqueues"}
	kueueLocalQueueGVR     = schema.GroupVersionResource{Group: "kueue.x-k8s.io", Version: "v1beta2", Resource: "localqueues"}
	kueueAdmissionCheckGVR = schema.GroupVersionResource{Group: "kueue.x-k8s.io", Version: "v1beta2", Resource: "admissionchecks"}
	kueueMultiConfigGVR    = schema.GroupVersionResource{Group: "kueue.x-k8s.io", Version: "v1beta2", Resource: "multikueueconfigs"}
	kueueMultiClusterGVR   = schema.GroupVersionResource{Group: "kueue.x-k8s.io", Version: "v1beta2", Resource: "multikueueclusters"}
	kueueWorkloadGVR       = schema.GroupVersionResource{Group: "kueue.x-k8s.io", Version: "v1beta2", Resource: "workloads"}
	batchJobGVR            = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	jobSetGVR              = schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"}
)

// MultiKueueSpecInput is the input for MultiKueueSpec.
type MultiKueueSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ManagerClusterName    string
	WorkerClusterNames    []string
	KueueVersion          string
	JobSetVersion         string
	StressJobCount        int
	SkipCleanup           bool
}

type multiKueueCluster struct {
	Name      string
	Proxy     framework.ClusterProxy
	Clientset *kubernetes.Clientset
	Dynamic   dynamic.Interface
}

type multiKueueWorker struct {
	Cluster *multiKueueCluster
	Source  multiKueueWorkerSource
}

type multiKueueWorkerSource struct {
	Name       string
	SecretName string
	// ClusterSource is intentionally generic so Fleet-provided ClusterProfile refs can replace kubeconfig secrets.
	ClusterSource map[string]interface{}
}

type multiKueueConnectionProvider interface {
	PrepareWorker(ctx context.Context, manager, worker *multiKueueCluster, sourceName, secretName string) multiKueueWorkerSource
}

type kubeconfigSecretMultiKueueConnectionProvider struct {
	specName string
}

type multiKueueSetupNames struct {
	Namespace           string
	ResourceFlavorName  string
	ClusterQueueName    string
	LocalQueueName      string
	AdmissionCheckName  string
	MultiKueueConfig    string
	WorkerClusterPrefix string
}

type multiKueueEnvironment struct {
	specName           string
	setup              multiKueueSetupNames
	manager            *multiKueueCluster
	workers            []multiKueueWorker
	workersByMKCName   map[string]multiKueueWorker
	connectionProvider multiKueueConnectionProvider
	workerCPUQuota     string
	workerMemoryQuota  string
	managerCPUQuota    string
	managerMemoryQuota string
}

// MultiKueueSpec installs Kueue and JobSet on CAPZ-managed AKS clusters and validates MultiKueue dispatch.
func MultiKueueSpec(ctx context.Context, inputGetter func() MultiKueueSpecInput) {
	specName := "multikueue"
	input := inputGetter()

	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ManagerClusterName).NotTo(BeEmpty(), "Invalid argument. input.ManagerClusterName can't be empty when calling %s spec", specName)
	Expect(len(input.WorkerClusterNames)).To(BeNumerically(">=", 2), "MultiKueue requires at least two worker clusters")

	if input.KueueVersion == "" {
		input.KueueVersion = defaultKueueVersion
	}
	if input.JobSetVersion == "" {
		input.JobSetVersion = defaultJobSetVersion
	}
	if input.StressJobCount == 0 {
		input.StressJobCount = multiKueueDefaultStressJobs
	}

	manager := newMultiKueueCluster(ctx, input.BootstrapClusterProxy, input.Namespace.Name, input.ManagerClusterName)
	workers := make([]*multiKueueCluster, 0, len(input.WorkerClusterNames))
	for _, workerClusterName := range input.WorkerClusterNames {
		workers = append(workers, newMultiKueueCluster(ctx, input.BootstrapClusterProxy, input.Namespace.Name, workerClusterName))
	}

	By("installing Kueue and JobSet on the MultiKueue manager and workers")
	for _, cluster := range append([]*multiKueueCluster{manager}, workers...) {
		InstallJobSet(ctx, cluster.Proxy, specName, input.JobSetVersion)
		InstallKueue(ctx, cluster.Proxy, specName, input.KueueVersion)
		ensureNamespace(ctx, cluster.Clientset, input.Namespace.Name)
	}

	env := newMultiKueueEnvironment(specName, input.Namespace.Name, manager, workers)

	By("configuring worker queues and manager worker attachments")
	env.configureWorkers(ctx)
	env.configureManager(ctx)
	env.waitForSetupActive(ctx)

	By("validating AllAtOnce fan-out for workloads that fit manager quota but not any single worker")
	env.validateAllAtOnceFanOut(ctx)

	By("validating quota exhaustion stays queued on the manager")
	env.validateManagerQuotaExhaustion(ctx)

	By("validating exactly-once batch/Job dispatch, status sync, and remote cleanup")
	env.runBatchJobScenario(ctx, "multikueue-job", "100m", multiKueueJobArgs, nil)

	By("validating exactly-once JobSet dispatch, status sync, and remote cleanup")
	env.runJobSetScenario(ctx, "multikueue-jobset")

	By("validating worker disconnect and recovery")
	env.validateWorkerDisconnectAndRecovery(ctx)

	By("validating concurrent workload stress")
	env.runStressScenario(ctx, input.StressJobCount)

	if !input.SkipCleanup {
		By("deleting MultiKueue test namespaces")
		for _, cluster := range append([]*multiKueueCluster{manager}, workers...) {
			err := cluster.Clientset.CoreV1().Namespaces().Delete(ctx, input.Namespace.Name, metav1.DeleteOptions{})
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		}
	}
}

// InstallKueue installs the Kueue Helm chart and waits for the controller manager.
func InstallKueue(ctx context.Context, clusterProxy framework.ClusterProxy, specName, version string) {
	valuesFile := writeKueueValuesFile()
	defer func() {
		Expect(os.Remove(valuesFile)).To(Succeed())
	}()

	InstallOCIHelmChart(ctx, clusterProxy, kueueNamespace, kueueHelmChart, kueueHelmReleaseName,
		"--version", version,
		"--values", valuesFile,
	)

	waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, kueueControllerName, kueueNamespace, specName)
	WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

func writeKueueValuesFile() string {
	valuesFile, err := os.CreateTemp("", "kueue-values-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	defer valuesFile.Close()

	_, err = valuesFile.WriteString(`controllerManager:
  manager:
    image:
      pullPolicy: IfNotPresent
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 512Mi
managerConfig:
  controllerManagerConfigYaml: |-
    apiVersion: config.kueue.x-k8s.io/v1beta2
    kind: Configuration
    health:
      healthProbeBindAddress: :8081
    metrics:
      bindAddress: :8443
    webhook:
      port: 9443
    leaderElection:
      leaderElect: true
      resourceName: c1f6bfd2.kueue.x-k8s.io
    controller:
      groupKindConcurrency:
        Job.batch: 5
        Workload.kueue.x-k8s.io: 5
        LocalQueue.kueue.x-k8s.io: 1
        Cohort.kueue.x-k8s.io: 1
        ClusterQueue.kueue.x-k8s.io: 1
        ResourceFlavor.kueue.x-k8s.io: 1
    clientConnection:
      qps: 50
      burst: 100
    integrations:
      frameworks:
      - "batch/job"
      - "jobset.x-k8s.io/jobset"
`)
	Expect(err).NotTo(HaveOccurred())
	return valuesFile.Name()
}

// InstallJobSet installs the JobSet Helm chart and waits for the controller manager.
func InstallJobSet(ctx context.Context, clusterProxy framework.ClusterProxy, specName, version string) {
	InstallOCIHelmChart(ctx, clusterProxy, jobSetNamespace, jobSetHelmChart, jobSetHelmReleaseName,
		"--version", version,
		"--set", "controller.resources.requests.cpu=100m",
		"--set", "controller.resources.requests.memory=64Mi",
		"--set", "controller.resources.limits.cpu=500m",
		"--set", "controller.resources.limits.memory=512Mi",
	)

	waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, jobSetControllerName, jobSetNamespace, specName)
	WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

// InstallOCIHelmChart installs an OCI Helm chart on a workload cluster.
func InstallOCIHelmChart(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, chartName, releaseName string, extraArgs ...string) {
	kubeconfigPath := clusterProxy.GetKubeconfigPath()
	Byf("Installing Helm chart %s as release %s using kubeconfig %s", chartName, releaseName, kubeconfigPath)

	helmArgs := []string{"install", releaseName, chartName,
		"--namespace", namespace,
		"--create-namespace",
		"--timeout", "10m0s",
	}
	helmArgs = append(helmArgs, extraArgs...)
	installCmd := exec.CommandContext(ctx, "helm", helmArgs...) //nolint:gosec
	installCmd.Env = append(installCmd.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := installCmd.CombinedOutput()
	Logf("helm install output: %s", string(output))
	Expect(err).NotTo(HaveOccurred(), "failed to install Helm chart %s: %s", chartName, string(output))
}

func newMultiKueueCluster(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, namespace, clusterName string) *multiKueueCluster {
	clusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	return &multiKueueCluster{
		Name:      clusterName,
		Proxy:     clusterProxy,
		Clientset: clientset,
		Dynamic:   newDynamicClient(clusterProxy),
	}
}

func newMultiKueueEnvironment(specName, namespace string, manager *multiKueueCluster, workerClusters []*multiKueueCluster) *multiKueueEnvironment {
	setup := multiKueueSetupNames{
		Namespace:           namespace,
		ResourceFlavorName:  namespace + "-flavor",
		ClusterQueueName:    namespace + "-cluster-queue",
		LocalQueueName:      multiKueueLocalQueueName,
		AdmissionCheckName:  namespace + "-multikueue",
		MultiKueueConfig:    namespace + "-multikueue",
		WorkerClusterPrefix: namespace + "-worker",
	}
	workers := make([]multiKueueWorker, 0, len(workerClusters))
	for i, worker := range workerClusters {
		sourceName := fmt.Sprintf("%s-%d", setup.WorkerClusterPrefix, i+1)
		workers = append(workers, multiKueueWorker{
			Cluster: worker,
			Source: multiKueueWorkerSource{
				Name:       sourceName,
				SecretName: sourceName + "-secret",
			},
		})
	}

	return &multiKueueEnvironment{
		specName:           specName,
		setup:              setup,
		manager:            manager,
		workers:            workers,
		workersByMKCName:   map[string]multiKueueWorker{},
		connectionProvider: kubeconfigSecretMultiKueueConnectionProvider{specName: specName},
		workerCPUQuota:     "1",
		workerMemoryQuota:  "2Gi",
		managerCPUQuota:    strconv.Itoa(len(workers)),
		managerMemoryQuota: fmt.Sprintf("%dGi", len(workers)*2),
	}
}

func (p kubeconfigSecretMultiKueueConnectionProvider) PrepareWorker(ctx context.Context, manager, worker *multiKueueCluster, sourceName, secretName string) multiKueueWorkerSource {
	ensureMultiKueueWorkerRBAC(ctx, worker.Clientset)
	kubeconfig := buildMultiKueueWorkerKubeconfig(ctx, worker, p.specName)
	createOrUpdateKubeconfigSecret(ctx, manager.Clientset, secretName, kubeconfig)
	return multiKueueWorkerSource{
		Name:       sourceName,
		SecretName: secretName,
		ClusterSource: map[string]interface{}{
			"kubeConfig": map[string]interface{}{
				"locationType": "Secret",
				"location":     secretName,
			},
		},
	}
}

func (e *multiKueueEnvironment) configureWorkers(ctx context.Context) {
	for i := range e.workers {
		worker := &e.workers[i]
		worker.Source = e.connectionProvider.PrepareWorker(ctx, e.manager, worker.Cluster, worker.Source.Name, worker.Source.SecretName)
		e.workersByMKCName[worker.Source.Name] = *worker
		createMultiKueueQueues(ctx, worker.Cluster.Dynamic, e.setup, e.workerCPUQuota, e.workerMemoryQuota, "")
	}
}

func (e *multiKueueEnvironment) configureManager(ctx context.Context) {
	workerNames := make([]string, 0, len(e.workers))
	for _, worker := range e.workers {
		workerNames = append(workerNames, worker.Source.Name)
		createOrUpdateUnstructured(ctx, e.manager.Dynamic, kueueMultiClusterGVR, "", newMultiKueueClusterObject(worker.Source.Name, worker.Source.ClusterSource))
	}
	createOrUpdateUnstructured(ctx, e.manager.Dynamic, kueueMultiConfigGVR, "", newMultiKueueConfig(e.setup.MultiKueueConfig, workerNames))
	createOrUpdateUnstructured(ctx, e.manager.Dynamic, kueueAdmissionCheckGVR, "", newAdmissionCheck(e.setup.AdmissionCheckName, e.setup.MultiKueueConfig))
	createMultiKueueQueues(ctx, e.manager.Dynamic, e.setup, e.managerCPUQuota, e.managerMemoryQuota, e.setup.AdmissionCheckName)
}

func (e *multiKueueEnvironment) waitForSetupActive(ctx context.Context) {
	waitForActiveCondition(ctx, e.manager.Dynamic, kueueClusterQueueGVR, "", e.setup.ClusterQueueName, "True", e.specName)
	waitForActiveCondition(ctx, e.manager.Dynamic, kueueAdmissionCheckGVR, "", e.setup.AdmissionCheckName, "True", e.specName)
	for _, worker := range e.workers {
		waitForActiveCondition(ctx, e.manager.Dynamic, kueueMultiClusterGVR, "", worker.Source.Name, "True", e.specName)
		waitForActiveCondition(ctx, worker.Cluster.Dynamic, kueueClusterQueueGVR, "", e.setup.ClusterQueueName, "True", e.specName)
		waitForActiveCondition(ctx, worker.Cluster.Dynamic, kueueLocalQueueGVR, e.setup.Namespace, e.setup.LocalQueueName, "True", e.specName)
	}
}

func (e *multiKueueEnvironment) validateAllAtOnceFanOut(ctx context.Context) {
	job := newMultiKueueBatchJob("multikueue-all-at-once", e.setup.Namespace, "2", "sleep 300")
	createdJob := e.createBatchJob(ctx, job)

	workload := waitForWorkloadForOwner(ctx, e.manager.Dynamic, e.setup.Namespace, createdJob.UID, e.specName)
	Eventually(func(g Gomega) {
		refreshed := getWorkload(ctx, e.manager.Dynamic, e.setup.Namespace, workload.GetName())
		g.Expect(workloadClusterName(refreshed)).To(BeEmpty())
		g.Expect(workloadNominatedClusterNames(refreshed)).To(ConsistOf(e.workerNames()))
		for _, worker := range e.workers {
			_, err := worker.Cluster.Dynamic.Resource(kueueWorkloadGVR).Namespace(e.setup.Namespace).Get(ctx, workload.GetName(), metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "expected AllAtOnce to fan out workload to %s", worker.Source.Name)
		}
	}, e2eConfig.GetIntervals(e.specName, "wait-workload-admitted")...).Should(Succeed())

	e.deleteBatchJob(ctx, createdJob.Name)
	e.deleteWorkloadEverywhere(ctx, workload.GetName())
}

func (e *multiKueueEnvironment) validateManagerQuotaExhaustion(ctx context.Context) {
	cpuRequest := strconv.Itoa(len(e.workers) + 1)
	job := newMultiKueueBatchJob("multikueue-quota-exhausted", e.setup.Namespace, cpuRequest, multiKueueJobArgs)
	createdJob := e.createBatchJob(ctx, job)

	workload := waitForWorkloadForOwner(ctx, e.manager.Dynamic, e.setup.Namespace, createdJob.UID, e.specName)
	Eventually(func(g Gomega) {
		refreshed := getWorkload(ctx, e.manager.Dynamic, e.setup.Namespace, workload.GetName())
		g.Expect(workloadClusterName(refreshed)).To(BeEmpty())
		g.Expect(workloadHasAdmission(refreshed)).To(BeFalse())
		for _, worker := range e.workers {
			err := workloadNotFound(ctx, worker.Cluster.Dynamic, e.setup.Namespace, workload.GetName())
			g.Expect(err).NotTo(HaveOccurred())
		}
	}, e2eConfig.GetIntervals(e.specName, "wait-workload-pending")...).Should(Succeed())

	e.deleteBatchJob(ctx, createdJob.Name)
	e.deleteWorkloadEverywhere(ctx, workload.GetName())
}

func (e *multiKueueEnvironment) validateWorkerDisconnectAndRecovery(ctx context.Context) {
	disconnected := e.workers[0]
	missingSecret := disconnected.Source.SecretName + "-missing"

	patchMultiKueueClusterSecret(ctx, e.manager.Dynamic, disconnected.Source.Name, missingSecret)
	waitForActiveCondition(ctx, e.manager.Dynamic, kueueMultiClusterGVR, "", disconnected.Source.Name, "False", e.specName)

	winner := e.runBatchJobScenario(ctx, "multikueue-recovery", "100m", multiKueueJobArgs, map[string]struct{}{disconnected.Source.Name: {}})
	Expect(winner).NotTo(Equal(disconnected.Source.Name), "workload should not dispatch to a disconnected worker")

	disconnected.Source = e.connectionProvider.PrepareWorker(ctx, e.manager, disconnected.Cluster, disconnected.Source.Name, disconnected.Source.SecretName)
	e.workers[0] = disconnected
	e.workersByMKCName[disconnected.Source.Name] = disconnected
	patchMultiKueueClusterSecret(ctx, e.manager.Dynamic, disconnected.Source.Name, disconnected.Source.SecretName)
	waitForActiveCondition(ctx, e.manager.Dynamic, kueueMultiClusterGVR, "", disconnected.Source.Name, "True", e.specName)
}

func (e *multiKueueEnvironment) runStressScenario(ctx context.Context, count int) {
	jobs := make([]*batchv1.Job, 0, count)
	workloads := make([]*unstructured.Unstructured, 0, count)

	for i := 0; i < count; i++ {
		job := newMultiKueueBatchJob(fmt.Sprintf("multikueue-stress-%02d", i), e.setup.Namespace, "100m", multiKueueJobArgs)
		jobs = append(jobs, e.createBatchJob(ctx, job))
	}

	for _, job := range jobs {
		workload := waitForWorkloadForOwner(ctx, e.manager.Dynamic, e.setup.Namespace, job.UID, e.specName)
		workloads = append(workloads, workload)
		winner := e.waitForWorkloadPlacement(ctx, workload.GetName(), nil)
		e.waitForRemoteAdmission(ctx, workload.GetName(), winner, batchJobGVR)
	}

	for i, job := range jobs {
		WaitForJobComplete(ctx, WaitForJobCompleteInput{
			Getter:    jobsClientAdapter{client: e.manager.Clientset.BatchV1().Jobs(e.setup.Namespace)},
			Job:       job,
			Clientset: e.manager.Clientset,
		}, e2eConfig.GetIntervals(e.specName, "wait-job-complete")...)
		waitForWorkloadFinished(ctx, e.manager.Dynamic, e.setup.Namespace, workloads[i].GetName(), e.specName)
		e.waitForNoRemoteObjects(ctx, workloads[i].GetName(), batchJobGVR)
	}
}

func (e *multiKueueEnvironment) runBatchJobScenario(ctx context.Context, name, cpuRequest, args string, excludedWorkers map[string]struct{}) string {
	job := newMultiKueueBatchJob(name, e.setup.Namespace, cpuRequest, args)
	createdJob := e.createBatchJob(ctx, job)

	workload := waitForWorkloadForOwner(ctx, e.manager.Dynamic, e.setup.Namespace, createdJob.UID, e.specName)
	winner := e.waitForWorkloadPlacement(ctx, workload.GetName(), excludedWorkers)
	e.waitForRemoteAdmission(ctx, workload.GetName(), winner, batchJobGVR)

	WaitForJobComplete(ctx, WaitForJobCompleteInput{
		Getter:    jobsClientAdapter{client: e.manager.Clientset.BatchV1().Jobs(e.setup.Namespace)},
		Job:       createdJob,
		Clientset: e.manager.Clientset,
	}, e2eConfig.GetIntervals(e.specName, "wait-job-complete")...)

	waitForWorkloadFinished(ctx, e.manager.Dynamic, e.setup.Namespace, workload.GetName(), e.specName)
	e.waitForNoRemoteObjects(ctx, workload.GetName(), batchJobGVR)
	e.deleteBatchJob(ctx, createdJob.Name)
	return winner
}

func (e *multiKueueEnvironment) runJobSetScenario(ctx context.Context, name string) {
	jobSet := newMultiKueueJobSet(name, e.setup.Namespace)
	createdJobSet, err := e.manager.Dynamic.Resource(jobSetGVR).Namespace(e.setup.Namespace).Create(ctx, jobSet, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	workload := waitForWorkloadForOwner(ctx, e.manager.Dynamic, e.setup.Namespace, createdJobSet.GetUID(), e.specName)
	winner := e.waitForWorkloadPlacement(ctx, workload.GetName(), nil)
	e.waitForRemoteAdmission(ctx, workload.GetName(), winner, jobSetGVR)
	waitForWorkloadFinished(ctx, e.manager.Dynamic, e.setup.Namespace, workload.GetName(), e.specName)
	e.waitForNoRemoteObjects(ctx, workload.GetName(), jobSetGVR)

	err = e.manager.Dynamic.Resource(jobSetGVR).Namespace(e.setup.Namespace).Delete(ctx, createdJobSet.GetName(), metav1.DeleteOptions{})
	Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
}

func (e *multiKueueEnvironment) createBatchJob(ctx context.Context, job *batchv1.Job) *batchv1.Job {
	var createdJob *batchv1.Job
	Eventually(func(g Gomega) {
		var err error
		createdJob, err = e.manager.Clientset.BatchV1().Jobs(e.setup.Namespace).Create(ctx, job.DeepCopy(), metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			createdJob, err = e.manager.Clientset.BatchV1().Jobs(e.setup.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, e2eConfig.GetIntervals(e.specName, "wait-workload-created")...).Should(Succeed())
	return createdJob
}

func (e *multiKueueEnvironment) waitForWorkloadPlacement(ctx context.Context, workloadName string, excludedWorkers map[string]struct{}) string {
	var winner string
	Eventually(func(g Gomega) {
		workload := getWorkload(ctx, e.manager.Dynamic, e.setup.Namespace, workloadName)
		winner = workloadClusterName(workload)
		g.Expect(winner).NotTo(BeEmpty(), "workload status: %v", workload.Object["status"])
		g.Expect(workloadNominatedClusterNames(workload)).To(BeEmpty())
		if excludedWorkers != nil {
			_, excluded := excludedWorkers[winner]
			g.Expect(excluded).To(BeFalse())
		}
	}, e2eConfig.GetIntervals(e.specName, "wait-workload-admitted")...).Should(Succeed())
	return winner
}

func (e *multiKueueEnvironment) waitForRemoteAdmission(ctx context.Context, workloadName, winnerName string, remoteGVR schema.GroupVersionResource) {
	winner, ok := e.workersByMKCName[winnerName]
	Expect(ok).To(BeTrue(), "manager chose unknown worker %q", winnerName)

	Eventually(func(g Gomega) {
		remoteWorkload, err := winner.Cluster.Dynamic.Resource(kueueWorkloadGVR).Namespace(e.setup.Namespace).Get(ctx, workloadName, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workloadHasAdmission(remoteWorkload)).To(BeTrue(), "remote workload status: %v", remoteWorkload.Object["status"])

		remoteObjects, err := listObjectsForPrebuiltWorkload(ctx, winner.Cluster.Dynamic, remoteGVR, e.setup.Namespace, workloadName)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(remoteObjects).To(HaveLen(1), "expected exactly one prebuilt workload copy on %s", winnerName)
	}, e2eConfig.GetIntervals(e.specName, "wait-remote-workload")...).Should(Succeed())

	Eventually(func(g Gomega) {
		for _, worker := range e.workers {
			if worker.Source.Name == winnerName {
				continue
			}
			err := workloadNotFound(ctx, worker.Cluster.Dynamic, e.setup.Namespace, workloadName)
			g.Expect(err).NotTo(HaveOccurred(), "expected loser workload cleanup on %s", worker.Source.Name)
			remoteObjects, err := listObjectsForPrebuiltWorkload(ctx, worker.Cluster.Dynamic, remoteGVR, e.setup.Namespace, workloadName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(remoteObjects).To(BeEmpty(), "expected no prebuilt workload copy on loser %s", worker.Source.Name)
		}
	}, e2eConfig.GetIntervals(e.specName, "wait-remote-cleanup")...).Should(Succeed())
}

func (e *multiKueueEnvironment) waitForNoRemoteObjects(ctx context.Context, workloadName string, remoteGVR schema.GroupVersionResource) {
	Eventually(func(g Gomega) {
		for _, worker := range e.workers {
			err := workloadNotFound(ctx, worker.Cluster.Dynamic, e.setup.Namespace, workloadName)
			g.Expect(err).NotTo(HaveOccurred(), "expected remote workload to be garbage-collected on %s", worker.Source.Name)
			remoteObjects, err := listObjectsForPrebuiltWorkload(ctx, worker.Cluster.Dynamic, remoteGVR, e.setup.Namespace, workloadName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(remoteObjects).To(BeEmpty(), "expected remote objects to be garbage-collected on %s", worker.Source.Name)
		}
	}, e2eConfig.GetIntervals(e.specName, "wait-remote-cleanup")...).Should(Succeed())
}

func listObjectsForPrebuiltWorkload(ctx context.Context, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace, workloadName string) ([]unstructured.Unstructured, error) {
	objects, err := dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	matching := []unstructured.Unstructured{}
	for _, obj := range objects.Items {
		if obj.GetLabels()[multiKueuePrebuiltWorkload] == workloadName || obj.GetAnnotations()[multiKueuePrebuiltWorkload] == workloadName {
			matching = append(matching, obj)
		}
	}
	return matching, nil
}

func (e *multiKueueEnvironment) deleteWorkloadEverywhere(ctx context.Context, workloadName string) {
	Expect(client.IgnoreNotFound(e.manager.Dynamic.Resource(kueueWorkloadGVR).Namespace(e.setup.Namespace).Delete(ctx, workloadName, metav1.DeleteOptions{}))).To(Succeed())
	for _, worker := range e.workers {
		Expect(client.IgnoreNotFound(worker.Cluster.Dynamic.Resource(kueueWorkloadGVR).Namespace(e.setup.Namespace).Delete(ctx, workloadName, metav1.DeleteOptions{}))).To(Succeed())
	}
}

func (e *multiKueueEnvironment) deleteBatchJob(ctx context.Context, jobName string) {
	err := e.manager.Clientset.BatchV1().Jobs(e.setup.Namespace).Delete(ctx, jobName, metav1.DeleteOptions{})
	Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
}

func (e *multiKueueEnvironment) workerNames() []string {
	names := make([]string, 0, len(e.workers))
	for _, worker := range e.workers {
		names = append(names, worker.Source.Name)
	}
	return names
}

func ensureNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) {
	_, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}, metav1.CreateOptions{})
	Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())
}

func ensureMultiKueueWorkerRBAC(ctx context.Context, clientset *kubernetes.Clientset) {
	ensureNamespace(ctx, clientset, kueueNamespace)
	_, err := clientset.CoreV1().ServiceAccounts(kueueNamespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: multiKueueServiceAccountName, Namespace: kueueNamespace},
	}, metav1.CreateOptions{})
	Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: multiKueueServiceAccountName + "-role"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"create", "delete", "get", "list", "watch"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs/status"}, Verbs: []string{"get"}},
			{APIGroups: []string{"jobset.x-k8s.io"}, Resources: []string{"jobsets"}, Verbs: []string{"create", "delete", "get", "list", "watch"}},
			{APIGroups: []string{"jobset.x-k8s.io"}, Resources: []string{"jobsets/status"}, Verbs: []string{"get"}},
			{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create", "delete", "get", "list", "watch"}},
			{APIGroups: []string{"kueue.x-k8s.io"}, Resources: []string{"workloads"}, Verbs: []string{"create", "delete", "get", "list", "watch"}},
			{APIGroups: []string{"kueue.x-k8s.io"}, Resources: []string{"workloads/status"}, Verbs: []string{"get", "patch", "update"}},
			{APIGroups: []string{"kueue.x-k8s.io"}, Resources: []string{"clusterqueues", "localqueues"}, Verbs: []string{"get", "list", "watch"}},
		},
	}
	createOrUpdateClusterRole(ctx, clientset, role)

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: multiKueueServiceAccountName + "-crb"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      multiKueueServiceAccountName,
			Namespace: kueueNamespace,
		}},
	}
	createOrUpdateClusterRoleBinding(ctx, clientset, binding)
}

func createOrUpdateClusterRole(ctx context.Context, clientset *kubernetes.Clientset, role *rbacv1.ClusterRole) {
	_, err := clientset.RbacV1().ClusterRoles().Create(ctx, role, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		current, getErr := clientset.RbacV1().ClusterRoles().Get(ctx, role.Name, metav1.GetOptions{})
		Expect(getErr).NotTo(HaveOccurred())
		role.ResourceVersion = current.ResourceVersion
		_, err = clientset.RbacV1().ClusterRoles().Update(ctx, role, metav1.UpdateOptions{})
	}
	Expect(err).NotTo(HaveOccurred())
}

func createOrUpdateClusterRoleBinding(ctx context.Context, clientset *kubernetes.Clientset, binding *rbacv1.ClusterRoleBinding) {
	_, err := clientset.RbacV1().ClusterRoleBindings().Create(ctx, binding, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		current, getErr := clientset.RbacV1().ClusterRoleBindings().Get(ctx, binding.Name, metav1.GetOptions{})
		Expect(getErr).NotTo(HaveOccurred())
		binding.ResourceVersion = current.ResourceVersion
		_, err = clientset.RbacV1().ClusterRoleBindings().Update(ctx, binding, metav1.UpdateOptions{})
	}
	Expect(err).NotTo(HaveOccurred())
}

func buildMultiKueueWorkerKubeconfig(ctx context.Context, worker *multiKueueCluster, specName string) []byte {
	secretName := multiKueueServiceAccountName
	_, err := worker.Clientset.CoreV1().Secrets(kueueNamespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: kueueNamespace,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: multiKueueServiceAccountName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}, metav1.CreateOptions{})
	Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	var tokenSecret *corev1.Secret
	Eventually(func(g Gomega) {
		tokenSecret, err = worker.Clientset.CoreV1().Secrets(kueueNamespace).Get(ctx, secretName, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(tokenSecret.Data).To(HaveKey("token"))
		g.Expect(tokenSecret.Data).To(HaveKey("ca.crt"))
	}, e2eConfig.GetIntervals(specName, "wait-service-account-token")...).Should(Succeed())

	restConfig := worker.Proxy.GetRESTConfig()
	clusterName := worker.Name
	userName := clusterName + "-" + multiKueueServiceAccountName
	return []byte(fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
kind: Config
preferences: {}
users:
- name: %s
  user:
    token: %s
`, base64.StdEncoding.EncodeToString(tokenSecret.Data["ca.crt"]), restConfig.Host, clusterName, clusterName, userName, clusterName, clusterName, userName, string(tokenSecret.Data["token"])))
}

func createOrUpdateKubeconfigSecret(ctx context.Context, clientset *kubernetes.Clientset, secretName string, kubeconfig []byte) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: kueueNamespace,
		},
		Data: map[string][]byte{
			multiKueueKubeconfigKey: kubeconfig,
		},
	}
	_, err := clientset.CoreV1().Secrets(kueueNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		current, getErr := clientset.CoreV1().Secrets(kueueNamespace).Get(ctx, secretName, metav1.GetOptions{})
		Expect(getErr).NotTo(HaveOccurred())
		current.Data = secret.Data
		_, err = clientset.CoreV1().Secrets(kueueNamespace).Update(ctx, current, metav1.UpdateOptions{})
	}
	Expect(err).NotTo(HaveOccurred())
}

func createMultiKueueQueues(ctx context.Context, dynamicClient dynamic.Interface, setup multiKueueSetupNames, cpuQuota, memoryQuota, admissionCheckName string) {
	createOrUpdateUnstructured(ctx, dynamicClient, kueueResourceFlavorGVR, "", newResourceFlavor(setup.ResourceFlavorName))
	createOrUpdateUnstructured(ctx, dynamicClient, kueueClusterQueueGVR, "", newClusterQueue(setup.ClusterQueueName, setup.ResourceFlavorName, cpuQuota, memoryQuota, admissionCheckName))
	createOrUpdateUnstructured(ctx, dynamicClient, kueueLocalQueueGVR, setup.Namespace, newLocalQueue(setup.LocalQueueName, setup.Namespace, setup.ClusterQueueName))
}

func createOrUpdateUnstructured(ctx context.Context, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) {
	var resource dynamic.ResourceInterface
	if namespace != "" {
		resource = dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		resource = dynamicClient.Resource(gvr)
	}
	_, err := resource.Create(ctx, obj, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		current, getErr := resource.Get(ctx, obj.GetName(), metav1.GetOptions{})
		Expect(getErr).NotTo(HaveOccurred())
		obj.SetResourceVersion(current.GetResourceVersion())
		_, err = resource.Update(ctx, obj, metav1.UpdateOptions{})
	}
	Expect(err).NotTo(HaveOccurred())
}

func newResourceFlavor(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kueue.x-k8s.io/v1beta2",
		"kind":       "ResourceFlavor",
		"metadata": map[string]interface{}{
			"name": name,
		},
	}}
}

func newClusterQueue(name, flavorName, cpuQuota, memoryQuota, admissionCheckName string) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"namespaceSelector": map[string]interface{}{},
		"resourceGroups": []interface{}{
			map[string]interface{}{
				"coveredResources": []interface{}{"cpu", "memory"},
				"flavors": []interface{}{
					map[string]interface{}{
						"name": flavorName,
						"resources": []interface{}{
							map[string]interface{}{"name": "cpu", "nominalQuota": cpuQuota},
							map[string]interface{}{"name": "memory", "nominalQuota": memoryQuota},
						},
					},
				},
			},
		},
	}
	if admissionCheckName != "" {
		spec["admissionChecksStrategy"] = map[string]interface{}{
			"admissionChecks": []interface{}{map[string]interface{}{"name": admissionCheckName}},
		}
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kueue.x-k8s.io/v1beta2",
		"kind":       "ClusterQueue",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": spec,
	}}
}

func newLocalQueue(name, namespace, clusterQueueName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kueue.x-k8s.io/v1beta2",
		"kind":       "LocalQueue",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"clusterQueue": clusterQueueName,
		},
	}}
}

func newAdmissionCheck(name, configName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kueue.x-k8s.io/v1beta2",
		"kind":       "AdmissionCheck",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"controllerName": multiKueueControllerName,
			"parameters": map[string]interface{}{
				"apiGroup": "kueue.x-k8s.io",
				"kind":     "MultiKueueConfig",
				"name":     configName,
			},
		},
	}}
}

func newMultiKueueConfig(name string, workerNames []string) *unstructured.Unstructured {
	clusters := make([]interface{}, 0, len(workerNames))
	for _, workerName := range workerNames {
		clusters = append(clusters, workerName)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kueue.x-k8s.io/v1beta2",
		"kind":       "MultiKueueConfig",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"clusters": clusters,
		},
	}}
}

func newMultiKueueClusterObject(name string, clusterSource map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kueue.x-k8s.io/v1beta2",
		"kind":       "MultiKueueCluster",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"clusterSource": clusterSource,
		},
	}}
}

func patchMultiKueueClusterSecret(ctx context.Context, dynamicClient dynamic.Interface, name, secretName string) {
	Eventually(func(g Gomega) {
		mkc, err := dynamicClient.Resource(kueueMultiClusterGVR).Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(unstructured.SetNestedField(mkc.Object, secretName, "spec", "clusterSource", "kubeConfig", "location")).To(Succeed())
		_, err = dynamicClient.Resource(kueueMultiClusterGVR).Update(ctx, mkc, metav1.UpdateOptions{})
		g.Expect(err).NotTo(HaveOccurred())
	}, e2eConfig.GetIntervals("multikueue", "wait-remote-cleanup")...).Should(Succeed())
}

func newMultiKueueBatchJob(name, namespace, cpuRequest, args string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				multiKueueQueueLabel: multiKueueLocalQueueName,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: int32Ptr(1),
			Completions: int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "workload",
						Image:   multiKueueJobImage,
						Command: []string{multiKueueJobCommand},
						Args:    []string{"-c", args},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resourceQuantity(cpuRequest),
								corev1.ResourceMemory: resourceQuantity("128Mi"),
							},
						},
					}},
				},
			},
		},
	}
}

func newMultiKueueJobSet(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				multiKueueQueueLabel: multiKueueLocalQueueName,
			},
		},
		"spec": map[string]interface{}{
			"network": map[string]interface{}{
				"enableDNSHostnames": false,
				"subdomain":          name,
			},
			"replicatedJobs": []interface{}{
				map[string]interface{}{
					"name":     "workers",
					"replicas": int64(1),
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"parallelism":  int64(1),
							"completions":  int64(1),
							"backoffLimit": int64(0),
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"restartPolicy": "Never",
									"containers": []interface{}{
										map[string]interface{}{
											"name":    "workload",
											"image":   multiKueueJobImage,
											"command": []interface{}{multiKueueJobCommand},
											"args":    []interface{}{"-c", multiKueueJobArgs},
											"resources": map[string]interface{}{
												"requests": map[string]interface{}{
													"cpu":    "100m",
													"memory": "128Mi",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}}
}

func resourceQuantity(value string) resource.Quantity {
	q, err := resource.ParseQuantity(value)
	Expect(err).NotTo(HaveOccurred())
	return q
}

func int32Ptr(v int32) *int32 {
	return &v
}

func waitForActiveCondition(ctx context.Context, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace, name, status, specName string) {
	Eventually(func(g Gomega) {
		var resource dynamic.ResourceInterface
		if namespace != "" {
			resource = dynamicClient.Resource(gvr).Namespace(namespace)
		} else {
			resource = dynamicClient.Resource(gvr)
		}
		obj, err := resource.Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(hasCondition(obj, "Active", status)).To(BeTrue(), "%s/%s conditions: %v", gvr.Resource, name, obj.Object["status"])
	}, e2eConfig.GetIntervals(specName, "wait-kueue-active")...).Should(Succeed())
}

func waitForWorkloadForOwner(ctx context.Context, dynamicClient dynamic.Interface, namespace string, ownerUID types.UID, specName string) *unstructured.Unstructured {
	var workload *unstructured.Unstructured
	Eventually(func(g Gomega) {
		var err error
		workload, err = findWorkloadForOwner(ctx, dynamicClient, namespace, ownerUID)
		g.Expect(err).NotTo(HaveOccurred())
	}, e2eConfig.GetIntervals(specName, "wait-workload-created")...).Should(Succeed())
	return workload
}

func findWorkloadForOwner(ctx context.Context, dynamicClient dynamic.Interface, namespace string, ownerUID types.UID) (*unstructured.Unstructured, error) {
	workloads, err := dynamicClient.Resource(kueueWorkloadGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range workloads.Items {
		workload := &workloads.Items[i]
		for _, owner := range workload.GetOwnerReferences() {
			if owner.UID == ownerUID {
				return workload, nil
			}
		}
	}
	return nil, apierrors.NewNotFound(kueueWorkloadGVR.GroupResource(), string(ownerUID))
}

func getWorkload(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) *unstructured.Unstructured {
	workload, err := dynamicClient.Resource(kueueWorkloadGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	return workload
}

func waitForWorkloadFinished(ctx context.Context, dynamicClient dynamic.Interface, namespace, name, specName string) {
	Eventually(func(g Gomega) {
		workload := getWorkload(ctx, dynamicClient, namespace, name)
		g.Expect(hasCondition(workload, "Finished", "True")).To(BeTrue(), "workload status: %v", workload.Object["status"])
	}, e2eConfig.GetIntervals(specName, "wait-workload-finished")...).Should(Succeed())
}

func workloadNotFound(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) error {
	_, err := dynamicClient.Resource(kueueWorkloadGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("workload %s/%s still exists", namespace, name)
	}
	return err
}

func hasCondition(obj *unstructured.Unstructured, conditionType, status string) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}
		if conditionMap["type"] == conditionType && conditionMap["status"] == status {
			return true
		}
	}
	return false
}

func workloadClusterName(workload *unstructured.Unstructured) string {
	clusterName, _, err := unstructured.NestedString(workload.Object, "status", "clusterName")
	Expect(err).NotTo(HaveOccurred())
	return clusterName
}

func workloadNominatedClusterNames(workload *unstructured.Unstructured) []string {
	names, _, err := unstructured.NestedStringSlice(workload.Object, "status", "nominatedClusterNames")
	Expect(err).NotTo(HaveOccurred())
	return names
}

func workloadHasAdmission(workload *unstructured.Unstructured) bool {
	_, found, err := unstructured.NestedMap(workload.Object, "status", "admission")
	Expect(err).NotTo(HaveOccurred())
	return found || hasCondition(workload, "Admitted", "True")
}

func e2eConfigVariableOrDefault(name, defaultValue string) string {
	if value, ok := e2eConfig.Variables[name]; ok && value != "" {
		return value
	}
	return defaultValue
}

func e2eConfigIntVariableOrDefault(name string, defaultValue int) int {
	value := e2eConfigVariableOrDefault(name, "")
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	Expect(err).NotTo(HaveOccurred(), "expected %s to be an integer", name)
	return parsed
}
