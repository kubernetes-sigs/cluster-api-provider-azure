//go:build e2e
// +build e2e

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

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sshPort                               = "22"
	deleteOperationTimeout                = 20 * time.Minute
	retryableOperationTimeout             = 30 * time.Second
	retryableOperationSleepBetweenRetries = 3 * time.Second
	sshConnectionTimeout                  = 30 * time.Second
)

// deploymentsClientAdapter adapts a Deployment to work with WaitForDeploymentsAvailable.
type deploymentsClientAdapter struct {
	client typedappsv1.DeploymentInterface
}

// Get fetches the deployment named by the key and updates the provided object.
func (c deploymentsClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	deployment, err := c.client.Get(ctx, key.Name, metav1.GetOptions{})
	if deployObj, ok := obj.(*appsv1.Deployment); ok {
		deployment.DeepCopyInto(deployObj)
	}
	return err
}

// WaitForDeploymentsAvailableInput is the input for WaitForDeploymentsAvailable.
type WaitForDeploymentsAvailableInput struct {
	Getter     framework.Getter
	Deployment *appsv1.Deployment
	Clientset  *kubernetes.Clientset
}

// WaitForDeploymentsAvailable waits until the Deployment has status.Available = True, that signals that
// all the desired replicas are in place.
// This can be used to check if Cluster API controllers installed in the management cluster are working.
func WaitForDeploymentsAvailable(ctx context.Context, input WaitForDeploymentsAvailableInput, intervals ...interface{}) {
	start := time.Now()
	namespace, name := input.Deployment.GetNamespace(), input.Deployment.GetName()
	Byf("waiting for deployment %s/%s to be available", namespace, name)
	Log("starting to wait for deployment to become available")
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, input.Deployment); err == nil {
			for _, c := range input.Deployment.Status.Conditions {
				if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedDeployment(ctx, input) })
	Logf("Deployment %s/%s is now available, took %v", namespace, name, time.Since(start))
}

// GetWaitForDeploymentsAvailableInput is a convenience func to compose a WaitForDeploymentsAvailableInput
func GetWaitForDeploymentsAvailableInput(ctx context.Context, clusterProxy framework.ClusterProxy, name, namespace string, specName string) WaitForDeploymentsAvailableInput {
	Expect(clusterProxy).NotTo(BeNil())
	cl := clusterProxy.GetClient()
	var d = &appsv1.Deployment{}
	Eventually(func() error {
		return cl.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, d)
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())
	clientset := clusterProxy.GetClientSet()
	return WaitForDeploymentsAvailableInput{
		Deployment: d,
		Clientset:  clientset,
		Getter:     cl,
	}
}

// DescribeFailedDeployment returns detailed output to help debug a deployment failure in e2e.
func DescribeFailedDeployment(ctx context.Context, input WaitForDeploymentsAvailableInput) string {
	namespace, name := input.Deployment.GetNamespace(), input.Deployment.GetName()
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Deployment %s/%s failed",
		namespace, name))
	b.WriteString(fmt.Sprintf("\nDeployment:\n%s\n", prettyPrint(input.Deployment)))
	b.WriteString(describeEvents(ctx, input.Clientset, namespace, name))
	return b.String()
}

// jobsClientAdapter adapts a Job to work with WaitForJobAvailable.
type jobsClientAdapter struct {
	client typedbatchv1.JobInterface
}

// Get fetches the job named by the key and updates the provided object.
func (c jobsClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	job, err := c.client.Get(ctx, key.Name, metav1.GetOptions{})
	if jobObj, ok := obj.(*batchv1.Job); ok {
		job.DeepCopyInto(jobObj)
	}
	return err
}

// WaitForJobCompleteInput is the input for WaitForJobComplete.
type WaitForJobCompleteInput struct {
	Getter    framework.Getter
	Job       *batchv1.Job
	Clientset *kubernetes.Clientset
}

// WaitForJobComplete waits until the Job completes with at least one success.
func WaitForJobComplete(ctx context.Context, input WaitForJobCompleteInput, intervals ...interface{}) {
	start := time.Now()
	namespace, name := input.Job.GetNamespace(), input.Job.GetName()
	Byf("waiting for job %s/%s to be complete", namespace, name)
	Logf("waiting for job %s/%s to be complete", namespace, name)
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, input.Job); err == nil {
			for _, c := range input.Job.Status.Conditions {
				if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
					return input.Job.Status.Succeeded > 0
				}
			}
		}
		return false
	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedJob(ctx, input) })
	Logf("job %s/%s is complete, took %v", namespace, name, time.Since(start))
}

// DescribeFailedJob returns a string with information to help debug a failed job.
func DescribeFailedJob(ctx context.Context, input WaitForJobCompleteInput) string {
	namespace, name := input.Job.GetNamespace(), input.Job.GetName()
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Job %s/%s failed",
		namespace, name))
	b.WriteString(fmt.Sprintf("\nJob:\n%s\n", prettyPrint(input.Job)))
	b.WriteString(describeEvents(ctx, input.Clientset, namespace, name))
	b.WriteString(getJobPodLogs(ctx, input))
	return b.String()
}

func getJobPodLogs(ctx context.Context, input WaitForJobCompleteInput) string {
	podsClient := input.Clientset.CoreV1().Pods(input.Job.GetNamespace())
	pods, err := podsClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", input.Job.GetName())})
	if err != nil {
		return err.Error()
	}
	logs := make(map[string]string, len(pods.Items))
	for _, pod := range pods.Items {
		logs[pod.Name] = getPodLogs(ctx, input.Clientset, pod)
	}
	b := strings.Builder{}
	args := input.Job.Spec.Template.Spec.Containers[0].Args
	b.WriteString(fmt.Sprintf("Output of \"kubescape %s\":\n", strings.Join(args, " ")))
	var lastLog string
	for podName, log := range logs {
		b.WriteString(fmt.Sprintf("\nLogs for pod %s:\n", podName))
		if logsAreSimilar(lastLog, log) {
			b.WriteString("(Omitted because of similarity to previous pod's logs.)")
		} else {
			b.WriteString(log)
		}
		lastLog = log
	}
	return b.String()
}

// logsAreSimilar compares two multi-line strings and returns true if at least 90% of the lines match.
func logsAreSimilar(a, b string) bool {
	if a == "" {
		return false
	}
	a1 := strings.Split(a, "\n")
	b1 := strings.Split(b, "\n")
	for i := len(a1) - 1; i >= 0; i-- {
		for _, v := range b1 {
			if a1[i] == v {
				a1 = append(a1[:i], a1[i+1:]...)
				break
			}
		}
	}
	return float32(len(a1))/float32(len(b1)) < 0.1
}

// servicesClientAdapter adapts a Service to work with WaitForServicesAvailable.
type servicesClientAdapter struct {
	client typedcorev1.ServiceInterface
}

// Get fetches the service named by the key and updates the provided object.
func (c servicesClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	service, err := c.client.Get(ctx, key.Name, metav1.GetOptions{})
	if serviceObj, ok := obj.(*corev1.Service); ok {
		service.DeepCopyInto(serviceObj)
	}
	return err
}

// WaitForServiceAvailableInput is the input for WaitForServiceAvailable.
type WaitForServiceAvailableInput struct {
	Getter    framework.Getter
	Service   *corev1.Service
	Clientset *kubernetes.Clientset
}

// WaitForServiceAvailable waits until the Service has an IP address available on each Ingress.
func WaitForServiceAvailable(ctx context.Context, input WaitForServiceAvailableInput, intervals ...interface{}) {
	start := time.Now()
	namespace, name := input.Service.GetNamespace(), input.Service.GetName()
	Byf("waiting for service %s/%s to be available", namespace, name)
	Logf("waiting for service %s/%s to be available", namespace, name)
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, input.Service); err == nil {
			ingress := input.Service.Status.LoadBalancer.Ingress
			if len(ingress) > 0 {
				for _, i := range ingress {
					if net.ParseIP(i.IP) == nil {
						return false
					}
				}
				return true
			}
		}
		return false
	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedService(ctx, input) })
	Logf("service %s/%s is available, took %v", namespace, name, time.Since(start))
}

