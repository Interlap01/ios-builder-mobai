package snapshot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateCapturesWorkingTreeWithoutTouchingRepo(t *testing.T) {
	repo := initRepo(t)

	write(t, repo, "tracked.txt", "modified")
	write(t, repo, "untracked.txt", "new")
	write(t, repo, "ignored.txt", "secret")

	statusBefore := run(t, repo, "status", "--porcelain")
	headBefore := run(t, repo, "rev-parse", "HEAD")

	sha, err := Create(context.Background(), "snapshot test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := run(t, repo, "show", sha+":tracked.txt"); got != "modified" {
		t.Errorf("tracked.txt = %q, want %q", got, "modified")
	}
	if got := run(t, repo, "show", sha+":untracked.txt"); got != "new" {
		t.Errorf("untracked.txt = %q, want %q", got, "new")
	}
	if _, err := exec.Command("git", "-C", repo, "show", sha+":ignored.txt").Output(); err == nil {
		t.Error("ignored.txt is in the snapshot, want it excluded by .gitignore")
	}

	if parent := run(t, repo, "rev-parse", sha+"^"); parent != headBefore {
		t.Errorf("parent = %s, want HEAD %s", parent, headBefore)
	}
	if got := run(t, repo, "rev-parse", "HEAD"); got != headBefore {
		t.Errorf("HEAD moved to %s, want %s", got, headBefore)
	}
	if got := run(t, repo, "status", "--porcelain"); got != statusBefore {
		t.Errorf("status = %q, want unchanged %q", got, statusBefore)
	}
}

// A machine where git has no identity (no global or system config, nothing
// in the repository) still snapshots, as the fallback; one with an identity
// keeps it.
func TestCreateWithoutGitIdentity(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := t.TempDir()
	run(t, repo, "init", "-b", "main")
	write(t, repo, "a.txt", "one")
	run(t, repo, "add", "-A")
	run(t, repo, "-c", "user.name=Setup", "-c", "user.email=setup@example.com", "commit", "-m", "initial")
	chdir(t, repo)

	write(t, repo, "a.txt", "two")
	sha, err := Create(context.Background(), "snapshot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := fallbackName + " <" + fallbackEmail + ">"
	if got := run(t, repo, "log", "-1", "--format=%an <%ae>|%cn <%ce>", sha); got != want+"|"+want {
		t.Errorf("snapshot by %q, want %q", got, want)
	}

	run(t, repo, "config", "user.name", "Dev")
	run(t, repo, "config", "user.email", "dev@example.com")
	sha, err = Create(context.Background(), "snapshot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := run(t, repo, "log", "-1", "--format=%an <%ae>", sha); got != "Dev <dev@example.com>" {
		t.Errorf("snapshot by %q, want the configured identity", got)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		run(t, dir, args...)
	}

	write(t, dir, "tracked.txt", "original")
	write(t, dir, ".gitignore", "ignored.txt\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-m", "initial")
	chdir(t, dir)

	return dir
}

// chdir moves into dir for the test: Create runs git in the process
// working directory.
func chdir(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(string(out))
}
