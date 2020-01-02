module sigs.k8s.io/cluster-api-provider-azure

go 1.12

require (
	github.com/Azure/azure-sdk-for-go v34.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.2
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.0
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/blang/semver v3.5.0+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.3.1
	github.com/google/uuid v1.1.1 // indirect
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pelletier/go-toml v1.6.0
	github.com/pkg/errors v0.8.1
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7
	golang.org/x/net v0.0.0-20190909003024-a7b16738d86b
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a
	sigs.k8s.io/cluster-api v0.2.7
	sigs.k8s.io/cluster-api-bootstrap-provider-kubeadm v0.1.3
	sigs.k8s.io/controller-runtime v0.3.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.2.0+incompatible
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
)
