package core

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// molScopedWorkFormula is the subset of mol-scoped-work.toml these tests
// inspect. Steps carry the agent-facing shell, so the step description is the
// executable contract of the formula.
type molScopedWorkFormula struct {
	Steps []struct {
		ID          string `toml:"id"`
		Title       string `toml:"title"`
		Description string `toml:"description"`
	} `toml:"steps"`
}

func readMolScopedWork(t *testing.T) (molScopedWorkFormula, string) {
	t.Helper()
	data, err := fs.ReadFile(PackFS, "formulas/mol-scoped-work.toml")
	if err != nil {
		t.Fatalf("reading formulas/mol-scoped-work.toml: %v", err)
	}
	var parsed molScopedWorkFormula
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		t.Fatalf("decoding formulas/mol-scoped-work.toml: %v", err)
	}
	return parsed, string(data)
}

func molScopedWorkStep(t *testing.T, f molScopedWorkFormula, id string) (title, description string) {
	t.Helper()
	for _, step := range f.Steps {
		if step.ID == id {
			return step.Title, step.Description
		}
	}
	t.Fatalf("mol-scoped-work has no step %q", id)
	return "", ""
}

// molScopedWorkScript extracts the step's bash block and renders the formula
// variables the workspace lifecycle steps use.
func molScopedWorkScript(t *testing.T, description string) string {
	t.Helper()
	const open = "```bash\n"
	start := strings.Index(description, open)
	if start < 0 {
		t.Fatalf("step has no bash block:\n%s", description)
	}
	body := description[start+len(open):]
	end := strings.Index(body, "```")
	if end < 0 {
		t.Fatalf("step bash block is not closed:\n%s", description)
	}
	return strings.NewReplacer(
		"{{convoy_id}}", "cv-1",
		"{{base_branch}}", "main",
		"{{setup_command}}", "",
	).Replace(body[:end])
}

// scopedWorkHarness is a city checkout cloned from a real origin repository,
// with a `gc` stand-in that answers the convoy and bead queries the workspace
// steps issue and records every bead update.
type scopedWorkHarness struct {
	t         *testing.T
	root      string
	city      string
	beadJSON  string
	updateLog string
	env       []string
}

