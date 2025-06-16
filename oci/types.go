package oci

// This file contains type definitions for OCI API objects.

// Instance represents an OCI compute instance.
type Instance struct {
	ID                 string `json:"id"`
	AvailabilityDomain string `json:"availabilityDomain"`
	CompartmentID      string `json:"compartmentId"`
	DisplayName        string `json:"displayName"`
	Shape              string `json:"shape"`
	LifecycleState     string `json:"lifecycleState"`
}

// AvailabilityDomain represents an OCI availability domain.
type AvailabilityDomain struct {
	Name          string `json:"name"`
	ID            string `json:"id"`
	CompartmentID string `json:"compartmentId"`
}

// CreateInstanceDetails is the request body for launching an instance.
type CreateInstanceDetails struct {
	AvailabilityDomain string                 `json:"availabilityDomain"`
	CompartmentID      string                 `json:"compartmentId"`
	Shape              string                 `json:"shape"`
	DisplayName        string                 `json:"displayName"`
	Metadata           map[string]string      `json:"metadata"`
	SourceDetails      map[string]interface{} `json:"sourceDetails"`
	CreateVnicDetails  *VnicDetails           `json:"createVnicDetails,omitempty"`
	ShapeConfig        *ShapeConfig           `json:"shapeConfig,omitempty"`
}

// VnicDetails for instance network interface.
type VnicDetails struct {
	SubnetID               string `json:"subnetId"`
	AssignPublicIP         bool   `json:"assignPublicIp"`
	AssignPrivateDNSRecord bool   `json:"assignPrivateDnsRecord"`
}

// ShapeConfig for flexible instance shapes.
type ShapeConfig struct {
	Ocpus       int `json:"ocpus"`
	MemoryInGBs int `json:"memoryInGBs"`
}
