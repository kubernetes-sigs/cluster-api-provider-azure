# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

GOPATH  := $(shell go env GOPATH)
GOARCH  := $(shell go env GOARCH)
GOOS    := $(shell go env GOOS)
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

# curl retries
CURL_RETRIES=3

# Directories.
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/bin)
TEMPLATES_DIR := $(ROOT_DIR)/templates
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
EXP_DIR := exp
GO_INSTALL = ./scripts/go_install.sh
E2E_DATA_DIR ?= $(ROOT_DIR)/test/e2e/data
KUBETEST_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/conformance.yaml)
KUBETEST_WINDOWS_CONFIG ?= upstream-windows.yaml
KUBETEST_WINDOWS_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/$(KUBETEST_WINDOWS_CONFIG))
KUBETEST_REPO_LIST_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/)
AZURE_TEMPLATES := $(E2E_DATA_DIR)/infrastructure-azure
ADDONS_DIR := templates/addons
CONVERSION_VERIFIER := $(TOOLS_BIN_DIR)/conversion-verifier

# use the project local tool binaries first
export PATH := $(TOOLS_BIN_DIR):$(PATH)

# set --output-base used for conversion-gen which needs to be different for in GOPATH and outside GOPATH dev
ifneq ($(abspath $(ROOT_DIR)),$(GOPATH)/src/sigs.k8s.io/cluster-api-provider-azure)
  OUTPUT_BASE := --output-base=$(ROOT_DIR)
endif

# Binaries.
CONTROLLER_GEN_VER := v0.8.0
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)

CONVERSION_GEN_VER := v0.23.1
CONVERSION_GEN_BIN := conversion-gen
CONVERSION_GEN := $(TOOLS_BIN_DIR)/$(CONVERSION_GEN_BIN)-$(CONVERSION_GEN_VER)

ENVSUBST_VER := v2.0.0-20210730161058-179042472c46
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)-$(ENVSUBST_VER)

GOLANGCI_LINT_VER := v1.45.2
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)

KUSTOMIZE_VER := v4.5.2
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER)

MOCKGEN_VER := v1.6.0
MOCKGEN_BIN := mockgen
MOCKGEN := $(TOOLS_BIN_DIR)/$(MOCKGEN_BIN)-$(MOCKGEN_VER)

RELEASE_NOTES_VER := v0.12.0
RELEASE_NOTES_BIN := release-notes
RELEASE_NOTES := $(TOOLS_BIN_DIR)/$(RELEASE_NOTES_BIN)-$(RELEASE_NOTES_VER)

KPROMO_VER := v3.3.0-beta.3
KPROMO_BIN := kpromo
KPROMO := $(TOOLS_BIN_DIR)/$(KPROMO_BIN)-$(KPROMO_VER)

GO_APIDIFF_VER := v0.1.0
GO_APIDIFF_BIN := go-apidiff
GO_APIDIFF := $(TOOLS_BIN_DIR)/$(GO_APIDIFF_BIN)

GINKGO_VER := v1.16.5
GINKGO_BIN := ginkgo
GINKGO := $(TOOLS_BIN_DIR)/$(GINKGO_BIN)-$(GINKGO_VER)

KUBECTL_VER := v1.22.4
KUBECTL_BIN := kubectl
KUBECTL := $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)-$(KUBECTL_VER)

HELM_VER := v3.8.1
HELM_BIN := helm
HELM := $(TOOLS_BIN_DIR)/$(HELM_BIN)-$(HELM_VER)

YQ_VER := v4.14.2
YQ_BIN := yq
YQ :=  $(TOOLS_BIN_DIR)/$(YQ_BIN)-$(YQ_VER)

KUBE_APISERVER=$(TOOLS_BIN_DIR)/kube-apiserver
ETCD=$(TOOLS_BIN_DIR)/etcd

