package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// clearManagedSessionHookEnv blanks every key selectManagedSessionHook reads so
// a test starts from a deterministic "no managed identity" baseline regardless
// of what the parent shell exported.
func clearManagedSessionHookEnv(t *testing.T) {
	t.Helper()
	for _, key := range managedSessionHookEnvKeys {
		t.Setenv(key, "")
	}
}

func TestSelectManagedSessionHook(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		want     managedSessionHookSelection
		wantMiss string
	}{
		{
			name: "absent when no GC identity is present",
			env:  map[string]string{},
			want: managedSessionHookAbsent,
		},
		{
			name: "absent when only unrelated keys are present",
			env:  map[string]string{"HOME": "/tmp", "PATH": "/bin"},
			want: managedSessionHookAbsent,
		},
		{
			name: "partial when only the session id is present",
			env:  map[string]string{"GC_SESSION_ID": "s1"},
			want: managedSessionHookPartial,
			wantMiss: strings.Join([]string{
				"GC_SESSION_NAME",
				"one of GC_ALIAS/GC_AGENT",
				"one of GC_CITY/GC_CITY_PATH/GC_CITY_ROOT",
			}, ", "),
		},
		{
			name: "selected with full identity",
			env: map[string]string{
				"GC_SESSION_ID":   "s1",
				"GC_SESSION_NAME": "n1",
				"GC_ALIAS":        "a1",
				"GC_CITY":         "c1",
			},
			want: managedSessionHookSelected,
		},
		{
			name: "selected with agent and city path alternates",
			env: map[string]string{
				"GC_SESSION_ID":   "s1",
				"GC_SESSION_NAME": "n1",
				"GC_AGENT":        "ag",
				"GC_CITY_PATH":    "/city",
			},
			want: managedSessionHookSelected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) string { return tt.env[key] }
			selection, missing := selectManagedSessionHook(lookup)
			if selection != tt.want {
				t.Fatalf("selection = %v, want %v", selection, tt.want)
			}
			if got := strings.Join(missing, ", "); got != tt.wantMiss {
				t.Fatalf("missing = %q, want %q", got, tt.wantMiss)
			}
		})
	}
}

func TestCmdHookRunWhenManagedSessionAbsentSkipsChild(t *testing.T) {
	clearGCEnv(t)
	clearManagedSessionHookEnv(t)
	setHookRunExecutableForTest(t)

	var stdout, stderr bytes.Buffer
	code := cmdHookRun(
		[]string{"handoff", "--auto", "context cycle"},
		hookRunOptions{Timeout: 5 * time.Second, TimeoutExitCode: 0, WhenManagedSession: true},
		strings.NewReader("{}"),
		&stdout, &stderr,
	)

	if code != 0 {
		t.Fatalf("cmdHookRun = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "NOT SELECTED: managed session context absent") {
		t.Fatalf("stderr = %q, want NOT SELECTED notice", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestCmdHookRunWhenManagedSessionPartialFailsBeforeChild(t *testing.T) {
	clearGCEnv(t)
	clearManagedSessionHookEnv(t)
	t.Setenv("GC_SESSION_ID", "s1")
	setHookRunExecutableForTest(t)

	var stdout, stderr bytes.Buffer
	code := cmdHookRun(
		[]string{"handoff", "--auto", "context cycle"},
		hookRunOptions{Timeout: 5 * time.Second, TimeoutExitCode: 0, WhenManagedSession: true},
		strings.NewReader("{}"),
		&stdout, &stderr,
	)

	if code != 1 {
		t.Fatalf("cmdHookRun = %d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "partial managed session context: missing") {
		t.Fatalf("stderr = %q, want partial managed session context notice", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestCmdHookRunWhenManagedSessionSelectedRunsChild(t *testing.T) {
	clearGCEnv(t)
	clearManagedSessionHookEnv(t)
	for key, value := range map[string]string{
		"GC_SESSION_ID":   "s1",
		"GC_SESSION_NAME": "n1",
		"GC_ALIAS":        "a1",
		"GC_CITY":         "c1",
	} {
		t.Setenv(key, value)
	}
	previous := hookRunExecutable
	hookRunExecutable = func() (string, error) { return exec.LookPath("true") }
	t.Cleanup(func() { hookRunExecutable = previous })

	var stdout, stderr bytes.Buffer
	code := cmdHookRun(
		[]string{"handoff", "--auto", "context cycle"},
		hookRunOptions{Timeout: 5 * time.Second, TimeoutExitCode: 0, WhenManagedSession: true},
		strings.NewReader("{}"),
		&stdout, &stderr,
	)

	if code != 0 {
		t.Fatalf("cmdHookRun = %d, want 0 (child true exited 0); stderr=%q", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "NOT SELECTED") || strings.Contains(stderr.String(), "partial managed session context") {
		t.Fatalf("stderr = %q, want no managed-session gating notice", stderr.String())
	}
}
