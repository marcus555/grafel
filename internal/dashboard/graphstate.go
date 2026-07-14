package dashboard

// graphstate.go — shared in-memory graph state for the dashboard REST layer.
//
// The dashboard loads graphs lazily from the registry (same on-disk
// graph.fb / graph.json that the MCP daemon reads) and caches them per
// group with a TTL-based invalidation so first-paint endpoints stay fast.
//
// Design: intentionally no dependency on internal/mcp.  The dashboard
// reads the same graph files directly via internal/graph.LoadGraphFromDir
// and internal/registry, duplicating only the thin helpers it needs
// (prefixedID, splitPrefixed, stripScopePrefix, entity scanning).  This
// keeps the import graph simple and the dashboard binary slim.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/gitmeta"
	"github.com/cajasmota/grafel/internal/graph"
	"github.com/cajasmota/grafel/internal/graph/fbreader"
	"github.com/cajasmota/grafel/internal/graph/groupalgo"
	"github.com/cajasmota/grafel/internal/registry"
	"github.com/cajasmota/grafel/internal/types"
)

// ---------------------------------------------------------------------------
// graphPayloadCache — serialised-bytes cache for GET /api/graph/{group}
// ---------------------------------------------------------------------------
//
// Each entry stores the already-JSON-encoded response body and a strong ETag
// so repeat requests can be served with a single map lookup and a memcpy,
// skipping the O(nodes + edges) build loop entirely.
//
// Cache key: "<group>::<params-fingerprint>" where the fingerprint is a
// hex-encoded SHA-256 of the sorted query parameters that affect the output
// (filterKind, filterRepo, repos, includeExternal, includeModules).
//
// Invalidation: any call to graphPayloadCache.InvalidateGroup(group) drops
// the v1 and v2 in-memory entries for that group. Disk artifacts are immutable
// and source-versioned, so changed inputs select a different path instead of
// requiring eager deletion during watcher or memory-pressure events.

// payloadEntry is one cached response.
type payloadEntry struct {
	body          []byte // raw JSON bytes (not compressed — withGzip compresses on write)
	etag          string // strong ETag value, including the surrounding quotes
	sourceVersion string // empty for legacy in-memory-only entries
}

// graphPayloadCache is a concurrency-safe store of pre-serialised graph
// payloads backed by an optional immutable disk cache.
type graphPayloadCache struct {
	mu      sync.RWMutex
	entries map[string]*payloadEntry // cache key → entry
	disk    *diskPayloadCache
}

func newGraphPayloadCache() *graphPayloadCache {
	home, err := registry.HomeDir()
	if err != nil {
		return &graphPayloadCache{entries: map[string]*payloadEntry{}}
	}
	return newGraphPayloadCacheAt(filepath.Join(home, "cache", "dashboard", "v1"))
}

func newGraphPayloadCacheAt(root string) *graphPayloadCache {
	return &graphPayloadCache{
		entries: map[string]*payloadEntry{},
		disk:    newDiskPayloadCache(root),
	}
}

// payloadCacheKey returns a stable, collision-resistant key for the given
// (group, params) combination.  The params fingerprint is a truncated
// SHA-256 of the sorted param string so the map key stays short.
//
// PH1c (#2087): the ref is included in the fingerprint so that two
// requests for the same group but different refs get distinct cache slots.
// Passing ref="" preserves the pre-PH1c behaviour (all refs share a slot).
func payloadCacheKey(group, filterKind, filterRepo, reposParam string, includeExternal, includeModules bool, ref ...string) string {
	refVal := ""
	if len(ref) > 0 {
		refVal = ref[0]
	}
	params := fmt.Sprintf("fk=%s&fr=%s&repos=%s&ext=%v&mod=%v&ref=%s",
		filterKind, filterRepo, reposParam, includeExternal, includeModules, refVal)
	sum := sha256.Sum256([]byte(params))
	return group + "::" + fmt.Sprintf("%x", sum[:8])
}

// Get returns the cached entry and true when a valid entry exists,
// or nil and false on a miss.
func (c *graphPayloadCache) Get(key string, sourceVersion ...string) (*payloadEntry, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if ok && (len(sourceVersion) == 0 || sourceVersion[0] == "" || e.sourceVersion == sourceVersion[0]) {
		return e, true
	}
	if c.disk == nil || len(sourceVersion) == 0 || sourceVersion[0] == "" {
		return nil, false
	}
	e, ok = c.disk.Get(key, sourceVersion[0])
	if !ok {
		return nil, false
	}
	c.mu.Lock()
	c.entries[key] = e
	c.mu.Unlock()
	return e, true
}

// Set stores (or replaces) a payload entry.
func (c *graphPayloadCache) Set(key string, body []byte, etag string, sourceVersion ...string) {
	version := ""
	if len(sourceVersion) > 0 {
		version = sourceVersion[0]
	}
	e := &payloadEntry{body: body, etag: etag, sourceVersion: version}
	c.mu.Lock()
	c.entries[key] = e
	c.mu.Unlock()
	if c.disk != nil && len(sourceVersion) > 0 && sourceVersion[0] != "" {
		c.disk.SetAsync(key, sourceVersion[0], e)
	}
}