# Define Docker related variables. Releases should modify and double check these vars.
REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
STAGING_REGISTRY := gcr.io/k8s-staging-cluster-api-azure
PROD_REGISTRY := us.gcr.io/k8s-artifacts-prod/cluster-api-azure
IMAGE_NAME ?= cluster-api-azure-controller
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= dev
ARCH ?= $(GOARCH)
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always

# Allow overriding the e2e configurations
GINKGO_FOCUS ?= \[REQUIRED\]
GINKGO_SKIP ?=
GINKGO_NODES ?= 3
GINKGO_NOCOLOR ?= false
GINKGO_ARGS ?=
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/azure-dev.yaml
E2E_CONF_FILE_ENVSUBST := $(ROOT_DIR)/test/e2e/config/azure-dev-envsubst.yaml
SKIP_CLEANUP ?= false
SKIP_CREATE_MGMT_CLUSTER ?= false
WIN_REPO_URL ?=

# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

CLUSTER_TEMPLATE ?= cluster-template.yaml
MANAGED_CLUSTER_TEMPLATE ?= cluster-template-aks.yaml

## --------------------------------------
## Binaries
## --------------------------------------

##@ Binaries:

.PHONY: binaries
binaries: manager ## Builds all binaries.

.PHONY: manager
manager: ## Build manager binary.
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/manager .

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

##@ Cleanup / Verification:

.PHONY: clean
clean: ## Remove bin and kubeconfigs.
	$(MAKE) clean-bin
	$(MAKE) clean-temporary

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries.
	rm -rf bin
	rm -rf hack/tools/bin

.PHONY: clean-temporary
clean-temporary: ## Remove all temporary files and folders.
	rm -f minikube.kubeconfig
	rm -f kubeconfig
	rm -f *.kubeconfig

.PHONY: clean-release
clean-release: ## Remove the release folder.
	rm -rf $(RELEASE_DIR)

.PHONY: apidiff
apidiff: $(GO_APIDIFF) ## Check for API differences.
	$(GO_APIDIFF) $(shell git rev-parse origin/main) --print-compatible

.PHONY: format-tiltfile
format-tiltfile: ## Format the Tiltfile.
	./hack/verify-starlark.sh fix

.PHONY: verify
verify: verify-boilerplate verify-modules verify-gen verify-shellcheck verify-conversions verify-tiltfile ## Run "verify-boilerplate", "verify-modules", "verify-gen", "verify-shellcheck", "verify-conversions", "verify-tiltfile" rules.

.PHONY: verify-boilerplate
verify-boilerplate: ## Verify boilerplate header.
	./hack/verify-boilerplate.sh

.PHONY: verify-modules
verify-modules: modules ## Verify go.sum go.mod are the latest.
	@if !(git diff --quiet HEAD -- go.sum go.mod); then \
		echo "go module files are out of date"; exit 1; \
	fi

.PHONY: verify-gen
verify-gen: generate ## Verify generated files are the latest.
	@if !(git diff --quiet HEAD); then \
		git diff; echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files are passing lint.
	./hack/verify-shellcheck.sh

.PHONY: verify-conversions
verify-conversions: $(CONVERSION_VERIFIER)  ## Verifies expected API conversion are in place.
	$(CONVERSION_VERIFIER)

.PHONY: verify-tiltfile
verify-tiltfile: ## Verify Tiltfile format.
	./hack/verify-starlark.sh

## --------------------------------------
## Development
## --------------------------------------

##@ Development:

.PHONY: install-tools # populate hack/tools/bin
install-tools: $(ENVSUBST) $(KUSTOMIZE) $(KUBECTL) $(HELM) $(GINKGO)

