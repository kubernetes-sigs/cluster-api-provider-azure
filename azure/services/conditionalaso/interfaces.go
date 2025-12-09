package conditionalaso

import (
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso"
)

// Scope represents the common functionality related to all scopes needed for ASO services.
type Scope interface {
	aso.Scope
	CreateIfNotExists() bool
}
