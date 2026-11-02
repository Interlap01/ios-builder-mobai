package xcodeproj

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"howett.net/plist"
)

// Entitlements reads the .entitlements file of the app target and of each
// extension target (CODE_SIGN_ENTITLEMENTS in the configuration named, else
// in any configuration that sets it) of every *.xcodeproj directly under
// dir, keyed by bundle ID. The app target is the one whose bundle ID is
// appBundleID, or the only app target when its ID is built from other
// settings; an extension whose ID is built from other settings is skipped,
// as ExtensionBundleIDs skips it. Values in the files keep their variables
// ($(AppIdentifierPrefix)); a dir without a project yields nil.
func Entitlements(dir, appBundleID, configuration string) (map[string]map[string]any, error) {
	if dir == "" {
		dir = "."
	}
	projects, _ := filepath.Glob(filepath.Join(dir, "*.xcodeproj", "project.pbxproj"))
	var out map[string]map[string]any
	for _, path := range projects {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files, err := entitlementFiles(data, appBundleID, configuration)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for id, file := range files {
			ents, err := readEntitlements(dir, file.path, file.target)
			if err != nil {
				return nil, err
			}
			if ents == nil {
				continue
			}
			if out == nil {
				out = map[string]map[string]any{}
			}
			out[id] = ents
		}
	}
	return out, nil
}

type entitlementFile struct{ path, target string }

// entitlementFiles maps bundle IDs to the CODE_SIGN_ENTITLEMENTS of their
// targets in one project.pbxproj.
func entitlementFiles(data []byte, appBundleID, configuration string) (map[string]entitlementFile, error) {
	var project struct {
		Objects map[string]struct {
			Isa                    string   `plist:"isa"`
			Name                   string   `plist:"name"`
			ProductType            string   `plist:"productType"`
			BuildConfigurationList string   `plist:"buildConfigurationList"`
			BuildConfigurations    []string `plist:"buildConfigurations"`
			BuildSettings          struct {
				BundleID     string `plist:"PRODUCT_BUNDLE_IDENTIFIER"`
				Entitlements string `plist:"CODE_SIGN_ENTITLEMENTS"`
			} `plist:"buildSettings"`
		} `plist:"objects"`
	}
	if _, err := plist.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("parse project.pbxproj: %w", err)
	}
	type target struct{ name, bundleID, entitlements string }
	var apps, extensions []target
	for _, t := range project.Objects {
		isApp := t.Isa == "PBXNativeTarget" && t.ProductType == "com.apple.product-type.application"
		isExtension := t.Isa == "PBXNativeTarget" && slices.Contains(ExtensionProductTypes, t.ProductType)
		if !isApp && !isExtension {
			continue
		}
		found := target{name: t.Name}
		for _, configID := range project.Objects[t.BuildConfigurationList].BuildConfigurations {
			c := project.Objects[configID]
			// The named configuration wins; any other only fills a gap.
			if c.Name == configuration || found.bundleID == "" {
				if c.BuildSettings.BundleID != "" {
					found.bundleID = c.BuildSettings.BundleID
				}
			}
			if c.Name == configuration || found.entitlements == "" {
				if c.BuildSettings.Entitlements != "" {
					found.entitlements = c.BuildSettings.Entitlements
				}
			}
		}
		if isApp {
			apps = append(apps, found)
		} else {
			extensions = append(extensions, found)
		}
	}
	files := map[string]entitlementFile{}
	for _, a := range apps {
		if a.entitlements == "" {
			continue
		}
		if a.bundleID == appBundleID || (len(apps) == 1 && strings.Contains(a.bundleID, "$")) {
			files[appBundleID] = entitlementFile{a.entitlements, a.name}
		}
	}
	for _, e := range extensions {
		if e.entitlements != "" && e.bundleID != "" && !strings.Contains(e.bundleID, "$") {
			files[e.bundleID] = entitlementFile{e.entitlements, e.name}
		}
	}
	return files, nil
}

// readEntitlements reads one .entitlements file, its path relative to the
// project's folder; a path built from settings other than the project's own
// folder and target name, or a file that is not there, is skipped (nil, nil).
func readEntitlements(dir, path, target string) (map[string]any, error) {
	path = strings.NewReplacer(
		"$(SRCROOT)", dir, "${SRCROOT}", dir,
		"$(PROJECT_DIR)", dir, "${PROJECT_DIR}", dir,
		"$(TARGET_NAME)", target, "${TARGET_NAME}", target,
	).Replace(strings.Trim(path, `"`))
	if strings.Contains(path, "$") {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// The build itself names a missing file; signing goes on without it.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the entitlements: %w", err)
	}
	var ents map[string]any
	if _, err := plist.Unmarshal(data, &ents); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return ents, nil
}
