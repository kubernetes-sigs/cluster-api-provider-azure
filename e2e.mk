# e2e.mk
# Make configuration that effects E2E behaviors should go in here,
# to allow us to maintain the core Makefile without having to execute
# long-running E2E jobs every time that file changes

##@ E2E Testing:

.PHONY: test-e2e-run
test-e2e-run: generate-e2e-templates install-tools create-bootstrap ## Run e2e tests.
	if [ "$(MGMT_CLUSTER_TYPE)" == "aks" ]; then \
		source ./scripts/peer-vnets.sh && source_tilt_settings tilt-settings.yaml; \
	fi; \
	$(ENVSUBST) < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST) && \
	if [ -z "${AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY}" ]; then \
		export AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY=$(shell cat $(AZURE_IDENTITY_ID_FILEPATH)); \
	fi; \
	$(GINKGO) -v --trace --timeout=4h --tags=e2e --focus="$(GINKGO_FOCUS)" --skip="$(GINKGO_SKIP)" --nodes=$(GINKGO_NODES) --no-color=$(GINKGO_NOCOLOR) --output-dir="$(ARTIFACTS)" --junit-report="junit.e2e_suite.1.xml" $(GINKGO_ARGS) ./test/e2e -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
		-e2e.skip-log-collection="$(SKIP_LOG_COLLECTION)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) -e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER) $(E2E_ARGS)

.PHONY: test-e2e-aro-hcp
test-e2e-aro-hcp: ## Run the opt-in ARO HCP lifecycle E2E test.
	@test -n "$${AZURE_SUBSCRIPTION_ID:-}" || { echo "AZURE_SUBSCRIPTION_ID must be set" >&2; exit 1; }
	@test -n "$${AZURE_TENANT_ID:-}" || { echo "AZURE_TENANT_ID must be set" >&2; exit 1; }
	@test -n "$${AZURE_LOCATION:-}" || { echo "AZURE_LOCATION must be set to an ARO HCP-supported region" >&2; exit 1; }
	@test -n "$${ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID:-}" || { echo "ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID must be set to a pre-registered Microsoft Entra application client ID" >&2; exit 1; }
	ASO_IMAGE="$${ASO_IMAGE:-localhost:5000/capz/azure-service-operator-rhel9:upstream}" EXP_ARO=true GINKGO_FOCUS="ARO HCP E2E" GINKGO_NODES=1 SKIP_CLEANUP="$${SKIP_CLEANUP:-false}" $(MAKE) test-e2e-skip-push

.PHONY: test-e2e-run-cleanup
test-e2e-run-cleanup: ## Run e2e cleanup tasks.
	$(MAKE) cleanup-workload-identity || true
	$(MAKE) clean-release-git || true
	if [ "$(MGMT_CLUSTER_TYPE)" == "aks" ] && [ "$(SKIP_CLEANUP)" != "true" ]; then \
		echo "Cleaning up AKS management cluster..."; \
		$(MAKE) aks-delete || true; \
	fi

.PHONY: test-e2e
test-e2e: ## Run "docker-build" and "docker-push" rules then run e2e tests.
	PULL_POLICY=IfNotPresent MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) docker-build docker-push \
	test-e2e-run;

.PHONY: test-e2e-skip-push
test-e2e-skip-push: ## Run "docker-build" rule then run e2e tests.
	PULL_POLICY=IfNotPresent MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) docker-build \
	test-e2e-run;

.PHONY: test-e2e-skip-build-and-push
test-e2e-skip-build-and-push:
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/capz/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/capz/manager_pull_policy.yaml" PULL_POLICY=IfNotPresent
	MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) test-e2e-run;

.PHONY: test-e2e-custom-image
test-e2e-custom-image: ## Run e2e tests with a custom image format (use MANAGER_IMAGE env var).
	@if [ -z "$(MANAGER_IMAGE)" ]; then \
		echo "MANAGER_IMAGE must be set"; \
		exit 1; \
	fi
	$(MAKE) set-manifest-image MANIFEST_IMG=$(shell echo $(MANAGER_IMAGE) | sed -E "s/^(.*):(.*)$$/\1/") MANIFEST_TAG=$(shell echo $(MANAGER_IMAGE) | sed -E "s/^(.*):(.*)$$/\2/") TARGET_RESOURCE="./config/capz/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/capz/manager_pull_policy.yaml" PULL_POLICY=IfNotPresent
	$(MAKE) test-e2e-run;
