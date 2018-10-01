package machinesetup

import (
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// MachineSetup holds all of the params
type MachineSetup struct {
	Items []Params `json:"items"`
}

type Params struct {
	Image         string        `json:"image"`
	Metadata      Metadata      `json:"metadata"`
	MachineParams MachineParams `json:"machineParams"`
}

type MachineParams struct {
	OS       string                       `json:"os"`
	Versions clusterv1.MachineVersionInfo `json:"versions"`
}

// Metadata only has the startup script right now.
type Metadata struct {
	StartupScript string `json:"startupScript"`
}
