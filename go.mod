module sigs.k8s.io/cluster-api-provider-azure

go 1.12

require (
	github.com/Azure/azure-sdk-for-go v34.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.2
	github.com/Azure/go-autorest/autorest/adal v0.8.0 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.0
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.3.1
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/pelletier/go-toml v1.6.0
	github.com/pkg/errors v0.8.1
	golang.org/x/crypto v0.0.0-20190923035154-9ee001bba392
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	gopkg.in/yaml.v2 v2.2.7 // indirect
	k8s.io/api v0.17.1
	k8s.io/apiextensions-apiserver v0.17.1 // indirect
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v0.17.1
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	sigs.k8s.io/cluster-api v0.2.6-0.20200106222425-660e6b945a27
	sigs.k8s.io/controller-runtime v0.4.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.2.0+incompatible
