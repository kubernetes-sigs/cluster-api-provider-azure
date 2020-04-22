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

package generators

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api/test/framework/exec"
)

type KubeadmControPlane struct {
	Version string
}

// GetName returns the name of the components being generated.
func (g *KubeadmControPlane) GetName() string {
	return fmt.Sprintf("Cluster API Kubeadm ControPlane Provider %s", g.Version)
}

func (g *KubeadmControPlane) kustomizePath() string {
	return fmt.Sprintf("https://github.com/kubernetes-sigs/cluster-api//controlplane/kubeadm/config")
}

// Manifests return the generated components and any error if there is one.
func (g *KubeadmControPlane) Manifests(ctx context.Context) ([]byte, error) {
	kustomize := exec.NewCommand(
		exec.WithCommand("kustomize"),
		exec.WithArgs("build", g.kustomizePath()),
	)
	stdout, stderr, err := kustomize.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return nil, errors.WithStack(err)
	}
	return stdout, nil
}
