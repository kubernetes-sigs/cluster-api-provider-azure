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

.DEFAULT_GOAL:=help

FASTBUILD ?= n ## Set FASTBUILD=y (case-sensitive) to skip some slow tasks

## Image URL to use all building/pushing image targets
STABLE_DOCKER_REPO ?= quay.io/k8s
MANAGER_IMAGE_NAME ?= cluster-api-azure-controller
MANAGER_IMAGE_TAG ?= 0.1.0-alpha.3
MANAGER_IMAGE ?= $(STABLE_DOCKER_REPO)/$(MANAGER_IMAGE_NAME):$(MANAGER_IMAGE_TAG)
DEV_DOCKER_REPO ?= quay.io/k8s
DEV_MANAGER_IMAGE ?= $(DEV_DOCKER_REPO)/$(MANAGER_IMAGE_NAME):$(MANAGER_IMAGE_TAG)-dev

DEPCACHEAGE ?= 24h # Enables caching for Dep
BAZEL_ARGS ?=

# Bazel variables
BAZEL_VERSION := $(shell command -v bazel 2> /dev/null)
DEP ?= bazel run dep

# determine the OS
HOSTOS := $(shell go env GOHOSTOS)
HOSTARCH := $(shell go env GOARCH)
BINARYPATHPATTERN :=${HOSTOS}_${HOSTARCH}_*

ifndef BAZEL_VERSION
    $(error "Bazel is not available. \
		Installation instructions can be found at \
		https://docs.bazel.build/versions/master/install.html")
endif

.PHONY: all
all: check-install test manager clusterctl #clusterazureadm

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: dep-ensure
dep-ensure: check-install ## Ensure dependencies are up to date
	@echo Checking status of dependencies
	@${DEP} status 2>&1 > /dev/null || make dep-install
	@echo Finished verifying dependencies

.PHONY: dep-install
dep-install: ## Force install go dependencies
	${DEP} ensure
	bazel run //:gazelle $(BAZEL_ARGS)

.PHONY: gazelle
gazelle: ## Run Bazel Gazelle
	bazel run //:gazelle $(BAZEL_ARGS)

.PHONY: check-install
check-install: ## Checks that you've installed this repository correctly
	@./scripts/check-install.sh

.PHONY: manager
manager: generate ## Build manager binary.
	bazel build //cmd/manager $(BAZEL_ARGS)
	install bazel-bin/cmd/manager/${BINARYPATHPATTERN}/manager $(shell go env GOPATH)/bin/azure-manager

.PHONY: clusterctl
clusterctl: generate ## Build clusterctl binary.
	bazel build --workspace_status_command=./hack/print-workspace-status.sh //cmd/clusterctl $(BAZEL_ARGS)
	install bazel-bin/cmd/clusterctl/${BINARYPATHPATTERN}/clusterctl $(shell go env GOPATH)/bin/clusterctl

# TODO: Uncomment once clusterazureadm exists
#.PHONY: clusterazureadm
#clusterazureadm: dep-ensure ## Build clusterazureadm binary.
#	bazel build --workspace_status_command=./hack/print-workspace-status.sh //cmd/clusterazureadm $(BAZEL_ARGS)
#	install bazel-bin/cmd/clusterazureadm/${BINARYPATHPATTERN}/clusterazureadm $(shell go env GOPATH)/bin/clusterazureadm

.PHONY: release-binaries
release-binaries: ## Build release binaries
	bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //cmd/clusterctl # TODO: Uncomment once clusterazureadm exists //cmd/clusterazureadm
	bazel build --platforms=@io_bazel_rules_go//go/toolchain:darwin_amd64 //cmd/clusterctl # TODO: Uncomment once clusterazureadm exists //cmd/clusterazureadm
	mkdir -p out
# TODO: Uncomment once clusterazureadm exists
#install bazel-bin/cmd/clusterazureadm/darwin_amd64_pure_stripped/clusterazureadm out/clusterazureadm-darwin-amd64
#install bazel-bin/cmd/clusterazureadm/linux_amd64_pure_stripped/clusterazureadm out/clusterazureadm-linux-amd64
	install bazel-bin/cmd/clusterctl/darwin_amd64_pure_stripped/clusterctl out/clusterctl-darwin-amd64
	install bazel-bin/cmd/clusterctl/linux_amd64_pure_stripped/clusterctl out/clusterctl-linux-amd64

.PHONY: test verify
test: generate verify ## Run tests
	bazel test --nosandbox_debug //pkg/... //cmd/... $(BAZEL_ARGS)

verify:
	./hack/verify_boilerplate.py

