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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/auth"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/framework/exec"
)

type Infra struct {
	Creds auth.Creds
}

// GetName returns the name of the components being generated.
func (g *Infra) GetName() string {
	return "Cluster API Provider Azure: file system"
}

func (g *Infra) kustomizePath(path string) string {
	return "../../config/" + path
}

// Manifests return the generated components and any error if there is one.
func (g *Infra) Manifests(ctx context.Context) ([]byte, error) {
	kustomize := exec.NewCommand(
		exec.WithCommand("kustomize"),
		exec.WithArgs("build", g.kustomizePath("default")),
	)
	stdout, stderr, err := kustomize.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return nil, errors.WithStack(err)
	}

	g.prepareEnvsubst()
	envsubst := exec.NewCommand(
		exec.WithCommand("envsubst"),
		exec.WithStdin(bytes.NewReader(stdout)),
	)
	stdout, stderr, err = envsubst.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return nil, errors.WithStack(err)
	}
	return stdout, nil
}

func (g *Infra) prepareEnvsubst() {
	os.Setenv("AZURE_CLIENT_ID_B64", base64.StdEncoding.EncodeToString([]byte(g.Creds.ClientID)))
	os.Setenv("AZURE_CLIENT_SECRET_B64", base64.StdEncoding.EncodeToString([]byte(g.Creds.ClientSecret)))
	os.Setenv("AZURE_SUBSCRIPTION_ID_B64", base64.StdEncoding.EncodeToString([]byte(g.Creds.SubscriptionID)))
	os.Setenv("AZURE_TENANT_ID_B64", base64.StdEncoding.EncodeToString([]byte(g.Creds.TenantID)))
}
