package signing

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/MobAI-App/ios-builder/internal/asc"
)

// An App ID's capabilities decide which entitlements its profiles carry, and
// a new App ID has none an app asks for. Auto turns on the capability behind
// each entitlement the app's targets ask for, and then reports what the
// profiles still lack: the parts the API cannot set (an App Group, an iCloud
// container, a merchant ID) and capabilities it does not know.

// IdentifiersURL is the portal page where App IDs' capabilities, App Groups
// and iCloud containers are edited by hand.
const IdentifiersURL = "https://developer.apple.com/account/resources/identifiers/list"

// capability is the App Store Connect capability behind an entitlement.
type capability struct {
	Type     string
	Name     string
	Settings []asc.CapabilitySetting
}

var iCloud = capability{Type: "ICLOUD", Name: "iCloud", Settings: []asc.CapabilitySetting{{Key: "ICLOUD_VERSION", Options: []asc.CapabilityOption{{Key: "XCODE_6", Enabled: true}}}}}

// capabilities maps entitlement keys to the capability that grants them.
var capabilities = map[string]capability{
	"aps-environment":                                                          {Type: "PUSH_NOTIFICATIONS", Name: "Push Notifications"},
	"com.apple.developer.associated-domains":                                   {Type: "ASSOCIATED_DOMAINS", Name: "Associated Domains"},
	"com.apple.developer.applesignin":                                          {Type: "APPLE_ID_AUTH", Name: "Sign in with Apple", Settings: []asc.CapabilitySetting{{Key: "APPLE_ID_AUTH_APP_CONSENT", Options: []asc.CapabilityOption{{Key: "PRIMARY_APP_CONSENT", Enabled: true}}}}},
	"com.apple.security.application-groups":                                    {Type: "APP_GROUPS", Name: "App Groups"},
	"com.apple.developer.icloud-container-identifiers":                         iCloud,
	"com.apple.developer.icloud-services":                                      iCloud,
	"com.apple.developer.ubiquity-kvstore-identifier":                          iCloud,
	"com.apple.developer.ubiquity-container-identifiers":                       iCloud,
	"com.apple.developer.healthkit":                                            {Type: "HEALTHKIT", Name: "HealthKit"},
	"com.apple.developer.homekit":                                              {Type: "HOMEKIT", Name: "HomeKit"},
	"com.apple.developer.in-app-payments":                                      {Type: "APPLE_PAY", Name: "Apple Pay"},
	"com.apple.developer.networking.wifi-info":                                 {Type: "ACCESS_WIFI_INFORMATION", Name: "Access Wi-Fi Information"},
	"com.apple.developer.networking.networkextension":                          {Type: "NETWORK_EXTENSIONS", Name: "Network Extensions"},
	"com.apple.developer.networking.vpn.api":                                   {Type: "PERSONAL_VPN", Name: "Personal VPN"},
	"com.apple.developer.networking.HotspotConfiguration":                      {Type: "HOT_SPOT", Name: "Hotspot"},
	"com.apple.developer.networking.multipath":                                 {Type: "MULTIPATH", Name: "Multipath"},
	"com.apple.developer.nfc.readersession.formats":                            {Type: "NFC_TAG_READING", Name: "NFC Tag Reading"},
	"com.apple.developer.siri":                                                 {Type: "SIRIKIT", Name: "SiriKit"},
	"com.apple.developer.ClassKit-environment":                                 {Type: "CLASSKIT", Name: "ClassKit"},
	"com.apple.developer.game-center":                                          {Type: "GAME_CENTER", Name: "Game Center"},
	"inter-app-audio":                                                          {Type: "INTER_APP_AUDIO", Name: "Inter-App Audio"},
	"com.apple.developer.pass-type-identifiers":                                {Type: "WALLET", Name: "Wallet"},
	"com.apple.external-accessory.wireless-configuration":                      {Type: "WIRELESS_ACCESSORY_CONFIGURATION", Name: "Wireless Accessory Configuration"},
	"com.apple.developer.authentication-services.autofill-credential-provider": {Type: "AUTOFILL_CREDENTIAL_PROVIDER", Name: "AutoFill Credential Provider"},
	"com.apple.developer.coremedia.hls.low-latency":                            {Type: "COREMEDIA_HLS_LOW_LATENCY", Name: "Low Latency HLS"},
	"com.apple.developer.default-data-protection":                              {Type: "DATA_PROTECTION", Name: "Data Protection"},
}

// dataProtectionLevels maps the entitlement's values to the setting's options.
var dataProtectionLevels = map[string]string{
	"NSFileProtectionComplete":                             "COMPLETE_PROTECTION",
	"NSFileProtectionCompleteUnlessOpen":                   "PROTECTED_UNLESS_OPEN",
	"NSFileProtectionCompleteUntilFirstUserAuthentication": "PROTECTED_UNTIL_FIRST_USER_AUTH",
}

// unchecked are entitlements every profile grants on its own or that signing
// fills in, never a capability.
var unchecked = []string{
	"application-identifier", "com.apple.developer.team-identifier", "get-task-allow",
	"keychain-access-groups", "beta-reports-active",
	// Xcode sets the environment from the distribution when it signs.
	"com.apple.developer.icloud-container-environment",
}

// CapabilityName is how Apple's portal names the capability behind an
// entitlement, the key itself when it is not one MobAI knows.
func CapabilityName(key string) string {
	if c, ok := capabilities[key]; ok {
		return c.Name
	}
	return key
}

