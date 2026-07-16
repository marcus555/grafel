package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/links"
	"github.com/cajasmota/grafel/internal/registry"
	"github.com/cajasmota/grafel/internal/testsupport"
	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// analysis_merges_test.go — dispatch tests for the ANALYSIS-cluster canonical
// tools (#5546/#5550). Each test asserts that a discriminator value on the new
// canonical handler produces the same output as the absorbed handler it routes
// to (order-insensitive via normalizeForCompare, defined in core_merges_test.go).
// Helpers coreTestServer / callBare / assertSameDispatch are shared from there.

// 1. grafel_debt kind= → dead_code/find_dead_code/cycles/import_cycles/stubs/impure/license.
func TestAnalysisDebtDispatch(t *testing.T) {
	srv := coreTestServer(t)
	g := map[string]any{"group": "g"}
	with := func(kind string) map[string]any {
		return map[string]any{"group": "g", "kind": kind}
	}
	stub := map[string]any{"group_v3": "g", "group_oracle": "g"}
	stubKind := map[string]any{"group_v3": "g", "group_oracle": "g", "kind": "stubs"}
	assertSameDispatch(t, "kind=dead_code", srv.handleAnalysisDebt, with("dead_code"), srv.handleDeadCode, g)
	assertSameDispatch(t, "kind=default", srv.handleAnalysisDebt, g, srv.handleDeadCode, g)
	assertSameDispatch(t, "kind=find_dead_code", srv.handleAnalysisDebt, with("find_dead_code"), srv.handleFindDeadCode, g)
	assertSameDispatch(t, "kind=cycles", srv.handleAnalysisDebt, with("cycles"), srv.handleQualityCycles, g)
	assertSameDispatch(t, "kind=import_cycles", srv.handleAnalysisDebt, with("import_cycles"), srv.handleModuleCyclesSidecar, g)
	assertSameDispatch(t, "kind=stubs", srv.handleAnalysisDebt, stubKind, srv.handleStubDetector, stub)
	assertSameDispatch(t, "kind=impure", srv.handleAnalysisDebt, with("impure"), srv.handlePureFunctions, g)
	assertSameDispatch(t, "kind=license", srv.handleAnalysisDebt, with("license"), srv.handleLicenseAudit, g)
}

// 2. grafel_security kind= → findings/secrets/auth_coverage.
func TestAnalysisSecurityDispatch(t *testing.T) {
	srv := coreTestServer(t)
	g := map[string]any{"group": "g"}
	with := func(kind string) map[string]any {
		return map[string]any{"group": "g", "kind": kind}
	}
	assertSameDispatch(t, "kind=findings", srv.handleAnalysisSecurity, with("findings"), srv.handleSecurityFindings, g)
	assertSameDispatch(t, "kind=default", srv.handleAnalysisSecurity, g, srv.handleSecurityFindings, g)
	assertSameDispatch(t, "kind=secrets", srv.handleAnalysisSecurity, with("secrets"), srv.handleSecrets, g)
	assertSameDispatch(t, "kind=auth_coverage", srv.handleAnalysisSecurity, with("auth_coverage"), srv.handleAuthCoverage, g)
}

// 3. grafel_test_analysis kind= → coverage/reachability/contract_effectiveness/coverage_effectiveness.
func TestAnalysisTestDispatch(t *testing.T) {
	srv := coreTestServer(t)
	g := map[string]any{"group": "g"}
	with := func(kind string) map[string]any {
		return map[string]any{"group": "g", "kind": kind}
	}
	assertSameDispatch(t, "kind=coverage", srv.handleAnalysisTest, with("coverage"), srv.handleTestCoverage, g)
	assertSameDispatch(t, "kind=default", srv.handleAnalysisTest, g, srv.handleTestCoverage, g)
	assertSameDispatch(t, "kind=reachability", srv.handleAnalysisTest, with("reachability"), srv.handleTestReachability, g)
	assertSameDispatch(t, "kind=contract_effectiveness", srv.handleAnalysisTest, with("contract_effectiveness"), srv.handleContractTestEffectiveness, g)
	assertSameDispatch(t, "kind=coverage_effectiveness", srv.handleAnalysisTest, with("coverage_effectiveness"), srv.handleCoverageEffectiveness, g)
}