.PHONY: create-management-cluster
create-management-cluster: $(KUSTOMIZE) $(ENVSUBST) ## Create a management cluster.
	# Create kind management cluster.
	$(MAKE) kind-create

	# Install cert manager and wait for availability
	./hack/install-cert-manager.sh

	# Create secret for AzureClusterIdentity
	./hack/create-identity-secret.sh

	# Deploy CAPI
	curl --retry $(CURL_RETRIES) -sSL https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.1.2/cluster-api-components.yaml | $(ENVSUBST) | kubectl apply -f -

	# Deploy CAPZ
	kind load docker-image $(CONTROLLER_IMG)-$(ARCH):$(TAG) --name=capz
	$(KUSTOMIZE) build config/default | $(ENVSUBST) | kubectl apply -f -

	# Wait for CAPI deployments
	kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment -l cluster.x-k8s.io/provider=cluster-api
	kubectl wait --for=condition=Available --timeout=5m -n capi-kubeadm-bootstrap-system deployment -l cluster.x-k8s.io/provider=bootstrap-kubeadm
	kubectl wait --for=condition=Available --timeout=5m -n capi-kubeadm-control-plane-system deployment -l cluster.x-k8s.io/provider=control-plane-kubeadm

	# apply CNI ClusterResourceSets
	kubectl create configmap calico-addon --from-file=templates/addons/calico.yaml
	kubectl create configmap calico-ipv6-addon --from-file=templates/addons/calico-ipv6.yaml
	kubectl create configmap calico-dual-stack-addon --from-file=templates/addons/calico-dual-stack.yaml
	kubectl create configmap calico-windows-addon --from-file=templates/addons/windows/calico

	kubectl apply -f templates/addons/calico-resource-set.yaml

	# Wait for CAPZ deployments
	kubectl wait --for=condition=Available --timeout=5m -n capz-system deployment -l cluster.x-k8s.io/provider=infrastructure-azure

	# required sleep for when creating management and workload cluster simultaneously
	sleep 10
	@echo 'Set kubectl context to the kind management cluster by running "kubectl config set-context kind-capz"'

.PHONY: create-workload-cluster
create-workload-cluster: $(ENVSUBST) ## Create a workload cluster.
	# Create workload Cluster.
	@if [ -f "$(TEMPLATES_DIR)/$(CLUSTER_TEMPLATE)" ]; then \
		$(ENVSUBST) < "$(TEMPLATES_DIR)/$(CLUSTER_TEMPLATE)" | kubectl apply -f -; \
	elif [ -f "$(CLUSTER_TEMPLATE)" ]; then \
		$(ENVSUBST) < "$(CLUSTER_TEMPLATE)" | kubectl apply -f -; \
	else \
		curl --retry "$(CURL_RETRIES)" "$(CLUSTER_TEMPLATE)" | "$(ENVSUBST)" | kubectl apply -f -; \
	fi

	# Wait for the kubeconfig to become available.
	timeout --foreground 300 bash -c "while ! kubectl get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	kubectl get secrets $(CLUSTER_NAME)-kubeconfig -o json | jq -r .data.value | base64 --decode > ./kubeconfig
	timeout --foreground 600 bash -c "while ! kubectl --kubeconfig=./kubeconfig get nodes | grep control-plane; do sleep 1; done"

	@echo 'run "kubectl --kubeconfig=./kubeconfig ..." to work with the new target cluster'

.PHONY: create-aks-cluster
create-aks-cluster: $(KUSTOMIZE) $(ENVSUBST) ## Create a aks cluster.
	# Create managed Cluster.
	$(ENVSUBST) < $(TEMPLATES_DIR)/$(MANAGED_CLUSTER_TEMPLATE) | kubectl apply -f -

	# Wait for the kubeconfig to become available.
	timeout --foreground 300 bash -c "while ! kubectl get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	kubectl get secrets $(CLUSTER_NAME)-kubeconfig -o json | jq -r .data.value | base64 --decode > ./kubeconfig
	timeout --foreground 600 bash -c "while ! kubectl --kubeconfig=./kubeconfig get nodes | grep control-plane; do sleep 1; done"

	@echo 'run "kubectl --kubeconfig=./kubeconfig ..." to work with the new target cluster'


