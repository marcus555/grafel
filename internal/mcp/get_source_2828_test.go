package mcp

// get_source_2828_test.go — #2828 token-cost controls for archigraph_get_source
// (the single busiest MCP tool, ~45% of live tool-call token spend).
//
// Coverage:
//   - a default call on a LARGE entity returns a bounded slice (the hard
//     200-line ceiling) AND a visible "truncated — request lines X-Y" marker so
//     the caller can fetch the rest precisely ([no-silent-caps]);
//   - an explicit start_line/end_line returns exactly that range (no marker);
//   - max_lines heads the emitted line count and signals truncation;
//   - the explicit-range payload is materially smaller than the full-span
//     default, while line-number facts survive;
//   - computeSourceSpan unit cases for the span/truncation policy.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/graph"
	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// build2828SourceServer writes an N-line source file on disk and returns a
// server whose single entity spans [start,end].
func build2828SourceServer(t *testing.T, nLines, start, end int) *Server {
	t.Helper()
	dir := t.TempDir()
	lines := make([]string, 0, nLines)
	for i := 1; i <= nLines; i++ {
		lines = append(lines, fmt.Sprintf("statement_%d := compute(%d)", i, i))
	}
	srcPath := dir + "/big.go"
	if err := os.WriteFile(srcPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	doc := &graph.Document{
		Entities: []graph.Entity{
			{ID: "big", Name: "Big", Kind: "Function", SourceFile: "big.go", StartLine: start, EndLine: end},
		},
	}
	srv := newTestServer(t, doc)
	srv.State.groups["test"].Repos["repo1"].Path = dir
	return srv
}

func callGetSource(t *testing.T, s *Server, args map[string]any) string {
	t.Helper()
	req := mcpapi.CallToolRequest{}
	req.Params.Arguments = args
	res, err := s.handleGetNodeSource(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	return extractResultText(t, res)
}

func TestGetSource_2828_DefaultLargeEntityTruncatesWithSignal(t *testing.T) {
	t.Parallel()
	// Entity spans 600 lines — well past the 200-line hard ceiling.
	s := build2828SourceServer(t, 700, 5, 605)
	out := callGetSource(t, s, map[string]any{"group": "test", "entity_id": "big", "context_lines": 0})

	// Bounded: emitted source lines must not exceed the hard ceiling.
	srcLines := 0
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "statement_") {
			srcLines++
		}
	}
	if srcLines > getSourceHardMaxLines {
		t.Fatalf("emitted %d source lines, exceeds hard cap %d", srcLines, getSourceHardMaxLines)
	}
	// Visible truncation marker with a precise continuation hint.
	if !strings.Contains(out, "archigraph: truncated") {
		t.Fatalf("missing truncation marker:\n%s", tail(out))
	}
	if !strings.Contains(out, "start_line=") || !strings.Contains(out, "end_line=") {
		t.Errorf("truncation marker lacks precise continuation range:\n%s", tail(out))
	}
}

func TestGetSource_2828_ExplicitRangeExactAndSmaller(t *testing.T) {
	t.Parallel()
	s := build2828SourceServer(t, 700, 5, 605)

	full := callGetSource(t, s, map[string]any{"group": "test", "entity_id": "big", "context_lines": 0})
	ranged := callGetSource(t, s, map[string]any{
		"group": "test", "entity_id": "big", "start_line": 10, "end_line": 14,
	})

	// Exactly the requested 5 lines (10..14), no others, no truncation marker.
	for _, want := range []int{10, 11, 12, 13, 14} {
		if !strings.Contains(ranged, fmt.Sprintf("statement_%d ", want)) {
			t.Errorf("explicit range missing line %d:\n%s", want, ranged)
		}
	}
	if strings.Contains(ranged, "statement_9 ") || strings.Contains(ranged, "statement_15 ") {
		t.Errorf("explicit range leaked lines outside [10,14]:\n%s", ranged)
	}
	if strings.Contains(ranged, "archigraph: truncated") {
		t.Errorf("explicit range should not be marked truncated:\n%s", ranged)
	}
	if len(ranged) >= len(full) {
		t.Fatalf("explicit range (%d B) not smaller than full-span default (%d B)", len(ranged), len(full))
	}
	t.Logf("get_source bytes: range=%d default=%d (%.1f%% smaller)",
		len(ranged), len(full), 100*float64(len(full)-len(ranged))/float64(len(full)))
}

func TestGetSource_2828_MaxLinesHeadsAndSignals(t *testing.T) {
	t.Parallel()
	s := build2828SourceServer(t, 700, 5, 605)
	out := callGetSource(t, s, map[string]any{
		"group": "test", "entity_id": "big", "context_lines": 0, "max_lines": 12,
	})
	srcLines := strings.Count(out, "statement_")
	// statement_ also appears inside the marker text once (in the hint), so allow
	// a small slack; the body itself must be <= 12.
	if srcLines > 14 {
		t.Fatalf("max_lines=12 emitted ~%d source lines", srcLines)
	}
	if !strings.Contains(out, "archigraph: truncated") {
		t.Fatalf("max_lines truncation not signaled:\n%s", tail(out))
	}
}

func TestComputeSourceSpan_2828_Policy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		e                  graph.Entity
		opts               sourceWindowOpts
		wantStart, wantEnd int
		wantTrunc          bool
	}{
		{
			name:      "small span no truncation",
			e:         graph.Entity{StartLine: 10, EndLine: 20},
			opts:      sourceWindowOpts{contextLines: 2},
			wantStart: 8, wantEnd: 22, wantTrunc: false,
		},
		{
			name:      "huge span hard-capped + truncated",
			e:         graph.Entity{StartLine: 1, EndLine: 1000},
			opts:      sourceWindowOpts{contextLines: 0},
			wantStart: 1, wantEnd: getSourceHardMaxLines, wantTrunc: true,
		},
		{
			name:      "explicit range literal no padding",
			e:         graph.Entity{StartLine: 1, EndLine: 1000},
			opts:      sourceWindowOpts{contextLines: 8, startLine: 50, endLine: 60},
			wantStart: 50, wantEnd: 60, wantTrunc: false,
		},
		{
			name:      "max_lines caps below hard ceiling",
			e:         graph.Entity{StartLine: 1, EndLine: 1000},
			opts:      sourceWindowOpts{contextLines: 0, maxLines: 30},
			wantStart: 1, wantEnd: 30, wantTrunc: true,
		},
		{
			name:      "degenerate span uses fallback window",
			e:         graph.Entity{StartLine: 0, EndLine: 0},
			opts:      sourceWindowOpts{contextLines: 0},
			wantStart: 1, wantEnd: 1 + getSourceFallbackSpan, wantTrunc: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp := computeSourceSpan(&tc.e, tc.opts)
			if sp.start != tc.wantStart || sp.end != tc.wantEnd {
				t.Errorf("span = [%d,%d], want [%d,%d]", sp.start, sp.end, tc.wantStart, tc.wantEnd)
			}
			if sp.truncated != tc.wantTrunc {
				t.Errorf("truncated = %v, want %v", sp.truncated, tc.wantTrunc)
			}
			if tc.wantTrunc && sp.truncationMarker("X") == "" {
				t.Error("expected non-empty truncation marker")
			}
			if !tc.wantTrunc && sp.truncationMarker("X") != "" {
				t.Error("expected empty truncation marker")
			}
		})
	}
}

// tail returns the last ~400 chars of s for compact failure output.
func tail(s string) string {
	if len(s) <= 400 {
		return s
	}
	return "..." + s[len(s)-400:]
}