// InvalidateGroup drops all v1 and v2 in-memory entries for group. Valid disk
// entries remain reusable after RAM eviction; source changes cannot hit them.
func (c *graphPayloadCache) InvalidateGroup(group string) {
	prefixes := []string{group + "::", "v2:" + group + "::"}
	c.mu.Lock()
	for k := range c.entries {
		for _, prefix := range prefixes {
			if strings.HasPrefix(k, prefix) {
				delete(c.entries, k)
				break
			}
		}
	}
	c.mu.Unlock()
}

// InvalidateAll drops every in-memory entry. Versioned disk artifacts remain.
func (c *graphPayloadCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = map[string]*payloadEntry{}
	c.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Helpers (mirrors of unexported mcp helpers; small enough to inline)
// ---------------------------------------------------------------------------

func dashPrefixedID(repo, id string) string { return repo + "::" + id }

func dashSplitPrefixed(s string) (string, string) {
	i := strings.Index(s, "::")
	if i < 0 {
		return "", s
	}
	return s[:i], s[i+2:]
}

func dashStripScopePrefix(s string) string {
	if after, ok := strings.CutPrefix(s, "SCOPE."); ok {
		return after
	}
	return s
}

// ---------------------------------------------------------------------------
// LoadedRepo — one repo's graph loaded into memory
// ---------------------------------------------------------------------------

// DashRepo is a loaded repo entry for the dashboard layer.
//
// S8 (#2159): Reader is an mmap'd fbreader.Reader opened alongside Doc.
// Handlers that only need to iterate entities/relationships should use
// Reader.IterateEntities / Reader.IterateRelationships to avoid holding
// a materialised *graph.Document in heap for the duration of the
// request. Reader is nil when graph.fb is not present (JSON-only fallback).
type DashRepo struct {
	Slug   string
	Path   string
	Doc    *graph.Document
	Reader *fbreader.Reader // mmap zero-copy reader (S8, #2159); nil when unavailable
	mtime  time.Time
	err    string
}

// ---------------------------------------------------------------------------
// DashGroup — all repos for one group plus cross-repo links
// ---------------------------------------------------------------------------

// DashGroup holds loaded repos and links for one group.
type DashGroup struct {
	Name   string
	Repos  map[string]*DashRepo // slug -> repo
	Links  []CrossRepoLink
	Search *SearchIndex // pre-built search index; nil until buildSearchIndex runs

	// sourceVersion fingerprints every disk artifact used to materialise this
	// group. Payload snapshots are restored only when this value matches.
	sourceVersion string
}

// CrossRepoLink mirrors mcp.CrossRepoLink.
//
// The on-disk links file written by the link pass uses "relation" for the
// edge kind; the dashboard struct also accepts "kind" for backward compat
// (tests and older files).  UnmarshalJSON prefers "relation" when present.
type CrossRepoLink struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence,omitempty"`
	Channel    string  `json:"channel,omitempty"`
	Method     string  `json:"method,omitempty"`

	// Enrichment fields resolved from the source/target entities at serve time
	// (#4596). They are NOT persisted to the on-disk links file — UnmarshalJSON
	// ignores them — and are populated by enrichLinkEndpoints just before the
	// /links payload is written, so the frontend can render readable names and
	// open a real source-peek instead of only a graph deep-link fallback.
	SourceName          string `json:"source_name,omitempty"`
	SourceQualifiedName string `json:"source_qualified_name,omitempty"`
	SourceFile          string `json:"source_file,omitempty"`
	SourceLine          int    `json:"source_line,omitempty"`
	TargetName          string `json:"target_name,omitempty"`
	TargetQualifiedName string `json:"target_qualified_name,omitempty"`
	TargetFile          string `json:"target_file,omitempty"`
	TargetLine          int    `json:"target_line,omitempty"`

	// Module sub-paths owning each endpoint within its repo (#4698), derived
	// from the source/target file and the repo's configured monorepo module
	// roots. Empty for single-repo groups or files under no module root. Let the
	// scope selector keep a link at module precision when either endpoint lives
	// in the scoped module.
	SourceModulePath string `json:"source_module_path,omitempty"`
	TargetModulePath string `json:"target_module_path,omitempty"`
}

// UnmarshalJSON handles the "relation" field used by the link pass as a
// synonym for "kind".  When the JSON object has a non-empty "relation" key
// and an absent or empty "kind" key, the relation value is used as Kind.
func (l *CrossRepoLink) UnmarshalJSON(b []byte) error {
	type plain struct {
		Source     string  `json:"source"`
		Target     string  `json:"target"`
		Kind       string  `json:"kind"`
		Relation   string  `json:"relation"`
		Confidence float64 `json:"confidence,omitempty"`
		Channel    string  `json:"channel,omitempty"`
		Method     string  `json:"method,omitempty"`
	}
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	l.Source = p.Source
	l.Target = p.Target
	l.Kind = p.Kind
	if l.Kind == "" && p.Relation != "" {
		l.Kind = p.Relation
	}
	l.Confidence = p.Confidence
	l.Channel = p.Channel
	l.Method = p.Method
	return nil
}

// ---------------------------------------------------------------------------
// GraphCache — mtime-driven per-group cache with TTL
// ---------------------------------------------------------------------------

// cacheEntry holds a loaded group plus the time it was last refreshed.
type cacheEntry struct {
	group    *DashGroup
	loadedAt time.Time
}

// GraphCache is the dashboard's in-memory graph store. It is safe for
// concurrent use. Reload is lazy: the first call for a group loads it; after
// TTL expiry a cheap artifact fingerprint renews unchanged entries and only
// changed sources trigger graph materialisation.
//
// GraphCache also owns a graphPayloadCache so that Invalidate/InvalidateAll
// atomically bust both the loaded-group cache and the pre-serialised payload
// cache.  Handlers call c.Payloads to access the payload cache directly.
type GraphCache struct {
	mu       sync.Mutex
	entries  map[string]*cacheEntry
	loading  map[string]*loadGate // in-flight loads, keyed by group (singleflight)
	warmErrs map[string]error     // last load error per cache key (#5722), cleared on success
	ttl      time.Duration
	Payloads *graphPayloadCache // pre-serialised dense graph JSON, keyed by group+params
}

// loadGate coordinates a single in-flight loadGroup call so that N concurrent
// GetGroup callers for the same group do not each kick off a disk load and
// overlay restore. The first caller loads; the
// rest wait on done and read the shared result.  Critically, loadGroup runs
// WITHOUT GraphCache.mu held, so a slow group never wedges unrelated groups or
// the cheap cached-read fast path used by first-paint endpoints (#1478).
type loadGate struct {
	done chan struct{}
	grp  *DashGroup
	err  error
}

// NewGraphCache returns a cache with the given TTL.  Use 60 * time.Second
// for production; tests may use a lower value.
func NewGraphCache(ttl time.Duration) *GraphCache {
	return &GraphCache{
		entries:  map[string]*cacheEntry{},
		loading:  map[string]*loadGate{},
		warmErrs: map[string]error{},
		ttl:      ttl,
		Payloads: newGraphPayloadCache(),
	}
}

// GetGroupCached returns the already-loaded group and true, or (nil, false)
// when the group is not warm.  It NEVER loads from disk or runs algorithms,
// so it is safe to call from first-paint / best-effort enrichment paths that
// must not block on a cold or slow group (#1478).  A best-effort warm is
// kicked off in the background so a subsequent request finds it ready.
func (c *GraphCache) GetGroupCached(groupName string) (*DashGroup, bool) {
	return c.GetGroupCachedForRef(groupName, "")
}

// GetGroupCachedForRef is the ref-scoped variant of GetGroupCached (PH1c).
func (c *GraphCache) GetGroupCachedForRef(groupName, ref string) (*DashGroup, bool) {
	cacheKey := groupName
	if ref != "" {
		cacheKey = groupName + "@" + ref
	}
	c.mu.Lock()
	ent, ok := c.entries[cacheKey]
	if ok && time.Since(ent.loadedAt) < c.ttl {
		grp := ent.group
		c.mu.Unlock()
		return grp, true
	}
	c.mu.Unlock()
	// Not warm (or stale): trigger an async warm so the next caller is fast,
	// but return immediately so we never block first paint.
	go func() { _, _ = c.GetGroupForRef(groupName, ref) }()
	return nil, false
}

