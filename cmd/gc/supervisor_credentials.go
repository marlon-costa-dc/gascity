package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/processenv"
	"github.com/gastownhall/gascity/internal/supervisor"
	"golang.org/x/sys/unix"
)

const (
	supervisorCredentialDirEnv       = "CREDENTIALS_DIRECTORY"
	maxSupervisorCredentialBytes int = 64 * 1024
)

type supervisorLoadedCredential struct {
	id        string
	value     string
	providers map[string]bool
}

var supervisorCredentialState = struct {
	sync.RWMutex
	activated bool
	byEnv     map[string]supervisorLoadedCredential
}{}

func activateSupervisorCredentials(creds supervisor.CredentialsConfig) error {
	if err := clearInheritedSupervisorCredentialEnv(creds); err != nil {
		return err
	}
	if len(creds.Encrypted) == 0 {
		supervisorCredentialState.Lock()
		supervisorCredentialState.activated = true
		supervisorCredentialState.byEnv = nil
		supervisorCredentialState.Unlock()
		return nil
	}
	rawDir := os.Getenv(supervisorCredentialDirEnv)
	dir := strings.TrimSpace(rawDir)
	if dir == "" {
		return fmt.Errorf("encrypted credentials are configured but %s is not set", supervisorCredentialDirEnv)
	}
	if rawDir != dir || !filepath.IsAbs(dir) || strings.ContainsAny(dir, "\x00\r\n") {
		return fmt.Errorf("%s must be an absolute path without boundary whitespace or control characters", supervisorCredentialDirEnv)
	}

	dirfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("opening systemd credentials directory: %w", err)
	}
	defer unix.Close(dirfd) //nolint:errcheck // best-effort close after credential activation

	loaded := make(map[string]supervisorLoadedCredential, len(creds.Encrypted))
	for _, cred := range creds.Encrypted {
		data, err := readSupervisorSystemdCredential(dirfd, cred)
		if err != nil {
			return err
		}
		providers := make(map[string]bool, len(cred.Providers))
		for _, provider := range cred.Providers {
			providers[provider] = true
		}
		loaded[cred.Env] = supervisorLoadedCredential{
			id:        cred.ID,
			value:     string(data),
			providers: providers,
		}
	}

	supervisorCredentialState.Lock()
	supervisorCredentialState.activated = true
	supervisorCredentialState.byEnv = loaded
	supervisorCredentialState.Unlock()
	return nil
}

func readSupervisorSystemdCredential(dirfd int, cred supervisor.EncryptedCredential) ([]byte, error) {
	fd, err := unix.Openat(dirfd, cred.ID, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("opening systemd credential id %q for env %q: %w", cred.ID, cred.Env, err)
	}
	defer func() {
		if fd >= 0 {
			_ = unix.Close(fd)
		}
	}()

	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return nil, fmt.Errorf("statting systemd credential id %q for env %q: %w", cred.ID, cred.Env, err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, fmt.Errorf("systemd credential id %q for env %q is not a regular file", cred.ID, cred.Env)
	}

	file := os.NewFile(uintptr(fd), "systemd credential "+cred.ID)
	if file == nil {
		return nil, fmt.Errorf("opening systemd credential id %q for env %q: invalid file descriptor", cred.ID, cred.Env)
	}
	fd = -1
	defer file.Close() //nolint:errcheck // best-effort close after bounded read

	data, err := io.ReadAll(io.LimitReader(file, int64(maxSupervisorCredentialBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("reading systemd credential id %q for env %q: %w", cred.ID, cred.Env, err)
	}
	if len(data) > maxSupervisorCredentialBytes {
		return nil, fmt.Errorf("systemd credential id %q for env %q exceeds %d bytes", cred.ID, cred.Env, maxSupervisorCredentialBytes)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("systemd credential id %q for env %q is empty", cred.ID, cred.Env)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("systemd credential id %q for env %q contains a NUL byte", cred.ID, cred.Env)
	}
	if !bytes.Equal(bytes.TrimSpace(data), data) {
		return nil, fmt.Errorf("systemd credential id %q for env %q has boundary whitespace", cred.ID, cred.Env)
	}
	return data, nil
}

func clearInheritedSupervisorCredentialEnv(creds supervisor.CredentialsConfig) error {
	configured := make(map[string]bool, len(creds.Encrypted))
	for _, cred := range creds.Encrypted {
		configured[cred.Env] = true
	}
	var firstErr error
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if configured[key] || supervisorServiceSensitiveEnvKeys[key] || processenv.IsProviderCredentialEnv(key) {
			if err := os.Unsetenv(key); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("clearing inherited credential env %q: %w", key, err)
			}
		}
	}
	return firstErr
}

func supervisorCredentialsActivated() bool {
	supervisorCredentialState.RLock()
	defer supervisorCredentialState.RUnlock()
	return supervisorCredentialState.activated
}

func supervisorCredentialEnvNameConfigured(key string) bool {
	supervisorCredentialState.RLock()
	defer supervisorCredentialState.RUnlock()
	_, ok := supervisorCredentialState.byEnv[key]
	return ok
}

func supervisorCredentialEnvForResolvedProvider(resolved *config.ResolvedProvider) map[string]string {
	selectors := supervisorCredentialProviderSelectors(resolved)
	if len(selectors) == 0 {
		return nil
	}
	supervisorCredentialState.RLock()
	defer supervisorCredentialState.RUnlock()
	if len(supervisorCredentialState.byEnv) == 0 {
		return nil
	}
	out := make(map[string]string)
	for env, cred := range supervisorCredentialState.byEnv {
		if supervisorLoadedCredentialMatchesProvider(cred, selectors) {
			out[env] = cred.value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func supervisorCredentialValueForResolvedProvider(resolved *config.ResolvedProvider, key string) (string, bool) {
	selectors := supervisorCredentialProviderSelectors(resolved)
	if len(selectors) == 0 {
		return "", false
	}
	supervisorCredentialState.RLock()
	defer supervisorCredentialState.RUnlock()
	cred, ok := supervisorCredentialState.byEnv[key]
	if !ok || !supervisorLoadedCredentialMatchesProvider(cred, selectors) {
		return "", false
	}
	return cred.value, true
}

func supervisorLoadedCredentialMatchesProvider(cred supervisorLoadedCredential, selectors map[string]bool) bool {
	for provider := range cred.providers {
		if selectors[provider] {
			return true
		}
	}
	return false
}

func supervisorCredentialProviderSelectors(resolved *config.ResolvedProvider) map[string]bool {
	if resolved == nil {
		return nil
	}
	selectors := make(map[string]bool)
	addProviderSelector(selectors, resolved.Name)
	addProviderSelector(selectors, resolved.BuiltinAncestor)
	for _, hop := range resolved.Chain {
		addProviderSelector(selectors, hop.Name)
		if hop.Kind != "" && hop.Name != "" {
			addProviderSelector(selectors, hop.Kind+":"+hop.Name)
		}
	}
	return selectors
}

func addProviderSelector(selectors map[string]bool, provider string) {
	provider = strings.TrimSpace(provider)
	if provider != "" {
		selectors[provider] = true
	}
}
