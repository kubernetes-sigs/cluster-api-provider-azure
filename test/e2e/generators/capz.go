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
	"encoding/base64"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/auth"
	"sigs.k8s.io/cluster-api/test/framework/exec"
)

type Infra struct {
	Creds auth.Creds
}

// GetName returns the name of the components being generated.
func (g *Infra) GetName() string {
	return "Cluster API Provider Azure: file system"
}

func (g *Infra) kustomizePath() string {
	return "../../config/"
}

// Manifests return the generated components and any error if there is one.
func (g *Infra) Manifests(ctx context.Context) ([]byte, error) {
	kustomize := exec.NewCommand(
		exec.WithCommand("kustomize"),
		exec.WithArgs("build", g.kustomizePath()),
	)
	stdout, stderr, err := kustomize.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return nil, errors.WithStack(err)
	}
	return expandCredVariables(stdout, g.Creds), nil
}

func expandCredVariables(stdout []byte, creds auth.Creds) []byte {
	os.Setenv("AZURE_CLIENT_ID_B64", base64.StdEncoding.EncodeToString([]byte(creds.ClientID)))
	os.Setenv("AZURE_CLIENT_SECRET_B64", base64.StdEncoding.EncodeToString([]byte(creds.ClientSecret)))
	os.Setenv("AZURE_SUBSCRIPTION_ID_B64", base64.StdEncoding.EncodeToString([]byte(creds.SubscriptionID)))
	os.Setenv("AZURE_TENANT_ID_B64", base64.StdEncoding.EncodeToString([]byte(creds.TenantID)))
	return []byte(os.ExpandEnv(string(stdout)))
}
