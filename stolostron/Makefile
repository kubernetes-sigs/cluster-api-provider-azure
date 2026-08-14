IMG_REG ?= registry.redhat.io
IMG_REPO ?= multicluster
IMG_NAME ?= cluster-api-provider-azure-rhel9
IMG_TAG ?= latest
IMG ?= ${IMG_REG}/${IMG_REPO}/${IMG_NAME}:${IMG_TAG}

docker-build:
	docker build -f Dockerfile.stolostron --build-arg GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown) ../ -t ${IMG}
