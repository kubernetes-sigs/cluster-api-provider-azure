module sigs.k8s.io/cluster-api-provider-azure

go 1.16

require (
	github.com/Azure/aad-pod-identity v1.8.0
	github.com/Azure/azure-sdk-for-go v55.8.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.18
	github.com/Azure/go-autorest/autorest/adal v0.9.13
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.3
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.5.0
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.2.0
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/hashicorp/golang-lru v0.5.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/pflag v1.0.5
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.23.0
	go.opentelemetry.io/otel v1.0.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.0.0
	go.opentelemetry.io/otel/exporters/prometheus v0.23.0
	go.opentelemetry.io/otel/metric v0.23.0
	go.opentelemetry.io/otel/sdk v1.0.0
	go.opentelemetry.io/otel/sdk/export/metric v0.23.0
	go.opentelemetry.io/otel/sdk/metric v0.23.0
	go.opentelemetry.io/otel/trace v1.0.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/mod v0.4.2
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/component-base v0.21.4
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubectl v0.21.4
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/cluster-api v0.4.3
	sigs.k8s.io/cluster-api/test v0.4.3
	sigs.k8s.io/controller-runtime v0.9.7
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/yaml v1.2.0
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.3
