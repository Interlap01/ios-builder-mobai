package build

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/MobAI-App/ios-builder/internal/procutil"
)

// ToolchainInput is the workflows' toolchain input: the Flutter and JDK
// versions given and this machine's Node, as JSON, "" when there are none.
// They travel as one input since workflow_dispatch takes ten.
func ToolchainInput(flutter, jdk string) string {
	toolchain := map[string]string{}
	if flutter != "" {
		toolchain["flutter"] = flutter
	}
	if jdk != "" {
		toolchain["jdk"] = jdk
	}
	if node := localNode(); node != "" {
		toolchain["node"] = node
	}
	if len(toolchain) == 0 {
		return ""
	}
	data, _ := json.Marshal(toolchain)
	return string(data)
}

// localNode is the major version of the Node on this machine's PATH, the one
// the developer's shell runs, "" without one. A build passes it to the runner
// so a project that pins no Node installs its packages with the npm that
// wrote its lockfile: npm 10's `npm ci` refuses a lockfile npm 11 wrote. A
// variable so tests can pin it.
var localNode = func() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := procutil.CommandContext(ctx, "node", "--version").Output()
	if err != nil {
		return ""
	}
	return nodeMajor(string(out))
}

// nodeMajor is the major version in `node --version`'s output: "24" for
// "v24.16.0", "" for anything else.
func nodeMajor(version string) string {
	major, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(version), "v"), ".")
	if major == "" {
		return ""
	}
	for _, r := range major {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return major
}