// 4. grafel_patterns kind= → code (agent store) / graph / template.
func TestAnalysisPatternsDispatch(t *testing.T) {
	srv := coreTestServer(t)
	// code: handlePatterns reads its own action=; query is the read path.
	codeArgs := map[string]any{"group": "g", "action": "query", "text": "x"}
	assertSameDispatch(t, "kind=code", srv.handleAnalysisPatterns,
		map[string]any{"group": "g", "kind": "code", "action": "query", "text": "x"},
		srv.handlePatterns, codeArgs)
	assertSameDispatch(t, "kind=default", srv.handleAnalysisPatterns, codeArgs, srv.handlePatterns, codeArgs)
	// graph: dispatcher defaults action=list.
	assertSameDispatch(t, "kind=graph", srv.handleAnalysisPatterns,
		map[string]any{"group": "g", "kind": "graph"},
		srv.handleGraphPatterns, map[string]any{"group": "g", "action": "list"})
	// template.
	assertSameDispatch(t, "kind=template", srv.handleAnalysisPatterns,
		map[string]any{"group": "g", "kind": "template"},
		srv.handleTemplatePatterns, map[string]any{"group": "g"})
}

// 4b. Regression for #5784 bug 1: grafel_patterns kind=template must NOT
// clobber handleTemplatePatterns's own `kind` literal-type filter (values
// like log_format/sql). Before the fix, the umbrella discriminator value
// "template" is passed straight through as the inner filter, so it never
// matches any real entry and `patterns` comes back empty even though
// `by_kind` shows real data — the exact live symptom from the issue.
func TestAnalysisPatternsTemplateKindNotClobbered(t *testing.T) {
	srv := coreTestServer(t)
	writeTemplatePatternSidecar(t, "g", templatePatternSidecarDoc{
		Version: 1,
		Method:  "test",
		Total:   2,
		ByKind:  map[string]int{"log_format": 1, "sql": 1},
		Entries: []templatePatternSidecarEntry{
			{Repo: "r1", SourceFile: "a.go", Line: 10, Kind: "log_format", Tag: "info", Literal: "starting %s"},
			{Repo: "r1", SourceFile: "b.go", Line: 20, Kind: "sql", Tag: "select", Literal: "SELECT * FROM x"},
		},
	})
	out := callBare(t, srv.handleAnalysisPatterns, map[string]any{"group": "g", "kind": "template"})
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("result not JSON object: %v (%s)", err, out)
	}
	var patterns []any
	if err := json.Unmarshal(obj["patterns"], &patterns); err != nil {
		t.Fatalf("patterns not an array: %v (%s)", err, out)
	}
	if len(patterns) == 0 {
		t.Fatalf("kind=template returned empty patterns despite non-empty by_kind sidecar data: %s", out)
	}
}

// writeTemplatePatternSidecar writes a <group>-links-template-patterns.json
// sidecar under $HOME (via t.Setenv, matching sidecarPath's resolution) so
// handleTemplatePatterns finds real data instead of the "missing" fallback.
func writeTemplatePatternSidecar(t *testing.T, group string, doc templatePatternSidecarDoc) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".grafel", "groups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sidecar dir: %v", err)
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	path := filepath.Join(dir, group+"-links-template-patterns.json")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
}

