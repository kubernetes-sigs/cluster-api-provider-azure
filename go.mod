module sigs.k8s.io/cluster-api-provider-azure

go 1.13

require (
	github.com/Azure/azure-sdk-for-go v46.4.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.7
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.2
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.2
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.1.2
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/net v0.0.0-20201002202402-0a1ea396d57c // indirect
	k8s.io/api v0.17.12
	k8s.io/apimachinery v0.17.12
	k8s.io/client-go v0.17.12
	k8s.io/component-base v0.17.12
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.17.12
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	sigs.k8s.io/cluster-api v0.3.10
	sigs.k8s.io/controller-runtime v0.5.11
	sigs.k8s.io/kind v0.7.1-0.20200303021537-981bd80d3802
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible
