/*
Copyright 2026 The Kubernetes Authors.

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

package rbac_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/repository"
	yaml "sigs.k8s.io/cluster-api/cmd/clusterctl/client/yamlprocessor"
)

func TestKubeadmBootstrapControlPlaneRoleBindingCustomTargetNamespace(t *testing.T) {
	const targetNamespace = "custom-provider-system"

	rawYAML, err := os.ReadFile("kubeadm_bootstrap_control_plane_role.yaml")
	if err != nil {
		t.Fatalf("failed to read RBAC manifest: %v", err)
	}

	configClient, err := config.New(t.Context(), "", config.InjectReader(&emptyConfigReader{}))
	if err != nil {
		t.Fatalf("failed to create clusterctl config client: %v", err)
	}

	components, err := repository.NewComponents(repository.ComponentsInput{
		Provider:     config.NewProvider("azure", "", clusterctlv1.InfrastructureProviderType),
		ConfigClient: configClient,
		Processor:    yaml.NewSimpleProcessor(),
		RawYaml:      rawYAML,
		Options: repository.ComponentsOptions{
			TargetNamespace:     targetNamespace,
			SkipTemplateProcess: true,
		},
	})
	if err != nil {
		t.Fatalf("failed to process provider components: %v", err)
	}

	var binding rbacv1.ClusterRoleBinding
	found := false
	for _, obj := range components.Objs() {
		if obj.GetKind() != "ClusterRoleBinding" {
			continue
		}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &binding); err != nil {
			t.Fatalf("failed to convert ClusterRoleBinding: %v", err)
		}
		found = true
		break
	}
	if !found {
		t.Fatal("ClusterRoleBinding not found")
	}

	var defaultNamespaceGroup, customNamespaceServiceAccount bool
	for _, subject := range binding.Subjects {
		switch {
		case subject.Kind == "Group" && subject.Name == "system:serviceaccounts:capi-kubeadm-bootstrap-system":
			defaultNamespaceGroup = true
		case subject.Kind == "ServiceAccount" &&
			subject.Name == "capi-kubeadm-bootstrap-manager" &&
			subject.Namespace == targetNamespace:
			customNamespaceServiceAccount = true
		}
	}

	if !defaultNamespaceGroup {
		t.Error("default CABPK namespace Group subject is missing")
	}
	if !customNamespaceServiceAccount {
		t.Errorf("CABPK ServiceAccount subject was not rewritten to target namespace %q", targetNamespace)
	}
}

type emptyConfigReader struct{}

func (*emptyConfigReader) Init(context.Context, string) error {
	return nil
}

func (*emptyConfigReader) Get(key string) (string, error) {
	return "", fmt.Errorf("config value %q not found", key)
}

func (*emptyConfigReader) Set(string, string) {}

func (*emptyConfigReader) UnmarshalKey(string, interface{}) error {
	return nil
}
