// v2_fs_protected_darwin.go — macOS TCC-protected-folder guard for the folder
// browser (v0.1.8 bug: normal wizard use fired a permission prompt for every
// protected home folder).
//
// On macOS, Desktop / Documents / Downloads / Library / … live directly under
// $HOME and are gated by TCC (Transparency, Consent & Control). READING INTO
// one of them — opendir, stat a path inside, or FOLLOWING a symlink into it —
// makes the OS prompt "grafel wants to access your Documents". When iCloud's
// "Desktop & Documents Folders" is on, ~/Desktop and ~/Documents are SYMLINKS
// into ~/Library/Mobile Documents, so the folder browser's symlink-Stat probe
// (and the shortcut existence probe) followed them and tripped the prompt on
// the very first, default home listing — before the user navigated anywhere.
//
// The fix: list these folders by NAME from the home dirents (which is free) but
// never probe INTO them. The one legitimate prompt happens only when the user
// explicitly clicks into a protected folder. This file is darwin-only; the
// !darwin stub in v2_fs_protected_other.go makes every predicate false so
// Linux/Windows behaviour is unchanged.

//go:build darwin

package dashboard

import (
	"path/filepath"
	"strings"
)

// macOSProtectedHomeDirs are the basenames of the TCC-protected folders that
// sit directly under $HOME on macOS. "Library" covers ~/Library/Mobile
// Documents (iCloud Drive).
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

// protectedHomeChild reports whether the dirent `name` under `parent` is a
// macOS TCC-protected folder we must not Stat-follow. It fires only when parent
// IS the home directory, so a folder merely named "Documents" elsewhere on disk
// is still probed normally.
func protectedHomeChild(home, parent, name string) bool {
	if home == "" || parent != home {
		return false
	}
	return macOSProtectedHomeDirs[name]
}

// protectedProbePath reports whether Stat-ing `path` would read a macOS
// TCC-protected home folder or anything inside one (e.g. ~/Documents/Projects).
// Used to skip probing shortcut candidates without an explicit navigation.
func protectedProbePath(home, path string) bool {
	if home == "" {
		return false
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	first := rel
	if i := strings.IndexRune(rel, filepath.Separator); i >= 0 {
		first = rel[:i]
	}
	return macOSProtectedHomeDirs[first]
}
