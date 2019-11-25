REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
IMAGE_NAME ?= cluster-api-azure-controller
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= dev
ARCH ?= amd64

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	AZURE_CLIENT_ID=
	AZURE_CLIENT_SECRET=
	AZURE_SUBSCRIPTION_ID=
	AZURE_TENANT_ID=
	MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) go test -v -tags=e2e -ginkgo.v -ginkgo.trace -count=1 -timeout=1h ./...
