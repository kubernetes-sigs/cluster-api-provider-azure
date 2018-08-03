GOCMD=go
REPO=github.com/platform9/azure-provider/cloud/azure/actuators/machine
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
TESTFLAGS=-test.timeout 0 -v
GOTEST=$(GOCMD) test $(REPO) $(TESTFLAGS)
GOGET=$(GOCMD) get


unit_test:
	$(GOTEST) -run "^TestParseProviderConfig|TestBase64Encoding|TestGetStartupScript|Test(\w)*Unit|TestNewMachineActuator"

integration_test:
	$(GOTEST) -run "^TestCreate|TestUpdate|TestDelete|TestExists|TestCreateOrUpdateDeployment|TestCreateOrUpdateDeploymentWExisting|TestVMIfExists|TestDeleteSingleVM|TestCreateGroup|TestGetIP"

clean:
	$(GOCLEAN)