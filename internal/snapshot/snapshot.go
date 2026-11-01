// Package snapshot captures the working tree as a Git commit and publishes it
// to a remote ref, leaving HEAD, the current branch and the index untouched.
package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MobAI-App/ios-builder/internal/procutil"
)

// refPrefix is the remote ref namespace holding build snapshots. Refs outside
// refs/heads and refs/tags do not appear as branches and fire no push events.
const refPrefix = "refs/ios-builder/jobs/"

// Ref returns the remote ref that holds the snapshot for a build.
func Ref(buildID string) string {
	return refPrefix + buildID
}

// Create commits the working tree, including staged, unstaged and untracked
// files, and returns the commit SHA. Files excluded by .gitignore are not
// included. The commit's parent is HEAD, so it is never reachable from any
// local branch.
func Create(ctx context.Context, message string) (string, error) {
	dir, err := os.MkdirTemp("", "ios-builder-index")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary index: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	index := filepath.Join(dir, "index")
	if _, err := git(ctx, index, "read-tree", "HEAD"); err != nil {
		return "", err
	}
	if _, err := git(ctx, index, "add", "-A"); err != nil {
		return "", err
	}

	tree, err := git(ctx, index, "write-tree")
	if err != nil {
		return "", err
	}

	return gitEnv(ctx, index, fallbackIdentity(ctx), "commit-tree", tree, "-p", "HEAD", "-m", message)
}

// The identity a snapshot is committed as when git has none. A fresh
// machine (a repository cloned through MobAI on Windows, say) often has no
// user.name or user.email, and commit-tree then refuses. The snapshot only
// lives on a hidden ref, so who it names does not matter.
const (
	fallbackName  = "MobAI"
	fallbackEmail = "snapshot@mobai.run"
)

// fallbackIdentity is the environment that fills in whichever of the name
// and email git has no value for, nil when it has both. The user's own
// identity is kept whenever one is set.
func fallbackIdentity(ctx context.Context) []string {
	var env []string
	// git config exits 1 for a key with no value: that reads as unset.
	if name, _ := git(ctx, "", "config", "user.name"); name == "" {
		env = append(env, "GIT_AUTHOR_NAME="+fallbackName, "GIT_COMMITTER_NAME="+fallbackName)
	}
	if email, _ := git(ctx, "", "config", "user.email"); email == "" {
		env = append(env, "GIT_AUTHOR_EMAIL="+fallbackEmail, "GIT_COMMITTER_EMAIL="+fallbackEmail)
	}
	return env
}

// Push publishes a snapshot commit to ref on the remote.
func Push(ctx context.Context, remote, sha, ref string) error {
	_, err := git(ctx, "", "push", remote, sha+":"+ref)
	return err
}

// Delete removes ref from the remote.
func Delete(ctx context.Context, remote, ref string) error {
	_, err := git(ctx, "", "push", "--delete", remote, ref)
	return err
}

func git(ctx context.Context, index string, args ...string) (string, error) {
	return gitEnv(ctx, index, nil, args...)
}

// gitEnv runs git with env added to its environment.
func gitEnv(ctx context.Context, index string, env []string, args ...string) (string, error) {
	cmd := procutil.CommandContext(ctx, "git", args...)
	// Without this, a repository using HTTPS without a credential helper blocks
	// on an interactive password prompt instead of failing.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if index != "" {
		cmd.Env = append(cmd.Env, "GIT_INDEX_FILE="+index)
	}
	cmd.Env = append(cmd.Env, env...)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(string(out)), nil
}
