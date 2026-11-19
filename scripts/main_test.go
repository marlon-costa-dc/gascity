package scripts_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain replaces version-manager shim directories on PATH with the
// directories of the tools they resolve to. Tests here run scripts with a
// sandboxed HOME; a mise shim there finds the host config as an untrusted
// project config and fails instead of running the tool.
func TestMain(m *testing.M) {
	if err := unshimPath(); err != nil {
		fmt.Fprintln(os.Stderr, "scripts tests:", err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func unshimPath() error {
	entries := filepath.SplitList(os.Getenv("PATH"))
	var out []string
	for _, dir := range entries {
		if filepath.Base(dir) != "shims" || !strings.Contains(dir, "mise") {
			out = append(out, dir)
			continue
		}
		bins, err := exec.Command("mise", "bin-paths").Output()
		if err != nil {
			return fmt.Errorf("mise bin-paths: %w", err)
		}
		out = append(out, strings.Fields(string(bins))...)
	}
	return os.Setenv("PATH", strings.Join(out, string(os.PathListSeparator)))
}