.PHONY: create-cluster
create-cluster: ## Create a workload development Kubernetes cluster on Azure in a kind management cluster.
	EXP_CLUSTER_RESOURCE_SET=true \
	EXP_AKS=true \
	EXP_MACHINE_POOL=true \
	$(MAKE) create-management-cluster \
	create-workload-cluster

.PHONY: delete-workload-cluster
delete-workload-cluster: ## Deletes the example workload Kubernetes cluster.
	@echo 'Your Azure resources will now be deleted, this can take up to 20 minutes'
	kubectl delete cluster $(CLUSTER_NAME)

## --------------------------------------
## Docker
## --------------------------------------

##@ Docker:

.PHONY: docker-pull-prerequisites
docker-pull-prerequisites: ## Pull prerequisites for building controller-manager.
	docker pull docker/dockerfile:1.1-experimental
	docker pull docker.io/library/golang:1.17
	docker pull gcr.io/distroless/static:latest

.PHONY: docker-build
docker-build: docker-pull-prerequisites ## Build the docker image for controller-manager.
	DOCKER_BUILDKIT=1 docker build --build-arg goproxy=$(GOPROXY) --build-arg ARCH=$(ARCH) --build-arg ldflags="$(LDFLAGS)" . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

##@ Docker - All Arch:

.PHONY: docker-build-all
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH)) ## Build all the architecture docker images.

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-push-all
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH)) ## Push all the architecture docker images.
	$(MAKE) docker-push-manifest

docker-push-%:
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: set-manifest-image
set-manifest-image: ## Update kustomize image patch file for default resource.
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_image_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy: ## Update kustomize pull policy file for default resource.
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml

## --------------------------------------
## Generate
## --------------------------------------

##@ Generate:

