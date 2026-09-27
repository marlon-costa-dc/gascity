package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	removeRetryAttempts = 10
	removeRetryDelay    = 50 * time.Millisecond
)

// retryRemoveAllForTest retries remove briefly to absorb a lingering
// embedded-dolt background writer that can hold files open a few dozen ms
// past the owning bd subprocess's apparent exit — which otherwise races
// t.TempDir()'s single-shot RemoveAll cleanup with an intermittent
// "directory not empty" error. It logs rather than fails on a final give-up,
// so TempDir's own best-effort cleanup still gets the last word while a
// future red run can still tell an insufficient guard from a missing one.
// cmd/gc dodges the same lingering-writer hazard by placement instead — see
// doltIdentityHomeDir (ga-7dgcg6).
func retryRemoveAllForTest(t *testing.T, dir string, remove func(string) error) {
	t.Helper()
	if err := retryRemoveAll(dir, remove, removeRetryAttempts, removeRetryDelay); err != nil {
		t.Logf("guarded removal of %s exhausted %d attempts: %v", dir, removeRetryAttempts, err)
	}
}

// retryRemoveAll calls remove(dir) until it reports success or the attempt
// budget runs out, pausing delay between tries but not after the last one.
// It returns nil once a removal succeeds, and otherwise the final failure so
// the caller can report the give-up rather than discard it.
func retryRemoveAll(dir string, remove func(string) error, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		lastErr = remove(dir)
		if lastErr == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(delay)
		}
	}
	return lastErr
}

// bdWalkUpSafeTempRoot returns the root new throwaway bd workspaces are
// created under: the ambient temp dir when nothing in its ancestry up to the
// filesystem root carries a .beads workspace record, and "/tmp" otherwise,
// whose only remaining ancestors are /tmp and /. Fleet agents legitimately
// run with TMPDIR under their home (TMPDIR=$HOME/tmp), and a home that hosts
// a city carries .beads/metadata.json; a bd subprocess spawned with cmd.Dir
// in a temp dir under it then walks up, resolves that OUTER workspace, and
// refuses a fresh default-mode init ("this workspace is recorded as
// proxied-server"). The env scrub and the test-owned HOME cover bd's env and
// config channels; this root is the same isolation applied to bd's
// directory-walk-up channel.
func bdWalkUpSafeTempRoot() string {
	root := os.TempDir()
	for dir := root; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".beads")); err == nil {
			return "/tmp"
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return root
		}
	}
}

// guardedTempDir returns a t.TempDir()-shaped directory whose removal is
// retried by retryRemoveAllForTest, placed under bdWalkUpSafeTempRoot so the
// bd subprocesses these tests spawn cannot resolve a workspace outside the
// throwaway dir. Every temp dir a bd subprocess writes into needs this, so
// bead ga-531fk — where the dir pinned as HOME was the one missing the guard
// and failed a Mac CI run after its test body had already passed — cannot
// repeat: the guard now comes with the directory instead of being a second
// line to remember.
func guardedTempDir(t *testing.T) string {
	t.Helper()
	return guardedTempDirWith(t, os.RemoveAll)
}

// guardedTempDirWith is guardedTempDir with the removal call injected. The
// registration is the whole point of the helper and yet is invisible to a
// dir-is-gone assertion, because an idle dir removed by the retrying removal
// leaves nothing to observe; injecting the removal is what lets a test
// observe that the cleanup was registered at all. Ordinary callers want
// guardedTempDir.
func guardedTempDirWith(t *testing.T, remove func(string) error) string {
	t.Helper()
	dir, err := os.MkdirTemp(bdWalkUpSafeTempRoot(), "gc-doctor-guarded-")
	if err != nil {
		t.Fatalf("MkdirTemp under %s: %v", bdWalkUpSafeTempRoot(), err)
	}
	t.Cleanup(func() { retryRemoveAllForTest(t, dir, remove) })
	return dir
}

// testOwnedHome pins HOME to a fresh guarded temp dir for the duration of the
// test and returns it. bd's config precedence falls through, as a last
// resort, to $HOME/.beads/config.yaml, so only a test-owned HOME keeps a
// machine-level dolt.shared-server setting out of the bd subprocesses these
// tests spawn (ga-zxpfic). bd then writes $HOME/.beads/ itself, which is why
// that dir needs the same retrying removal as the working dir.
//
// The pin deliberately hides ONLY bd's own config namespace, not the host's
// version-manager state: on a mise-managed host PATH resolves bd through a
// shim, and a shim resolves its version from the manager's global config and
// install store — both rooted at the real home. Hiding HOME without carrying
// those two directories turns every bd subprocess into "No version is set
// for shim: bd". The manager's config namespace (.config/mise) is disjoint
// from the .beads fallback this pin exists to exclude, so the directories
// are carried across by their own explicit env vars, which the shim reads
// regardless of HOME.
func testOwnedHome(t *testing.T) string {
	t.Helper()
	home := guardedTempDir(t)
	if realHome, err := os.UserHomeDir(); err == nil && realHome != "" {
		miseConfig := filepath.Join(realHome, ".config", "mise")
		if _, err := os.Stat(miseConfig); err == nil {
			t.Setenv("MISE_CONFIG_DIR", miseConfig)
		}
		miseData := filepath.Join(realHome, ".local", "share", "mise")
		if _, err := os.Stat(miseData); err == nil {
			t.Setenv("MISE_DATA_DIR", miseData)
		}
	}
	t.Setenv("HOME", home)
	return home
}