// 5. grafel_findings action= → list / save.
func TestAnalysisFindingsDispatch(t *testing.T) {
	srv := coreTestServer(t)
	g := map[string]any{"group": "g"}
	assertSameDispatch(t, "action=list", srv.handleAnalysisFindings,
		map[string]any{"group": "g", "action": "list"}, srv.handleListFindings, g)
	assertSameDispatch(t, "action=default", srv.handleAnalysisFindings, g, srv.handleListFindings, g)
	// save: handleSaveResult requires question + answer. The result is
	// {"path": "<memDir>/<ts>-<hash>.json"}; the <ts> segment is wall-clock and
	// two independent saves can straddle a second boundary (flaky on slow CI),
	// so we compare the saved path with its volatile timestamp normalized away
	// rather than pinning identical filenames.
	save := map[string]any{"group": "g", "question": "q", "answer": "a"}
	assertSameSaveDispatch(t, "action=save", srv.handleAnalysisFindings,
		map[string]any{"group": "g", "action": "save", "question": "q", "answer": "a"},
		srv.handleSaveResult, save)
}

// savedPathTimestamp matches the leading "<YYYYMMDDThhmmssZ>-" of a saved
// findings filename (see handleSaveResult). The timestamp is wall-clock and
// therefore non-deterministic between two independent saves; the trailing hash
// segment is deterministic (sha256 of question+answer).
var savedPathTimestamp = regexp.MustCompile(`/\d{8}T\d{6}Z-`)

// normalizeSaveResult rewrites the volatile timestamp in a {"path": ...} save
// result to a fixed sentinel so two genuinely-equivalent saves compare equal,
// while still asserting that a path was returned and that the deterministic
// (memDir + hash) portion matches. Non-object/error results pass through.
func normalizeSaveResult(t *testing.T, s string) string {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return s // not a JSON object (error/text result) — compare verbatim
	}
	raw, ok := obj["path"]
	if !ok {
		t.Errorf("save result missing path key: %s", s)
		return s
	}
	var path string
	if err := json.Unmarshal(raw, &path); err != nil {
		t.Errorf("save result path not a string: %s", raw)
		return s
	}
	path = savedPathTimestamp.ReplaceAllString(path, "/<ts>-")
	obj["path"], _ = json.Marshal(path)
	out, _ := json.Marshal(obj)
	return string(out)
}

// assertSameSaveDispatch is assertSameDispatch specialized for the findings
// save path: it verifies the canonical dispatcher routes to handleSaveResult
// with the same args, comparing the structural result with the non-deterministic
// timestamp in the saved filename normalized away (see normalizeSaveResult).
func assertSameSaveDispatch(t *testing.T, label string,
	canonical func(context.Context, mcpapi.CallToolRequest) (*mcpapi.CallToolResult, error), canonArgs map[string]any,
	old func(context.Context, mcpapi.CallToolRequest) (*mcpapi.CallToolResult, error), oldArgs map[string]any) {
	t.Helper()
	got := normalizeSaveResult(t, callBare(t, canonical, canonArgs))
	want := normalizeSaveResult(t, callBare(t, old, oldArgs))
	if got != want {
		t.Errorf("%s: canonical dispatch differs from absorbed handler\n got=%s\nwant=%s", label, got, want)
	}
}

// 6. grafel_diff aspect= → response_shape/payload/auth/literals/refs.
// The return is a discriminated union keyed by `aspect`: we compare the
// canonical result with the absorbed handler's result after STRIPPING the
// injected aspect key, then separately assert the aspect key is present + correct.
func TestAnalysisDiffDispatch(t *testing.T) {
	srv := coreTestServer(t)
	cross := func(aspect string) map[string]any {
		return map[string]any{"group_oracle": "g", "group_v3": "g", "aspect": aspect}
	}
	crossBare := map[string]any{"group_oracle": "g", "group_v3": "g"}

	type diffCase struct {
		aspect  string
		canon   map[string]any
		old     func(context.Context, mcpapi.CallToolRequest) (*mcpapi.CallToolResult, error)
		oldArgs map[string]any
	}
	cases := []diffCase{
		{"response_shape", cross("response_shape"), srv.handleResponseShapeDiff, crossBare},
		{"payload", map[string]any{"group": "g", "aspect": "payload"}, srv.handlePayloadDrift, map[string]any{"group": "g"}},
		{"auth", cross("auth"), srv.handleAuthPostureDiff, crossBare},
		{"literals", map[string]any{"group_oracle": "g", "group_v3": "g", "set": "page_slugs", "aspect": "literals"}, srv.handleLiteralParity, map[string]any{"group_oracle": "g", "group_v3": "g", "set": "page_slugs"}},
		{"refs", map[string]any{"group": "g", "repo": "r1", "ref_a": "main", "ref_b": "main", "aspect": "refs"}, srv.handleDiffRefs, map[string]any{"group": "g", "repo": "r1", "ref_a": "main", "ref_b": "main"}},
	}
	for _, c := range cases {
		got := callBare(t, srv.handleAnalysisDiff, c.canon)
		want := callBare(t, c.old, c.oldArgs)
		assertDiffAspect(t, c.aspect, got, want)
	}

	// default aspect=response_shape.
	gotDefault := callBare(t, srv.handleAnalysisDiff, crossBare)
	wantDefault := callBare(t, srv.handleResponseShapeDiff, crossBare)
	assertDiffAspect(t, "response_shape", gotDefault, wantDefault)
}