.PHONY: generate
generate: ## Generate go related targets, manifests, flavors, e2e-templates and addons.
	$(MAKE) generate-go
	$(MAKE) generate-manifests
	$(MAKE) generate-flavors
	$(MAKE) generate-e2e-templates
	$(MAKE) generate-addons

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(MOCKGEN) $(CONVERSION_GEN) ## Runs Go related generate targets.
	$(CONTROLLER_GEN) \
		paths=./api/... \
		paths=./$(EXP_DIR)/api/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt
	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha3 \
		--build-tag=ignore_autogenerated_core_v1alpha3 \
		--extra-peer-dirs=sigs.k8s.io/cluster-api/api/v1alpha3 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt $(OUTPUT_BASE)
	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha4 \
		--build-tag=ignore_autogenerated_core_v1alpha4 \
		--extra-peer-dirs=sigs.k8s.io/cluster-api/api/v1alpha4 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt $(OUTPUT_BASE)
	$(CONVERSION_GEN) \
		--input-dirs=./$(EXP_DIR)/api/v1alpha3 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt $(OUTPUT_BASE)
	$(CONVERSION_GEN) \
		--input-dirs=./$(EXP_DIR)/api/v1alpha4 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt $(OUTPUT_BASE)
	go generate ./...

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./api/... \
		paths=./$(EXP_DIR)/api/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=$(CRD_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook
	$(CONTROLLER_GEN) \
		paths=./controllers/... \
		paths=./$(EXP_DIR)/controllers/... \
		output:rbac:dir=$(RBAC_ROOT) \
		rbac:roleName=manager-role

.PHONY: generate-flavors ## Generate template flavors.
generate-flavors: $(KUSTOMIZE) generate-addons
	./hack/gen-flavors.sh

.PHONY: generate-e2e-templates
generate-e2e-templates: $(KUSTOMIZE) ## Generate Azure infrastructure templates for the v1alpha4 CAPI test suite.
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-md-remediation --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template-md-remediation.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-remediation --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-remediation.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-adoption/step1 --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-adoption.yaml
	echo "---" >> $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-adoption.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-adoption/step2 --load-restrictor LoadRestrictionsNone >> $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-adoption.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-machine-pool --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template-machine-pool.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-node-drain --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template-node-drain.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-upgrades --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template-upgrades.yaml
	$(KUSTOMIZE) build $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-scale-in --load-restrictor LoadRestrictionsNone > $(AZURE_TEMPLATES)/v1beta1/cluster-template-kcp-scale-in.yaml

.PHONY: generate-addons
generate-addons: fetch-calico-manifests ## Generate metric-server, calico calico-ipv6 addons.
	$(KUSTOMIZE) build $(ADDONS_DIR)/metrics-server > $(ADDONS_DIR)/metrics-server/metrics-server.yaml
	$(KUSTOMIZE) build $(ADDONS_DIR)/calico > $(ADDONS_DIR)/calico.yaml
	$(KUSTOMIZE) build $(ADDONS_DIR)/calico-ipv6 > $(ADDONS_DIR)/calico-ipv6.yaml
	$(KUSTOMIZE) build $(ADDONS_DIR)/calico-dual-stack > $(ADDONS_DIR)/calico-dual-stack.yaml

# When updating this, make sure to also update the Windows image version in templates/addons/windows/calico.
CALICO_VERSION := v3.23.0

.PHONY: fetch-calico-manifests
fetch-calico-manifests: ## Get Calico release manifests and unzip them.
	@echo "Fetching Calico release manifests from release artifacts, this might take a minute..."
	wget -qO- https://github.com/projectcalico/calico/releases/download/$(CALICO_VERSION)/release-$(CALICO_VERSION).tgz | tar xz release-$(CALICO_VERSION)/manifests/calico-vxlan.yaml release-$(CALICO_VERSION)/manifests/calico-policy-only.yaml
	mv release-$(CALICO_VERSION)/manifests/calico-vxlan.yaml $(ADDONS_DIR)/calico
	mv release-$(CALICO_VERSION)/manifests/calico-policy-only.yaml $(ADDONS_DIR)/calico-ipv6

.PHONY: modules
modules: ## Runs go mod tidy to ensure proper vendoring.
	go mod tidy

## --------------------------------------
## Help
## --------------------------------------

##@ Help:

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Linting
## --------------------------------------

##@ Linting:

.PHONY: lint
lint: $(GOLANGCI_LINT) lint-latest ## Lint codebase.
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS)

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Lint the codebase and run auto-fixers if supported by the linter.
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues.
	$(GOLANGCI_LINT) run -v --fast=false

.PHONY: lint-latest
lint-latest:
	./hack/lint-latest.sh

## --------------------------------------
## Release
## --------------------------------------

##@ Release:

RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
# if the release tag contains a hyphen, treat it as a pre-release
ifneq (,$(findstring -,$(RELEASE_TAG)))
    PRE_RELEASE=true
endif
# the previous release tag, e.g., v0.3.9, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)
RELEASE_DIR ?= out
RELEASE_NOTES_DIR := _releasenotes
GIT_REPO_NAME ?= cluster-api-provider-azure
GIT_ORG_NAME ?= kubernetes-sigs
FULL_VERSION := $(RELEASE_TAG:v%=%)
MINOR_VERSION := $(shell v='$(FULL_VERSION)'; echo "$${v%.*}")
RELEASE_BRANCH ?= release-$(MINOR_VERSION)
USER_FORK ?= $(shell git config --get remote.origin.url | cut -d/ -f4)
IMAGE_REVIEWERS ?= $(shell ./hack/get-project-maintainers.sh)

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests
	$(MAKE) release-templates
	$(MAKE) release-metadata

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release.
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

.PHONY: release-templates
release-templates: $(RELEASE_DIR)
	cp templates/cluster-template* $(RELEASE_DIR)/

