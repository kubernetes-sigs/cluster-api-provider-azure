module sigs.k8s.io/cluster-api-provider-azure

go 1.13

require (
	github.com/Azure/azure-sdk-for-go v45.0.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.2
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.0
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.1
	github.com/google/gofuzz v1.1.0
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	k8s.io/api v0.17.8
	k8s.io/apimachinery v0.17.8
	k8s.io/client-go v0.17.8
	k8s.io/component-base v0.17.8
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.17.8
	k8s.io/utils v0.0.0-20200731180307-f00132d28269
	sigs.k8s.io/cluster-api v0.3.8
	sigs.k8s.io/controller-runtime v0.5.9
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible
