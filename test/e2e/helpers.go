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
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sshPort = "22"
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
	return b.String()
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
			if ingress != nil && len(ingress) > 0 {
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
	file, err := ioutil.ReadFile(filepath.Join(wd, "data/availableZonesPerLocation.json"))
	if err != nil {
		return nil, err
	}
	var data map[string][]string

	if err = json.Unmarshal(file, &data); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s_%s", location, size)

	return data[key], nil
}

// logCheckpoint prints a message indicating the start or end of the current test spec,
// including which Ginkgo node it's running on.
//
// Example output:
//   INFO: "With 1 worker node" started at Tue, 22 Sep 2020 13:19:08 PDT on Ginkgo node 2 of 3
//   INFO: "With 1 worker node" ran for 18m34s on Ginkgo node 2 of 3
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

	Expect(os.Setenv(AzureResourceGroup, clusterName)).NotTo(HaveOccurred())
	Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", clusterName))).NotTo(HaveOccurred())
	return clusterName
}

// nodeSSHInfo provides information to establish an SSH connection to a VM or VMSS instance.
type nodeSSHInfo struct {
	Endpoint string // Endpoint is the control plane hostname or IP address for initial connection.
	Hostname string // Hostname is the name or IP address of the destination VM or VMSS instance.
	Port     string // Port is the TCP port used for the SSH connection.
}

// getClusterSSHInfo returns the information needed to establish a SSH connection through a
// control plane endpoint to each node in the cluster.
func getClusterSSHInfo(ctx context.Context, mgmtClusterProxy framework.ClusterProxy, namespace, clusterName string) ([]nodeSSHInfo, error) {
	var (
		sshInfo               []nodeSSHInfo
		mgmtClusterClient     = mgmtClusterProxy.GetClient()
		workloadClusterClient = mgmtClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName).GetClient()
	)
	// Collect the info for each VM / Machine.
	machines, err := getMachinesInCluster(ctx, mgmtClusterClient, namespace, clusterName)
	if err != nil {
		return sshInfo, errors.Wrap(err, "failed to get machines in the cluster")
	}
	for i := range machines.Items {
		m := &machines.Items[i]
		cluster, err := util.GetClusterFromMetadata(ctx, mgmtClusterClient, m.ObjectMeta)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get cluster from metadata")
		}
		sshInfo = append(sshInfo, nodeSSHInfo{
			Endpoint: cluster.Spec.ControlPlaneEndpoint.Host,
			Hostname: m.Spec.InfrastructureRef.Name,
			Port:     sshPort,
		})
	}

	// Collect the info for each instance in a VMSS / MachinePool.
	machinePools, err := getMachinePoolsInCluster(ctx, mgmtClusterClient, namespace, clusterName)
	if err != nil {
		return sshInfo, errors.Wrap(err, "failed to find machine pools in cluster")
	}

	for i := range machinePools.Items {
		p := &machinePools.Items[i]
		cluster, err := util.GetClusterFromMetadata(ctx, mgmtClusterClient, p.ObjectMeta)
		if err != nil {
			return sshInfo, errors.Wrap(err, "failed to get cluster from metadata")
		}

		nodes, err := getReadyNodes(ctx, workloadClusterClient, p.Status.NodeRefs)
		if err != nil {
			return sshInfo, errors.Wrap(err, "failed to get ready nodes")
		}

		if p.Spec.Replicas != nil && len(nodes) < int(*p.Spec.Replicas) {
			message := fmt.Sprintf("machine pool %s/%s expected replicas %d, but only found %d ready nodes", p.Namespace, p.Name, *p.Spec.Replicas, len(nodes))
			Log(message)
			return sshInfo, errors.New(message)
		}

		for _, node := range nodes {
			sshInfo = append(sshInfo, nodeSSHInfo{
				Endpoint: cluster.Spec.ControlPlaneEndpoint.Host,
				Hostname: node.Name,
				Port:     sshPort,
			})
		}
	}

	return sshInfo, nil
}

func getReadyNodes(ctx context.Context, c client.Client, refs []corev1.ObjectReference) ([]corev1.Node, error) {
	var nodes []corev1.Node
	for _, ref := range refs {
		var node corev1.Node
		if err := c.Get(ctx, client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		}, &node); err != nil {
			if apierrors.IsNotFound(err) {
				// If 404, continue. Likely the node refs have not caught up to infra providers
				continue
			}

			return nodes, err
		}

		if !noderefutil.IsNodeReady(&node) {
			Logf("node is not ready and won't be counted for ssh info %s/%s", node.Namespace, node.Name)
			continue
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// getMachinesInCluster returns a list of all machines in the given cluster.
// This is adapted from CAPI's test/framework/cluster_proxy.go.
func getMachinesInCluster(ctx context.Context, c framework.Lister, namespace, name string) (*clusterv1.MachineList, error) {
	if name == "" {
		return nil, nil
	}

	machineList := &clusterv1.MachineList{}
	labels := map[string]string{clusterv1.ClusterLabelName: name}

	if err := c.List(ctx, machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return machineList, nil
}

// getMachinePoolsInCluster returns a list of all machine pools in the given cluster.
func getMachinePoolsInCluster(ctx context.Context, c framework.Lister, namespace, name string) (*clusterv1exp.MachinePoolList, error) {
	if name == "" {
		return nil, nil
	}

	machinePoolList := &clusterv1exp.MachinePoolList{}
	labels := map[string]string{clusterv1.ClusterLabelName: name}

	if err := c.List(ctx, machinePoolList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return machinePoolList, nil
}

// execOnHost runs the specified command directly on a node's host, using an SSH connection
// proxied through a control plane host.
func execOnHost(controlPlaneEndpoint, hostname, port string, f io.StringWriter, command string,
	args ...string) error {
	config, err := newSSHConfig()
	if err != nil {
		return err
	}

	// Init a client connection to a control plane node via the public load balancer
	lbClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", controlPlaneEndpoint, port), config)
	if err != nil {
		return errors.Wrapf(err, "dialing public load balancer at %s", controlPlaneEndpoint)
	}

	// Init a connection from the control plane to the target node
	c, err := lbClient.Dial("tcp", fmt.Sprintf("%s:%s", hostname, port))
	if err != nil {
		return errors.Wrapf(err, "dialing from control plane to target node at %s", hostname)
	}

	// Establish an authenticated SSH conn over the client -> control plane -> target transport
	conn, chans, reqs, err := ssh.NewClientConn(c, hostname, config)
	if err != nil {
		return errors.Wrap(err, "getting a new SSH client connection")
	}
	client := ssh.NewClient(conn, chans, reqs)
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		User:            azure.DefaultUserName,
		Auth:            []ssh.AuthMethod{pubkey},
	}
	return &sshConfig, nil
}

// publicKeyFile parses and returns the public key from the specified private key file.
func publicKeyFile(file string) (ssh.AuthMethod, error) {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
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
	resp, err := http.Get(ciVersionURL)
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
