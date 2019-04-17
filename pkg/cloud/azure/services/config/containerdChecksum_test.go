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

func TestHash(t *testing.T) {
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
