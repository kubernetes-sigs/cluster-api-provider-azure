package components

import (
	"slices"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// IsValidDisableComponent validates if the provided value is a valid disable component by checking if the value exists
// in the infrav1.ValidDisableableComponents map.
func IsValidDisableComponent(value string) bool {
	_, ok := infrav1.ValidDisableableComponents[infrav1.DisableComponent(value)]
	return ok
}

// IsComponentDisabled checks if the provided component is in the list of disabled components.
func IsComponentDisabled(disabledComponents []string, component infrav1.DisableComponent) bool {
	return slices.Contains(disabledComponents, string(component))
}
