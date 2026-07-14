package dashboard

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/registry"
	"github.com/cajasmota/grafel/internal/testsupport"
)

func TestGraphPayloadCacheRestoresDiskSnapshot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "dashboard-cache")
	first := newGraphPayloadCacheAt(root)
	key := "v2:assessment::default"
	version := "graph-v1"
	entry := &payloadEntry{body: []byte(`{"ok":true}`), etag: `"etag-1"`, sourceVersion: version}
	if err := first.disk.Set(key, version, entry); err != nil {
		t.Fatal(err)
	}

	second := newGraphPayloadCacheAt(root)
	entry, ok := second.Get(key, version)
	if !ok {
		t.Fatal("expected disk-backed payload hit after in-memory cache restart")
	}
	if got := string(entry.body); got != `{"ok":true}` {
		t.Fatalf("body = %q", got)
	}
	if entry.etag != `"etag-1"` || entry.sourceVersion != version {
		t.Fatalf("metadata not restored: %+v", entry)
	}
}

func TestGraphPayloadCacheRejectsStaleSourceVersion(t *testing.T) {
	cache := newGraphPayloadCacheAt(t.TempDir())
	key := "assessment::default"
	cache.Set(key, []byte(`{"version":1}`), `"etag-1"`, "graph-v1")

	if _, ok := cache.Get(key, "graph-v2"); ok {
		t.Fatal("stale in-memory or disk payload was served for a changed graph")
	}
}

func TestGraphPayloadCacheInvalidatesV1AndV2MemoryEntries(t *testing.T) {
	cache := newGraphPayloadCacheAt(t.TempDir())
	cache.Set("assessment::default", []byte("v1"), `"v1"`)
	cache.Set("v2:assessment::default", []byte("v2"), `"v2"`)
	cache.Set("v2:other::default", []byte("other"), `"other"`)

	cache.InvalidateGroup("assessment")
	if _, ok := cache.Get("assessment::default"); ok {
		t.Fatal("v1 entry survived group invalidation")
	}
	if _, ok := cache.Get("v2:assessment::default"); ok {
		t.Fatal("v2 entry survived group invalidation")
	}
	if _, ok := cache.Get("v2:other::default"); !ok {
		t.Fatal("unrelated group was invalidated")
	}
}

func TestGraphPayloadCacheTreatsCorruptionAsMiss(t *testing.T) {
	root := t.TempDir()
	cache := newGraphPayloadCacheAt(root)
	key := "assessment::default"
	version := "graph-v1"
	entry := &payloadEntry{body: []byte(`{"ok":true}`), etag: `"etag-1"`, sourceVersion: version}
	if err := cache.disk.Set(key, version, entry); err != nil {
		t.Fatal(err)
	}

	path, ok := cache.disk.path(key, version)
	if !ok {
		t.Fatal("expected cache path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	restarted := newGraphPayloadCacheAt(root)
	if _, ok := restarted.Get(key, version); ok {
		t.Fatal("corrupt disk payload must degrade to a cache miss")
	}
}

func TestDiskPayloadCachePrunesOldSourceVersions(t *testing.T) {
	cache := newDiskPayloadCache(t.TempDir())
	key := "assessment::default"
	entry := &payloadEntry{body: []byte(`{"ok":true}`), etag: `"etag"`}
	for i := 0; i < diskPayloadVersionsPerGroup+2; i++ {
		version := "graph-v" + string(rune('a'+i))
		if err := cache.Set(key, version, entry); err != nil {
			t.Fatal(err)
		}
	}
	groupDir := filepath.Join(cache.root, shortPayloadHash("assessment"))
	dirs, err := os.ReadDir(groupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != diskPayloadVersionsPerGroup {
		t.Fatalf("retained source versions = %d, want %d", len(dirs), diskPayloadVersionsPerGroup)
	}
}

func TestV2GraphRestoresDiskPayloadBeforeLoadingGraph(t *testing.T) {
	root := testsupport.IsolateHome(t)
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir := daemon.StateDirForRepoRef(repoPath, "")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately invalid graph bytes: the request can only succeed if the
	// persisted HTTP payload is served before graph materialisation.
	if err := os.WriteFile(filepath.Join(stateDir, "graph.fb"), []byte("not-a-graph"), 0o644); err != nil {
		t.Fatal(err)
	}

	const group = "cold-disk-payload"
	cfgPath, err := registry.ConfigPathFor(group)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.SaveGroupConfig(cfgPath, &registry.GroupConfig{
		Name:  group,
		Repos: []registry.Repo{{Slug: "repo", Path: repoPath}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.AddGroup(group, cfgPath); err != nil {
		t.Fatal(err)
	}
	version, err := dashboardSourceVersion(group, "")
	if err != nil {
		t.Fatal(err)
	}
	key := "v2:" + payloadCacheKey(group, "", "", "", false, false, "") + ":lod="
	body := []byte(`{"ok":true,"data":{"nodes":[]}}` + "\n")
	first := NewGraphCache(0)
	entry := &payloadEntry{body: body, etag: `"cold-etag"`, sourceVersion: version}
	if err := first.Payloads.disk.Set(key, version, entry); err != nil {
		t.Fatal(err)
	}

	restarted := NewGraphCache(0)
	server := &Server{graphs: restarted}
	req := httptest.NewRequest(http.MethodGet, "/api/v2/graph/"+group, nil)
	req.SetPathValue("group", group)
	rec := httptest.NewRecorder()
	server.handleV2Graph(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != string(body) {
		t.Fatalf("cold disk response = status %d body %q", rec.Code, rec.Body.String())
	}
	if len(restarted.entries) != 0 {
		t.Fatal("graph was materialised even though a valid disk payload existed")
	}
}
