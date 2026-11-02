package asc

import (
	"context"
)

// CapabilitySetting is one setting of a capability, such as the iCloud
// version or the data protection level, with the options picked for it.
type CapabilitySetting struct {
	Key     string             `json:"key"`
	Options []CapabilityOption `json:"options,omitempty"`
}

// CapabilityOption is one picked option of a setting.
type CapabilityOption struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled,omitempty"`
}

type capabilityAttributes struct {
	CapabilityType string              `json:"capabilityType,omitempty"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// BundleIDCapabilities lists the capability types turned on for an App ID
// (its resource id, not the identifier).
func (c *Client) BundleIDCapabilities(ctx context.Context, bundleIDResourceID string) ([]string, error) {
	// A related collection that takes no limit: Apple answers it in one page.
	rs, err := getPage[capabilityAttributes](ctx, c, "/v1/bundleIds/"+bundleIDResourceID+"/bundleIdCapabilities", nil)
	if err != nil {
		return nil, err
	}
	types := make([]string, 0, len(rs))
	for _, r := range rs {
		types = append(types, r.Attributes.CapabilityType)
	}
	return types, nil
}

// EnableCapability turns a capability on for an App ID. Profiles made before
// carry the old entitlements and must be made again.
func (c *Client) EnableCapability(ctx context.Context, bundleIDResourceID, capabilityType string, settings []CapabilitySetting) error {
	req := Resource[capabilityAttributes]{
		Type:          "bundleIdCapabilities",
		Attributes:    capabilityAttributes{CapabilityType: capabilityType, Settings: settings},
		Relationships: Relationships{"bundleId": ToOne("bundleIds", bundleIDResourceID)},
	}
	_, err := post[capabilityAttributes, capabilityAttributes](ctx, c, "/v1/bundleIdCapabilities", req)
	return err
}
