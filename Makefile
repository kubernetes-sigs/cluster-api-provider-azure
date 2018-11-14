GOFLAGS += -ldflags '-extldflags "-static"'
IMAGE_TAG ?= latest

all: generate build images

.PHONY: vendor generate unit-tests machine-unit-tests cluster-unit-tests

vendor:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v
vendor-update:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v -update


generate: vendor
	go build -o $$GOPATH/bin/deepcopy-gen github.com/platform9/azure-provider/vendor/k8s.io/code-generator/cmd/deepcopy-gen
	$$GOPATH/bin/deepcopy-gen \
	  -i ./cloud/azure/providerconfig,./cloud/azure/providerconfig/v1alpha1 \
	  -O zz_generated.deepcopy \
	  -h boilerplate.go.txt

build: clusterctl cluster-controller machine-controller

clusterctl: vendor
	CGO_ENABLED=0 go install $(GOFLAGS) github.com/platform9/azure-provider/clusterctl

cluster-controller: vendor
	CGO_ENABLED=0 go install $(GOFLAGS) github.com/platform9/azure-provider/cmd/cluster-controller

machine-controller: vendor
	CGO_ENABLED=0 go install $(GOFLAGS) github.com/platform9/azure-provider/cmd/machine-controller

images: vendor
	$(MAKE) -C cmd/cluster-controller image TAG=$(IMAGE_TAG)
	$(MAKE) -C cmd/machine-controller image TAG=$(IMAGE_TAG)

push: vendor
	$(MAKE) -C cmd/cluster-controller push TAG=$(IMAGE_TAG)
	$(MAKE) -C cmd/machine-controller push TAG=$(IMAGE_TAG)

images_dev:
	$(MAKE) -C cmd/cluster-controller dev_image TAG=$(IMAGE_TAG)
	$(MAKE) -C cmd/machine-controller dev_image TAG=$(IMAGE_TAG)

push_dev:
	$(MAKE) -C cmd/cluster-controller dev_push TAG=$(IMAGE_TAG)
	$(MAKE) -C cmd/machine-controller dev_push TAG=$(IMAGE_TAG)

machine-unit-tests:
	go test -v github.com/platform9/azure-provider/cloud/azure/actuators/machine

cluster-unit-tests:
	go test -v github.com/platform9/azure-provider/cloud/azure/actuators/cluster

unit-tests: machine-unit-tests cluster-unit-tests

clean:
	go clean