// 6b. Regression for #5784 bug 3: stampAspect edits res.Content, but the
// payload/response_shape/auth/literals handlers return via jsonResult, which
// stashes the structured value on res.StructuredContent (the "deferred"
// path). wrap() (server.go) re-marshals the FINAL wire bytes from that
// deferred value, discarding the res.Content edit stampAspect made — so
// going through the real registered tool (callTool, which invokes wrap())
// the "aspect" key is silently dropped for every aspect except "refs" (which
// uses mcpapi.NewToolResultText directly, no deferred value). callBare above
// doesn't exercise this because it never calls wrap(). This test does.
func TestAnalysisDiffAspectStampSurvivesWrap(t *testing.T) {
	testsupport.IsolateHome(t)
	dir := t.TempDir()
	repo := filepath.Join(dir, "r1")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGraph(t, repo, fixtureDoc("r1"))
	regPath := makeRegistry(t, dir, map[string]map[string]string{"g": {"r1": repo}})
	srv, err := NewServer(Config{RegistryPath: regPath})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(srv.Close)

	// handleDiffRefs (aspect=refs) resolves the repo path via the
	// internal/registry package directly (diffToolRepoPath), independent of
	// the mcp.Registry loaded above — register "g" there too so the
	// same-ref fast path in handleDiffRefs can find repo "r1".
	cfgPath, err := registry.ConfigPathFor("g")
	if err != nil {
		t.Fatalf("registry.ConfigPathFor: %v", err)
	}
	cfg := &registry.GroupConfig{Name: "g", Repos: []registry.Repo{{Slug: "r1", Path: repo}}}
	if err := registry.SaveGroupConfig(cfgPath, cfg); err != nil {
		t.Fatalf("registry.SaveGroupConfig: %v", err)
	}
	if err := registry.AddGroup("g", cfgPath); err != nil {
		t.Fatalf("registry.AddGroup: %v", err)
	}

	writePayloadDriftSidecar(t, "g")

	assertWrappedAspect := func(aspect string, args map[string]any) {
		t.Helper()
		res := callTool(t, srv, "grafel_diff", args)
		text := resultText(res)
		// Non-deferred results (aspect=refs, built via mcpapi.NewToolResultText)
		// carry a trailing "\n# elapsed_ms=N\n" comment appended by
		// appendElapsedTrailer; strip it before parsing. Deferred results
		// (aspect=payload) fold elapsed_ms into the JSON object itself and have
		// no trailer.
		if i := strings.Index(text, "\n# elapsed_ms="); i >= 0 {
			text = text[:i]
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(text), &obj); err != nil {
			t.Fatalf("aspect=%s: result not a JSON object: %v (%s)", aspect, err, text)
		}
		a, ok := obj["aspect"]
		if !ok {
			t.Errorf("aspect=%s: wrapped result missing injected aspect key: %s", aspect, text)
			return
		}
		if string(a) != `"`+aspect+`"` {
			t.Errorf("aspect=%s: wrapped result aspect=%s, want %q", aspect, a, aspect)
		}
	}

	// payload rides the deferred (StructuredContent) path — this is the RED
	// case pre-fix.
	assertWrappedAspect("payload", map[string]any{"group": "g", "aspect": "payload"})
	// refs uses mcpapi.NewToolResultText directly (no deferred value) and must
	// keep working post-fix.
	assertWrappedAspect("refs", map[string]any{
		"group": "g", "repo": "r1", "ref_a": "main", "ref_b": "main", "aspect": "refs",
	})
}