.PHONY: copy-genmocks
copy-genmocks: ## Copies generated mocks into the repository
	cp -Rf bazel-genfiles/pkg/* pkg/

BAZEL_DOCKER_ARGS_COMMON := --define=MANAGER_IMAGE_NAME=$(MANAGER_IMAGE_NAME) --define=MANAGER_IMAGE_TAG=$(MANAGER_IMAGE_TAG) $(BAZEL_ARGS)
BAZEL_DOCKER_ARGS := --define=DOCKER_REPO=$(STABLE_DOCKER_REPO) $(BAZEL_DOCKER_ARGS_COMMON)
BAZEL_DOCKER_ARGS_DEV := --define=DOCKER_REPO=$(DEV_DOCKER_REPO) $(BAZEL_DOCKER_ARGS_COMMON)

.PHONY: docker-build
docker-build: generate ## Build the production docker image
	docker build . -t $(MANAGER_IMAGE)

# TODO: Move this to docker-build target once we figure out multi-stage builds and using a thinner image
.PHONY: docker-build-new
docker-build-new: generate ## Build the production docker image
	bazel run //cmd/manager:manager-image $(BAZEL_DOCKER_ARGS)

.PHONY: docker-build-dev
docker-build-dev: generate ## Build the development docker image
	bazel run //cmd/manager:manager-image $(BAZEL_DOCKER_ARGS_DEV)

.PHONY: docker-push
docker-push: generate ## Push production docker image
	docker push $(MANAGER_IMAGE)

# TODO: Move this to docker-push target once we figure out multi-stage builds and using a thinner image
.PHONY: docker-push-new
docker-push-new: generate ## Push production docker image
	bazel run //cmd/manager:manager-push $(BAZEL_DOCKER_ARGS)

.PHONY: docker-push-dev
docker-push-dev: generate ## Push development image
	bazel run //cmd/manager:manager-push $(BAZEL_DOCKER_ARGS_DEV)

.PHONY: clean
clean: ## Remove all generated files
	rm -rf cmd/clusterctl/examples/azure/out/
	rm -f kubeconfig
	rm -f minikube.kubeconfig
	rm -f bazel-*
	rm -rf out/

.PHONY: reset-bazel
reset-bazel: ## Deep cleaning for bazel
	bazel clean --expunge

cmd/clusterctl/examples/azure/out:
	./cmd/clusterctl/examples/azure/generate-yaml.sh

# TODO: Uncomment once clusterazureadm exists
#cmd/clusterctl/examples/azure/out/credentials: cmd/clusterctl/examples/azure/out ## Generate k8s secret for Azure credentials
#	clusterazureadm alpha bootstrap generate-azure-default-profile > cmd/clusterctl/examples/azure/out/credentials

.PHONY: examples
examples: ## Generate example output
	$(MAKE) cmd/clusterctl/examples/azure/out MANAGER_IMAGE=${MANAGER_IMAGE}

.PHONY: examples-dev
examples-dev: ## Generate example output with developer image
	$(MAKE) cmd/clusterctl/examples/azure/out MANAGER_IMAGE=${DEV_MANAGER_IMAGE}

.PHONY: manifests
manifests: #cmd/clusterctl/examples/azure/out/credentials ## Generate manifests for clusterctl
	kustomize build config/default/ > cmd/clusterctl/examples/azure/out/provider-components.yaml
	echo "---" >> cmd/clusterctl/examples/azure/out/provider-components.yaml
	kustomize build vendor/sigs.k8s.io/cluster-api/config/default/ >> cmd/clusterctl/examples/azure/out/provider-components.yaml

.PHONY: manifests-dev
manifests-dev: dep-ensure dep-install binaries-dev crds ## Builds development manifests
	MANAGER_IMAGE=$(DEV_MANAGER_IMAGE) MANAGER_IMAGE_PULL_POLICY="Always" $(MAKE) manifests

.PHONY: crds
crds:
	bazel build //config
	cp -R bazel-genfiles/config/crds/* config/crds/
	cp -R bazel-genfiles/config/rbac/* config/rbac/

# TODO(vincepri): This should move to rebuild Bazel binaries once every
# make target uses Bazel bins to run operations.
.PHONY: binaries-dev
binaries-dev: ## Builds and installs the binaries on the local GOPATH
	go get -v ./...
	go install -v ./...

.PHONY: create-cluster
create-cluster: ## Create a Kubernetes cluster on Azure using examples
	clusterctl create cluster -v 3 \
	--provider azure \
	--bootstrap-type kind \
	-m ./cmd/clusterctl/examples/azure/out/machines.yaml \
	-c ./cmd/clusterctl/examples/azure/out/cluster.yaml \
	-p ./cmd/clusterctl/examples/azure/out/provider-components.yaml \
	-a ./cmd/clusterctl/examples/azure/out/addons.yaml

lint-full: dep-ensure ## Run slower linters to detect possible issues
	bazel run //:lint-full $(BAZEL_ARGS)

## Define kind dependencies here.

kind-reset: ## Destroys the "clusterapi" kind cluster.
	kind delete cluster --name=clusterapi || true

ifneq ($(FASTBUILD),y)

## Define slow dependency targets here

.PHONY: generate
generate: gazelle dep-ensure ## Run go generate
	GOPATH=$(shell go env GOPATH) bazel run //:generate $(BAZEL_ARGS)
	$(MAKE) dep-ensure
# TODO: Uncomment once we solve mocks
#bazel build $(BAZEL_ARGS) //pkg/cloud/azure/services/mocks:go_mock_interfaces \
		//pkg/cloud/azure/services/ec2/mock_ec2iface:go_default_library \
		//pkg/cloud/azure/services/elb/mock_elbiface:go_default_library
#cp -Rf bazel-genfiles/pkg/* pkg/

.PHONY: lint
lint: dep-ensure ## Lint codebase
	@echo If you have generated new mocks, run make copy-genmocks before linting
	bazel run //:lint $(BAZEL_ARGS)

else

## Add skips for slow depedency targets here

.PHONY: generate
generate:
	@echo FASTBUILD is set: Skipping generate

.PHONY: lint
lint:
	@echo FASTBUILD is set: Skipping lint

endif

## Old make targets
# TODO: Migrate old make targets

vendor:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v
vendor-update:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v -update
vendor-validate:
	dep check

machine-unit-tests:
	go test ./pkg/cloud/azure/actuators/machine -coverprofile machine-actuator-cover.out

cluster-unit-tests:
	go test ./pkg/cloud/azure/actuators/cluster -coverprofile cluster-actuator-cover.out

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet -composites=false ./pkg/... ./cmd/... 

verify-boilerplate:
	./hack/verify-boilerplate.sh

check: verify-boilerplate bootstrap vendor-validate lint
