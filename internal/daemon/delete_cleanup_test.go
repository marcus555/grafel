package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/proto"
	"github.com/cajasmota/grafel/internal/registry"
	"github.com/cajasmota/grafel/internal/statusfile"
)

// registerCleanupGroup writes a synthetic fleet config + registry entry for
// group with the given repo paths (slug derived from basename). It mirrors the
// production registration shape closely enough for DeleteGroup's cleanup path.
func registerCleanupGroup(t *testing.T, group string, repoPaths ...string) {
	t.Helper()
	cfgPath, err := registry.ConfigPathFor(group)
	if err != nil {
		t.Fatal(err)
	}
	cfg := registry.GroupConfig{Name: group}
	for _, p := range repoPaths {
		cfg.Repos = append(cfg.Repos, registry.Repo{Slug: filepath.Base(p), Path: p})
	}
	if err := registry.SaveGroupConfig(cfgPath, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := registry.AddGroup(group, cfgPath); err != nil {
		t.Fatal(err)
	}
}

// writeFile creates parent dirs and writes b bytes to path.
func writeFile(t *testing.T, path string, b string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedRepoState lays down the per-repo artifacts DeleteGroup should clean up:
// the in-repo manifest, the engine status-plane sidecar, and the per-repo
// store (with a graph.fb). Returns the three paths.
func seedRepoState(t *testing.T, repoPath string) (manifest, status, storeGraphFB string) {
	t.Helper()
	manifest = filepath.Join(repoPath, ".grafel", "group.json")
	writeFile(t, manifest, `{"group":"x"}`)

	status, err := statusfile.PathFor(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, status, `{}`)

	stateDir := StateDirForRepo(repoPath)
	storeGraphFB = filepath.Join(stateDir, "graph.fb")
	writeFile(t, storeGraphFB, "fb")
	return manifest, status, storeGraphFB
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// TestDeleteGroup_CleansUpSingleGroupRepoState verifies case (a): deleting a
// group whose repo is referenced by NO other group removes the in-repo
// manifest, the status sidecar, the per-repo store, the group-level artifacts,
// and the group store dir.
func TestDeleteGroup_CleansUpSingleGroupRepoState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))

	repo := t.TempDir()
	registerCleanupGroup(t, "solo", repo)
	manifest, status, storeFB := seedRepoState(t, repo)

	// Group-level artifacts (category 2) + group store dir (category 4).
	groupLinks := filepath.Join(home, "groups", "solo-links.json")
	groupAlgo := filepath.Join(home, "groups", "solo-algo.json")
	writeFile(t, groupLinks, "[]")
	writeFile(t, groupAlgo, "{}")
	groupStore := filepath.Join(StoreDir(), "group-solo-abcd1234", "union.fb")
	writeFile(t, groupStore, "u")

	s := &Service{}
	var reply proto.DeleteGroupReply
	if err := s.DeleteGroup(&proto.DeleteGroupArgs{Group: "solo"}, &reply); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}

	for name, p := range map[string]string{
		"manifest":        manifest,
		"status sidecar":  status,
		"repo store fb":   storeFB,
		"group links":     groupLinks,
		"group algo":      groupAlgo,
		"group store dir": groupStore,
	} {
		if exists(t, p) {
			t.Errorf("%s should be removed but still exists: %s", name, p)
		}
	}
}

// TestDeleteGroup_SharedRepoStatePreserved verifies case (b), the #34 guard:
// deleting a group whose repo is ALSO referenced by another still-registered
// group leaves that repo's manifest, status sidecar, and store INTACT.
func TestDeleteGroup_SharedRepoStatePreserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))

	repo := t.TempDir()
	registerCleanupGroup(t, "alpha", repo)
	registerCleanupGroup(t, "beta", repo) // same repo, still registered after alpha delete
	manifest, status, storeFB := seedRepoState(t, repo)

	s := &Service{}
	var reply proto.DeleteGroupReply
	if err := s.DeleteGroup(&proto.DeleteGroupArgs{Group: "alpha"}, &reply); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}

	for name, p := range map[string]string{
		"manifest":       manifest,
		"status sidecar": status,
		"repo store fb":  storeFB,
	} {
		if !exists(t, p) {
			t.Errorf("shared repo %s must be preserved (#34) but was removed: %s", name, p)
		}
	}
}

// TestDeleteGroup_HyphenPrefixSurvivorArtifactsPreserved verifies the group-
// level glob does NOT destroy a surviving group whose name has the deleted
// group's name as a hyphen prefix. Deleting "api" must not sweep away "api-v2"'s
// artifacts (groups/api-v2-*.json is matched by the glob "groups/api-*.json",
// and store/group-api-v2-* by "store/group-api-*").
func TestDeleteGroup_HyphenPrefixSurvivorArtifactsPreserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))

	registerCleanupGroup(t, "api", t.TempDir())
	registerCleanupGroup(t, "api-v2", t.TempDir())

	// "api"'s own artifacts (must be removed).
	apiLinks := filepath.Join(home, "groups", "api-links.json")
	writeFile(t, apiLinks, "[]")
	apiStore := filepath.Join(StoreDir(), "group-api-cafe0001", "union.fb")
	writeFile(t, apiStore, "u")

	// Surviving "api-v2"'s artifacts (must be preserved).
	apiV2Links := filepath.Join(home, "groups", "api-v2-links.json")
	writeFile(t, apiV2Links, "[]")
	apiV2Store := filepath.Join(StoreDir(), "group-api-v2-cafe0002", "union.fb")
	writeFile(t, apiV2Store, "u")

	s := &Service{}
	var reply proto.DeleteGroupReply
	if err := s.DeleteGroup(&proto.DeleteGroupArgs{Group: "api"}, &reply); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}

	if !exists(t, apiV2Links) {
		t.Errorf("surviving group api-v2 links artifact must be preserved: %s", apiV2Links)
	}
	if !exists(t, apiV2Store) {
		t.Errorf("surviving group api-v2 store dir must be preserved: %s", apiV2Store)
	}
	if exists(t, apiLinks) {
		t.Errorf("deleted group api links artifact should be removed: %s", apiLinks)
	}
	if exists(t, apiStore) {
		t.Errorf("deleted group api store dir should be removed: %s", apiStore)
	}
}

// TestDeleteGroup_GroupArtifactsRemovedEvenWhenRepoShared verifies case (c):
// group-level artifacts keyed by the deleted group name are removed regardless
// of whether the repo is shared.
func TestDeleteGroup_GroupArtifactsRemovedEvenWhenRepoShared(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))

	repo := t.TempDir()
	registerCleanupGroup(t, "alpha", repo)
	registerCleanupGroup(t, "beta", repo)
	seedRepoState(t, repo)

	groupLinks := filepath.Join(home, "groups", "alpha-links.json")
	writeFile(t, groupLinks, "[]")
	groupStore := filepath.Join(StoreDir(), "group-alpha-deadbeef", "union.fb")
	writeFile(t, groupStore, "u")

	s := &Service{}
	var reply proto.DeleteGroupReply
	if err := s.DeleteGroup(&proto.DeleteGroupArgs{Group: "alpha"}, &reply); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}

	if exists(t, groupLinks) {
		t.Errorf("group-level links artifact should be removed: %s", groupLinks)
	}
	if exists(t, groupStore) {
		t.Errorf("group store dir should be removed: %s", groupStore)
	}
}
