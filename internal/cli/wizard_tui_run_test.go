package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/cajasmota/grafel/internal/cli/wiztui"
	"github.com/cajasmota/grafel/internal/install/detect"
	"github.com/cajasmota/grafel/internal/install/mcpreg"
	"github.com/cajasmota/grafel/internal/install/tooladapter"
	"github.com/cajasmota/grafel/internal/testsupport"
)

// TestWizardUseTUI_NonTTYFallsBack: a non-*os.File writer (e.g. a bytes.Buffer
// used in tests / pipes) must NOT launch the full-screen TUI.
func TestWizardUseTUI_NonTTYFallsBack(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	if wizardUseTUI(&bytes.Buffer{}) {
		t.Error("wizardUseTUI(bytes.Buffer) = true; want false (not a terminal)")
	}
}

// TestWizardUseTUI_DumbTermFallsBack: $TERM=dumb forces the line-based flow.
func TestWizardUseTUI_DumbTermFallsBack(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if wizardUseTUI(os.Stdout) {
		t.Error("wizardUseTUI with TERM=dumb = true; want false")
	}
}

// TestWizardUseTUI_EmptyTermFallsBack: an empty $TERM forces the line-based flow.
func TestWizardUseTUI_EmptyTermFallsBack(t *testing.T) {
	t.Setenv("TERM", "")
	if wizardUseTUI(os.Stdout) {
		t.Error("wizardUseTUI with empty TERM = true; want false")
	}
}

// TestWizardUseTUI_OptOutEnv: GRAFEL_NO_TUI disables the TUI even on a TTY.
func TestWizardUseTUI_OptOutEnv(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("GRAFEL_NO_TUI", "1")
	if wizardUseTUI(os.Stdout) {
		t.Error("GRAFEL_NO_TUI did not disable the TUI")
	}
}

// TestReposForResult_Monorepo: a monorepo action must map to EXACTLY ONE
// registry.Repo rooted at the monorepo path, with the chosen packages
// recorded as Modules — NOT one flattened repo per package (D2/D3: flattening
// breaks the graph model and makes hooksDir stat a non-existent <pkg>/.git).
// This mirrors `monorepo add`, which appends packages into r.Modules on a
// single Repo (internal/cli/monorepo.go newMonorepoAddCmd).
func TestReposForResult_Monorepo(t *testing.T) {
	root := t.TempDir()
	class := mustClassify(root)
	// Force a monorepo-style result with multiple chosen packages.
	r := wiztui.Result{Action: wiztui.ActionMonorepo, Repos: []string{"services/auth", "packages/ui"}}
	repos := reposForResult(class, r)
	if len(repos) != 1 {
		t.Fatalf("got %d repos, want 1 (single repo with Modules, not one per package)", len(repos))
	}
	got := repos[0]
	if got.Path != class.AbsPath {
		t.Errorf("path = %q, want monorepo root %q", got.Path, class.AbsPath)
	}
	base := filepath.Base(class.AbsPath)
	if got.Slug != base {
		t.Errorf("slug = %q, want root-based slug %q", got.Slug, base)
	}
	wantModules := []string{"packages/ui", "services/auth"}
	gotModules := append([]string(nil), got.Modules...)
	sort.Strings(gotModules)
	if !reflect.DeepEqual(gotModules, wantModules) {
		t.Errorf("modules = %v, want %v", gotModules, wantModules)
	}
}

// TestMCPRegistrationReusesToolsSelection is the regression test for #44 (ask
// AI-tools once): the wizard used to ask about AI tools/agents TWICE — once
// for rules-scaffolding (promptTools, captured into toolIDs/cfg.Tools) and
// again on a separate in-TUI "Configure MCP for which tools?" screen (scrMCP)
// that set wiztui.Result.MCPTools and could only ever NARROW the first
// choice. That screen is gone; makeIndexFunc must no longer read
// r.MCPTools at all — MCP registration reuses the SAME single toolIDs
// selection instead. This test drives makeIndexFunc directly with a stray,
// narrower r.MCPTools value (exactly what the old scrMCP picker would have
// produced) to prove it is now ignored: with toolIDs = {claude, cursor} but
// r.MCPTools = {claude} only, the grafel MCP server must still be registered
// in BOTH tools' config files, not just the one the (now-removed) second
// screen would have picked.
func TestMCPRegistrationReusesToolsSelection(t *testing.T) {
	dir := testsupport.IsolateHome(t)
	repo := t.TempDir()

	var out, errOut bytes.Buffer
	class, _ := detect.ClassifyPath(repo)
	opts := wizardOptions{NoIndex: true, RunInstall: true}
	toolIDs := []string{"claude", "cursor"}
	idxFn := makeIndexFunc(&out, &errOut, class, opts, toolIDs)

	staleMCPSelection := []string{"claude"} // what a stray scrMCP screen would have narrowed to
	res := wiztui.Result{
		Action:    wiztui.ActionSingle,
		Repos:     []string{repo},
		GroupName: "mcp-once-test-group-" + filepath.Base(dir),
		MCPTools:  &staleMCPSelection,
	}

	evCh, outCh := idxFn(res)
	for range evCh {
	}
	outcome := <-outCh
	if outcome.Err != nil {
		t.Fatalf("IndexFunc returned error: %v", outcome.Err)
	}

	wantPaths := map[string]bool{}
	for _, id := range toolIDs {
		a, ok := tooladapter.Lookup(id)
		if !ok || !a.SupportsMCP() {
			t.Fatalf("tool %q unexpectedly has no MCP support in this test", id)
		}
		p, err := mcpreg.SettingsPath(a.MCPTool())
		if err != nil {
			t.Fatalf("SettingsPath(%s): %v", id, err)
		}
		wantPaths[p] = true
	}

	if outcome.Install.MCP != len(wantPaths) {
		t.Errorf("Install.MCP = %d, want %d (claude+cursor both registered; a stray Result.MCPTools must no longer narrow the selection)",
			outcome.Install.MCP, len(wantPaths))
	}
}
