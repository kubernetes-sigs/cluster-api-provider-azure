/*
Copyright 2021 The Kubernetes Authors.

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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("migrator")

	int := make(chan os.Signal, 1)
	signal.Notify(int, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-int; cancel() }()

	if err := run(ctx, cancel, log); err != nil {
		log.Error(err, "failed to run migrator")
		os.Exit(1)
	}
}

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger) error {
	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	scheme := runtime.NewScheme()
	if err := setupScheme(scheme); err != nil {
		return fmt.Errorf("failed to setup kubeclient scheme: %w", err)
	}

	kubeclient, err := client.New(kubeconfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("failed to create kubeclient: %w", err)
	}

	if err := migrate(ctx, cancel, log, kubeclient); err != nil {
		return fmt.Errorf("failed to migrate clusters: %w", err)
	}

	log.Info("Deleting AzureManagedCluster CRD")
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "azuremanagedclusters.infrastructure.cluster.x-k8s.io",
		},
	}

	if err := kubeclient.Delete(ctx, crd); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("AzureManagedCluster CRD already deleted, successfully cleaned up")
			return nil
		}
		log.Error(err, "failed to delete AzureManagedCluster CRD; delete it manually")
	}

	return nil
}

func migrate(ctx context.Context, cancel context.CancelFunc, log logr.Logger, kubeclient client.Client) error {
	var clusterList clusterv1.ClusterList
	if err := kubeclient.List(ctx, &clusterList); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("no Cluster API Clusters found. no migration required.")
			return nil
		}
		return fmt.Errorf("failed to list Cluster API Clusters: %w", err)
	}

	cursor := 0
	for _, cluster := range clusterList.Items {
		if cluster.Spec.InfrastructureRef != nil && cluster.Spec.InfrastructureRef.Kind == "AzureManagedCluster" {
			log.Info("will migrate cluster", cluster.Namespace, cluster.Name)
			clusterList.Items[cursor] = cluster
			cursor++
		} else {
			log.Info("skipping cluster", cluster.Namespace, cluster.Name)
		}
	}
	clusterList.Items = clusterList.Items[:cursor]

	if len(clusterList.Items) == 0 {
		log.Info("no Cluster API Clusters found using AzureManagedCluster. no migration necessary.")
		return nil
	}

	for idx := range clusterList.Items {
		cluster := &clusterList.Items[idx]
		copy := cluster.DeepCopy()
		copy.Spec.InfrastructureRef = copy.Spec.ControlPlaneRef

		if err := kubeclient.Patch(ctx, copy, client.MergeFrom(cluster)); err != nil {
			return fmt.Errorf("failed to patch Cluster %s/%s with error: %w", cluster.Namespace, cluster.Name, err)
		}

		log.Info("Successfully patched Cluster", cluster.Namespace, cluster.Name)
	}

	log.Info("Successfully patched all clusters!")

	return nil
}

func setupScheme(scheme *runtime.Scheme) error {
	schemeFn := []func(*runtime.Scheme) error{
		clusterv1.AddToScheme,
		apiextensionsv1.AddToScheme,
	}
	for _, fn := range schemeFn {
		fn := fn
		if err := fn(scheme); err != nil {
			return err
		}
	}
	return nil
}
