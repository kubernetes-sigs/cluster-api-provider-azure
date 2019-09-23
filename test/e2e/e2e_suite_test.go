// /*
// Copyright 2019 The Kubernetes Authors.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package e2e_test

// import (
// 	"bytes"
// 	"context"
// 	"flag"
// 	"fmt"
// 	"io/ioutil"
// 	"testing"
// 	"time"

// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"

// 	appsv1 "k8s.io/api/apps/v1"
// 	apimachinerytypes "k8s.io/apimachinery/pkg/types"
// 	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/util/kind"
// 	"sigs.k8s.io/cluster-api/pkg/util"
// 	crclient "sigs.k8s.io/controller-runtime/pkg/client"
// )

// func TestE2e(t *testing.T) {
// 	if err := initLocation(); err != nil {
// 		t.Fatal(err)
// 	}
// 	RegisterFailHandler(Fail)
// 	RunSpecs(t, "e2e Suite")
// }

// const (
// 	setupTimeout          = 10 * 60
// 	capzProviderNamespace = "azure-provider-system"
// 	capzStatefulSetName   = "azure-provider-controller-manager"
// )

// var (
// 	// TODO: Do we want to do file-based auth? Not suggested. If we determine no, remove this deadcode
// 	//credFile               = flag.String("credFile", "", "path to an Azure credentials file")
// 	locationFile           = flag.String("locationFile", "", "The path to a text file containing the Azure location")
// 	providerComponentsYAML = flag.String("providerComponentsYAML", "", "path to the provider components YAML for the cluster API")
// 	managerImageTar        = flag.String("managerImageTar", "", "a script to load the manager Docker image into Docker")

// 	kindCluster kind.Cluster
// 	kindClient  crclient.Client
// 	location    string
// )

// var _ = BeforeSuite(func() {
// 	fmt.Fprintf(GinkgoWriter, "Setting up kind cluster\n")
// 	kindCluster = kind.Cluster{
// 		Name: "capz-test-" + util.RandomString(6),
// 	}
// 	kindCluster.Setup()
// 	loadManagerImage(kindCluster)

// 	fmt.Fprintf(GinkgoWriter, "Applying Provider Components to the kind cluster\n")
// 	applyProviderComponents(kindCluster)
// 	cfg := kindCluster.RestConfig()
// 	var err error
// 	kindClient, err = crclient.New(cfg, crclient.Options{})
// 	Expect(err).To(BeNil())

// 	fmt.Fprintf(GinkgoWriter, "Creating Azure prerequisites\n")
// 	// TODO: Probably need to init auth session to Azure here

// 	fmt.Fprintf(GinkgoWriter, "Ensuring ProviderComponents are deployed\n")
// 	Eventually(
// 		func() (int32, error) {
// 			statefulSet := &appsv1.StatefulSet{}
// 			if err := kindClient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: capzProviderNamespace, Name: capzStatefulSetName}, statefulSet); err != nil {
// 				return 0, err
// 			}
// 			return statefulSet.Status.ReadyReplicas, nil
// 		}, 5*time.Minute, 15*time.Second,
// 	).ShouldNot(BeZero())

// 	fmt.Fprintf(GinkgoWriter, "Running in Azure location: %s\n", location)
// }, setupTimeout)

// var _ = AfterSuite(func() {
// 	fmt.Fprintf(GinkgoWriter, "Tearing down kind cluster\n")
// 	kindCluster.Teardown()
// })

// // TODO: Determine if we need this
// func initLocation() error {
// 	if locationFile != nil && *locationFile != "" {
// 		data, err := ioutil.ReadFile(*locationFile)
// 		if err != nil {
// 			return fmt.Errorf("error reading AWS location file: %v", err)
// 		}
// 		location = string(bytes.TrimSpace(data))
// 		return nil
// 	}

// 	location = "eastus"
// 	return nil
// }

// // TODO: Add function to handle auth to Azure

// func loadManagerImage(kindCluster kind.Cluster) {
// 	if managerImageTar != nil && *managerImageTar != "" {
// 		kindCluster.LoadImageArchive(*managerImageTar)
// 	}
// }

// func applyProviderComponents(kindCluster kind.Cluster) {
// 	Expect(providerComponentsYAML).ToNot(BeNil())
// 	Expect(*providerComponentsYAML).ToNot(BeEmpty())
// 	kindCluster.ApplyYAML(*providerComponentsYAML)
// }
