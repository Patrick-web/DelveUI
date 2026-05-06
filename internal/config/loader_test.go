package config

import (
	"os"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home dir")
	}
	root := "/workspace/foo"

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "/abs/path", "/abs/path"},
		{"empty", "", ""},
		{"tilde-prefix", "~/projects", home + "/projects"},
		{"tilde-only", "~", home},
		{"userHome", "${userHome}/code/foo", home + "/code/foo"},
		{"env-HOME", "${env:HOME}/code/foo", home + "/code/foo"},
		{"dollar-HOME", "$HOME/code/foo", home + "/code/foo"},
		{"workspaceFolder", "${workspaceFolder}/main.go", root + "/main.go"},
		{"workspaceRoot", "${workspaceRoot}/main.go", root + "/main.go"},
		{"zed-worktree", "$ZED_WORKTREE_ROOT/main.go", root + "/main.go"},
		{"mixed", "${userHome}/cfg/${workspaceFolder}", home + "/cfg/" + root},
		// Tilde mid-string is not expanded (matches shell behaviour).
		{"tilde-mid", "/foo/~/bar", "/foo/~/bar"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExpandPath(c.in, root)
			if got != c.want {
				t.Errorf("ExpandPath(%q)\n  got: %q\n want: %q", c.in, got, c.want)
			}
		})
	}
}

// TestExpandPath_UnknownLeftAlone keeps the function from over-reaching: only
// documented variables are substituted, anything else stays intact so we
// don't accidentally munge a path that legitimately contains "${X}".
func TestExpandPath_UnknownLeftAlone(t *testing.T) {
	got := ExpandPath("${unknown}/path", "/root")
	if !strings.Contains(got, "${unknown}") {
		t.Errorf("unknown var was substituted: %q", got)
	}
}