.PHONY: release-metadata
release-metadata: $(RELEASE_DIR)
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release-binary
release-binary: $(RELEASE_DIR) ## Compile and build release binaries.
	docker run \
		--rm \
		-e CGO_ENABLED=0 \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		golang:1.17 \
		go build -a -ldflags '$(LDFLAGS) -extldflags "-static"' \
		-o $(RELEASE_DIR)/$(notdir $(RELEASE_BINARY))-$(GOOS)-$(GOARCH) $(RELEASE_BINARY)

.PHONY: release-staging
release-staging: ## Builds and push container images to the staging bucket.
	REGISTRY=$(STAGING_REGISTRY) $(MAKE) docker-build-all docker-push-all release-alias-tag

RELEASE_ALIAS_TAG=$(PULL_BASE_REF)

.PHONY: release-alias-tag
release-alias-tag: ## Adds the tag to the last build tag.
	gcloud container images add-tag $(CONTROLLER_IMG):$(TAG) $(CONTROLLER_IMG):$(RELEASE_ALIAS_TAG)

.PHONY: release-notes
release-notes: $(RELEASE_NOTES) $(RELEASE_NOTES_DIR) ## Generate/update release notes.
	@if [ -n "${PRE_RELEASE}" ]; then echo ":rotating_light: This is a RELEASE CANDIDATE. Use it only for testing purposes. If you find any bugs, file an [issue](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/new)." > $(RELEASE_NOTES_DIR)/release-notes-$(RELEASE_TAG).md; \
	else $(RELEASE_NOTES) --org $(GIT_ORG_NAME) --repo $(GIT_REPO_NAME) --branch $(RELEASE_BRANCH)  --start-rev $(PREVIOUS_TAG) --end-rev $(RELEASE_TAG) --output $(RELEASE_NOTES_DIR)/tmp-release-notes.md --list-v2; \
	sed 's/\[SIG Cluster Lifecycle\]//g' $(RELEASE_NOTES_DIR)/tmp-release-notes.md > $(RELEASE_NOTES_DIR)/release-notes-$(RELEASE_TAG).md; \
	rm -f $(RELEASE_NOTES_DIR)/tmp-release-notes.md; \
	fi

.PHONY: promote-images
promote-images: $(KPROMO) ## Promote images.
	$(KPROMO) pr --project cluster-api-azure --tag $(RELEASE_TAG) --reviewers "$(IMAGE_REVIEWERS)" --fork $(USER_FORK)

## --------------------------------------
## Testing
## --------------------------------------

##@ Testing:

.PHONY: test
test: generate lint go-test ## Run "generate", "lint" and "go-tests" rules.

envs-test:
export TEST_ASSET_KUBECTL = $(KUBECTL)
export TEST_ASSET_KUBE_APISERVER = $(KUBE_APISERVER)
export TEST_ASSET_ETCD = $(ETCD)

.PHONY: go-test
go-test: envs-test $(KUBECTL) $(KUBE_APISERVER) $(ETCD) ## Run go tests.
	echo $(TEST_ASSET_KUBECTL)
	go test ./...

.PHONY: test-cover
test-cover: envs-test $(KUBECTL) $(KUBE_APISERVER) $(ETCD) ## Run tests with code coverage and code generate reports.
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out -o coverage.txt
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test-e2e-run
test-e2e-run: generate-e2e-templates install-tools ## Run e2e tests.
	$(ENVSUBST) < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST) && \
    $(GINKGO) -v -trace -tags=e2e -focus="$(GINKGO_FOCUS)" -skip="$(GINKGO_SKIP)" -nodes=$(GINKGO_NODES) --noColor=$(GINKGO_NOCOLOR) $(GINKGO_ARGS) ./test/e2e -- \
    	-e2e.artifacts-folder="$(ARTIFACTS)" \
    	-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
    	-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) -e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER) $(E2E_ARGS)

.PHONY: test-e2e
test-e2e: ## Run "docker-build" and "docker-push" rules then run e2e tests.
	PULL_POLICY=IfNotPresent MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) docker-build docker-push \
	test-e2e-run

