//go:build integration

package tmux

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A suspended city has no server. Exercise the public provider through real
// tmux commands with the public cache duration set to expire between reads.
func TestProvider_AbsentServerRemainsAnEmptyFleet(t *testing.T) {
	t.Setenv("GC_TMUX_CACHE_TTL", "1ns")
	cfg := DefaultConfig()
	cfg.SocketName = fmt.Sprintf("gctest-absent-%d-%d", os.Getpid(), time.Now().UnixNano())
	provider := NewProviderWithConfig(cfg)
	var logs bytes.Buffer
	restore := captureLog(&logs)
	defer restore()
	for range 2 {
		for range 5 {
			if provider.IsRunning("never-started") {
				t.Fatal("absent server reported a running session")
			}
		}
	}
	if logs.Len() != 0 {
		t.Fatalf("absent server reported observation failures: %s", logs.String())
	}
}

// A listening socket that does not answer the tmux protocol is not an empty
// fleet. Use a real listener and the public provider, without a canned reply.
func TestProvider_BlockedSocket(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	cfg := DefaultConfig()
	cfg.SocketName = "blocked"
	path := namedSocketPath(cfg.SocketName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})
	provider := NewProviderWithConfig(cfg)
	var logs bytes.Buffer
	restore := captureLog(&logs)
	defer restore()
	if provider.IsRunning("unknown") {
		t.Fatal("unresponsive socket reported a running session")
	}
	if !strings.Contains(logs.String(), "refresh failed") {
		t.Fatalf("unresponsive socket did not report its observation failure: %s", logs.String())
	}
	if err := listener.SetDeadline(time.Now().Add(fetchTimeout)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := conn.SetReadDeadline(time.Now().Add(fetchTimeout)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, conn); err != nil {
		t.Fatalf("canceled tmux client kept its socket connection open: %v", err)
	}
}
