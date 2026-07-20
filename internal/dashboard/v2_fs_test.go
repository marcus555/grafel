package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/proto"
	"github.com/cajasmota/grafel/internal/testsupport"
)

// fsListEnvelope is the decoded GET /api/v2/fs/list response.
type fsListEnvelope struct {
	OK   bool          `json:"ok"`
	Data v2FsListReply `json:"data"`
}

func getFsList(t *testing.T, baseURL, path string) fsListEnvelope {
	t.Helper()
	u := baseURL + "/api/v2/fs/list"
	if path != "" {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		q := req.URL.Query()
		q.Set("path", path)
		req.URL.RawQuery = q.Encode()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET fs/list: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d; want 200", resp.StatusCode)
		}
		var env fsListEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return env
	}
	resp, err := http.Get(u)
	if err != nil {
		t.Fatalf("GET fs/list: %v", err)
	}
	defer resp.Body.Close()
	var env fsListEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return env
}

// TestV2FsList_ListsSubdirsWithAbsPaths verifies the folder browser returns
// subdirectories (not files) with their ABSOLUTE on-disk paths — the key
// requirement that lets the wizard proceed without a manual paste.
func TestV2FsList_ListsSubdirsWithAbsPaths(t *testing.T) {
	ts, _ := newWizardTestServer(t, func(proto.RebuildArgs) (proto.RebuildReply, error) {
		return proto.RebuildReply{}, nil
	})
	root := t.TempDir()
	for _, d := range []string{"alpha", "beta", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "afile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := getFsList(t, ts.URL, root)
	if !env.OK {
		t.Fatalf("ok = false; reply = %+v", env.Data)
	}
	if env.Data.Path != root {
		t.Fatalf("Path = %q; want %q", env.Data.Path, root)
	}
	if env.Data.Parent != filepath.Dir(root) {
		t.Fatalf("Parent = %q; want %q", env.Data.Parent, filepath.Dir(root))
	}

	byName := map[string]v2FsEntry{}
	for _, e := range env.Data.Entries {
		byName[e.Name] = e
	}
	// Directories present; file absent.
	if _, ok := byName["afile.txt"]; ok {
		t.Fatal("file afile.txt should not be listed")
	}
	for _, want := range []string{"alpha", "beta", ".hidden"} {
		e, ok := byName[want]
		if !ok {
			t.Fatalf("missing dir %q", want)
		}
		if e.Path != filepath.Join(root, want) {
			t.Fatalf("%q Path = %q; want absolute %q", want, e.Path, filepath.Join(root, want))
		}
		if !e.IsDir {
			t.Fatalf("%q IsDir = false", want)
		}
		if !filepath.IsAbs(e.Path) {
			t.Fatalf("%q Path %q is not absolute", want, e.Path)
		}
	}
	if !byName[".hidden"].Hidden {
		t.Fatal(".hidden should be flagged Hidden")
	}
	// Sorted ascending case-insensitively: .hidden, alpha, beta.
	if env.Data.Entries[0].Name != ".hidden" || env.Data.Entries[1].Name != "alpha" {
		t.Fatalf("entries not sorted: %+v", env.Data.Entries)
	}
}

// TestV2FsList_DefaultsToHome verifies that an empty path defaults to the
// daemon's home directory.
func TestV2FsList_DefaultsToHome(t *testing.T) {
	// Isolate HOME so "default to home" resolves to a temp dir we list, rather
	// than enumerating the developer's real home directory.
	home := testsupport.IsolateHome(t)
	ts, _ := newWizardTestServer(t, func(proto.RebuildArgs) (proto.RebuildReply, error) {
		return proto.RebuildReply{}, nil
	})
	env := getFsList(t, ts.URL, "")
	if !env.OK {
		t.Fatalf("ok = false; reply = %+v", env.Data)
	}
	if env.Data.Path != home {
		t.Fatalf("Path = %q; want home %q", env.Data.Path, home)
	}
}

// TestV2FsList_DoesNotFollowProtectedHomeSymlink is the v0.1.8 TCC-prompt
// regression test. On macOS, iCloud makes ~/Desktop and ~/Documents SYMLINKS
// into ~/Library/Mobile Documents; the folder browser used to Stat-follow every
// non-dir symlink child to confirm it was a directory, and following one into a
// TCC-protected folder fired a permission prompt on the very first default
// listing — before any user navigation.
//
// We model this in a synthetic home by making the protected-named child a
// symlink to a *regular file*. The Stat-follow is observable by its effect:
//   - old behaviour: Stat the symlink, see a non-dir, DROP it from the listing;
//   - fixed behaviour (darwin): skip the Stat for a protected name, KEEP it.
//
// A same-shaped but non-protected symlink must still be Stat-probed and dropped,
// proving we only skip probing for the protected set.
func TestV2FsList_DoesNotFollowProtectedHomeSymlink(t *testing.T) {
	home := testsupport.IsolateHome(t)
	ts, _ := newWizardTestServer(t, func(proto.RebuildArgs) (proto.RebuildReply, error) {
		return proto.RebuildReply{}, nil
	})

	// A real file that both symlinks point at (so Stat-follow finds a non-dir).
	fileTarget := filepath.Join(home, "target.txt")
	if err := os.WriteFile(fileTarget, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// "Documents" is a protected basename; "NotProtected" is not.
	if err := os.Symlink(fileTarget, filepath.Join(home, "Documents")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fileTarget, filepath.Join(home, "NotProtected")); err != nil {
		t.Fatal(err)
	}

	env := getFsList(t, ts.URL, "")
	if !env.OK {
		t.Fatalf("ok = false; reply = %+v", env.Data)
	}
	present := map[string]bool{}
	for _, e := range env.Data.Entries {
		present[e.Name] = true
	}

	// The non-protected symlink is Stat-probed and, pointing at a file, dropped
	// on every OS.
	if present["NotProtected"] {
		t.Errorf("non-protected symlink-to-file should be Stat-probed and dropped")
	}

	if runtime.GOOS == "darwin" {
		// The protected name is listed WITHOUT a Stat-follow (no TCC prompt).
		if !present["Documents"] {
			t.Errorf("protected home symlink should be listed by name without Stat-follow; entries=%+v", env.Data.Entries)
		}
	} else {
		// Off darwin there is no TCC guard: it is Stat-probed and, being a file,
		// dropped like any other symlink-to-file.
		if present["Documents"] {
			t.Errorf("off darwin, symlink-to-file should be dropped")
		}
	}
}

// TestV2FsList_ExplicitNavIntoProtectedStillLists verifies the fix does not
// break normal use: when the user explicitly navigates INTO a protected folder
// (an explicit click → path=<protected dir>), its contents are still listed.
func TestV2FsList_ExplicitNavIntoProtectedStillLists(t *testing.T) {
	home := testsupport.IsolateHome(t)
	ts, _ := newWizardTestServer(t, func(proto.RebuildArgs) (proto.RebuildReply, error) {
		return proto.RebuildReply{}, nil
	})
	downloads := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(filepath.Join(downloads, "myrepo"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := getFsList(t, ts.URL, downloads)
	if !env.OK {
		t.Fatalf("ok = false; reply = %+v", env.Data)
	}
	if env.Data.Path != downloads {
		t.Fatalf("Path = %q; want %q", env.Data.Path, downloads)
	}
	found := false
	for _, e := range env.Data.Entries {
		if e.Name == "myrepo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("explicit navigation into Downloads should list myrepo; entries=%+v", env.Data.Entries)
	}
}

// TestV2FsList_HomeShortcutsSkipProtected verifies the default home view does
// not Stat-probe protected shortcut candidates on macOS. The Documents/Projects
// shortcuts (which live in the protected Documents folder) are dropped there;
// the Home shortcut always remains.
func TestV2FsList_HomeShortcutsSkipProtected(t *testing.T) {
	home := testsupport.IsolateHome(t)
	ts, _ := newWizardTestServer(t, func(proto.RebuildArgs) (proto.RebuildReply, error) {
		return proto.RebuildReply{}, nil
	})
	if err := os.MkdirAll(filepath.Join(home, "Documents", "Projects"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := getFsList(t, ts.URL, "")
	if !env.OK {
		t.Fatalf("ok = false; reply = %+v", env.Data)
	}
	labels := map[string]bool{}
	for _, sc := range env.Data.Shortcuts {
		labels[sc.Label] = true
	}
	if !labels["Home"] {
		t.Errorf("Home shortcut should always be present; shortcuts=%+v", env.Data.Shortcuts)
	}
	if runtime.GOOS == "darwin" {
		if labels["Documents"] || labels["Projects"] {
			t.Errorf("protected shortcuts must not be probed/offered on darwin; shortcuts=%+v", env.Data.Shortcuts)
		}
	} else {
		if !labels["Documents"] || !labels["Projects"] {
			t.Errorf("off darwin, existing shortcuts should be offered; shortcuts=%+v", env.Data.Shortcuts)
		}
	}
}

// TestV2FsList_NonexistentPathGraceful verifies a bad path returns a graceful
// error in the envelope (HTTP 200), not a hard failure.
func TestV2FsList_NonexistentPathGraceful(t *testing.T) {
	ts, _ := newWizardTestServer(t, func(proto.RebuildArgs) (proto.RebuildReply, error) {
		return proto.RebuildReply{}, nil
	})
	missing := filepath.Join(t.TempDir(), "does-not-exist-xyz")
	env := getFsList(t, ts.URL, missing)
	if !env.OK {
		t.Fatalf("ok should stay true for graceful error; reply = %+v", env.Data)
	}
	if env.Data.Error == "" {
		t.Fatal("expected Error for nonexistent path")
	}
	if len(env.Data.Entries) != 0 {
		t.Fatalf("expected no entries; got %d", len(env.Data.Entries))
	}
}