// LastWarmError returns the error from the most recent load attempt for
// groupName/ref, if any load has been attempted and the most recent one
// failed. It returns (nil, false) when no attempt has failed yet — either
// because the group is warm, or because it simply hasn't finished its first
// warm attempt (the ordinary "still warming" case). Callers use this to
// distinguish a genuine load failure (#5722) from a group that just hasn't
// warmed up yet, so a failure can be surfaced instead of retried forever.
func (c *GraphCache) LastWarmError(groupName, ref string) (error, bool) {
	cacheKey := groupName
	if ref != "" {
		cacheKey = groupName + "@" + ref
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	err, ok := c.warmErrs[cacheKey]
	return err, ok
}

// Invalidate drops the cached entry for group and all per-ref variants
// (called on re-index events). It also busts the pre-serialised payload
// cache for that group so the next GET /api/graph/{group} request rebuilds
// a fresh payload from the new graph.
//
// S8 (#2159): mmap readers held by DashRepo entries are closed so the OS
// can reclaim file descriptors and page-cache pages for the old graph.fb.
//
// #5722 follow-up: a re-index/invalidate means the underlying condition that
// may have caused a PRIOR warm failure has presumably changed, so any
// recorded warmErrs entry for group (and its per-ref variants) is dropped
// here too. Without this, LastWarmError would keep surfacing the stale
// failure to a connected dashboard client until some unrelated caller
// happened to trigger another load attempt for that exact group/ref. A
// genuinely still-broken source will simply re-record the error on the next
// load attempt (kicked off in the background by GetGroupCachedForRef) — this
// only clears the stale signal, it never suppresses a real recurring one.
func (c *GraphCache) Invalidate(group string) {
	prefix := group + "@"
	c.mu.Lock()
	if ent, ok := c.entries[group]; ok {
		closeDashGroupReaders(ent.group)
		delete(c.entries, group)
	}
	// Also evict all per-ref entries for this group.
	for k, ent := range c.entries {
		if strings.HasPrefix(k, prefix) {
			closeDashGroupReaders(ent.group)
			delete(c.entries, k)
		}
	}
	// Clear any recorded warm-load failure for group and its per-ref variants
	// (#5722 follow-up) so a stale error does not keep being surfaced after a
	// re-index resolves the underlying problem.
	delete(c.warmErrs, group)
	for k := range c.warmErrs {
		if strings.HasPrefix(k, prefix) {
			delete(c.warmErrs, k)
		}
	}
	c.mu.Unlock()
	c.Payloads.InvalidateGroup(group)
}

// InvalidateAll drops every cached entry and every pre-serialised payload.
// S8 (#2159): mmap readers are closed before clearing the map.
//
// #5722 follow-up: every recorded warmErrs entry is cleared too, for the
// same reason as Invalidate — a global re-index/invalidation should not
// leave stale warm-failure signals behind for callers to keep surfacing.
func (c *GraphCache) InvalidateAll() {
	c.mu.Lock()
	for _, ent := range c.entries {
		closeDashGroupReaders(ent.group)
	}
	c.entries = map[string]*cacheEntry{}
	c.warmErrs = map[string]error{}
	c.mu.Unlock()
	c.Payloads.InvalidateAll()
}

// closeDashGroupReaders releases every mmap'd fbreader.Reader held by repos
// in grp. Safe to call with nil grp. Errors are intentionally swallowed —
// closing a reader is best-effort; a leaked fd is never worse than a crash.
func closeDashGroupReaders(grp *DashGroup) {
	if grp == nil {
		return
	}
	for _, dr := range grp.Repos {
		if dr != nil && dr.Reader != nil {
			_ = dr.Reader.Close()
			dr.Reader = nil
		}
	}
}

// GetGroup returns the loaded group, refreshing from disk when the TTL has
// elapsed or when graph files have changed on disk.
func (c *GraphCache) GetGroup(groupName string) (*DashGroup, error) {
	return c.GetGroupForRef(groupName, "")
}

// GetGroupForRef returns the loaded group for a specific git ref.
// When ref is "" it behaves identically to GetGroup (reads the current
// HEAD ref via daemon.StateDirForRepo for each repo).
//
// PH1c (#2087): passing a non-empty ref scopes the graph read to that
// ref's state directory (refs/<ref-safe>/graph.fb). The cache key
// includes groupName+ref so different refs coexist in the cache.
func (c *GraphCache) GetGroupForRef(groupName, ref string) (*DashGroup, error) {
	cacheKey := groupName
	if ref != "" {
		cacheKey = groupName + "@" + ref
	}
	c.mu.Lock()
	now := time.Now()
	ent, cached := c.entries[cacheKey]
	if cached {
		if now.Sub(ent.loadedAt) < c.ttl {
			grp := ent.group
			c.mu.Unlock()
			return grp, nil
		}
		// TTL expiry is only a prompt to validate freshness, not evidence that
		// graph.fb changed. Check the cheap disk-artifact fingerprint before
		// paying for a full graph reload. Watcher invalidation remains the fast
		// path for actual changes; this also covers missed watcher events.
		if c.ttl > 0 && ent.group != nil && ent.group.sourceVersion != "" {
			staleVersion := ent.group.sourceVersion
			c.mu.Unlock()
			currentVersion, versionErr := dashboardSourceVersion(groupName, ref)
			c.mu.Lock()
			// Another goroutine may have invalidated or replaced the entry while
			// the fingerprint was computed. Renew only the exact entry observed.
			if current, ok := c.entries[cacheKey]; ok && current == ent && versionErr == nil && currentVersion == staleVersion {
				current.loadedAt = time.Now()
				grp := current.group
				c.mu.Unlock()
				return grp, nil
			}
		}
	}

	// Cold / stale. Coordinate a single in-flight load via a loadGate so
	// concurrent callers for the same group share one disk load and overlay
	// restore instead of each launching their own. We deliberately
	// release c.mu before running loadGroup: the load can take seconds on a
	// large group, and holding the cache mutex across it would serialise
	// EVERY other group + the cheap cached-read fast path behind it — the
	// exact wedge that made first-paint endpoints (and thus the dashboard)
	// return 000 with a large group registered (#1478).
	if g, ok := c.loading[cacheKey]; ok {
		c.mu.Unlock()
		<-g.done
		return g.grp, g.err
	}
	gate := &loadGate{done: make(chan struct{})}
	c.loading[cacheKey] = gate
	c.mu.Unlock()

	grp, err := c.loadGroupForRef(groupName, ref)

	c.mu.Lock()
	if err == nil {
		c.entries[cacheKey] = &cacheEntry{group: grp, loadedAt: time.Now()}
		delete(c.warmErrs, cacheKey)
	} else {
		// Record the failure (#5722) so a subsequent best-effort caller (e.g.
		// the graph-stream endpoint) can distinguish "genuinely failed" from
		// "just hasn't warmed yet" instead of surfacing an eternal retry.
		c.warmErrs[cacheKey] = err
	}
	delete(c.loading, cacheKey)
	c.mu.Unlock()

	gate.grp, gate.err = grp, err
	close(gate.done)
	return grp, err
}

// loadGroup reads the registry for groupName and loads each repo's graph.
// It runs WITHOUT c.mu held (see GetGroup) so a slow load never blocks the
// rest of the cache.
func (c *GraphCache) loadGroup(groupName string) (*DashGroup, error) {
	return c.loadGroupForRef(groupName, "")
}

// loadGroupForRef loads the group graph for a specific ref. When ref is ""
// it reads the current HEAD directly and uses daemon.StateDirForRepoRef,
// avoiding a full git metadata capture per module. When ref is non-empty it
// reads from daemon.StateDirForRepoRef unchanged.
// It runs WITHOUT c.mu held (see GetGroupForRef).
func (c *GraphCache) loadGroupForRef(groupName, ref string) (*DashGroup, error) {
	groups, err := registry.Groups()
	if err != nil {
		return nil, err
	}
	var cfgPath string
	for _, g := range groups {
		if g.Name == groupName {
			cfgPath = g.ConfigPath
			break
		}
	}
	if cfgPath == "" {
		return nil, fmt.Errorf("group %q not registered", groupName)
	}
	cfg, err := registry.LoadGroupConfig(cfgPath)
	if err != nil {
		return nil, err
	}
	grp := &DashGroup{
		Name:  groupName,
		Repos: map[string]*DashRepo{},
	}
	sourceMtimes := make(map[string]int64, len(cfg.Repos))
	for _, r := range cfg.Repos {
		dr := &DashRepo{Slug: r.Slug, Path: r.Path}
		var stateDir string
		if ref != "" {
			stateDir = daemon.StateDirForRepoRef(r.Path, ref)
		} else {
			stateDir = daemon.StateDirForRepoRef(r.Path, gitmeta.CurrentRef(r.Path))
		}
		doc, err := graph.LoadGraphFromDir(stateDir)
		if err != nil {
			dr.err = err.Error()
		} else {
			dr.Doc = doc
			// S8 (#2159): open a zero-copy mmap reader alongside the Document
			// so handlers that only need to iterate entities/relationships can
			// avoid materialising the full heap slice. Best-effort: failures
			// leave Reader nil and callers fall back to doc.Entities.
			fbPath := filepath.Join(stateDir, "graph.fb")
			if rdr, rerr := fbreader.Open(fbPath); rerr == nil {
				dr.Reader = rdr
			}
			// Record the newer of fb/json mtime for cache invalidation.
			if info, e := os.Stat(filepath.Join(stateDir, "graph.fb")); e == nil {
				dr.mtime = info.ModTime()
				sourceMtimes[r.Slug] = info.ModTime().UnixNano()
			} else if info, e = os.Stat(filepath.Join(stateDir, "graph.json")); e == nil {
				dr.mtime = info.ModTime()
			}
		}
		grp.Repos[r.Slug] = dr
	}

	// Group-scope algorithms are computed by the daemon after indexing and
	// persisted as an overlay. A dashboard cold wake must only restore that
	// result; running Louvain/PageRank/Betweenness synchronously here turns a
	// cheap graph.fb reload into a minute-long request on large groups.
	//
	// The overlay currently represents HEAD. Ref-scoped overlays need their own
	// artifact namespace, so explicit refs retain the algorithm attributes that
	// graph.fb already carries and never consume the HEAD overlay accidentally.
	if ref == "" {
		if path, pathErr := groupalgo.OverlayPath(groupName); pathErr == nil {
			if ov, ok := groupalgo.ReadOverlay(path, sourceMtimes); ok {
				applyGroupAlgorithmOverlay(grp, ov)
			}
		}
	}

	// Load cross-repo links file.  The file can be either a bare array of
	// CrossRepoLink objects or the wrapper format {"version":N,"links":[...]}.
	// Both forms are accepted; the wrapper form is written by the link pass.
	lf := defaultLinksFile(groupName)
	if data, err := os.ReadFile(lf); err == nil {
		if links, err2 := readCrossRepoLinks(data); err2 == nil {
			grp.Links = normalizeLinkEndpoints(links, grp.Repos)
		}
	}
	if version, versionErr := dashboardSourceVersion(groupName, ref); versionErr == nil {
		grp.sourceVersion = version
	}

	// Build the in-memory search index once at load time so that
	// /api/search/{group} does not need to scan all entities on every request
	// (#2104).
	grp.Search = buildSearchIndex(grp)

	return grp, nil
}

// dashboardSourceVersion fingerprints the files that define a dashboard group
// without loading graph.fb. Handlers use it to validate disk payloads before
// materialising the graph, making a valid snapshot a true cold-start fast path.
func dashboardSourceVersion(groupName, ref string) (string, error) {
	groups, err := registry.Groups()
	if err != nil {
		return "", err
	}
	var cfgPath string
	for _, group := range groups {
		if group.Name == groupName {
			cfgPath = group.ConfigPath
			break
		}
	}
	if cfgPath == "" {
		return "", fmt.Errorf("group %q not registered", groupName)
	}
	cfg, err := registry.LoadGroupConfig(cfgPath)
	if err != nil {
		return "", err
	}
	repos := append([]registry.Repo(nil), cfg.Repos...)
	sort.Slice(repos, func(i, j int) bool { return repos[i].Slug < repos[j].Slug })

	h := sha256.New()
	_, _ = fmt.Fprintf(h, "dashboard-payload-v1\x00%s\x00%s\x00", groupName, ref)
	hashDashboardArtifact(h, "config", cfgPath)
	for _, repo := range repos {
		stateRef := ref
		if stateRef == "" {
			stateRef = gitmeta.CurrentRef(repo.Path)
		}
		stateDir := daemon.StateDirForRepoRef(repo.Path, stateRef)
		graphPath := filepath.Join(stateDir, "graph.fb")
		if _, statErr := os.Stat(graphPath); statErr != nil {
			graphPath = filepath.Join(stateDir, "graph.json")
		}
		_, _ = fmt.Fprintf(h, "repo\x00%s\x00%s\x00", repo.Slug, stateDir)
		hashDashboardArtifact(h, "graph", graphPath)
	}
	if ref == "" {
		if overlayPath, pathErr := groupalgo.OverlayPath(groupName); pathErr == nil {
			hashDashboardArtifact(h, "overlay", overlayPath)
		}
	}
	hashDashboardArtifact(h, "links", defaultLinksFile(groupName))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func hashDashboardArtifact(h interface{ Write([]byte) (int, error) }, kind, path string) {
	info, err := os.Stat(path)
	if err != nil {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00missing\x00", kind, path)
		return
	}
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%d\x00%d\x00", kind, path, info.Size(), info.ModTime().UnixNano())
}

// normalizeLinkEndpoints rewrites the "<repo-slug>::<entity-id>" endpoints of
// cross-repo links so each one matches the canonical prefixed node ID used by
// the served graph (built with dashPrefixedID(rp.Slug, entity.ID)). Without
// this rewrite the merge guard `visible[l.Source] && visible[l.Target]` in
// buildV2Graph / serveGraphDense never matches and every cross-repo edge is
// silently dropped from the served graph — the #1582 symptom (0 of 37,104
// served edges were cross-repo).
//
// The repo-slug PREFIX written by the link pass diverges from the dashboard
// repo slugs in (at least) two ways observed in the corpus:
//
//   - underscore vs dash:   "acme_core"  vs  "acme-core"
//   - short name vs full:   "catalog"      vs  "polyglot-platform-services-catalog"
//
// Because the <entity-id> suffix is globally unique within a group (verified:
// 0 collisions across acme's 19,613 and polyglot-platform's 1,316 nodes),
// the most reliable resolution is by that suffix: we index every entity ID ->
// its canonical prefixed node ID and rewrite each endpoint to the canonical
// form of the entity it points at. Endpoints whose entity ID is unknown (or
// whose suffix collides — a defensive guard) are left untouched, so the merge
// guard drops them exactly as before.
func normalizeLinkEndpoints(links []CrossRepoLink, repos map[string]*DashRepo) []CrossRepoLink {
	if len(repos) == 0 || len(links) == 0 {
		return links
	}
	// entity-ID suffix -> canonical prefixed node ID. Suffixes seen more than
	// once are recorded as ambiguous and never used for rewriting.
	canonical := make(map[string]string)
	ambiguous := make(map[string]bool)
	for _, rp := range repos {
		if rp == nil || rp.Doc == nil {
			continue
		}
		for i := range rp.Doc.Entities {
			id := rp.Doc.Entities[i].ID
			if _, seen := canonical[id]; seen {
				ambiguous[id] = true
				continue
			}
			canonical[id] = dashPrefixedID(rp.Slug, id)
		}
	}
	rewrite := func(endpoint string) string {
		_, local := dashSplitPrefixed(endpoint)
		if local == "" || ambiguous[local] {
			return endpoint
		}
		if cid, ok := canonical[local]; ok {
			return cid
		}
		return endpoint // unknown entity; leave as-is (merge guard drops it)
	}
	out := make([]CrossRepoLink, len(links))
	for i, l := range links {
		l.Source = rewrite(l.Source)
		l.Target = rewrite(l.Target)
		out[i] = l
	}
	return out
}

// enrichLinkEndpoints resolves each link's source and target entity (by its
// prefixed "<repo>::<localId>" id) via the same findEntity lookup the rest of
// the dashboard uses, and copies the entity's name / qualified name / source
// file / start line onto the link (#4596). This lets the /links page render a
// readable source name and open a real source-peek rather than only a graph
// deep-link fallback.
//
// It is additive and best-effort: an endpoint that does not resolve to an
// entity (e.g. a synthetic scope.operation node #4554 or a bare-external
// target #4558 that has no source-derived name) simply leaves its enrichment
// fields empty, and the frontend falls back to the graph deep-link.
// enrichLinkEndpoints resolves each link's source/target entity for readable
// rendering and source-peek (#4596) and stamps each endpoint's monorepo
// module_path (#4698) using moduleRoots (parent-repo slug → configured module
// roots). moduleRoots may be nil for single-repo / non-monorepo groups, in
// which case module paths stay empty.
func enrichLinkEndpoints(grp *DashGroup, links []CrossRepoLink, moduleRoots map[string][]string) []CrossRepoLink {
	if grp == nil || len(links) == 0 {
		return links
	}
	out := make([]CrossRepoLink, len(links))
	for i, l := range links {
		if rp, e := findEntity(grp, l.Source); e != nil {
			l.SourceName = e.Name
			l.SourceQualifiedName = e.QualifiedName
			l.SourceFile = e.SourceFile
			l.SourceLine = e.StartLine
			if rp != nil {
				l.SourceModulePath = modulePathFor(rp.Slug, e.SourceFile, moduleRoots)
			}
		}
		if rp, e := findEntity(grp, l.Target); e != nil {
			l.TargetName = e.Name
			l.TargetQualifiedName = e.QualifiedName
			l.TargetFile = e.SourceFile
			l.TargetLine = e.StartLine
			if rp != nil {
				l.TargetModulePath = modulePathFor(rp.Slug, e.SourceFile, moduleRoots)
			}
		}
		out[i] = l
	}
	return out
}

// applyGroupAlgorithmOverlay restores the daemon's persisted group-scope
// algorithm output without rerunning Pass 4 during a dashboard request. The
// dashboard wire format still groups community summaries by repository, so a
// cross-repo community is projected into deterministic per-repo summaries
// while every entity retains the authoritative group community ID.
func applyGroupAlgorithmOverlay(grp *DashGroup, ov *groupalgo.Overlay) {
	if grp == nil || ov == nil {
		return
	}

	communityByID := make(map[int]graph.CommunityResult, len(ov.Communities))
	for _, community := range ov.Communities {
		communityByID[community.ID] = community
	}

	for _, repo := range grp.Repos {
		if repo == nil || repo.Doc == nil {
			continue
		}
		repo.Doc.Communities = nil
		members := map[int][]*graph.Entity{}
		for i := range repo.Doc.Entities {
			entity := &repo.Doc.Entities[i]
			result, ok := ov.Results[entity.ID]
			if !ok {
				continue
			}
			cid, pageRank, centrality := result.CommunityID, result.PageRank, result.Centrality
			entity.CommunityID = &cid
			entity.PageRank = &pageRank
			entity.Centrality = &centrality
			entity.IsGodNode = result.IsGodNode
			entity.IsArticulationPt = result.IsArticulationPoint
			if cid >= 0 {
				members[cid] = append(members[cid], entity)
			}
		}

		communityIDs := make([]int, 0, len(members))
		for cid := range members {
			communityIDs = append(communityIDs, cid)
		}
		sort.Ints(communityIDs)
		for _, cid := range communityIDs {
			entities := members[cid]
			sort.SliceStable(entities, func(i, j int) bool {
				return entityPageRank(entities[i]) > entityPageRank(entities[j])
			})
			summary := communityByID[cid]
			summary.ID = cid
			summary.Size = len(entities)
			topCount := 3
			if len(entities) < topCount {
				topCount = len(entities)
			}
			summary.TopEntities = make([]string, topCount)
			for i := 0; i < topCount; i++ {
				summary.TopEntities[i] = entities[i].ID
			}
			repo.Doc.Communities = append(repo.Doc.Communities, summary)
		}
		stats := ov.Stats
		repo.Doc.AlgorithmStats = &stats
	}
}

func entityPageRank(entity *graph.Entity) float64 {
	if entity == nil || entity.PageRank == nil {
		return 0
	}
	return *entity.PageRank
}

// defaultLinksFile mirrors mcp.defaultLinksFile.
func defaultLinksFile(group string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".grafel", "groups", group+"-links.json")
}

// readCrossRepoLinks parses cross-repo links from raw JSON.
// The file can be either a bare array or the wrapper object
// {"version": N, "links": [...]}.  Both formats are accepted.
func readCrossRepoLinks(data []byte) ([]CrossRepoLink, error) {
	// Try bare array first (older format).
	var asArr []CrossRepoLink
	if err := json.Unmarshal(data, &asArr); err == nil {
		return asArr, nil
	}
	// Try wrapped object format {"links":[...]}.
	var asObj struct {
		Links []CrossRepoLink `json:"links"`
	}
	if err := json.Unmarshal(data, &asObj); err != nil {
		return nil, err
	}
	return asObj.Links, nil
}

// sortedRepos returns the repos of a group in deterministic slug order.
func sortedRepos(g *DashGroup) []*DashRepo {
	slugs := make([]string, 0, len(g.Repos))
	for s := range g.Repos {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	out := make([]*DashRepo, 0, len(slugs))
	for _, s := range slugs {
		if r := g.Repos[s]; r != nil && r.Doc != nil {
			out = append(out, r)
		}
	}
	return out
}

// findEntity looks up an entity by prefixed-or-bare id/label within a group.
func findEntity(g *DashGroup, key string) (*DashRepo, *graph.Entity) {
	// Prefixed "<repo>::<local>".
	if rp, local := dashSplitPrefixed(key); rp != "" {
		if r, ok := g.Repos[rp]; ok && r.Doc != nil {
			// First try exact ID match.
			for i := range r.Doc.Entities {
				if r.Doc.Entities[i].ID == local {
					return r, &r.Doc.Entities[i]
				}
			}
			// Synthetic IDs produced during JS/TS extraction have the form
			// "Kind:Name" (e.g. "Function:confirmTransfer") — they never match
			// hex entity IDs. Resolve by extracting the name portion and falling
			// back to a name-based lookup within the same repo.
			if idx := strings.LastIndex(local, ":"); idx >= 0 {
				name := local[idx+1:]
				for i := range r.Doc.Entities {
					if r.Doc.Entities[i].Name == name {
						return r, &r.Doc.Entities[i]
					}
				}
			}
		}
		return nil, nil
	}
	// Bare: try ID then Name match across repos.
	for _, r := range sortedRepos(g) {
		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			if e.ID == key || e.Name == key {
				return r, e
			}
		}
	}
	return nil, nil
}

// groupEntityHit bundles an entity with its owning repo slug for group-wide lookups.
type groupEntityHit struct {
	repo   string
	entity *graph.Entity
}

// buildGroupEntityIndex builds a group-wide map of entity ID → {repo, entity}.
// When the same entity ID appears in multiple repos (should not happen given the
// hash salting, but defensive), the first repo encountered (sorted) wins.
// Used by handleFlowDetail to resolve bridge-step entity IDs that live in
// companion repos (#1905).
func buildGroupEntityIndex(g *DashGroup) map[string]groupEntityHit {
	total := 0
	for _, r := range g.Repos {
		if r.Doc != nil {
			total += len(r.Doc.Entities)
		}
	}
	out := make(map[string]groupEntityHit, total)
	for _, r := range sortedRepos(g) {
		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			if _, exists := out[e.ID]; !exists {
				out[e.ID] = groupEntityHit{repo: r.Slug, entity: e}
			}
		}
	}
	return out
}