func newScopedWorkHarness(t *testing.T) *scopedWorkHarness {
	t.Helper()
	for _, tool := range []string{"bash", "git", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("mol-scoped-work workspace steps require %s: %v", tool, err)
		}
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &scopedWorkHarness{
		t:         t,
		root:      root,
		city:      filepath.Join(root, "city"),
		beadJSON:  filepath.Join(root, "bead.json"),
		updateLog: filepath.Join(root, "updates.log"),
	}
	gcStub := `#!/bin/sh
set -eu
case "$1 $2" in
  "convoy status") printf '{"children":[{"id":"gc-1"}]}\n' ;;
  "bd show") cat "$SCOPED_WORK_BEAD_JSON" ;;
  "bd update") shift 2; printf '%s\n' "$*" >> "$SCOPED_WORK_UPDATE_LOG" ;;
  *) echo "unexpected gc invocation: $*" >&2; exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gc"), []byte(gcStub), 0o755); err != nil {
		t.Fatal(err)
	}
	h.env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+root,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(root, "gitconfig"),
		"GIT_AUTHOR_NAME=scoped-work",
		"GIT_AUTHOR_EMAIL=scoped-work@example.invalid",
		"GIT_COMMITTER_NAME=scoped-work",
		"GIT_COMMITTER_EMAIL=scoped-work@example.invalid",
		"SCOPED_WORK_BEAD_JSON="+h.beadJSON,
		"SCOPED_WORK_UPDATE_LOG="+h.updateLog,
	)

	seed := filepath.Join(root, "seed")
	h.git(root, "init", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.git(seed, "add", "README")
	h.git(seed, "commit", "-m", "seed")
	h.git(root, "clone", seed, h.city)
	return h
}

// command runs one harness process. It is the single process call site of
// these tests: git fixtures and the formula step scripts both go through it.
func (h *scopedWorkHarness) command(dir string, stdout, stderr *bytes.Buffer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = h.env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (h *scopedWorkHarness) git(dir string, args ...string) string {
	h.t.Helper()
	var out bytes.Buffer
	if err := h.command(dir, &out, &out, "git", args...); err != nil {
		h.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out.String())
	}
	return strings.TrimSpace(out.String())
}

func (h *scopedWorkHarness) setBeadMetadata(metadata string) {
	h.t.Helper()
	if err := os.WriteFile(h.beadJSON, []byte(`[{"metadata":`+metadata+`}]`), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

// run executes a step script without errexit, so every failure the step
// reports is one it raises itself.
func (h *scopedWorkHarness) run(script string) (stderr string, err error) {
	h.t.Helper()
	var outBuf, errBuf bytes.Buffer
	err = h.command(h.city, &outBuf, &errBuf, "bash", "-c", script)
	return errBuf.String(), err
}

func (h *scopedWorkHarness) updates() string {
	h.t.Helper()
	data, err := os.ReadFile(h.updateLog)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		h.t.Fatal(err)
	}
	return string(data)
}

func TestMolScopedWorkCreatesTrackedBranchWithoutRetry(t *testing.T) {
	formula, raw := readMolScopedWork(t)
	title, _ := molScopedWorkStep(t, formula, "workspace-setup")
	if title != "Set up a worktree and branch" {
		t.Fatalf("workspace-setup title = %q, want the tracked-branch contract", title)
	}
	if _, implement := molScopedWorkStep(t, formula, "implement"); !strings.Contains(implement, "on the branch created by\nworkspace-setup") {
		t.Fatalf("implement must direct work onto the branch created by workspace-setup:\n%s", implement)
	}
	if strings.Contains(raw, "[steps.retry]") {
		t.Fatal("mol-scoped-work must propagate the first failure without formula retries")
	}
}

func TestMolScopedWorkSetupCreatesAndCleanupReleasesTrackedBranch(t *testing.T) {
	formula, _ := readMolScopedWork(t)
	_, setupStep := molScopedWorkStep(t, formula, "workspace-setup")
	_, cleanupStep := molScopedWorkStep(t, formula, "cleanup-worktree")
	h := newScopedWorkHarness(t)
	h.setBeadMetadata(`{}`)

	if stderr, err := h.run(molScopedWorkScript(t, setupStep)); err != nil {
		t.Fatalf("workspace-setup on a fresh bead: %v\n%s", err, stderr)
	}
	worktree := filepath.Join(h.city, "worktrees", "gc-1")
	if got := h.git(worktree, "symbolic-ref", "--short", "HEAD"); got != "work/gc-1" {
		t.Fatalf("new worktree branch = %q, want work/gc-1", got)
	}
	wantCreate := "gc-1 --set-metadata work_dir=" + worktree + " --set-metadata gc.work_branch=work/gc-1\n"
	if got := h.updates(); got != wantCreate {
		t.Fatalf("bead updates = %q, want %q", got, wantCreate)
	}

	h.setBeadMetadata(`{"work_dir":"` + worktree + `","gc.work_branch":"work/gc-1"}`)
	if stderr, err := h.run(molScopedWorkScript(t, cleanupStep)); err != nil {
		t.Fatalf("cleanup-worktree: %v\n%s", err, stderr)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("cleanup left the worktree behind: stat err = %v", err)
	}
	if got := h.updates(); !strings.HasSuffix(got, "--unset-metadata work_dir --unset-metadata gc.work_branch\n") {
		t.Fatalf("cleanup must release both work_dir and gc.work_branch; updates = %q", got)
	}
}

func TestMolScopedWorkSetupAcceptsRecordedWorktreeOnTrackedBranch(t *testing.T) {
	formula, _ := readMolScopedWork(t)
	_, setupStep := molScopedWorkStep(t, formula, "workspace-setup")
	h := newScopedWorkHarness(t)
	worktree := filepath.Join(h.root, "recorded")
	h.git(h.city, "worktree", "add", "-b", "work/gc-1", worktree, "origin/main")
	h.setBeadMetadata(`{"work_dir":"` + worktree + `"}`)

	if stderr, err := h.run(molScopedWorkScript(t, setupStep)); err != nil {
		t.Fatalf("workspace-setup on a recorded tracked worktree: %v\n%s", err, stderr)
	}
	if got, want := h.updates(), "gc-1 --set-metadata gc.work_branch=work/gc-1\n"; got != want {
		t.Fatalf("bead updates = %q, want %q", got, want)
	}
}

// TestMolScopedWorkSetupRejectsUntrackedRecordedWorktree covers every recorded
// worktree the setup step must refuse instead of repairing. A detached HEAD in
// particular fails exactly like a foreign branch: the step never switches or
// creates a branch inside an existing worktree.
func TestMolScopedWorkSetupRejectsUntrackedRecordedWorktree(t *testing.T) {
	formula, _ := readMolScopedWork(t)
	_, setupStep := molScopedWorkStep(t, formula, "workspace-setup")

	for _, tc := range []struct {
		name        string
		prepare     func(h *scopedWorkHarness, worktree string)
		recorded    string
		wantStderr  string
		afterReject func(t *testing.T, h *scopedWorkHarness, worktree string)
	}{
		{
			name: "detached HEAD",
			prepare: func(h *scopedWorkHarness, worktree string) {
				h.git(h.city, "worktree", "add", "--detach", worktree, "origin/main")
			},
			wantStderr: "mol-scoped-work worktree has a detached HEAD: expected work/gc-1 at ",
			afterReject: func(t *testing.T, h *scopedWorkHarness, worktree string) {
				if branches := h.git(h.city, "branch", "--list", "work/gc-1"); branches != "" {
					t.Fatalf("rejected detached worktree was repaired: branch %q exists", branches)
				}
				if head := h.git(worktree, "rev-parse", "--abbrev-ref", "HEAD"); head != "HEAD" {
					t.Fatalf("rejected detached worktree HEAD = %q, want it still detached", head)
				}
			},
		},
		{
			name: "foreign branch",
			prepare: func(h *scopedWorkHarness, worktree string) {
				h.git(h.city, "worktree", "add", "-b", "feature/other", worktree, "origin/main")
			},
			wantStderr: "mol-scoped-work worktree branch mismatch: expected work/gc-1, found feature/other",
		},
		{
			name: "contradictory recorded branch",
			prepare: func(h *scopedWorkHarness, worktree string) {
				h.git(h.city, "worktree", "add", "-b", "work/gc-1", worktree, "origin/main")
			},
			recorded:   "work/other",
			wantStderr: "mol-scoped-work recorded branch mismatch: expected work/gc-1, found work/other",
		},
		{
			name:       "missing directory",
			prepare:    func(*scopedWorkHarness, string) {},
			wantStderr: "mol-scoped-work recorded worktree does not exist: ",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newScopedWorkHarness(t)
			worktree := filepath.Join(h.root, "recorded")
			tc.prepare(h, worktree)
			metadata := `{"work_dir":"` + worktree + `"`
			if tc.recorded != "" {
				metadata += `,"gc.work_branch":"` + tc.recorded + `"`
			}
			h.setBeadMetadata(metadata + `}`)

			stderr, err := h.run(molScopedWorkScript(t, setupStep))
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("workspace-setup err = %v, want exit status 1\n%s", err, stderr)
			}
			if !strings.Contains(stderr, tc.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, tc.wantStderr)
			}
			if got := h.updates(); got != "" {
				t.Fatalf("rejected setup updated the bead: %q", got)
			}
			if tc.afterReject != nil {
				tc.afterReject(t, h, worktree)
			}
		})
	}
}
