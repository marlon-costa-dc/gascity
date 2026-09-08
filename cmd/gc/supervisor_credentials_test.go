package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/supervisor"
)

func resetSupervisorCredentialStateForTest(t *testing.T) {
	t.Helper()
	supervisorCredentialState.Lock()
	oldActivated := supervisorCredentialState.activated
	oldByEnv := supervisorCredentialState.byEnv
	supervisorCredentialState.activated = false
	supervisorCredentialState.byEnv = nil
	supervisorCredentialState.Unlock()
	t.Cleanup(func() {
		supervisorCredentialState.Lock()
		supervisorCredentialState.activated = oldActivated
		supervisorCredentialState.byEnv = oldByEnv
		supervisorCredentialState.Unlock()
	})
}

func TestActivateSupervisorCredentialsClearsInheritedAndInjectsOnlySelectedProvider(t *testing.T) {
	resetSupervisorCredentialStateForTest(t)
	dir := t.TempDir()
	const credentialID = "openai-token"
	const credentialValue = "test-openai-token-value"
	if err := os.WriteFile(filepath.Join(dir, credentialID), []byte(credentialValue), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(supervisorCredentialDirEnv, dir)
	t.Setenv("OPENAI_API_KEY", "ambient-openai-token")
	t.Setenv("ANTHROPIC_API_KEY", "ambient-anthropic-token")

	creds := supervisor.CredentialsConfig{Encrypted: []supervisor.EncryptedCredential{{
		ID:        credentialID,
		Path:      "/non-secret/provisioned/openai-token.cred",
		Env:       "OPENAI_API_KEY",
		Providers: []string{"codex"},
	}}}
	if err := activateSupervisorCredentials(creds); err != nil {
		t.Fatalf("activateSupervisorCredentials: %v", err)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != "" {
		t.Fatalf("OPENAI_API_KEY remained in supervisor env after activation")
	}
	if got := os.Getenv("ANTHROPIC_API_KEY"); got != "" {
		t.Fatalf("ANTHROPIC_API_KEY remained in supervisor env after activation")
	}

	codex := &config.ResolvedProvider{Name: "codex"}
	claude := &config.ResolvedProvider{Name: "claude"}
	env := supervisorCredentialEnvForResolvedProvider(codex)
	if got := env["OPENAI_API_KEY"]; got != credentialValue {
		t.Fatalf("codex credential mapping missing selected env value")
	}
	if got := supervisorCredentialEnvForResolvedProvider(claude); got["OPENAI_API_KEY"] != "" {
		t.Fatalf("credential mapping leaked to unselected provider")
	}
	if got := providerProcessPassthroughEnvForResolvedProvider(codex)["OPENAI_API_KEY"]; got != credentialValue {
		t.Fatalf("provider passthrough did not inject selected credential")
	}
	if got := providerProcessPassthroughEnvForResolvedProvider(claude)["OPENAI_API_KEY"]; got != "" {
		t.Fatalf("provider passthrough leaked credential to unselected provider")
	}

	t.Setenv("OPENAI_API_KEY", "late-ambient-openai-token")
	expanded, err := expandUpstreamEnvValueForResolvedProvider("$OPENAI_API_KEY", codex)
	if err != nil {
		t.Fatalf("selected upstream env expansion failed: %v", err)
	}
	if expanded != credentialValue {
		t.Fatalf("selected upstream env expansion did not use encrypted credential")
	}
	if got := expandSessionEnvValueForResolvedProvider("$OPENAI_API_KEY", claude); got != "" {
		t.Fatalf("session env expansion fell back to ambient provider credential")
	}
	_, err = expandUpstreamEnvValueForResolvedProvider("$OPENAI_API_KEY", claude)
	if err == nil {
		t.Fatal("upstream env expansion for unselected provider fell back to ambient credential")
	}
	if strings.Contains(err.Error(), "late-ambient-openai-token") {
		t.Fatalf("unselected-provider error leaked ambient credential: %v", err)
	}
}

func TestActivateSupervisorCredentialsFailsWithoutCredentialsDirectory(t *testing.T) {
	resetSupervisorCredentialStateForTest(t)
	t.Setenv(supervisorCredentialDirEnv, "")
	t.Setenv("OPENAI_API_KEY", "ambient-openai-token")

	err := activateSupervisorCredentials(supervisor.CredentialsConfig{Encrypted: []supervisor.EncryptedCredential{{
		ID:        "openai-token",
		Path:      "/non-secret/provisioned/openai-token.cred",
		Env:       "OPENAI_API_KEY",
		Providers: []string{"codex"},
	}}})
	if err == nil {
		t.Fatal("configured encrypted credential succeeded without CREDENTIALS_DIRECTORY")
	}
	if !strings.Contains(err.Error(), supervisorCredentialDirEnv) {
		t.Fatalf("missing-directory error does not name %s: %v", supervisorCredentialDirEnv, err)
	}
	if strings.Contains(err.Error(), "ambient-openai-token") {
		t.Fatalf("missing-directory error leaked ambient credential: %v", err)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != "" {
		t.Fatalf("OPENAI_API_KEY remained in supervisor env after failed activation")
	}
}

func TestActivateSupervisorCredentialsRejectsSymlinkAndBoundaryWhitespaceWithoutValueLeak(t *testing.T) {
	resetSupervisorCredentialStateForTest(t)
	dir := t.TempDir()
	const credentialID = "openai-token"
	const credentialValue = "credential-value-that-must-not-leak"
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(credentialValue), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, credentialID)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Setenv(supervisorCredentialDirEnv, dir)

	creds := supervisor.CredentialsConfig{Encrypted: []supervisor.EncryptedCredential{{
		ID:        credentialID,
		Path:      "/non-secret/provisioned/openai-token.cred",
		Env:       "OPENAI_API_KEY",
		Providers: []string{"codex"},
	}}}
	err := activateSupervisorCredentials(creds)
	if err == nil {
		t.Fatal("symlinked credential was accepted")
	}
	if !strings.Contains(err.Error(), credentialID) || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("symlink error does not name credential id and env: %v", err)
	}
	if strings.Contains(err.Error(), credentialValue) {
		t.Fatalf("symlink error leaked credential value: %v", err)
	}

	resetSupervisorCredentialStateForTest(t)
	dir = t.TempDir()
	t.Setenv(supervisorCredentialDirEnv, dir)
	if err := os.WriteFile(filepath.Join(dir, credentialID), []byte(credentialValue+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = activateSupervisorCredentials(creds)
	if err == nil {
		t.Fatal("credential with boundary whitespace was accepted")
	}
	if !strings.Contains(err.Error(), "boundary whitespace") {
		t.Fatalf("boundary-whitespace error not causal: %v", err)
	}
	if strings.Contains(err.Error(), credentialValue) {
		t.Fatalf("boundary-whitespace error leaked credential value: %v", err)
	}
}

func TestSupervisorSystemdCredentialPurgeNamesCollectsLegacyAndManagerCredentialEnv(t *testing.T) {
	oldOutput := supervisorSystemctlOutput
	supervisorSystemctlOutput = func(args ...string) ([]byte, error) {
		if strings.Join(args, " ") != "--user show-environment" {
			t.Fatalf("unexpected systemctl output call: %v", args)
		}
		return []byte("OPENROUTER_API_KEY=redacted\nUNRELATED=value\n"), nil
	}
	t.Cleanup(func() { supervisorSystemctlOutput = oldOutput })
	t.Setenv("ANTHROPIC_API_KEY", "ambient-anthropic-token")

	keys := supervisorSystemdCredentialPurgeNames(&supervisorServiceData{
		Credentials: []supervisorServiceCredential{{ID: "openai-token", Env: "OPENAI_API_KEY", Path: "/non-secret/openai-token.cred"}},
	}, []byte("PassEnvironment=GEMINI_API_KEY INVALID-NAME? GC_DOLT_PASSWORD\n"))
	for _, want := range []string{"ANTHROPIC_API_KEY", "GC_DOLT_PASSWORD", "GEMINI_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		if !slicesContainsString(keys, want) {
			t.Fatalf("purge names %v missing %s", keys, want)
		}
	}
	if slicesContainsString(keys, "UNRELATED") || slicesContainsString(keys, "INVALID-NAME?") {
		t.Fatalf("purge names included unrelated or invalid names: %v", keys)
	}
}

func slicesContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