// serializeEntity converts a graph.Entity into the REST wire shape.
func serializeEntity(repo string, e *graph.Entity) map[string]any {
	out := map[string]any{
		"id":             dashPrefixedID(repo, e.ID),
		"label":          e.Name,
		"qualified_name": e.QualifiedName,
		"kind":           dashStripScopePrefix(e.Kind),
		"source_file":    e.SourceFile,
		"start_line":     e.StartLine,
		"end_line":       e.EndLine,
		"language":       e.Language,
		"repo":           repo,
	}
	if e.PageRank != nil {
		out["pagerank"] = *e.PageRank
	}
	if e.CommunityID != nil {
		out["community_id"] = *e.CommunityID
	}
	if len(e.Properties) > 0 {
		out["properties"] = e.Properties
	}
	return out
}

// hashStr returns a short stable hash for use as a path-hash key.
func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

// groupTopFrameworks scans all http_endpoint / Endpoint / Route entities
// across the group and returns the top-N framework names sorted by usage
// frequency (descending). cap controls the maximum number returned (≤8).
func groupTopFrameworks(grp *DashGroup, cap int) []string {
	freq := map[string]int{}
	for _, r := range sortedRepos(grp) {
		if r.Doc == nil {
			continue
		}
		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			kind := dashStripScopePrefix(e.Kind)
			// #1217 backward compat: accept all three http endpoint kind strings.
			if !types.IsHTTPEndpointKind(kind) && !strings.EqualFold(kind, httpEndpointKind) &&
				kind != "Endpoint" && kind != "Route" {
				continue
			}
			if fw := e.Properties["framework"]; fw != "" {
				freq[fw]++
			}
		}
	}
	if len(freq) == 0 {
		return nil
	}

	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(freq))
	for k, v := range freq {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	if cap > 0 && len(pairs) > cap {
		pairs = pairs[:cap]
	}
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.k
	}
	return out
}
