GOFLAGS += -ldflags '-extldflags "-static"'

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
	$(MAKE) -C cmd/cluster-controller image
	$(MAKE) -C cmd/machine-controller image

push: vendor
	$(MAKE) -C cmd/cluster-controller push
	$(MAKE) -C cmd/machine-controller push

images_dev:
	$(MAKE) -C cmd/cluster-controller dev_image
	$(MAKE) -C cmd/machine-controller dev_image

push_dev:
	$(MAKE) -C cmd/cluster-controller dev_push
	$(MAKE) -C cmd/machine-controller dev_push

images_ci:
	$(MAKE) -C cmd/cluster-controller ci_image TAG=$(COMMIT_ID)
	$(MAKE) -C cmd/machine-controller ci_image TAG=$(COMMIT_ID)

push_ci:
	$(MAKE) -C cmd/cluster-controller ci_push TAG=$(COMMIT_ID)
	$(MAKE) -C cmd/machine-controller ci_push TAG=$(COMMIT_ID)

machine-unit-tests:
	go test -v github.com/platform9/azure-provider/cloud/azure/actuators/machine

cluster-unit-tests:
	go test -v github.com/platform9/azure-provider/cloud/azure/actuators/cluster

unit-tests: machine-unit-tests cluster-unit-tests

clean:
	go clean