// DescribeFailedService returns a string with information to help debug a failed service.
func DescribeFailedService(ctx context.Context, input WaitForServiceAvailableInput) string {
	namespace, name := input.Service.GetNamespace(), input.Service.GetName()
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Service %s/%s failed",
		namespace, name))
	b.WriteString(fmt.Sprintf("\nService:\n%s\n", prettyPrint(input.Service)))
	b.WriteString(describeEvents(ctx, input.Clientset, namespace, name))
	return b.String()
}

// describeEvents returns a string summarizing recent events involving the named object(s).
func describeEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	b := strings.Builder{}
	if clientset == nil {
		b.WriteString("clientset is nil, so skipping output of relevant events")
	} else {
		opts := metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
			Limit:         20,
		}
		evts, err := clientset.CoreV1().Events(namespace).List(ctx, opts)
		if err != nil {
			b.WriteString(err.Error())
		} else {
			w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', tabwriter.FilterHTML)
			fmt.Fprintln(w, "LAST SEEN\tTYPE\tREASON\tOBJECT\tMESSAGE")
			for _, e := range evts.Items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s/%s\t%s\n", e.LastTimestamp, e.Type, e.Reason,
					strings.ToLower(e.InvolvedObject.Kind), e.InvolvedObject.Name, e.Message)
			}
			w.Flush()
		}
	}
	return b.String()
}

// prettyPrint returns a formatted JSON version of the object given.
func prettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// getAvailabilityZonesForRegion uses zone information in availableZonesPerLocation.json
// and returns the number of availability zones per region that would support the VM type used for e2e tests.
// will return an error if the region isn't recognized
// availableZonesPerLocation.json was generated by
// az vm list-skus -r "virtualMachines"  -z | jq 'map({(.locationInfo[0].location + "_" + .name): .locationInfo[0].zones}) | add' > availableZonesPerLocation.json
func getAvailabilityZonesForRegion(location string, size string) ([]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	file, err := os.ReadFile(filepath.Join(wd, "data", "availableZonesPerLocation.json"))
	if err != nil {
		return nil, err
	}
	var data map[string][]string

	if err := json.Unmarshal(file, &data); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s_%s", location, size)

	return data[key], nil
}

// logCheckpoint prints a message indicating the start or end of the current test spec,
// including which Ginkgo node it's running on.
//
// Example output:
//
//	INFO: "With 1 worker node" started at Tue, 22 Sep 2020 13:19:08 PDT on Ginkgo node 2 of 3
//	INFO: "With 1 worker node" ran for 18m34s on Ginkgo node 2 of 3
func logCheckpoint(specTimes map[string]time.Time) {
	text := CurrentGinkgoTestDescription().TestText
	start, started := specTimes[text]
	if !started {
		start = time.Now()
		specTimes[text] = start
		fmt.Fprintf(GinkgoWriter, "INFO: \"%s\" started at %s on Ginkgo node %d of %d\n", text,
			start.Format(time.RFC1123), GinkgoParallelNode(), config.GinkgoConfig.ParallelTotal)
	} else {
		elapsed := time.Since(start)
		fmt.Fprintf(GinkgoWriter, "INFO: \"%s\" ran for %s on Ginkgo node %d of %d\n", text,
			elapsed.Round(time.Second), GinkgoParallelNode(), config.GinkgoConfig.ParallelTotal)
	}
}

// getClusterName gets the cluster name for the test cluster
// and sets the environment variables that depend on it.
func getClusterName(prefix, specName string) string {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = fmt.Sprintf("%s-%s", prefix, specName)
	}
	fmt.Fprintf(GinkgoWriter, "INFO: Cluster name is %s\n", clusterName)

	Expect(os.Setenv(AzureResourceGroup, clusterName)).To(Succeed())
	Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", clusterName))).To(Succeed())
	return clusterName
}

func isAzureMachineWindows(am *infrav1.AzureMachine) bool {
	return am.Spec.OSDisk.OSType == azure.WindowsOS
}

func isAzureMachinePoolWindows(amp *infrav1exp.AzureMachinePool) bool {
	return amp.Spec.Template.OSDisk.OSType == azure.WindowsOS
}

