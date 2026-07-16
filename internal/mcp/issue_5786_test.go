package mcp

import (
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// #5786: `grafel_find(search="substring", kind_filter="MessageTopic",
// cross_repo=true)` returned {"count":0,"results":null,"total":0} even though
// the group had 13 broker-prefixed SCOPE.MessageTopic entities — a silent-
// empty trap (reads as "no topics exist" when they do).
//
// Root cause (grounded against the pre-fix code path, now superseded):
// handleSearchEntities' substring matcher required `strings.Contains(nameL,
// ql) || strings.Contains(qnL, ql)` for EVERY row, including when kind_filter
// was set. A broker-prefixed topic name like "kafka:feedback-topic" doesn't
// literally contain the query text "kafka topic" (space vs colon), so the
// name-substring gate silently dropped every topic before kind_filter ever
// got a chance to enumerate them. The bm25 path (handleQueryGraph) had
// already been fixed for the identical shape of bug in #5781
// (enumerateByKind, tools.go) — kind_filter there enumerates in-scope kind
// members independent of text score. handleSearchEntities lacked the
// equivalent enumeration, which is the exact divergence between the working
// bm25 path and the broken substring path for this issue's literal repro.
//
// That gap was closed in commit 430821cf8 ("substring find enumerates
// kind_filter for parity with bm25"): when kind_filter is set,
// handleSearchEntities now enumerates every in-scope kind member (name hits
// ranked first, the rest folded in) instead of requiring a literal name
// match. This file adds permanent regression coverage tied to #5786,
// routed through the ACTUAL grafel_find dispatcher (handleCoreFind) — the
// existing #5781 tests called handleSearchEntities/handleQueryGraph
// directly and never exercised the search= discriminator routing the real
// MCP tool call goes through.
func TestIssue5786_FindDispatcher_SubstringKindFilterMessageTopic_CrossRepo(t *testing.T) {
	feedback := &graph.Document{Version: 1, Repo: "event-driven-ai-services-feedback-ingest-service", Entities: []graph.Entity{
		{ID: "t1", Name: "kafka:feedback-topic", Kind: "SCOPE.MessageTopic"},
		{ID: "s1", Name: "FeedbackTopicSchema", Kind: "SCOPE.Schema"}, // must NOT leak in
	}}
	triage := &graph.Document{Version: 1, Repo: "event-driven-ai-services-ai-triage-service", Entities: []graph.Entity{
		{ID: "t2", Name: "kafka:orders-topic", Kind: "SCOPE.MessageTopic"},
	}}
	srv := newTestServer(t, feedback, triage)

	// Literal issue #5786 repro: query text that is NOT a substring of any
	// topic's broker-prefixed name ("kafka topic" vs "kafka:feedback-topic").
	out := callDashboardTool(t, srv.handleCoreFind, map[string]any{
		"group":       "test",
		"query":       "kafka topic",
		"search":      "substring",
		"kind_filter": "MessageTopic",
		"cross_repo":  true,
	})
	count, _ := out["count"].(float64)
	if count != 2 {
		t.Fatalf("expected count:2 (both cross-repo topics), got count:%v results=%v", out["count"], out["results"])
	}
	results, _ := out["results"].([]any)
	names := map[string]bool{}
	for _, r := range results {
		names[r.(map[string]any)["name"].(string)] = true
	}
	if !names["kafka:feedback-topic"] || !names["kafka:orders-topic"] {
		t.Fatalf("expected both cross-repo topics by name, got %v", names)
	}
	if names["FeedbackTopicSchema"] {
		t.Errorf("non-topic entity leaked into MessageTopic kind_filter results")
	}
}

// Regression guard: the bm25 path (search default, i.e. handleQueryGraph via
// the dispatcher) must keep working for the same cross-repo MessageTopic
// query — this was already fixed by #5781 and must not regress.
func TestIssue5786_FindDispatcher_BM25KindFilterMessageTopic_CrossRepo_Unaffected(t *testing.T) {
	feedback := &graph.Document{Version: 1, Repo: "event-driven-ai-services-feedback-ingest-service", Entities: []graph.Entity{
		{ID: "t1", Name: "kafka:feedback-topic", Kind: "SCOPE.MessageTopic"},
	}}
	triage := &graph.Document{Version: 1, Repo: "event-driven-ai-services-ai-triage-service", Entities: []graph.Entity{
		{ID: "t2", Name: "kafka:orders-topic", Kind: "SCOPE.MessageTopic"},
	}}
	srv := newTestServer(t, feedback, triage)

	out := callDashboardTool(t, srv.handleCoreFind, map[string]any{
		"group":       "test",
		"query":       "kafka topic",
		"kind_filter": "MessageTopic",
		"cross_repo":  true,
		"full":        true,
	})
	matches, _ := out["matches"].([]any)
	if len(matches) != 2 {
		t.Fatalf("bm25 path: expected 2 cross-repo topic matches, got %d (%v)", len(matches), matches)
	}
}

// Regression guard: kind_filter=ChannelBinding cross_repo=true (the "works
// fine" comparison case from the issue) must remain unaffected by the
// substring-path fix — narrow fix, not a behavior change for other kinds.
func TestIssue5786_FindDispatcher_SubstringChannelBinding_CrossRepo_Unaffected(t *testing.T) {
	repoA := &graph.Document{Version: 1, Repo: "svc-a", Entities: []graph.Entity{
		{ID: "b1", Name: "orders-binding", Kind: "SCOPE.ChannelBinding"},
	}}
	repoB := &graph.Document{Version: 1, Repo: "svc-b", Entities: []graph.Entity{
		{ID: "b2", Name: "users-binding", Kind: "SCOPE.ChannelBinding"},
	}}
	srv := newTestServer(t, repoA, repoB)

	out := callDashboardTool(t, srv.handleCoreFind, map[string]any{
		"group":       "test",
		"query":       "no literal match here",
		"search":      "substring",
		"kind_filter": "ChannelBinding",
		"cross_repo":  true,
	})
	count, _ := out["count"].(float64)
	if count != 2 {
		t.Fatalf("ChannelBinding: expected count:2, got count:%v results=%v", out["count"], out["results"])
	}
}

// Regression guard: without cross_repo, substring+kind_filter=MessageTopic
// must still enumerate (scoped to the explicit repo_filter), not regress to
// name-only matching.
func TestIssue5786_FindDispatcher_SubstringKindFilterMessageTopic_NonCrossRepo(t *testing.T) {
	doc := &graph.Document{Version: 1, Repo: "svc-a", Entities: []graph.Entity{
		{ID: "t1", Name: "kafka:feedback-topic", Kind: "SCOPE.MessageTopic"},
	}}
	srv := newTestServer(t, doc)

	out := callDashboardTool(t, srv.handleCoreFind, map[string]any{
		"group":       "test",
		"query":       "kafka topic",
		"search":      "substring",
		"kind_filter": "MessageTopic",
		"repo_filter": []any{"svc-a"},
	})
	count, _ := out["count"].(float64)
	if count != 1 {
		t.Fatalf("non-cross-repo: expected count:1, got count:%v results=%v", out["count"], out["results"])
	}
}
