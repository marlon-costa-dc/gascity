package main

import (
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/fsys"
	"github.com/gastownhall/gascity/internal/git"
)

// cityGitignoreEntries are the paths that gc init writes into .gitignore.
var cityGitignoreEntries = []string{".gc/", ".beads/*", "!.beads/identity.toml", "hooks/"}

// rigGitignoreEntries are the paths that gc rig add writes into
// the rig-scoped .gitignore.
var rigGitignoreEntries = []string{".beads/*", "!.beads/identity.toml"}

func usesCanonicalBeadsEntries(entries []string) bool {
	for _, entry := range entries {
		if entry == ".beads/*" {
			return true
		}
	}
	return false
}

func isLegacyWholeBeadsIgnore(line string) bool {
	switch strings.TrimSpace(line) {
	case ".beads", ".beads/", "/.beads", "/.beads/":
		return true
	default:
		return false
	}
}

func isObsoleteBeadsRuntimeUnignore(line string) bool {
	switch strings.TrimSpace(line) {
	case "!.beads/config.yaml", "!/.beads/config.yaml", "!**/.beads/config.yaml",
		"!.beads/metadata.json", "!/.beads/metadata.json", "!**/.beads/metadata.json":
		return true
	default:
		return false
	}
}

// unignoredRuntimePath returns the path, relative to the .gitignore's
// directory, that an obsolete runtime un-ignore line names:
// "!.beads/config.yaml", "!/.beads/config.yaml" and
// "!**/.beads/config.yaml" all name ".beads/config.yaml".
func unignoredRuntimePath(line string) string {
	path := strings.TrimPrefix(strings.TrimSpace(line), "!")
	path = strings.TrimPrefix(path, "**/")
	return strings.TrimPrefix(path, "/")
}

// ensureGitignoreEntries is an idempotent append helper for .gitignore files.
// It reads the existing .gitignore at dir/.gitignore (if any), skips entries
// that are already present, and appends a "# Gas City" section for new ones.
// Preserves all existing content including user-added entries.
//
// An un-ignore line for a beads runtime file is dropped only while git does
// not track that file: it was written for runtime files left untracked, which
// must stay ignored so `git clean` keeps them. A repository that commits its
// .beads/config.yaml or metadata.json keeps the line, so registering it never
// rewrites its tracked .gitignore.
func ensureGitignoreEntries(fs fsys.FS, dir string, entries []string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	existing, err := fs.ReadFile(gitignorePath)
	if err != nil {
		// File doesn't exist — start fresh.
		existing = nil
	}

	upgradeCanonicalBeads := usesCanonicalBeadsEntries(entries)

	repo := git.New(dir)
	repoKnown, isRepo := false, false
	tracksUnignoredPath := func(line string) (bool, error) {
		if !repoKnown {
			repoKnown, isRepo = true, repo.IsRepo()
		}
		if !isRepo {
			return false, nil
		}
		return repo.TracksPath(unignoredRuntimePath(line))
	}

	existingLines := strings.Split(string(existing), "\n")
	cleanedLines := make([]string, 0, len(existingLines))
	presentLines := make(map[string]bool)
	removedLegacyBeadsIgnore := false
	for _, line := range existingLines {
		trimmed := strings.TrimSpace(line)
		if upgradeCanonicalBeads && isLegacyWholeBeadsIgnore(trimmed) {
			removedLegacyBeadsIgnore = true
			continue
		}
		if upgradeCanonicalBeads && isObsoleteBeadsRuntimeUnignore(trimmed) {
			tracked, err := tracksUnignoredPath(trimmed)
			if err != nil {
				return err
			}
			if !tracked {
				removedLegacyBeadsIgnore = true
				continue
			}
		}
		cleanedLines = append(cleanedLines, line)
		presentLines[trimmed] = true
	}
	cleanedExisting := strings.Join(cleanedLines, "\n")

	// Collect entries that need to be added.
	var newEntries []string
	for _, entry := range entries {
		if !presentLines[entry] {
			newEntries = append(newEntries, entry)
		}
	}

	if len(newEntries) == 0 {
		if !removedLegacyBeadsIgnore {
			return nil // nothing to add
		}
		return fs.WriteFile(gitignorePath, []byte(cleanedExisting), 0o644)
	}

	// Build the new content: existing + separator + section header + entries.
	var b strings.Builder
	if len(cleanedExisting) > 0 {
		b.WriteString(cleanedExisting)
		// Ensure there's a blank line before our section.
		if !strings.HasSuffix(cleanedExisting, "\n") {
			b.WriteByte('\n')
		}
		if !strings.HasSuffix(cleanedExisting, "\n\n") {
			b.WriteByte('\n')
		}
	}
	b.WriteString("# Gas City\n")
	for _, entry := range newEntries {
		b.WriteString(entry)
		b.WriteByte('\n')
	}

	return fs.WriteFile(gitignorePath, []byte(b.String()), 0o644)
}
