//go:build beads_rowlock

// The CAS (row_lock) half of the native store tests. These compile only under
// beads_rowlock because they exercise library APIs -- Issue.RowVersion,
// UpdateIssueChecked, CloseIssueChecked -- that exist solely on a beads line
// whose embedded migrations reach 0054, the migration creating issues.row_lock.
//
// Every live store in this city is at schema 53, so the column does not exist
// and the fence cannot function; beads.conditional_writes is `off` accordingly.
// See beads gc-5oauf and gct-83zky.

package beads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	beadslib "github.com/steveyegge/beads"
)

func TestNativeDoltStoreCloseWithMetadataIfMatchRetriesWholeTransaction(t *testing.T) {
	for _, conflict := range []error{
		errors.New("Error 1213 (40001): deadlock"),
		errors.New("Error 1205 (HY000): lock wait timeout exceeded"),
	} {
		t.Run(conflict.Error(), func(t *testing.T) {
			storage := &retryingNativeDoltStorage{
				nativeDoltMemStorage: newNativeDoltMemStorage(),
				txErrors:             []error{conflict},
			}
			store := newNativeDoltStoreForTest(storage)
			created, err := store.Create(Bead{Title: "retry whole transaction"})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, map[string]string{"state": "drained"})
			if err != nil {
				t.Fatalf("CloseWithMetadataIfMatch: %v", err)
			}
			if storage.txCalls != 2 {
				t.Fatalf("RunInTransaction calls = %d, want 2", storage.txCalls)
			}
			if closed.Status != "closed" || closed.Metadata["state"] != "drained" {
				t.Fatalf("returned bead = %#v, want closed row from replay", closed)
			}
		})
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchRetryRereadsFence(t *testing.T) {
	storage := &retryingNativeDoltStorage{nativeDoltMemStorage: newNativeDoltMemStorage()}
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{Title: "retry stale fence"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	storage.txErrors = []error{errors.New("Error 1213 (40001): deadlock")}
	storage.afterConflict = func() {
		if err := storage.store.SetMetadata(created.ID, "intervening", "write"); err != nil {
			t.Fatalf("intervening write: %v", err)
		}
	}

	closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, map[string]string{"state": "drained"})
	if !IsPreconditionFailed(err) {
		t.Fatalf("CloseWithMetadataIfMatch error = %v, want precondition failure", err)
	}
	if !reflect.DeepEqual(closed, Bead{}) {
		t.Fatalf("failed replay returned %#v, want zero bead", closed)
	}
	if storage.txCalls != 2 {
		t.Fatalf("RunInTransaction calls = %d, want 2", storage.txCalls)
	}
	fresh, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fresh.Status != "open" || fresh.Metadata["state"] != "" || fresh.Metadata["intervening"] != "write" {
		t.Fatalf("replay fence result = %#v, want later open row without close", fresh)
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchDoesNotRetryAmbiguousFailure(t *testing.T) {
	sentinel := errors.New("connection reset by peer")
	storage := &retryingNativeDoltStorage{
		nativeDoltMemStorage: newNativeDoltMemStorage(),
		txErrors:             []error{sentinel},
	}
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{Title: "ambiguous close"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("CloseWithMetadataIfMatch error = %v, want %v", err, sentinel)
	}
	if !reflect.DeepEqual(closed, Bead{}) {
		t.Fatalf("ambiguous failure returned %#v, want zero bead", closed)
	}
	if storage.txCalls != 1 {
		t.Fatalf("RunInTransaction calls = %d, want 1", storage.txCalls)
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchReturnsZeroAfterRetryExhaustion(t *testing.T) {
	conflict := errors.New("Error 1213 (40001): deadlock")
	storage := &retryingNativeDoltStorage{
		nativeDoltMemStorage: newNativeDoltMemStorage(),
		txErrors:             []error{conflict, conflict, conflict},
	}
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{Title: "exhaust close retries"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, nil)
	if !errors.Is(err, conflict) {
		t.Fatalf("CloseWithMetadataIfMatch error = %v, want %v", err, conflict)
	}
	if !reflect.DeepEqual(closed, Bead{}) {
		t.Fatalf("exhausted retry returned %#v, want zero bead", closed)
	}
	if storage.txCalls != nativeWriteAttempts {
		t.Fatalf("RunInTransaction calls = %d, want %d", storage.txCalls, nativeWriteAttempts)
	}
}

// DeleteIfMatch runs its fence check and delete inside one transaction, so a
// serialization conflict must replay the WHOLE transaction — re-reading the row
// version each attempt — exactly like the close path. Retrying only the delete
// would fence against a RowVersion the rolled-back read already invalidated.
func TestNativeDoltStoreDeleteIfMatchRetriesWholeTransaction(t *testing.T) {
	for _, conflict := range []error{
		errors.New("Error 1213 (40001): deadlock"),
		errors.New("Error 1205 (HY000): lock wait timeout exceeded"),
	} {
		t.Run(conflict.Error(), func(t *testing.T) {
			storage := &retryingNativeDoltStorage{
				nativeDoltMemStorage: newNativeDoltMemStorage(),
				txErrors:             []error{conflict},
			}
			store := newNativeDoltStoreForTest(storage)
			created, err := store.Create(Bead{Title: "retry whole delete transaction"})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			if err := store.DeleteIfMatch(created.ID, created.Revision); err != nil {
				t.Fatalf("DeleteIfMatch: %v", err)
			}
			if storage.txCalls != 2 {
				t.Fatalf("RunInTransaction calls = %d, want 2", storage.txCalls)
			}
			if _, err := store.Get(created.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Get after replayed delete = %v, want ErrNotFound", err)
			}
		})
	}
}

// An ambiguous failure — one that may have committed — must NOT replay a delete,
// or a delete that already applied would run again against a moved fence. This
// pins the same transient/ambiguous split the close path draws.
func TestNativeDoltStoreDeleteIfMatchDoesNotRetryAmbiguousFailure(t *testing.T) {
	sentinel := errors.New("connection reset by peer")
	storage := &retryingNativeDoltStorage{
		nativeDoltMemStorage: newNativeDoltMemStorage(),
		txErrors:             []error{sentinel},
	}
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{Title: "ambiguous delete"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.DeleteIfMatch(created.ID, created.Revision); !errors.Is(err, sentinel) {
		t.Fatalf("DeleteIfMatch error = %v, want %v", err, sentinel)
	}
	if storage.txCalls != 1 {
		t.Fatalf("RunInTransaction calls = %d, want 1", storage.txCalls)
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchCommitsOneFencedTerminalState(t *testing.T) {
	storage := &commitCountingMemStorage{nativeDoltMemStorage: newNativeDoltMemStorage()}
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{
		Title:    "atomic terminal state",
		Metadata: map[string]string{"sibling": "preserved"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	commitsBeforeClose := storage.commits
	closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, map[string]string{
		"state":        "drained",
		"close_reason": "reconciler stop",
	})
	if err != nil {
		t.Fatalf("CloseWithMetadataIfMatch: %v", err)
	}
	if closed.Status != "closed" || closed.Metadata["state"] != "drained" || closed.Metadata["sibling"] != "preserved" {
		t.Fatalf("returned closed bead = %#v, want exact merged terminal row", closed)
	}
	if got := storage.commits - commitsBeforeClose; got != 1 {
		t.Fatalf("atomic metadata close issued %d commits, want exactly one", got)
	}

	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "closed" {
		t.Fatalf("status = %q, want closed", got.Status)
	}
	for key, want := range map[string]string{
		"sibling":      "preserved",
		"state":        "drained",
		"close_reason": "reconciler stop",
	} {
		if got.Metadata[key] != want {
			t.Fatalf("metadata[%q] = %q, want %q (metadata and close must share one commit)", key, got.Metadata[key], want)
		}
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchRejectsStaleRevisionWithoutMutation(t *testing.T) {
	storage := newNativeDoltMemStorage()
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{Title: "stale fenced close", Metadata: map[string]string{"sibling": "before"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.SetMetadata(created.ID, "intervening", "write"); err != nil {
		t.Fatalf("intervening SetMetadata: %v", err)
	}
	before, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get before stale close: %v", err)
	}
	beforeIssue, err := storage.GetIssue(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetIssue before stale close: %v", err)
	}
	beforeRawMetadata := bytes.Clone(beforeIssue.Metadata)

	closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, map[string]string{"state": "drained"})
	if !IsPreconditionFailed(err) {
		t.Fatalf("CloseWithMetadataIfMatch stale error = %v, want precondition failure", err)
	}
	if !reflect.DeepEqual(closed, Bead{}) {
		t.Fatalf("stale close result = %#v, want zero bead", closed)
	}
	after, getErr := store.Get(created.ID)
	if getErr != nil {
		t.Fatalf("Get after stale close: %v", getErr)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("stale close mutated bead:\n got: %#v\nwant: %#v", after, before)
	}
	afterIssue, err := storage.GetIssue(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetIssue after stale close: %v", err)
	}
	if !bytes.Equal(afterIssue.Metadata, beforeRawMetadata) {
		t.Fatalf("stale close changed raw metadata bytes:\n got: %q\nwant: %q", afterIssue.Metadata, beforeRawMetadata)
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchReturnsZeroOnMalformedMetadata(t *testing.T) {
	storage := &nativeDoltStorageSpy{
		getIssue: func(_ context.Context, id string) (*beadslib.Issue, error) {
			return &beadslib.Issue{
				ID:         id,
				Status:     beadslib.StatusOpen,
				IssueType:  beadslib.TypeTask,
				Metadata:   json.RawMessage(`{"broken":`),
				RowVersion: 17,
			}, nil
		},
	}
	store := newNativeDoltStoreForTest(storage)

	closed, err := store.CloseWithMetadataIfMatch("gc-malformed", 17, map[string]string{"state": "drained"})
	if err == nil {
		t.Fatal("CloseWithMetadataIfMatch error = nil, want malformed metadata error")
	}
	if !reflect.DeepEqual(closed, Bead{}) {
		t.Fatalf("malformed close result = %#v, want zero bead", closed)
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchRollsBackMetadataWhenCloseFails(t *testing.T) {
	sentinel := errors.New("injected close failure")
	storage := &nativeDoltFailingCloseStorage{
		nativeDoltMemStorage: newNativeDoltMemStorage(),
		closeIssue: func(context.Context, string, string, string, string) error {
			return sentinel
		},
	}
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{Title: "rollback fenced close", Metadata: map[string]string{"sibling": "before"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get before close: %v", err)
	}
	beforeIssue, err := storage.GetIssue(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetIssue before close: %v", err)
	}
	beforeRawMetadata := bytes.Clone(beforeIssue.Metadata)

	closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, map[string]string{"state": "drained"})
	if !reflect.DeepEqual(closed, Bead{}) {
		t.Fatalf("CloseWithMetadataIfMatch result = %#v, want zero bead on failure", closed)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("CloseWithMetadataIfMatch error = %v, want injected close failure", err)
	}
	after, getErr := store.Get(created.ID)
	if getErr != nil {
		t.Fatalf("Get after failed close: %v", getErr)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed close left partial mutation:\n got: %#v\nwant: %#v", after, before)
	}
	afterIssue, err := storage.GetIssue(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetIssue after failed close: %v", err)
	}
	if !bytes.Equal(afterIssue.Metadata, beforeRawMetadata) {
		t.Fatalf("failed close changed raw metadata bytes:\n got: %q\nwant: %q", afterIssue.Metadata, beforeRawMetadata)
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchRejectsUnclosedResult(t *testing.T) {
	storage := &nativeDoltFailingCloseStorage{
		nativeDoltMemStorage: newNativeDoltMemStorage(),
		closeIssue: func(context.Context, string, string, string, string) error {
			return nil
		},
	}
	store := newNativeDoltStoreForTest(storage)
	created, err := store.Create(Bead{Title: "close postcondition"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	closed, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, map[string]string{"state": "drained"})
	if err == nil {
		t.Fatal("CloseWithMetadataIfMatch error = nil, want unclosed-result refusal")
	}
	if !reflect.DeepEqual(closed, Bead{}) {
		t.Fatalf("unclosed transaction returned %#v, want zero bead", closed)
	}
	fresh, getErr := store.Get(created.ID)
	if getErr != nil {
		t.Fatalf("Get after refused close: %v", getErr)
	}
	if fresh.Status != "open" || fresh.Metadata["state"] != "" {
		t.Fatalf("refused close left a partial mutation: %#v", fresh)
	}
}

func TestNativeDoltStoreCloseWithMetadataIfMatchHasOneSameRevisionWinner(t *testing.T) {
	store := newNativeDoltStoreForTest(newNativeDoltMemStorage())
	created, err := store.Create(Bead{Title: "racing fenced close"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	type closeResult struct {
		bead Bead
		err  error
	}
	results := make(chan closeResult, 2)
	for _, value := range []string{"first", "second"} {
		value := value
		go func() {
			bead, err := store.CloseWithMetadataIfMatch(created.ID, created.Revision, map[string]string{"winner": value})
			results <- closeResult{bead: bead, err: err}
		}()
	}
	var wins, losses int
	for range 2 {
		result := <-results
		err := result.err
		switch {
		case err == nil:
			wins++
			if result.bead.Status != "closed" || result.bead.Metadata["winner"] == "" {
				t.Fatalf("winning result = %#v, want exact closed winner", result.bead)
			}
		case IsPreconditionFailed(err):
			losses++
			if !reflect.DeepEqual(result.bead, Bead{}) {
				t.Fatalf("losing result = %#v, want zero bead", result.bead)
			}
		default:
			t.Fatalf("same-revision close error = %v, want nil or precondition failure", err)
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("same-revision results wins=%d losses=%d, want exactly one of each", wins, losses)
	}
	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get after race: %v", err)
	}
	if got.Status != "closed" || (got.Metadata["winner"] != "first" && got.Metadata["winner"] != "second") {
		t.Fatalf("winner state = %#v, want one complete terminal result", got)
	}
}

// UpdateIssueChecked satisfies the CAS arm of the library's Storage interface
// so the spy remains a valid backing under beads_rowlock.
//
// The hook fields this once dispatched through were dropped: no test in this
// package sets them (the CAS behaviour under test is driven through
// nativeDoltMemStorage and retryingNativeDoltStorage), and keeping them would
// leave two unused func fields on a struct the default build also compiles.
func (s *nativeDoltStorageSpy) UpdateIssueChecked(_ context.Context, _ string, _ map[string]interface{}, _ string, _ beadslib.UpdateIssueOptions) error {
	return nil
}

// CloseIssueChecked satisfies the CAS arm of the library's Storage interface.
// See UpdateIssueChecked for why it carries no hook.
func (s *nativeDoltStorageSpy) CloseIssueChecked(_ context.Context, _ string, _ string, _ beadslib.CloseIssueOptions) (beadslib.CloseIssueResult, error) {
	return beadslib.CloseIssueResult{}, nil
}

func (s *nativeDoltMemStorage) UpdateIssueChecked(
	_ context.Context,
	id string,
	updates map[string]interface{},
	_ string,
	opts beadslib.UpdateIssueOptions,
) error {
	updateOpts, err := nativeDoltMemUpdateOpts(updates)
	if err != nil {
		return err
	}
	if opts.ExpectedVersion == nil {
		return s.store.Update(id, updateOpts)
	}
	return nativeDoltMemCheckedError(s.store.UpdateIfMatch(id, *opts.ExpectedVersion, updateOpts))
}

func (s *nativeDoltMemStorage) CloseIssueChecked(
	_ context.Context,
	id string,
	_ string,
	opts beadslib.CloseIssueOptions,
) (beadslib.CloseIssueResult, error) {
	current, err := s.store.Get(id)
	if err != nil {
		return beadslib.CloseIssueResult{}, err
	}
	if opts.ExpectedVersion == nil {
		err = s.store.Close(id)
	} else {
		err = s.store.CloseIfMatch(id, *opts.ExpectedVersion)
	}
	if err != nil {
		return beadslib.CloseIssueResult{}, nativeDoltMemCheckedError(err)
	}
	return beadslib.CloseIssueResult{Unchanged: current.Status == "closed"}, nil
}

func nativeDoltMemCheckedError(err error) error {
	if !IsPreconditionFailed(err) {
		return err
	}
	return fmt.Errorf("%w: %w", beadslib.ErrVersionMismatch, err)
}

type nativeDoltFailingCloseStorage struct {
	*nativeDoltMemStorage
	closeIssue func(context.Context, string, string, string, string) error
}

type retryingNativeDoltStorage struct {
	*nativeDoltMemStorage
	txCalls       int
	txErrors      []error
	afterConflict func()
}

func (s *retryingNativeDoltStorage) RunInTransaction(_ context.Context, _ string, fn func(beadslib.Transaction) error) error {
	s.txCalls++
	var txErr error
	if len(s.txErrors) > 0 {
		txErr = s.txErrors[0]
		s.txErrors = s.txErrors[1:]
	}
	err := runNativeDoltMemStorageTransactionForTest(s.nativeDoltMemStorage, func() error {
		if err := fn(nativeDoltTransactionForTest{storage: s}); err != nil {
			return err
		}
		return txErr
	})
	if txErr != nil && s.afterConflict != nil {
		s.afterConflict()
	}
	return err
}

func (s *nativeDoltFailingCloseStorage) RunInTransaction(_ context.Context, _ string, fn func(beadslib.Transaction) error) error {
	return runNativeDoltMemStorageTransactionForTest(s.nativeDoltMemStorage, func() error {
		return fn(nativeDoltTransactionForTest{storage: s})
	})
}

func (s *nativeDoltFailingCloseStorage) CloseIssue(ctx context.Context, id, reason, actor, session string) error {
	if s.closeIssue != nil {
		return s.closeIssue(ctx, id, reason, actor, session)
	}
	return s.nativeDoltMemStorage.CloseIssue(ctx, id, reason, actor, session)
}
