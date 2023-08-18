module sigs.k8s.io/cluster-api-provider-azure/hack/tools

go 1.20

replace sigs.k8s.io/cluster-api-provider-azure => ../../

require (
	github.com/microsoft/azure-devops-go-api/azuredevops/v7 v7.1.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
)

require github.com/google/uuid v1.3.0 // indirect
