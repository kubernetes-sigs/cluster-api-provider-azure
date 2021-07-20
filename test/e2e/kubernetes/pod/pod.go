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
	"bytes"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

func Exec(clientset *kubernetes.Clientset, config *restclient.Config, pod v1.Pod, command []string) error {
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(pod.GetName()).
		Namespace(pod.GetNamespace()).SubResource("exec")
	option := &v1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}

	return nil
}

func ExecWithOutput(clientset *kubernetes.Clientset, config *restclient.Config, pod v1.Pod, command []string) (*bytes.Buffer, error) {
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(pod.GetName()).
		Namespace(pod.GetNamespace()).SubResource("exec")
	option := &v1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, err
	}
	stdout := bytes.NewBuffer(nil)
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: stdout,
		// needs to be populated or else exec hangs, but we don't need the output. Failures write to stdout when TTY enabled.
		Stderr: bytes.NewBuffer(nil),
	})

	return stdout, err
}
