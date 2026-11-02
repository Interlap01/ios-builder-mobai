package signing

import (
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/MobAI-App/ios-builder/internal/signing/signingtest"
)

func TestAutoTurnsOnCapabilities(t *testing.T) {
	p := signingtest.New(t)
	p.PlistProfiles = true
	dir := t.TempDir()

	// A first run without entitlements leaves the App ID as it is.
	first := run(t, p, devOpts(dir))
	if p.Count("POST /v1/bundleIdCapabilities") != 0 || len(first.Missing) != 0 {
		t.Fatalf("no entitlements: calls %v, missing %v", p.Calls(), first.Missing)
	}

	keyPEM, err := os.ReadFile(first.Files.Key)
	if err != nil {
		t.Fatal(err)
	}
	opts := devOpts(dir)
	opts.KeyPEM = keyPEM
	opts.Entitlements = map[string]map[string]any{"com.example.app": {
		"aps-environment":                        "development",
		"com.apple.developer.associated-domains": []any{"applinks:example.com"},
		"com.apple.security.application-groups":  []any{"group.com.example"},
		"keychain-access-groups":                 []any{"$(AppIdentifierPrefix)com.example.app"},
	}}
	res := run(t, p, opts)
	if got := p.Capabilities["com.example.app"]; !slices.Equal(got, []string{"PUSH_NOTIFICATIONS", "ASSOCIATED_DOMAINS", "APP_GROUPS"}) {
		t.Errorf("capabilities = %v", got)
	}
	// The profile made before the capabilities lacks their entitlements.
	if !res.Profile.Created || res.Profile.Reason != "capabilities changed" {
		t.Errorf("profile = %+v", res.Profile)
	}
	// The App Group itself is the one thing the API cannot add.
	want := []MissingEntitlement{{BundleID: "com.example.app", Key: "com.apple.security.application-groups", Capability: "App Groups", Values: []string{"group.com.example"}}}
	if !reflect.DeepEqual(res.Missing, want) {
		t.Errorf("missing = %+v", res.Missing)
	}

	// Nothing to turn on the next time, and the profile stays.
	again := run(t, p, opts)
	if p.Count("POST /v1/bundleIdCapabilities") != 0 || again.Profile.Created || !reflect.DeepEqual(again.Missing, want) {
		t.Errorf("second run: calls %v, profile %+v, missing %+v", p.Calls(), again.Profile, again.Missing)
	}
}

func TestMissingEntitlements(t *testing.T) {
	profile := []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>TeamIdentifier</key><array><string>TEAM1</string></array><key>Entitlements</key><dict>` +
		`<key>keychain-access-groups</key><array><string>TEAM1.*</string></array>` +
		`<key>com.apple.developer.associated-domains</key><string>*</string>` +
		`<key>com.apple.security.application-groups</key><array><string>group.one</string></array>` +
		`<key>aps-environment</key><string>development</string>` +
		`</dict></dict></plist>`)
	wanted := ResolveEntitlements(map[string]any{
		"keychain-access-groups":                 []any{"$(AppIdentifierPrefix)shared"},
		"com.apple.developer.associated-domains": []any{"applinks:example.com"},
		"com.apple.security.application-groups":  []any{"group.one", "group.two"},
		"aps-environment":                        "production",
		"com.apple.developer.healthkit":          true,
		"get-task-allow":                         true,
	}, "TEAM1", "com.example.app")
	got := MissingEntitlements("com.example.app", wanted, profile)
	want := []MissingEntitlement{
		{BundleID: "com.example.app", Key: "com.apple.developer.healthkit", Capability: "HealthKit"},
		{BundleID: "com.example.app", Key: "com.apple.security.application-groups", Capability: "App Groups", Values: []string{"group.two"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingEntitlements = %+v", got)
	}
	if got := MissingEntitlements("x", wanted, []byte("profile:1")); got != nil {
		t.Errorf("unreadable profile: %+v", got)
	}
}

func TestResolveEntitlements(t *testing.T) {
	got := ResolveEntitlements(map[string]any{
		"keychain-access-groups":                          []any{"$(AppIdentifierPrefix)$(PRODUCT_BUNDLE_IDENTIFIER)"},
		"com.apple.developer.ubiquity-kvstore-identifier": "$(TeamIdentifierPrefix)$(CFBundleIdentifier)",
		"aps-environment":                                 "development",
		"com.apple.developer.healthkit":                   true,
	}, "TEAM1", "com.example.app")
	want := map[string]any{
		"keychain-access-groups":                          []any{"TEAM1.com.example.app"},
		"com.apple.developer.ubiquity-kvstore-identifier": "TEAM1.com.example.app",
		"aps-environment":                                 "development",
		"com.apple.developer.healthkit":                   true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResolveEntitlements = %v", got)
	}
}
