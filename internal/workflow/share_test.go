package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestShareBuildUsesMobaiJSON runs the share workflow's build step: the
// debug build command in the nearest mobai.json at or above the iOS project
// builds the first app, and the generated script stays the fallback without
// one (a release entry alone does not count).
func TestShareBuildUsesMobaiJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell test")
	}
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable", tool)
		}
	}
	all := steps(t, "ios-share.yml")
	script := all[indexOfStep(t, all, "Build for the simulator")].Run

	run := func(t *testing.T, mobaiJSON, iosPath, projectType string) (string, string, error) {
		t.Helper()
		ws, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		tmp, bin := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, iosPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if mobaiJSON != "" {
			if err := os.WriteFile(filepath.Join(ws, "apps/mobile/mobai.json"), []byte(mobaiJSON), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		// The fallback script's Flutter path, stubbed.
		flutter := "#!/bin/bash\nmkdir -p build/ios/iphonesimulator/Runner.app\n"
		if err := os.WriteFile(filepath.Join(bin, "flutter"), []byte(flutter), 0o755); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(tmp, "out")
		cmd := exec.Command("bash", "-e", "-c", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GITHUB_WORKSPACE="+ws, "RUNNER_TEMP="+tmp,
			"GITHUB_OUTPUT="+out, "IOS_PATH="+iosPath, "SCHEME=", "PROJECT_TYPE="+projectType)
		log, err := cmd.CombinedOutput()
		outputs, _ := os.ReadFile(out)
		for _, line := range strings.Split(string(outputs), "\n") {
			if p, ok := strings.CutPrefix(line, "app_path="); ok {
				return p, ws, err
			}
		}
		t.Logf("%s", log)
		return "", ws, err
	}

	t.Run("mobai.json command", func(t *testing.T) {
		got, ws, err := run(t, `{"build": {"ios": {"debug": {"command": "mkdir -p build/Shelf.app", "app": "build/*.app"}, "release": {"command": "exit 7", "app": "x.ipa"}}}}`, "apps/mobile/ios", "native")
		if err != nil || got != filepath.Join(ws, "apps/mobile/build/Shelf.app") {
			t.Fatalf("app_path %q, err %v", got, err)
		}
	})
	t.Run("no build entry falls back to the script", func(t *testing.T) {
		got, ws, err := run(t, `{"project": "p", "build": {"ios": {"release": {"command": "exit 7", "app": "x.ipa"}}}}`, "apps/mobile", "flutter")
		if err != nil || got != filepath.Join(ws, "build/ios/iphonesimulator/Runner.app") {
			t.Fatalf("app_path %q, err %v", got, err)
		}
	})
	t.Run("command that makes no app fails", func(t *testing.T) {
		if got, _, err := run(t, `{"build": {"ios": {"debug": {"command": "true", "app": "build/*.app"}}}}`, "apps/mobile/ios", "native"); err == nil {
			t.Fatalf("passed with app_path %q", got)
		}
	})
}
