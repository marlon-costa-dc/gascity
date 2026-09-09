package dolt_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const cleanupScript = "commands/cleanup/run.sh"

type cleanupFixture struct {
	root     string
	cityPath string
	dataDir  string
	binDir   string
}

func newCleanupFixture(t *testing.T) cleanupFixture {
	t.Helper()

	root := repoRoot(t)
	base := t.TempDir()
	cityPath := filepath.Join(base, "city")
	dataDir := filepath.Join(base, "data")
	binDir := filepath.Join(base, "bin")

	for _, dir := range []string{
		filepath.Join(cityPath, ".beads"),
		filepath.Join(cityPath, "rigs"),
		filepath.Join(dataDir, "target_orphan", ".dolt"),
		filepath.Join(dataDir, "keep_orphan", ".dolt"),
		filepath.Join(dataDir, "city_registered", ".dolt"),
		filepath.Join(dataDir, "not_dolt"),
		binDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	metadata := `{"database":"dolt","backend":"dolt","dolt_database":"city_registered"}`
	if err := os.WriteFile(filepath.Join(cityPath, ".beads", "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	writeExecutable(t, filepath.Join(binDir, "gc"), `#!/bin/sh
if [ "$1" = "rig" ] && [ "$2" = "list" ] && [ "$3" = "--json" ]; then
  printf '%s\n' '{"rigs":[]}'
  exit 0
fi
echo "unexpected gc invocation: $*" >&2
exit 1
`)

	return cleanupFixture{
		root:     root,
		cityPath: cityPath,
		dataDir:  dataDir,
		binDir:   binDir,
	}
}

func (f cleanupFixture) run(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command("sh", append([]string{filepath.Join(f.root, cleanupScript)}, args...)...)
	cmd.Env = append(filteredEnv(
		"PATH",
		"GC_CITY_PATH",
		"GC_PACK_DIR",
		"GC_DOLT_DATA_DIR",
		"GC_DOLT_HOST",
		"GC_DOLT_PORT",
		"GC_DOLT_USER",
		"GC_DOLT_PASSWORD",
	),
		"PATH="+f.binDir+":"+os.Getenv("PATH"),
		"GC_CITY_PATH="+f.cityPath,
		"GC_PACK_DIR="+f.root,
		"GC_DOLT_DATA_DIR="+f.dataDir,
		"GC_DOLT_HOST=127.0.0.1",
		"GC_DOLT_PORT=19999",
		"GC_DOLT_USER=root",
		"GC_DOLT_PASSWORD=",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCleanupDatabaseSelectorDryRunFiltersBeforeCount(t *testing.T) {
	f := newCleanupFixture(t)

	out, err := f.run(t, "--database", "target_orphan")
	if err != nil {
		t.Fatalf("cleanup exact dry-run failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"target_orphan",
		"1 orphaned database(s). Use --force to remove.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("exact dry-run output missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"keep_orphan", "city_registered"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("exact dry-run output included %q:\n%s", forbidden, out)
		}
	}
}

func TestCleanupWithoutDatabaseKeepsAllOrphanMode(t *testing.T) {
	f := newCleanupFixture(t)

	out, err := f.run(t)
	if err != nil {
		t.Fatalf("cleanup all-orphan dry-run failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"target_orphan",
		"keep_orphan",
		"2 orphaned database(s). Use --force to remove.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("all-orphan output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "city_registered") {
		t.Fatalf("all-orphan output included registered database:\n%s", out)
	}
}

func TestCleanupDatabaseSelectorRejectsInvalidAbsentRegisteredAndNonDoltTargets(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "invalid name",
			target: "bad.name",
			want:   "gc dolt cleanup: invalid --database target 'bad.name': name contains forbidden characters (allowed: A-Z, a-z, 0-9, _, -)",
		},
		{
			name:   "absent target",
			target: "missing_orphan",
			want:   "gc dolt cleanup: database 'missing_orphan' not found under ",
		},
		{
			name:   "registered target",
			target: "city_registered",
			want:   "gc dolt cleanup: database 'city_registered' is registered; refusing exact orphan cleanup",
		},
		{
			name:   "existing non-dolt path",
			target: "not_dolt",
			want:   "gc dolt cleanup: database 'not_dolt' is not a Dolt database under ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCleanupFixture(t)

			out, err := f.run(t, "--database", tt.target)
			if err == nil {
				t.Fatalf("cleanup exact selector succeeded for %s:\n%s", tt.target, out)
			}
			if !strings.Contains(out, tt.want) {
				t.Fatalf("cleanup exact selector output missing %q:\n%s", tt.want, out)
			}
			if _, statErr := os.Stat(filepath.Join(f.dataDir, "keep_orphan", ".dolt")); statErr != nil {
				t.Fatalf("keep_orphan changed after reject: %v", statErr)
			}
		})
	}
}
