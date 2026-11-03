package build

import "testing"

func TestNodeMajor(t *testing.T) {
	for in, want := range map[string]string{"v24.16.0\n": "24", "v22.23.4": "22", "24": "24", "": "", "vX.1": "", "node: command not found": ""} {
		if got := nodeMajor(in); got != want {
			t.Errorf("nodeMajor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToolchainInput(t *testing.T) {
	pinNode(t, "")
	if got := ToolchainInput("", ""); got != "" {
		t.Fatalf("nothing to send: %q", got)
	}
	pinNode(t, "24")
	if got := ToolchainInput("", ""); got != `{"node":"24"}` {
		t.Fatalf("node only: %q", got)
	}
}
