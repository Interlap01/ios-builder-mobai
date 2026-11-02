package xcodeproj

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// entitlementsFixture is an app whose bundle ID is built from a setting, a
// widget, a share extension whose ID is built from a setting, and a
// framework, each app and extension with an .entitlements file.
const entitlementsFixture = `// !$*UTF8*$!
{
	objects = {
		A1 = { isa = PBXNativeTarget; buildConfigurationList = LA; name = App; productType = "com.apple.product-type.application"; };
		W1 = { isa = PBXNativeTarget; buildConfigurationList = LW; name = Widget; productType = "com.apple.product-type.app-extension"; };
		S1 = { isa = PBXNativeTarget; buildConfigurationList = LS; name = Share; productType = "com.apple.product-type.app-extension"; };
		K1 = { isa = PBXNativeTarget; buildConfigurationList = LK; name = Kit; productType = "com.apple.product-type.framework"; };
		AD = { isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = "org.example.$(PRODUCT_NAME:rfc1034identifier)"; CODE_SIGN_ENTITLEMENTS = "$(TARGET_NAME)/App.entitlements"; }; name = Debug; };
		AR = { isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = "org.example.$(PRODUCT_NAME:rfc1034identifier)"; CODE_SIGN_ENTITLEMENTS = App/AppRelease.entitlements; }; name = Release; };
		WD = { isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.app.widget; CODE_SIGN_ENTITLEMENTS = Widget/Widget.entitlements; }; name = Debug; };
		WR = { isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.app.widget; }; name = Release; };
		SD = { isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = "$(APP_BUNDLE_ID).share"; CODE_SIGN_ENTITLEMENTS = Share/Share.entitlements; }; name = Debug; };
		KD = { isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.Kit; CODE_SIGN_ENTITLEMENTS = Kit/Kit.entitlements; }; name = Debug; };
		LA = { isa = XCConfigurationList; buildConfigurations = ( AD, AR, ); };
		LW = { isa = XCConfigurationList; buildConfigurations = ( WD, WR, ); };
		LS = { isa = XCConfigurationList; buildConfigurations = ( SD, ); };
		LK = { isa = XCConfigurationList; buildConfigurations = ( KD, ); };
	};
}
`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func plistDict(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>` + body + `</dict></plist>`
}

func TestEntitlements(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "App.xcodeproj", "project.pbxproj"), entitlementsFixture)
	writeFile(t, filepath.Join(dir, "App", "App.entitlements"), plistDict(`<key>aps-environment</key><string>development</string>`))
	writeFile(t, filepath.Join(dir, "App", "AppRelease.entitlements"), plistDict(`<key>aps-environment</key><string>production</string>`))
	writeFile(t, filepath.Join(dir, "Widget", "Widget.entitlements"), plistDict(`<key>com.apple.security.application-groups</key><array><string>group.example</string></array>`))
	writeFile(t, filepath.Join(dir, "Share", "Share.entitlements"), plistDict(``))

	got, err := Entitlements(dir, "org.example.App", "Debug")
	if err != nil {
		t.Fatal(err)
	}
	// The only app target is the app, whatever its ID setting; the share
	// extension's ID cannot be read, the framework is not a target that
	// signs with a profile of its own.
	want := map[string]map[string]any{
		"org.example.App":        {"aps-environment": "development"},
		"com.example.app.widget": {"com.apple.security.application-groups": []any{"group.example"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Debug = %v", got)
	}
	// Release has its own file for the app; the widget sets its file only
	// in Debug, which fills the gap.
	got, err = Entitlements(dir, "org.example.App", "Release")
	if err != nil {
		t.Fatal(err)
	}
	if got["org.example.App"]["aps-environment"] != "production" || got["com.example.app.widget"] == nil {
		t.Errorf("Release = %v", got)
	}

	if got, err := Entitlements(filepath.Join(dir, "missing"), "x", "Debug"); err != nil || got != nil {
		t.Errorf("no project: %v, %v", got, err)
	}
	// A file the project names but the folder lacks is skipped.
	if err := os.Remove(filepath.Join(dir, "Widget", "Widget.entitlements")); err != nil {
		t.Fatal(err)
	}
	if got, err := Entitlements(dir, "org.example.App", "Debug"); err != nil || got["com.example.app.widget"] != nil {
		t.Errorf("missing file: %v, %v", got, err)
	}
}