LOCAL_GINKGO_ARGS ?= -stream --progress
LOCAL_GINKGO_ARGS += $(GINKGO_ARGS)
.PHONY: test-e2e-local
test-e2e-local: ## Run "docker-build" rule then run e2e tests.
	PULL_POLICY=IfNotPresent MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) docker-build \
	GINKGO_ARGS='$(LOCAL_GINKGO_ARGS)' \
	test-e2e-run

CONFORMANCE_E2E_ARGS ?= -kubetest.config-file=$(KUBETEST_CONF_PATH)
CONFORMANCE_E2E_ARGS += $(E2E_ARGS)
.PHONY: test-conformance
test-conformance: ## Run conformance test on workload cluster.
	$(MAKE) test-e2e-local GINKGO_FOCUS="Conformance" E2E_ARGS='$(CONFORMANCE_E2E_ARGS)' GINKGO_ARGS='$(LOCAL_GINKGO_ARGS)'

test-conformance-fast: ## Run conformance test on workload cluster using a subset of the conformance suite in parallel.
	$(MAKE) test-conformance CONFORMANCE_E2E_ARGS="-kubetest.config-file=$(KUBETEST_FAST_CONF_PATH) -kubetest.ginkgo-nodes=5 $(E2E_ARGS)"

.PHONY: test-windows-upstream
test-windows-upstream: ## Run windows upstream tests on workload cluster.
ifneq ($(WIN_REPO_URL), )
	curl --retry $(CURL_RETRIES) $(WIN_REPO_URL) -o $(KUBETEST_REPO_LIST_PATH)/custom-repo-list.yaml
endif
	$(MAKE) test-conformance CONFORMANCE_E2E_ARGS="-kubetest.config-file=$(KUBETEST_WINDOWS_CONF_PATH) -kubetest.repo-list-path=$(KUBETEST_REPO_LIST_PATH) $(E2E_ARGS)"

$(KUBE_APISERVER) $(ETCD): ## Install test asset kubectl, kube-apiserver, etcd.
	source ./scripts/fetch_ext_bins.sh && fetch_tools

## --------------------------------------
## Tilt / Kind
## --------------------------------------

##@ Tilt / Kind:

.PHONY: kind-create
kind-create: $(KUBECTL) ## Create capz kind cluster if needed.
	./scripts/kind-with-registry.sh

.PHONY: tilt-up
tilt-up: install-tools kind-create ## Start tilt and build kind cluster if needed.
	EXP_CLUSTER_RESOURCE_SET=true EXP_AKS=true EXP_MACHINE_POOL=true tilt up

.PHONY: delete-cluster
delete-cluster: delete-workload-cluster  ## Deletes the example kind cluster "capz".
	kind delete cluster --name=capz

.PHONY: kind-reset
kind-reset: ## Destroys the "capz" and "capz-e2e" kind clusters.
	kind delete cluster --name=capz || true
	kind delete cluster --name=capz-e2e || true

## --------------------------------------
## Tooling Binaries
## --------------------------------------

##@ Tooling Binaries:

conversion-verifier: $(CONVERSION_VERIFIER) go.mod go.sum ## Build a local copy of CAPI's conversion verifier.
controller-gen: $(CONTROLLER_GEN) ## Build a local copy of controller-gen.
conversion-gen: $(CONVERSION_GEN) ## Build a local copy of conversion-gen.
envsubst: $(ENVSUBST) ## Build a local copy of envsubst.
golangci-lint: $(GOLANGCI_LINT) ## Build a local copy of golang ci-lint.
kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize.
mockgen: $(MOCKGEN) ## Build a local copy of mockgen.
kpromo: $(KPROMO) ## Build a local copy of kpromo.
release-notes: $(RELEASE_NOTES) ## Build a local copy of release notes.
goapi-diff: $(GO_APIDIFF) ## Build a local copy of go api-diff.
ginkgo: $(GINKGO) ## Build a local copy of ginkgo.
kubectl: $(KUBECTL) ## Build a local copy of kubectl.
helm: $(HELM) ## Build a local copy of helm.
yq: $(YQ) ## Build a local copy of yq.

