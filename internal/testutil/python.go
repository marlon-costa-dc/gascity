package testutil

import (
	"os/exec"
	"strings"
	"testing"
)

// RealPython3 returns the absolute path of the python3 interpreter that the
// host's python3 actually executes, skipping the test when none is installed.
//
// The first python3 on PATH may be a version-manager shim (mise, pyenv, asdf).
// A shim resolves its target from the caller's environment: under a hermetic
// test environment (pinned HOME, PATH restricted to a temp dir) it can fail
// before any Python runs ("mise ERROR No version is set for shim: python3",
// "config files ... are not trusted"). Hermetic tests that hand python3 to a
// child process must use the interpreter itself, which this asks for through
// sys.executable in the caller's own, unrestricted environment.
func RealPython3(t testing.TB) string {
	t.Helper()
	shim, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	out, err := exec.Command(shim, "-c", "import sys; print(sys.executable)").Output()
	if err != nil {
		t.Fatalf("resolve the interpreter behind %s: %v", shim, err)
	}
	interpreter := strings.TrimSpace(string(out))
	if interpreter == "" {
		t.Fatalf("%s reported an empty sys.executable", shim)
	}
	return interpreter
}
