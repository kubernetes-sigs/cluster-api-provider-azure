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
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

type Bootstrap struct {
	Version string
}

// GetName returns the name of the components being generated.
func (g *Bootstrap) GetName() string {
	return fmt.Sprintf("Cluster API Bootstrap Provider Kubeadm %s", g.Version)
}

func (g *Bootstrap) releaseYAMLPath() string {
	return fmt.Sprintf("https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/download/%s/bootstrap-components.yaml", g.Version)
}

// Manifests return the generated components and any error if there is one.
func (g *Bootstrap) Manifests(ctx context.Context) ([]byte, error) {
	resp, err := http.Get(g.releaseYAMLPath())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close()
	return out, nil
}
