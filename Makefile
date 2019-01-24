# Image URL to use all building/pushing image targets
PREFIX ?= platform9
NAME = cluster-api-azure-provider-controller
TAG ?= latest
IMG=${PREFIX}/${NAME}:${TAG}

# Keep an existing GOPATH/GOBIN, make a private one if it is not defined
PWD := $(shell pwd)
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
GOBIN_DEFAULT := $(GOPATH)/bin
export GOBIN ?= $(GOBIN_DEFAULT)
PATH := $(GOBIN):$(PATH)

HAS_DEP          := $(shell which dep)
HAS_GOLANGCI     := $(shell which golangci-lint)

all: bootstrap test manager clusterctl

# Creates GOBIN path if not present.
$(GOBIN):
	echo "Creating GOBIN"
	mkdir -p $(GOBIN)

createpath: $(GOBIN)

bootstrap: createpath
ifndef HAS_DEP
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOBIN)
endif

vendor:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v
vendor-update:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v -update
vendor-validate:
	dep check

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

machine-unit-tests:
	go test ./pkg/cloud/azure/actuators/machine -coverprofile machine-actuator-cover.out

cluster-unit-tests:
	go test ./pkg/cloud/azure/actuators/cluster -coverprofile cluster-actuator-cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager sigs.k8s.io/cluster-api-provider-azure/cmd/manager

# Build clusterctl
clusterctl: generate fmt vet
	go build -o bin/clusterctl sigs.k8s.io/cluster-api-provider-azure/cmd/clusterctl

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

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet -composites=false ./pkg/... ./cmd/... 

# Run golang-ci linter against the code
lint:
	golangci-lint run

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

check: bootstrap vendor-validate lint

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# Build the docker dev image
docker-build-dev: test
	docker build . -t "${IMG}-dev"
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker dev image
docker-push-dev:
	docker push "${IMG}-dev"
