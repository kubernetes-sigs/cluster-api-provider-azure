REPO=github.com/platform9/azure-provider/cloud/azure/actuators/machine
GOFLAGS += -ldflags '-extldflags "-static"'
TESTFLAGS=-test.timeout 0 -v
GOTEST=$(GOCMD) test $(REPO) $(TESTFLAGS)

all: generate build images

.PHONY: vendor

vendor:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure
vendor-update:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -update

.PHONY: generate

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
	CGO_ENABLED=0 go install $(GOFLAGS) $(GOREBUILD) github.com/platform9/azure-provider/cmd/cluster-controller

machine-controller: vendor
	CGO_ENABLED=0 go install $(GOFLAGS) $(GOREBUILD) github.com/platform9/azure-provider/cmd/machine-controller

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

unit_test:
	$(GOTEST) -run "^TestParseProviderConfig|TestBase64Encoding|TestGetStartupScript|Test(\w)*Unit|TestNewMachineActuator"

integration_test:
	$(GOTEST) -run "^TestCreate|TestUpdate|TestDelete|TestExists|TestCreateOrUpdateDeployment|TestCreateOrUpdateDeploymentWExisting|TestVMIfExists|TestDeleteSingleVM|TestCreateGroup|TestGetIP"

clean:
	go clean
