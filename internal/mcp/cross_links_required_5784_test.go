package mcp

import (
	"context"
	"strings"
	"testing"

	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// cross_links_required_5784_test.go — #5784 Category 3 regression guard:
// grafel_cross_links action=accept|reject REQUIRES candidate_id (previously
// undeclared in the schema — see server.go grafel_cross_links registration
// and handleResolveLinkCandidateAction in tools.go). These tests assert the
// existing runtime enforcement produces a clear, actionable error when
// candidate_id is omitted, now that the param is discoverable in the schema.

// crossLinksTestServer builds a minimal one-group/one-repo server.
func crossLinksTestServer(t *testing.T) *Server {
	t.Helper()
	return coreTestServer(t)
}

func TestCrossLinksAcceptRequiresCandidateID(t *testing.T) {
	srv := crossLinksTestServer(t)
	req := mcpapi.CallToolRequest{}
	req.Params.Arguments = map[string]any{"group": "g", "action": "accept"}
	res, err := srv.handleCrossLinks(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCrossLinks: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected an error result for action=accept without candidate_id, got: %s", resultText(res))
	}
	if msg := resultText(res); !strings.Contains(msg, "candidate_id") {
		t.Errorf("error message should mention candidate_id, got: %s", msg)
	}
}

func TestCrossLinksRejectRequiresCandidateID(t *testing.T) {
	srv := crossLinksTestServer(t)
	req := mcpapi.CallToolRequest{}
	req.Params.Arguments = map[string]any{"group": "g", "action": "reject"}
	res, err := srv.handleCrossLinks(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCrossLinks: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected an error result for action=reject without candidate_id, got: %s", resultText(res))
	}
	if msg := resultText(res); !strings.Contains(msg, "candidate_id") {
		t.Errorf("error message should mention candidate_id, got: %s", msg)
	}
}

// TestCrossLinksSchemaDeclaresRequiredParams asserts the #5784 fix: the
// grafel_cross_links tool schema now declares candidate_id, reason, and
// override_target (previously invisible to an agent reading the schema).
func TestCrossLinksSchemaDeclaresRequiredParams(t *testing.T) {
	srv := crossLinksTestServer(t)
	tools := srv.MCP.ListTools()
	st, ok := tools["grafel_cross_links"]
	if !ok {
		t.Fatal("grafel_cross_links not registered")
	}
	props := st.Tool.InputSchema.Properties
	for _, want := range []string{"candidate_id", "reason", "override_target"} {
		if _, ok := props[want]; !ok {
			t.Errorf("grafel_cross_links schema missing declared param %q", want)
		}
	}
}

// TestDocgenApplyRepairsSubmitSchemaDeclaresRequiredParams asserts the #5784
// fix: grafel_docgen_apply now declares resolution/residual_id (REQUIRED for
// kind=repairs action=submit) plus the rest of the submit-only params.
func TestDocgenApplyRepairsSubmitSchemaDeclaresRequiredParams(t *testing.T) {
	srv := crossLinksTestServer(t)
	tools := srv.MCP.ListTools()
	st, ok := tools["grafel_docgen_apply"]
	if !ok {
		t.Fatal("grafel_docgen_apply not registered")
	}
	props := st.Tool.InputSchema.Properties
	for _, want := range []string{"residual_id", "resolution", "reasoning", "target_entity_id", "module",
		"new_target", "dynamic_reason", "abandon_reason", "source", "repo", "offset"} {
		if _, ok := props[want]; !ok {
			t.Errorf("grafel_docgen_apply schema missing declared param %q", want)
		}
	}
}

// TestDocgenApplyRepairsSubmitRequiresResolutionAndResidualID exercises the
// full umbrella-tool path (kind=repairs, action=submit) and asserts both
// REQUIRED params produce a clear error when missing.
func TestDocgenApplyRepairsSubmitRequiresResolutionAndResidualID(t *testing.T) {
	srv := crossLinksTestServer(t)

	req := mcpapi.CallToolRequest{}
	req.Params.Arguments = map[string]any{"group": "g", "kind": "repairs", "action": "submit"}
	res, err := srv.handleWorkflowDocgenApply(context.Background(), req)
	if err != nil {
		t.Fatalf("handleWorkflowDocgenApply: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected an error result for kind=repairs action=submit without residual_id, got: %s", resultText(res))
	}
	if msg := resultText(res); !strings.Contains(msg, "residual_id") {
		t.Errorf("error message should mention residual_id, got: %s", msg)
	}

	req2 := mcpapi.CallToolRequest{}
	req2.Params.Arguments = map[string]any{"group": "g", "kind": "repairs", "action": "submit", "residual_id": "bogus"}
	res2, err := srv.handleWorkflowDocgenApply(context.Background(), req2)
	if err != nil {
		t.Fatalf("handleWorkflowDocgenApply: %v", err)
	}
	if !res2.IsError {
		t.Fatalf("expected an error result for kind=repairs action=submit without resolution, got: %s", resultText(res2))
	}
	if msg := resultText(res2); !strings.Contains(msg, "resolution") {
		t.Errorf("error message should mention resolution, got: %s", msg)
	}
}
