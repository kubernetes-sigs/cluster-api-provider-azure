module sigs.k8s.io/cluster-api-provider-azure/test/e2e

go 1.12

require (
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/pkg/errors v0.8.1
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/cluster-api v0.2.7
	sigs.k8s.io/cluster-api-bootstrap-provider-kubeadm v0.1.1
	sigs.k8s.io/cluster-api-provider-azure v0.0.0-00010101000000-000000000000
	sigs.k8s.io/controller-runtime v0.3.0
	sigs.k8s.io/kind v0.5.1
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.2.0+incompatible
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
	sigs.k8s.io/cluster-api-provider-azure => ../..
)
