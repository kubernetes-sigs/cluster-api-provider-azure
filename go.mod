module sigs.k8s.io/cluster-api-provider-azure

go 1.12

require (
	github.com/Azure/azure-sdk-for-go v34.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.2
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.0
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.2.0
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.8.1
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7
	golang.org/x/net v0.0.0-20190909003024-a7b16738d86b
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a
	sigs.k8s.io/cluster-api v0.2.6-0.20191018152358-b01a13ef030f
	sigs.k8s.io/controller-runtime v0.3.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.2.0+incompatible
