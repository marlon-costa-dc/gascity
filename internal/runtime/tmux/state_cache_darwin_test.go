package tmux

import (
	"strings"
	"testing"
)

func TestProcessSnapshotPSArgsRejectsLinuxSyntaxOnDarwin(t *testing.T) {
	// Regression: macOS ps rejects the BSD `:N=` column-width form. Confirm
	// we don't emit it on Darwin. The filename selects the actual platform.
	args := processSnapshotPSArgs()
	for _, a := range args {
		if strings.Contains(a, ":") {
			t.Fatalf("processSnapshotPSArgs returned %v on darwin; contains Linux-only `:N=` width specifier", args)
		}
	}
}
