module sigs.k8s.io/cluster-api-provider-azure

go 1.12

require (
	github.com/Azure/azure-sdk-for-go v31.1.0+incompatible
	github.com/Azure/go-autorest/autorest v0.5.0
	github.com/Azure/go-autorest/autorest/azure/auth v0.1.0
	github.com/Azure/go-autorest/autorest/to v0.2.0
	github.com/Azure/go-autorest/autorest/validation v0.1.0 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/golang/mock v1.2.0
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/procfs v0.0.0-20190227231451-bbced9601137 // indirect
	github.com/renstrom/dedent v0.0.0-00010101000000-000000000000 // indirect
	github.com/spf13/cobra v0.0.3
	golang.org/x/crypto v0.0.0-20190418165655-df01cb2cc480
	k8s.io/api v0.0.0-20190711103429-37c3b8b1ca65
	k8s.io/apimachinery v0.0.0-20190711103026-7bf792636534
	k8s.io/apiserver v0.0.0-20190202011929-26bc712632e1 // indirect
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/cluster-bootstrap v0.0.0-20190223141759-fab9a0a63c55
	k8s.io/code-generator v0.0.0-20190711102700-42c1e9a4dc7a
	k8s.io/klog v0.3.2
	k8s.io/kube-proxy v0.0.0-20190711111910-7518d09d5a98 // indirect
	k8s.io/kubelet v0.0.0-20190711112132-7f0a9f55a012 // indirect
	k8s.io/kubernetes v1.13.3
	sigs.k8s.io/cluster-api v0.1.8
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools v0.1.11
	sigs.k8s.io/testing_frameworks v0.1.1
	sigs.k8s.io/yaml v1.1.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.3.0+incompatible
	github.com/renstrom/dedent => github.com/lithammer/dedent v1.1.0
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
)
