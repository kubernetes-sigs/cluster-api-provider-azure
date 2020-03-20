module sigs.k8s.io/cluster-api-provider-azure/hack/tools

go 1.13

require (
	github.com/Azure/go-autorest/autorest v0.9.2 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.0 // indirect
	github.com/a8m/envsubst v1.1.0
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20191027212112-611e8accdfc9 // indirect
	github.com/golang/mock v1.3.1
	github.com/golangci/golangci-lint v1.23.8
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gophercloud/gophercloud v0.6.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	k8s.io/code-generator v0.18.0-alpha.2.0.20200130061103-7dfd5e9157ef
	sigs.k8s.io/cluster-api/hack/tools v0.0.0-20200319204836-a97903fa1e7e
	sigs.k8s.io/controller-tools v0.2.6
	sigs.k8s.io/kustomize/kustomize/v3 v3.5.4
	sigs.k8s.io/testing_frameworks v0.1.2
)

replace (
	// TODO(vincepri) Remove the following replace directives, needed for golangci-lint until
	// https://github.com/golangci/golangci-lint/issues/595 is fixed.
	github.com/go-critic/go-critic v0.0.0-20181204210945-1df300866540 => github.com/go-critic/go-critic v0.0.0-20190526074819-1df300866540
	github.com/golangci/errcheck v0.0.0-20181003203344-ef45e06d44b6 => github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6
	github.com/golangci/go-tools v0.0.0-20180109140146-af6baa5dc196 => github.com/golangci/go-tools v0.0.0-20190318060251-af6baa5dc196
	github.com/golangci/gofmt v0.0.0-20181105071733-0b8337e80d98 => github.com/golangci/gofmt v0.0.0-20181222123516-0b8337e80d98
	github.com/golangci/gosec v0.0.0-20180901114220-66fb7fc33547 => github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547
	github.com/golangci/ineffassign v0.0.0-20180808204949-42439a7714cc => github.com/golangci/ineffassign v0.0.0-20190609212857-42439a7714cc
	github.com/golangci/lint-1 v0.0.0-20180610141402-ee948d087217 => github.com/golangci/lint-1 v0.0.0-20190420132249-ee948d087217
	github.com/timakin/bodyclose => github.com/golangci/bodyclose v0.0.0-20190714144026-65da19158fa2
	mvdan.cc/unparam v0.0.0-20190124213536-fbb59629db34 => mvdan.cc/unparam v0.0.0-20190209190245-fbb59629db34
)