$(CONVERSION_VERIFIER): go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $@ sigs.k8s.io/cluster-api/hack/tools/conversion-verifier

$(CONTROLLER_GEN): ## Build controller-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-tools/cmd/controller-gen $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)

$(CONVERSION_GEN): ## Build conversion-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) k8s.io/code-generator/cmd/conversion-gen $(CONVERSION_GEN_BIN) $(CONVERSION_GEN_VER)

$(ENVSUBST): ## Build envsubst from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/drone/envsubst/v2/cmd/envsubst $(ENVSUBST_BIN) $(ENVSUBST_VER)

$(GOLANGCI_LINT): ## Build golangci-lint from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

$(KUSTOMIZE): ## Build kustomize from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) sigs.k8s.io/kustomize/kustomize/v4 $(KUSTOMIZE_BIN) $(KUSTOMIZE_VER)

$(MOCKGEN): ## Build mockgen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/golang/mock/mockgen $(MOCKGEN_BIN) $(MOCKGEN_VER)

$(KPROMO): ## Build kpromo from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) sigs.k8s.io/promo-tools/v3/cmd/kpromo $(KPROMO_BIN) $(KPROMO_VER)

$(RELEASE_NOTES): ## Build release notes from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) k8s.io/release/cmd/release-notes $(RELEASE_NOTES_BIN) $(RELEASE_NOTES_VER)

$(GO_APIDIFF): ## Build go-apidiff from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/joelanford/go-apidiff $(GO_APIDIFF_BIN) $(GO_APIDIFF_VER)

$(GINKGO): ## Build ginkgo from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/onsi/ginkgo/ginkgo $(GINKGO_BIN) $(GINKGO_VER)

$(KUBECTL): ## Build kubectl from tools folder.
	mkdir -p $(TOOLS_BIN_DIR)
	rm -f "$(TOOLS_BIN_DIR)/$(KUBECTL_BIN)*"
	curl --retry $(CURL_RETRIES) -fsL https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VER)/bin/$(GOOS)/$(GOARCH)/kubectl -o $(KUBECTL)
	ln -sf $(KUBECTL) $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)
	chmod +x $(KUBECTL) $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)

$(HELM): ## Put helm into tools folder.
	mkdir -p $(TOOLS_BIN_DIR)
	rm -f "$(TOOLS_BIN_DIR)/$(HELM_BIN)*"
	curl -fsSL -o $(TOOLS_BIN_DIR)/get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
	chmod 700 $(TOOLS_BIN_DIR)/get_helm.sh
	USE_SUDO=false HELM_INSTALL_DIR=$(TOOLS_BIN_DIR) DESIRED_VERSION=$(HELM_VER) BINARY_NAME=$(HELM_BIN)-$(HELM_VER) $(TOOLS_BIN_DIR)/get_helm.sh
	ln -sf $(HELM) $(TOOLS_BIN_DIR)/$(HELM_BIN)
	rm -f $(TOOLS_BIN_DIR)/get_helm.sh

.PHONY: $(ENVSUBST_BIN)
$(ENVSUBST_BIN): $(ENVSUBST)

.PHONY: $(KUBECTL_BIN)
$(KUBECTL_BIN): $(KUBECTL)

.PHONY: $(HELM_BIN)
$(HELM_BIN): $(HELM)

.PHONY: $(GO_APIDIFF_BIN)
$(GO_APIDIFF_BIN): $(GO_APIDIFF)

$(YQ): ## Build yq from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/mikefarah/yq/v4 $(YQ_BIN) $(YQ_VER)

.PHONY: $(YQ_BIN)
$(YQ_BIN): $(YQ) ## Building yq from the tools folder.