// getProxiedSSHClient creates a SSH client object that connects to a target node
// proxied through a control plane node.
func getProxiedSSHClient(controlPlaneEndpoint, hostname, port string) (*ssh.Client, error) {
	config, err := newSSHConfig()
	if err != nil {
		return nil, err
	}

	// Init a client connection to a control plane node via the public load balancer
	lbClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", controlPlaneEndpoint, port), config)
	if err != nil {
		return nil, errors.Wrapf(err, "dialing public load balancer at %s", controlPlaneEndpoint)
	}

	// Init a connection from the control plane to the target node
	c, err := lbClient.Dial("tcp", fmt.Sprintf("%s:%s", hostname, port))
	if err != nil {
		return nil, errors.Wrapf(err, "dialing from control plane to target node at %s", hostname)
	}

	// Establish an authenticated SSH conn over the client -> control plane -> target transport
	conn, chans, reqs, err := ssh.NewClientConn(c, hostname, config)
	if err != nil {
		return nil, errors.Wrap(err, "getting a new SSH client connection")
	}
	client := ssh.NewClient(conn, chans, reqs)
	return client, nil
}

// execOnHost runs the specified command directly on a node's host, using a SSH connection
// proxied through a control plane host and copies the output to a file.
func execOnHost(controlPlaneEndpoint, hostname, port string, f io.StringWriter, command string,
	args ...string) error {
	client, err := getProxiedSSHClient(controlPlaneEndpoint, hostname, port)
	if err != nil {
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		return errors.Wrap(err, "opening SSH session")
	}
	defer session.Close()

	// Run the command and write the captured stdout to the file
	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}
	if err = session.Run(command); err != nil {
		return errors.Wrapf(err, "running command \"%s\"", command)
	}
	if _, err = f.WriteString(stdoutBuf.String()); err != nil {
		return errors.Wrap(err, "writing output to file")
	}

	return nil
}

// sftpCopyFile copies a file from a node to the specified destination, using a SSH connection
// proxied through a control plane node.
func sftpCopyFile(controlPlaneEndpoint, hostname, port, sourcePath, destPath string) error {
	Logf("Attempting to copy file %s on node %s to %s", sourcePath, hostname, destPath)

	client, err := getProxiedSSHClient(controlPlaneEndpoint, hostname, port)
	if err != nil {
		return err
	}

	sftp, err := sftp.NewClient(client)
	if err != nil {
		return errors.Wrapf(err, "getting a new sftp client connection")
	}
	defer sftp.Close()

	// copy file
	sourceFile, err := sftp.Open(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "opening file %s on node %s", sourcePath, hostname)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return errors.Wrapf(err, "creating file %s on locally", sourcePath)
	}
	defer destFile.Close()

	_, err = sourceFile.WriteTo(destFile)
	if err != nil {
		return errors.Wrapf(err, "writing to %s", destPath)
	}

	return nil
}

// fileOnHost creates the specified path, including parent directories if needed.
func fileOnHost(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(path)
}

// newSSHConfig returns an SSH config for a workload cluster in the current e2e test run.
func newSSHConfig() (*ssh.ClientConfig, error) {
	// find private key file used for e2e workload cluster
	keyfile := os.Getenv("AZURE_SSH_PUBLIC_KEY_FILE")
	if len(keyfile) > 4 && strings.HasSuffix(keyfile, "pub") {
		keyfile = keyfile[:(len(keyfile) - 4)]
	}
	if keyfile == "" {
		keyfile = ".sshkey"
	}
	if _, err := os.Stat(keyfile); os.IsNotExist(err) {
		if !filepath.IsAbs(keyfile) {
			// current working directory may be test/e2e, so look in the project root
			keyfile = filepath.Join("..", "..", keyfile)
		}
	}

	pubkey, err := publicKeyFile(keyfile)
	if err != nil {
		return nil, err
	}
	sshConfig := ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // Non-production code
		User:            azure.DefaultUserName,
		Auth:            []ssh.AuthMethod{pubkey},
		Timeout:         sshConnectionTimeout,
	}
	return &sshConfig, nil
}

