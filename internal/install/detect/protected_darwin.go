// protected_darwin.go — macOS TCC-protected-folder guard for the sibling-repo
// scan (v0.1.8 bug: classifying a repo that lives directly under $HOME read the
// home dir and probed each protected sibling for .git, firing permission
// prompts).
//
// ClassifyPath scans the PARENT of the selected repo for "sibling git repos".
// When that parent is $HOME (a repo cloned straight into ~) — or a TCC-protected
// folder like ~/Documents — enumerating it and Stat-ing each child's .git reads
// INTO Desktop/Documents/Downloads and trips a macOS permission prompt during
// normal wizard use. A repo whose parent is the home dir has no meaningful
// "siblings" to offer anyway, so we simply skip the scan there.
//
// darwin-only; the !darwin stub keeps Linux/Windows behaviour unchanged.

//go:build darwin

package detect

import (
	"os"
	"path/filepath"
	"strings"
)

// macOSProtectedHomeDirs are the basenames of TCC-protected folders directly
// under $HOME on macOS.
var macOSProtectedHomeDirs = map[string]bool{
	"Desktop":   true,
	"Documents": true,
	"Downloads": true,
	"Library":   true,
	"Movies":    true,
	"Music":     true,
	"Pictures":  true,
	"Public":    true,
}

// isProtectedScanParent reports whether enumerating `parent` for sibling repos
// would read the home dir itself or a macOS TCC-protected folder (or anything
// inside one). ClassifyPath skips the sibling scan when this is true.
func isProtectedScanParent(parent string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	if filepath.Clean(parent) == filepath.Clean(home) {
		return true
	}
	rel, err := filepath.Rel(home, parent)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	first := rel
	if i := strings.IndexRune(rel, filepath.Separator); i >= 0 {
		first = rel[:i]
	}
	return macOSProtectedHomeDirs[first]
}
