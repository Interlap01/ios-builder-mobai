package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shelfProject is a project.pbxproj with one application target, enough
// for the signing step to find and sign it.
const shelfProject = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>objects</key><dict>
<key>T1</key><dict><key>isa</key><string>PBXNativeTarget</string><key>name</key><string>Shelf</string>
<key>productType</key><string>com.apple.product-type.application</string><key>buildConfigurationList</key><string>L1</string></dict>
<key>L1</key><dict><key>isa</key><string>XCConfigurationList</string><key>buildConfigurations</key><array><string>C1</string></array></dict>
<key>C1</key><dict><key>isa</key><string>XCBuildConfiguration</string><key>name</key><string>Release</string>
<key>buildSettings</key><dict><key>PRODUCT_BUNDLE_IDENTIFIER</key><string>run.mobai.shelf</string></dict></dict>
</dict></dict></plist>
`

// TestBuildIPAUsesMobaiJSONRelease runs the build workflow's IPA step: a
// signed build runs the iOS release command in the nearest mobai.json with
// the signing in its environment and ships the IPA it names; an unsigned
// build, or one with a wildcard profile, never runs it.
func TestBuildIPAUsesMobaiJSONRelease(t *testing.T) {
	for _, tool := range []string{"bash", "jq", "plutil", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable", tool)
		}
	}
	all := steps(t, "ios-build.yml")
	script := all[indexOfStep(t, all, "Build IPA")].Run
	// The command checks what it was handed, then makes the IPA.
	command := `grep -q 'CODE_SIGN_STYLE = Manual' "$XCODE_XCCONFIG_FILE" && grep -q 'CURRENT_PROJECT_VERSION = 7' "$XCODE_XCCONFIG_FILE" && ` +
		`grep -q 'MARKETING_VERSION = 1.2.0' "$XCODE_XCCONFIG_FILE" && [ "$(plutil -extract method raw -o - "$MOBAI_EXPORT_OPTIONS")" = app-store ] && ` +
		`touch ran && mkdir -p out && echo ipa > out/Shelf.ipa`

	run := func(t *testing.T, signing, appID string) (string, error) {
		t.Helper()
		ws, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		tmp, bin := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, "ios", "Shelf.xcodeproj"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "ios", "Shelf.xcodeproj", "project.pbxproj"), []byte(shelfProject), 0o644); err != nil {
			t.Fatal(err)
		}
		mobai := `{"build": {"ios": {"debug": {"command": "exit 7", "app": "x.app"}, "release": {"command": ` + jsonString(command) + `, "app": "out/*.ipa"}}}}`
		if err := os.WriteFile(filepath.Join(ws, "mobai.json"), []byte(mobai), 0o644); err != nil {
			t.Fatal(err)
		}
		// The workflow's own path ends in xcodebuild, which must not run here.
		if err := os.WriteFile(filepath.Join(bin, "xcodebuild"), []byte("#!/bin/bash\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "-e", "-c", script)
		cmd.Dir = ws
		cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GITHUB_WORKSPACE="+ws, "RUNNER_TEMP="+tmp,
			"IOS_PATH=ios", "BUILD_ID=b1", "SCHEME=", "PROJECT_TYPE=native", "CONFIGURATION=Release", "USE_SIGNING="+signing,
			"BUILD_NUMBER=1.2.0+7", "DISTRIBUTION=store", "DEVELOPMENT_TEAM=TEAM1", "PROVISIONING_PROFILE_NAME=Builder store run.mobai.shelf",
			"PROFILE_BUNDLE_ID="+appID, "CODE_SIGN_IDENTITY=Apple Distribution", "EXPORT_METHOD=app-store", "EXTENSION_PROFILES={}")
		log, err := cmd.CombinedOutput()
		if _, statErr := os.Stat(filepath.Join(ws, "ran")); statErr == nil && signing != "true" {
			t.Fatalf("the release command ran on an unsigned build:\n%s", log)
		}
		t.Logf("%s", log)
		return ws, err
	}

	t.Run("signed build runs the release command", func(t *testing.T) {
		ws, err := run(t, "true", "run.mobai.shelf")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(ws, "build", "b1.ipa")); err != nil {
			t.Fatalf("no build/b1.ipa: %v", err)
		}
		project, _ := os.ReadFile(filepath.Join(ws, "ios", "Shelf.xcodeproj", "project.pbxproj"))
		if !strings.Contains(string(project), "Builder store run.mobai.shelf") {
			t.Fatalf("the app target got no profile:\n%s", project)
		}
	})
	t.Run("unsigned build keeps the workflow's steps", func(t *testing.T) {
		if _, err := run(t, "false", "run.mobai.shelf"); err == nil {
			t.Fatal("the stubbed xcodebuild path passed")
		}
	})
	t.Run("wildcard profile fails before the command", func(t *testing.T) {
		ws, err := run(t, "true", "run.mobai.*")
		if err == nil {
			t.Fatal("passed with a wildcard profile")
		}
		if _, statErr := os.Stat(filepath.Join(ws, "ran")); statErr == nil {
			t.Fatal("the release command ran")
		}
	})
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
