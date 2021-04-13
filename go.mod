module sigs.k8s.io/cluster-api-provider-azure

go 1.16

require (
	github.com/Azure/aad-pod-identity v1.7.1
	github.com/Azure/azure-sdk-for-go v48.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.12
	github.com/Azure/go-autorest/autorest/adal v0.9.5
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.3
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.5
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.1.2
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/hashicorp/golang-lru v0.5.4
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0
	github.com/spf13/pflag v1.0.5
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.13.0
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.13.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.13.0
	go.opentelemetry.io/otel/sdk v0.13.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	k8s.io/api v0.21.0-beta.1
	k8s.io/apimachinery v0.21.0-beta.1
	k8s.io/client-go v0.21.0-beta.1
	k8s.io/component-base v0.21.0-beta.1
	k8s.io/klog/v2 v2.8.0
	k8s.io/kubectl v0.21.0-beta.1
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/cluster-api v0.0.0-20210325161731-7124a659accd
	sigs.k8s.io/controller-runtime v0.9.0-alpha.1
	sigs.k8s.io/kind v0.9.0
)
