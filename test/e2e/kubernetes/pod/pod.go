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

package pod

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	podExecOperationTimeout             = 3 * time.Minute
	podExecOperationSleepBetweenRetries = 3 * time.Second
)

func Exec(clientset *kubernetes.Clientset, config *restclient.Config, pod corev1.Pod, command []string, testSuccess bool) error {
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(pod.GetName()).
		Namespace(pod.GetNamespace()).SubResource("exec")
	option := &corev1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	if !testSuccess {
		option.Stderr = false
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	Eventually(func(g Gomega) {
		err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		if testSuccess {
			g.Expect(err).NotTo(HaveOccurred())
		} else {
			// If we get here we are validating that the command returned an expected error
			g.Expect(err).To(HaveOccurred())
		}
	}, podExecOperationTimeout, podExecOperationSleepBetweenRetries).Should(Succeed())

	return nil
}
