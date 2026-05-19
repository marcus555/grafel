// Package watch implements the daemon's reactive fsnotify-driven file
// watcher (Phase B). For each registered repo the watcher subscribes
// recursively to the working tree, applies a coarse skip list to keep
// the watch count bounded, and emits a single coalesced event after a
// per-repo debounce window has elapsed.
//
// The package is intentionally agnostic to what happens when a repo
// settles — it just calls back into a caller-supplied function. The
// scheduler in internal/daemon/sched is the natural consumer.
package watch

import (
	"path/filepath"
	"strings"
)

// SkipDirs is the static list of directory basenames the watcher never
// recurses into. Source trees pin most of their churn behind these
// names; recursing into them would balloon the inotify watch count
// without producing any signal we care about.
//
// We honour these by basename so any depth matches (e.g. nested
// node_modules under a monorepo).
var SkipDirs = map[string]struct{}{
	".archigraph":  {},
	".git":         {},
	".next":        {},
	".expo":        {},
	".venv":        {},
	"venv":         {},
	"node_modules": {},
	"target":       {},
	"build":        {},
	"dist":         {},
	"__pycache__":  {},
	".idea":        {},
	".vscode":      {},
}

// SkipExts is the suffix list for files we never re-index for. Lock
// files, IDE cache files and the daemon's own outputs would otherwise
// trigger debounce loops.
var SkipExts = map[string]struct{}{
	".lock":  {},
	".tmp":   {},
	".swp":   {},
	".swo":   {},
	".bak":   {},
	".orig":  {},
	".log":   {},
	".pyc":   {},
	".class": {},
	".o":     {},
	".a":     {},
	".so":    {},
	".dylib": {},
	".dll":   {},
	".exe":   {},
}

// ShouldSkipDir reports whether a directory basename is on the skip
// list. It is the watcher's only mechanism for excluding directories —
// we deliberately avoid parsing every .gitignore in the tree (correct
// .gitignore semantics need a full implementation we do not want to
// reproduce here).
func ShouldSkipDir(base string) bool {
	_, ok := SkipDirs[base]
	return ok
}

// ShouldSkipPath reports whether a path event should be dropped before
// it ever reaches the debouncer. We drop:
//   - paths that traverse any SkipDirs basename, and
//   - file extensions on SkipExts (with a special case for vim swap
//     files which look like .foo.swp).
func ShouldSkipPath(p string) bool {
	parts := strings.Split(filepath.ToSlash(p), "/")
	for _, part := range parts {
		if _, ok := SkipDirs[part]; ok {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(p))
	if _, ok := SkipExts[ext]; ok {
		return true
	}
	// Vim creates `4913` test files and `.foo.swp` style swap files;
	// the latter is caught by .swp above, the former is rare enough to
	// ignore (it lives at most for a few ms).
	return false
}