// publicKeyFile parses and returns the public key from the specified private key file.
func publicKeyFile(file string) (ssh.AuthMethod, error) {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// validateStableReleaseString validates the string format that declares "get be the latest stable release for this <Major>.<Minor>"
// it should be called wherever we process a stable version string expression like "stable-1.22"
func validateStableReleaseString(stableVersion string) (isStable bool, matches []string) {
	stableReleaseFormat := regexp.MustCompile(`^stable-(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
	matches = stableReleaseFormat.FindStringSubmatch(stableVersion)
	return len(matches) > 0, matches
}

// resolveCIVersion resolves kubernetes version labels (e.g. latest, latest-1.xx) to the corresponding CI version numbers.
// Go implementation of https://github.com/kubernetes-sigs/cluster-api/blob/d1dc87d5df3ab12a15ae5b63e50541a191b7fec4/scripts/ci-e2e-lib.sh#L75-L95.
func resolveCIVersion(label string) (string, error) {
	if ciVersion, ok := os.LookupEnv("CI_VERSION"); ok {
		return ciVersion, nil
	}
	if strings.HasPrefix(label, "latest") {
		if kubernetesVersion, err := latestCIVersion(label); err == nil {
			return kubernetesVersion, nil
		}
	}

	// default to https://dl.k8s.io/ci/latest.txt if the label can't be resolved
	return kubernetesversions.LatestCIRelease()
}

// latestCIVersion returns the latest CI version of a given label in the form of latest-1.xx.
func latestCIVersion(label string) (string, error) {
	ciVersionURL := fmt.Sprintf("https://dl.k8s.io/ci/%s.txt", label)
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, ciVersionURL, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

// resolveKubetestRepoListPath will set the correct repo list for Windows:
// - if WIN_REPO_URL is set use the custom file downloaded via makefile
// - if CI version is "latest" do not set repo list since they are not needed K8s v1.25+
// - if CI version is  "latest-1.xx" will compare values and use correct repoList
// - if standard version will compare values and use correct repoList
// - if unable to determine version falls back to using latest
func resolveKubetestRepoListPath(version string, path string) (string, error) {
	if _, ok := os.LookupEnv("WIN_REPO_URL"); ok {
		return filepath.Join(path, "custom-repo-list.yaml"), nil
	}

	if version == "latest" {
		return "", nil
	}

	version = strings.TrimPrefix(version, "latest-")
	currentVersion, err := semver.ParseTolerant(version)
	if err != nil {
		return "", err
	}

	v125, err := semver.Make("1.25.0-alpha.0.0")
	if err != nil {
		return "", err
	}

	if currentVersion.GT(v125) {
		return "", nil
	}

	// - prior to K8s v1.21 repo-list-k8sprow.yaml should be used
	//   since all test images need to come from k8sprow.azurecr.io
	// - starting with K8s v1.25 repo lists repo list is not needed
	// - use repo-list.yaml for everything in between which has only
	//   some images in k8sprow.azurecr.io

	return filepath.Join(path, "repo-list.yaml"), nil
}

// resolveKubernetesVersions looks at Kubernetes versions set as variables in the e2e config and sets them to a valid k8s version
// that has an existing capi offer image available. For example, if the version is "stable-1.22", the function will set it to the latest 1.22 version that has a published reference image.
func resolveKubernetesVersions(config *clusterctl.E2EConfig) {
	ubuntuVersions := getVersionsInOffer(context.TODO(), os.Getenv(AzureLocation), capiImagePublisher, capiOfferName)
	windowsVersions := getVersionsInOffer(context.TODO(), os.Getenv(AzureLocation), capiImagePublisher, capiWindowsOfferName)

	// find the intersection of ubuntu and windows versions available, since we need an image for both.
	var versions semver.Versions
	for k, v := range ubuntuVersions {
		if _, ok := windowsVersions[k]; ok {
			versions = append(versions, v)
		}
	}

	if config.HasVariable(capi_e2e.KubernetesVersion) {
		resolveKubernetesVersion(config, versions, capi_e2e.KubernetesVersion)
	}
	if config.HasVariable(capi_e2e.KubernetesVersionUpgradeFrom) {
		resolveKubernetesVersion(config, versions, capi_e2e.KubernetesVersionUpgradeFrom)
	}
	if config.HasVariable(capi_e2e.KubernetesVersionUpgradeTo) {
		resolveKubernetesVersion(config, versions, capi_e2e.KubernetesVersionUpgradeTo)
	}
}

func resolveKubernetesVersion(config *clusterctl.E2EConfig, versions semver.Versions, varName string) {
	oldVersion := config.GetVariable(varName)
	v := getLatestSkuForMinor(oldVersion, versions)
	if _, ok := os.LookupEnv(varName); ok {
		Expect(os.Setenv(varName, v)).To(Succeed())
	}
	config.Variables[varName] = v
	Logf("Resolved %s (set to %s) to %s", varName, oldVersion, v)
}

// newImagesClient returns a new VM images client using environmental settings for auth.
func newImagesClient() compute.VirtualMachineImagesClient {
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	imagesClient := compute.NewVirtualMachineImagesClient(subscriptionID)
	imagesClient.Authorizer = authorizer

	return imagesClient
}

// getVersionsInOffer returns a map of Kubernetes versions as strings to semver.Versions.
func getVersionsInOffer(ctx context.Context, location, publisher, offer string) map[string]semver.Version {
	Logf("Finding image skus and versions for offer %s/%s in %s", publisher, offer, location)
	var versions map[string]semver.Version
	capiSku := regexp.MustCompile(`^[\w-]+-gen[12]$`)
	capiVersion := regexp.MustCompile(`^(\d)(\d{1,2})\.(\d{1,2})\.\d{8}$`)
	oldCapiSku := regexp.MustCompile(`^k8s-(0|[1-9][0-9]*)dot(0|[1-9][0-9]*)dot(0|[1-9][0-9]*)-[a-z]*.*$`)
	imagesClient := newImagesClient()
	skus, err := imagesClient.ListSkus(ctx, location, publisher, offer)
	Expect(err).NotTo(HaveOccurred())

	if skus.Value != nil {
		versions = make(map[string]semver.Version, len(*skus.Value))
		for _, sku := range *skus.Value {
			res, err := imagesClient.List(ctx, location, publisher, offer, *sku.Name, "", nil, "")
			Expect(err).NotTo(HaveOccurred())
			// Don't use SKUs without existing images. See https://github.com/Azure/azure-cli/issues/20115.
			if res.Value != nil && len(*res.Value) > 0 {
				// New SKUs don't contain the Kubernetes version and are named like "ubuntu-2004-gen1".
				if match := capiSku.FindStringSubmatch(*sku.Name); len(match) > 0 {
					for _, vmImage := range *res.Value {
						// Versions are named like "121.13.20220601", for Kubernetes v1.21.13 published on June 1, 2022.
						match = capiVersion.FindStringSubmatch(*vmImage.Name)
						stringVer := fmt.Sprintf("%s.%s.%s", match[1], match[2], match[3])
						versions[stringVer] = semver.MustParse(stringVer)
					}
					continue
				}
				// Old SKUs before 1.21.12, 1.22.9, or 1.23.6 are named like "k8s-1dot21dot2-ubuntu-2004".
				if match := oldCapiSku.FindStringSubmatch(*sku.Name); len(match) > 0 {
					stringVer := fmt.Sprintf("%s.%s.%s", match[1], match[2], match[3])
					versions[stringVer] = semver.MustParse(stringVer)
				}
			}
		}
	}

	return versions
}

// getLatestSkuForMinor gets the latest available patch version in the provided list of sku versions that corresponds to the provided k8s version.
func getLatestSkuForMinor(version string, skus semver.Versions) string {
	isStable, match := validateStableReleaseString(version)
	if isStable {
		// if the version is in the format "stable-1.21", we find the latest 1.21.x version.
		major, err := strconv.ParseUint(match[1], 10, 64)
		Expect(err).NotTo(HaveOccurred())
		minor, err := strconv.ParseUint(match[2], 10, 64)
		Expect(err).NotTo(HaveOccurred())
		semver.Sort(skus)
		for i := len(skus) - 1; i >= 0; i-- {
			if skus[i].Major == major && skus[i].Minor == minor {
				version = "v" + skus[i].String()
				break
			}
		}
	} else if v, err := semver.ParseTolerant(version); err == nil {
		if len(v.Pre) == 0 {
			// if the version is in the format "v1.21.2", we make sure we have an existing image for it.
			Expect(skus).To(ContainElement(v), fmt.Sprintf("Provided Kubernetes version %s does not have a corresponding VM image in the capi offer", version))
		}
	}
	// otherwise, we just return the version as-is. This allows for versions in other formats, such as "latest" or "latest-1.21".
	return version
}

// getPodLogs returns the logs of a pod, or an error in string format.
func getPodLogs(ctx context.Context, clientset *kubernetes.Clientset, pod corev1.Pod) string {
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	logs, err := req.Stream(ctx)
	if err != nil {
		return fmt.Sprintf("error streaming logs for pod %s: %v", pod.Name, err)
	}
	defer logs.Close()

	b := new(bytes.Buffer)
	if _, err = io.Copy(b, logs); err != nil {
		return fmt.Sprintf("error copying logs for pod %s: %v", pod.Name, err)
	}
	return b.String()
}

// InstallHelmChart takes a helm repo URL, a chart name, and release name, and installs a helm release onto the E2E workload cluster.
func InstallHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, repoURL, chartName, releaseName string, values []string) {
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	kubeConfigPath := clusterProxy.GetKubeconfigPath()
	settings := helmCli.New()
	settings.KubeConfig = kubeConfigPath
	actionConfig := new(helmAction.Configuration)
	ns := "default"
	err := actionConfig.Init(settings.RESTClientGetter(), ns, "secret", Logf)
	Expect(err).To(BeNil())
	i := helmAction.NewInstall(actionConfig)
	if repoURL != "" {
		i.RepoURL = repoURL
	}
	i.ReleaseName = releaseName
	i.Namespace = ns
	Eventually(func() error {
		cp, err := i.ChartPathOptions.LocateChart(chartName, helmCli.New())
		if err != nil {
			return err
		}
		p := helmGetter.All(settings)
		valueOpts := &helmVals.Options{}
		valueOpts.Values = values
		vals, err := valueOpts.MergeValues(p)
		if err != nil {
			return err
		}
		chartRequested, err := helmLoader.Load(cp)
		if err != nil {
			return err
		}
		release, err := i.RunWithContext(ctx, chartRequested, vals)
		if err != nil {
			return err
		}
		Logf(release.Info.Description)
		return nil
	}, input.WaitForControlPlaneIntervals...).Should(Succeed())
}

// WaitForDaemonset retries during E2E until a daemonset's pods are all Running.
func WaitForDaemonset(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, cl client.Client, name, namespace string) {
	Eventually(func() bool {
		ds := &appsv1.DaemonSet{}
		if err := cl.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, ds); err != nil {
			return false
		}
		if ds.Status.DesiredNumberScheduled == ds.Status.NumberReady {
			return true
		}
		return false
	}, input.WaitForControlPlaneIntervals...).Should(Equal(true))
}

func defaultConfigCluster(clusterName, namespace string) clusterctl.ConfigClusterInput {
	return clusterctl.ConfigClusterInput{
		LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
		ClusterctlConfigPath:     clusterctlConfigPath,
		KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
		InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
		Flavor:                   clusterctl.DefaultFlavor,
		Namespace:                namespace,
		ClusterName:              clusterName,
		KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
		ControlPlaneMachineCount: pointer.Int64Ptr(3),
		WorkerMachineCount:       pointer.Int64Ptr(0),
	}
}

func createClusterWithControlPlaneWaiters(ctx context.Context, configCluster clusterctl.ConfigClusterInput,
	cpWaiters clusterctl.ControlPlaneWaiters,
	result *clusterctl.ApplyClusterTemplateAndWaitResult) (*clusterv1.Cluster,
	*controlplanev1.KubeadmControlPlane) {
	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy:                 bootstrapClusterProxy,
		ConfigCluster:                configCluster,
		WaitForClusterIntervals:      e2eConfig.GetIntervals("", "wait-cluster"),
		WaitForControlPlaneIntervals: e2eConfig.GetIntervals("", "wait-control-plane"),
		WaitForMachineDeployments:    e2eConfig.GetIntervals("", "wait-worker-nodes"),
		ControlPlaneWaiters:          cpWaiters,
	}, result)
	return result.Cluster, result.ControlPlane
}