// ensureCapabilities turns on, for one App ID, the capability behind each
// entitlement the target asks for, and reports whether it turned any on. A
// capability Apple refuses is only logged: the profile then lacks its
// entitlement, which MissingEntitlements reports.
func ensureCapabilities(ctx context.Context, client *asc.Client, bundle *asc.BundleID, entitlements map[string]any, log io.Writer) (bool, error) {
	if len(entitlements) == 0 {
		return false, nil
	}
	have, err := client.BundleIDCapabilities(ctx, bundle.ID)
	if err != nil {
		return false, err
	}
	keys := make([]string, 0, len(entitlements))
	for k := range entitlements {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	changed := false
	for _, key := range keys {
		c, ok := capabilities[key]
		if !ok || slices.Contains(have, c.Type) {
			continue
		}
		settings := c.Settings
		if c.Type == "DATA_PROTECTION" {
			level, _ := entitlements[key].(string)
			option := dataProtectionLevels[level]
			if option == "" {
				option = "COMPLETE_PROTECTION"
			}
			settings = []asc.CapabilitySetting{{Key: "DATA_PROTECTION_PERMISSION_LEVEL", Options: []asc.CapabilityOption{{Key: option, Enabled: true}}}}
		}
		logf(log, "Turning on %s for %s...", c.Name, bundle.Identifier)
		if err := client.EnableCapability(ctx, bundle.ID, c.Type, settings); err != nil {
			logf(log, "Warning: Apple did not turn on %s for %s: %v", c.Name, bundle.Identifier, err)
			continue
		}
		have = append(have, c.Type)
		changed = true
	}
	return changed, nil
}

// MissingEntitlement is an entitlement a target asks for that its profile
// does not grant, with the values it lacks (an App Group's id, say); no
// values means the whole entitlement.
type MissingEntitlement struct {
	BundleID   string   `json:"bundle_id"`
	Key        string   `json:"key"`
	Capability string   `json:"capability"`
	Values     []string `json:"values,omitempty"`
}

// missingFor is MissingEntitlements for one target of AutoOptions.Entitlements,
// its variables resolved with the profile's team.
func missingFor(bundleID string, entitlements map[string]map[string]any, profile []byte) []MissingEntitlement {
	wanted := entitlements[bundleID]
	if len(wanted) == 0 {
		return nil
	}
	team, err := ProfileTeam(profile)
	if err != nil {
		return nil
	}
	return MissingEntitlements(bundleID, ResolveEntitlements(wanted, team, bundleID), profile)
}

// MissingEntitlements compares what a target asks for (variables already
// resolved) with what its profile grants.
func MissingEntitlements(bundleID string, wanted map[string]any, profile []byte) []MissingEntitlement {
	dict, err := profilePlist(profile)
	if err != nil {
		return nil
	}
	granted, _ := dict["Entitlements"].(map[string]any)
	keys := make([]string, 0, len(wanted))
	for k := range wanted {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var missing []MissingEntitlement
	for _, key := range keys {
		if slices.Contains(unchecked, key) {
			continue
		}
		have, ok := granted[key]
		if !ok {
			missing = append(missing, MissingEntitlement{BundleID: bundleID, Key: key, Capability: CapabilityName(key)})
			continue
		}
		var lacking []string
		for _, v := range stringsOf(wanted[key]) {
			if !covered(v, have) {
				lacking = append(lacking, v)
			}
		}
		if len(lacking) > 0 {
			missing = append(missing, MissingEntitlement{BundleID: bundleID, Key: key, Capability: CapabilityName(key), Values: lacking})
		}
	}
	return missing
}

// stringsOf is an entitlement's values when it lists strings; a single
// value (a bool, the push environment) is checked by its key alone.
func stringsOf(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// covered reports whether a profile's value grants one value an app asks
// for: an equal value, "*", or a prefix ending in "*" (TEAM.*).
func covered(want string, have any) bool {
	values := stringsOf(have)
	if s, ok := have.(string); ok {
		values = []string{s}
	}
	for _, h := range values {
		if h == want || h == "*" || (strings.HasSuffix(h, "*") && strings.HasPrefix(want, strings.TrimSuffix(h, "*"))) {
			return true
		}
	}
	return false
}

// ResolveEntitlements fills in the build settings an .entitlements file
// names, as Xcode does when it signs: the team prefix and the bundle ID.
// Values naming other settings are left as they are.
func ResolveEntitlements(entitlements map[string]any, team, bundleID string) map[string]any {
	r := strings.NewReplacer(
		"$(AppIdentifierPrefix)", team+".",
		"$(TeamIdentifierPrefix)", team+".",
		"$(DEVELOPMENT_TEAM)", team,
		"$(PRODUCT_BUNDLE_IDENTIFIER)", bundleID,
		"$(CFBundleIdentifier)", bundleID,
		"${PRODUCT_BUNDLE_IDENTIFIER}", bundleID,
	)
	var resolve func(v any) any
	resolve = func(v any) any {
		switch t := v.(type) {
		case string:
			return r.Replace(t)
		case []any:
			out := make([]any, len(t))
			for i, item := range t {
				out[i] = resolve(item)
			}
			return out
		case map[string]any:
			out := make(map[string]any, len(t))
			for k, item := range t {
				out[k] = resolve(item)
			}
			return out
		}
		return v
	}
	out := make(map[string]any, len(entitlements))
	for k, v := range entitlements {
		out[k] = resolve(v)
	}
	return out
}

// ProfileTeam is the team a profile belongs to.
func ProfileTeam(profile []byte) (string, error) {
	dict, err := profilePlist(profile)
	if err != nil {
		return "", err
	}
	if teams, _ := dict["TeamIdentifier"].([]any); len(teams) > 0 {
		if team, _ := teams[0].(string); team != "" {
			return team, nil
		}
	}
	return "", fmt.Errorf("provisioning profile names no team")
}