// writePayloadDriftSidecar writes a minimal payload-drift findings sidecar
// under the caller's already-isolated $HOME so handlePayloadDrift returns a
// real JSON object instead of the "no sidecar" error result — the error path
// short-circuits stampAspect (res.IsError) and would mask the #5784 bug 3
// regression this test targets. Callers must isolate $HOME themselves (e.g.
// via testsupport.IsolateHome) before calling this.
func writePayloadDriftSidecar(t *testing.T, group string) {
	t.Helper()
	paths, err := links.PathsFor("", group)
	if err != nil {
		t.Fatalf("links.PathsFor: %v", err)
	}
	sidecarPath := links.DriftSidecarPath(paths)
	if err := os.MkdirAll(filepath.Dir(sidecarPath), 0o755); err != nil {
		t.Fatalf("mkdir sidecar dir: %v", err)
	}
	doc := links.DriftDocument{
		Version:     1,
		Method:      "test",
		Group:       group,
		Total:       1,
		SchemaCount: 1,
		Findings: []links.SchemaDrift{
			{
				EndpointID:   "r1::e1",
				EndpointName: "http:POST:/api/x",
				Direction:    "request",
				Severity:     "high",
				DriftClass:   "schema",
				Confidence:   0.9,
				Explanation:  "test finding",
			},
		},
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	if err := os.WriteFile(sidecarPath, buf, 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
}

// assertDiffAspect verifies the canonical grafel_diff result equals the absorbed
// handler's result once the injected aspect key is removed, and that the aspect
// key was present with the expected value. Non-JSON-object (error) results pass
// through unchanged, in which case got must equal want verbatim.
func assertDiffAspect(t *testing.T, aspect, got, want string) {
	t.Helper()
	var gotObj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
		// Not a JSON object (error/text result) — stampAspect is a no-op, so
		// the canonical result must be byte-identical to the absorbed handler.
		if got != want {
			t.Errorf("aspect=%s (non-object): canonical=%q want=%q", aspect, got, want)
		}
		return
	}
	a, ok := gotObj["aspect"]
	if !ok {
		t.Errorf("aspect=%s: result missing injected aspect key", aspect)
		return
	}
	if string(a) != `"`+aspect+`"` {
		t.Errorf("aspect=%s: injected aspect=%s, want %q", aspect, a, aspect)
	}
	delete(gotObj, "aspect")
	stripped, err := json.Marshal(gotObj)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	// want is the absorbed handler's native JSON object; re-marshal it through
	// the same map round-trip so key ordering matches.
	var wantObj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		t.Fatalf("aspect=%s: absorbed handler result not a JSON object: %v", aspect, err)
	}
	wantNorm, _ := json.Marshal(wantObj)
	if string(stripped) != string(wantNorm) {
		t.Errorf("aspect=%s: stripped canonical differs from absorbed handler\n got=%s\nwant=%s", aspect, stripped, wantNorm)
	}
}

// 7. All six ANALYSIS canonical tools are registered (#5546/#5550).
func TestAnalysisCanonicalToolsRegistered(t *testing.T) {
	srv := coreTestServer(t)
	registered := map[string]bool{}
	for _, st := range srv.MCP.ListTools() {
		registered[st.Tool.Name] = true
	}
	for _, n := range []string{
		"grafel_debt", "grafel_security", "grafel_test_analysis",
		"grafel_patterns", "grafel_findings", "grafel_diff",
	} {
		if !registered[n] {
			t.Errorf("ANALYSIS canonical tool %q not registered", n)
		}
	}
}