func TestRetryRemoveAllRetriesUntilRemovalSucceeds(t *testing.T) {
	calls := 0
	err := retryRemoveAll(t.TempDir(), func(string) error {
		calls++
		if calls < 3 {
			return errors.New("directory not empty")
		}
		return nil
	}, removeRetryAttempts, 0)
	if calls != 3 {
		t.Fatalf("remove called %d times, want 3 (two failures, then success)", calls)
	}
	if err != nil {
		t.Fatalf("retryRemoveAll returned %v, want nil once a removal succeeds", err)
	}
}

func TestRetryRemoveAllStopsAtItsAttemptBudget(t *testing.T) {
	calls := 0
	wantErr := errors.New("directory not empty")
	err := retryRemoveAll(t.TempDir(), func(string) error {
		calls++
		return wantErr
	}, 4, 0)
	if calls != 4 {
		t.Fatalf("remove called %d times, want 4 (the attempt budget)", calls)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("retryRemoveAll returned %v, want the final failure %v", err, wantErr)
	}
}

// TestGuardedTempDirRegistersTheRetryingRemoval pins the wiring between the
// two halves the tests below and above cover separately: that
// guardedTempDirWith registers the retrying removal on the dir it hands
// back. Deleting that registration — exactly the pre-ga-531fk shape — drives
// calls to 0 and turns this red, which no dir-is-gone assertion can do,
// since t.TempDir() removes an idle dir on its own. The injected remove
// deliberately never removes anything: TempDir's own cleanup still clears
// the dir. That the guardedTempDir wrapper every real caller uses reaches
// this seam with a real removal is pinned separately by
// TestGuardedTempDirRemovalRunsBeforeTempDirsOwnCleanup.
func TestGuardedTempDirRegistersTheRetryingRemoval(t *testing.T) {
	calls := 0
	t.Run("guarded", func(t *testing.T) {
		guardedTempDirWith(t, func(string) error {
			calls++
			if calls < 3 {
				return errors.New("directory not empty")
			}
			return nil
		})
	})
	if calls != 3 {
		t.Fatalf("registered remove called %d times, want 3 (two failures, then success)", calls)
	}
}

// TestGuardedTempDirRemovesItsDirWhenTheTestEnds pins the structural half of
// the contract — the returned dir is test-scoped and gone once the owning
// test finishes. The retry half is covered by the retryRemoveAll tests above,
// since a single RemoveAll of an idle dir succeeds on the first attempt; the
// wiring between the two halves is covered by
// TestGuardedTempDirRegistersTheRetryingRemoval for the seam and by
// TestGuardedTempDirRemovalRunsBeforeTempDirsOwnCleanup for the wrapper.
func TestGuardedTempDirRemovesItsDirWhenTheTestEnds(t *testing.T) {
	var dir string
	t.Run("guarded", func(t *testing.T) {
		dir = guardedTempDir(t)
		if err := os.WriteFile(filepath.Join(dir, "leftover"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Stat(%s) after the subtest returned err=%v, want the dir removed", dir, err)
	}
}

func TestTestOwnedHomePinsHOMEToAGuardedTempDir(t *testing.T) {
	var home string
	t.Run("pinned", func(t *testing.T) {
		home = testOwnedHome(t)
		if got := os.Getenv("HOME"); got != home {
			t.Fatalf("HOME = %q, want the test-owned dir %q", got, home)
		}
		if err := os.MkdirAll(filepath.Join(home, ".beads"), 0o700); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("Stat(%s) after the subtest returned err=%v, want the test-owned HOME removed", home, err)
	}
}

// TestGuardedTempDirPlacesTheDirOutsideBeadsAncestry pins the walk-up half
// of the guard: the dir must be created under a root whose ancestry carries
// no .beads workspace record, so a bd subprocess spawned with cmd.Dir in it
// cannot resolve an outer workspace and refuse a fresh init. The red this
// pins is a placement regression back under the ambient temp root on a host
// whose TMPDIR lives under a beads-recorded home.
func TestGuardedTempDirPlacesTheDirOutsideBeadsAncestry(t *testing.T) {
	dir := guardedTempDir(t)
	for ancestor := dir; ; ancestor = filepath.Dir(ancestor) {
		if _, err := os.Stat(filepath.Join(ancestor, ".beads")); err == nil {
			t.Fatalf("guarded temp dir %s sits under %s, which carries a .beads record a bd walk-up would resolve", dir, ancestor)
		}
		if parent := filepath.Dir(ancestor); parent == ancestor {
			break
		}
	}
}
