/*
Copyright 2019 The Kubernetes Authors.

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

package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"k8s.io/klog"

	"sigs.k8s.io/cluster-api/pkg/util"
)

func TestContainerdSHA256(t *testing.T) {
	fileName := fmt.Sprintf("/tmp/test-%s", util.RandomString(6))
	defer os.Remove(fileName)

	url := fmt.Sprintf("https://storage.googleapis.com/cri-containerd-release/cri-containerd-%s.linux-amd64.tar.gz", ContainerdVersion)

	klog.Infof("saving to %s", fileName)

	cmd := exec.Command("wget",
		url,
		"-O",
		fileName)

	var buf bytes.Buffer
	cmd.Stderr = &buf
	cmd.Stdout = &buf

	cmd.Start()
	err := cmd.Wait()

	if err != nil {
		t.Errorf("Error dowinloading containerd: %v", buf.String())
	}
	cmd = exec.Command("sha256sum", "--check", "-")

	buf.Reset()
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	var in bytes.Buffer
	cmd.Stdin = &in

	fmt.Fprintf(&in, "%s %s", ContainerdSHA256, fileName)

	cmd.Start()
	err = cmd.Wait()

	fmt.Println(buf.String())

	if err != nil {
		t.Error(err)
	}
}
