package machinesetup

import (
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// MachineSetup holds an array of the Params type
type MachineSetup struct {
	Items []Params `json:"items"`
}

// Params holds all of the parameters to fully represent an Machine on Azure
type Params struct {
	Image         string        `json:"image"`
	Metadata      Metadata      `json:"metadata"`
	MachineParams MachineParams `json:"machineParams"`
}

// MachineParams inherits the MachineVersionInfo type.
type MachineParams struct {
	OS       string                       `json:"os"`
	Versions clusterv1.MachineVersionInfo `json:"versions"`
}

// Metadata only has the startup script right now.
type Metadata struct {
	StartupScript string `json:"startupScript"`
}
