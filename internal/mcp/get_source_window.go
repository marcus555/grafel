package mcp

// get_source_window.go — #2828 token-cost policy for archigraph_get_source.
//
// get_source is the single busiest MCP tool (~45% of live tool-call token
// spend). Two cost levers, both ESSENTIAL-data-preserving (less-by-default,
// opt-in for more, every truncation SIGNALED — the [no-silent-caps] rule):
//
//  1. Precise slicing. Callers can pass an explicit line range
//     (start_line/end_line) or a head cap (max_lines) so they pay only for the
//     slice they need instead of the whole entity span + padding. The legacy
//     entity-span behaviour is unchanged when none are passed.
//  2. Visible truncation. The pre-#2828 handler silently clamped any span to a
//     hard 200-line cap with NO signal — a caller reading a 600-line class got
//     the first 200 lines and could not tell it was cut nor how to get the
//     rest. computeSourceSpan now reports whether the emitted window was capped
//     and the full available range, so the handler can append a precise
//     "request lines X-Y" continuation hint.
//
// These params are read off the request map (not declared in the tool schema)
// per the #1639 token-ceiling pattern and are allow-listed in
// schema_contract_ast_test.go. They are pure opt-in: the default call shape is
// byte-for-byte identical to the pre-#2828 behaviour apart from the appended
// truncation marker, which only appears WHEN a window is actually clamped.

import (
	"fmt"

	mcpapi "github.com/mark3labs/mcp-go/mcp"

	"github.com/cajasmota/archigraph/internal/graph"
)

const (
	// getSourceFallbackSpan is the window used when an entity records a
	// degenerate span (end<=start or either bound 0) — common for synthetic /
	// shadow / route entities.
	getSourceFallbackSpan = 60
	// getSourceHardMaxLines is the absolute ceiling on emitted source lines so a
	// single get_source can never become a whole-file dump regardless of the
	// recorded span or an over-large caller max_lines.
	getSourceHardMaxLines = 200
)

// sourceSpan is the resolved emit window plus the metadata needed to signal a
// truncation to the caller without silently dropping data.
type sourceSpan struct {
	start int // first line emitted (1-based, inclusive)
	end   int // last line emitted (1-based, inclusive)

	// truncated is true when the emitted [start,end] window is a strict prefix
	// of the entity's available span because a cap (hard ceiling or caller
	// max_lines) fired. When true, fullEnd is the last line of the available
	// span so the caller can request the remainder precisely.
	truncated bool
	fullEnd   int
}

// sourceWindowOpts carries the caller's opt-in slicing controls. Zero values
// mean "use the entity span" (legacy behaviour).
type sourceWindowOpts struct {
	contextLines int // padding lines on each side of the entity span (default 8)
	startLine    int // explicit window start; 0 = derive from entity
	endLine      int // explicit window end; 0 = derive from entity
	maxLines     int // head cap on emitted lines; 0 = use hard ceiling only
}

// computeSourceSpan resolves the line window to emit for entity e under opts,
// applying the degenerate-span clamp, an optional explicit range, an optional
// caller max_lines head cap, and the absolute hard-max ceiling — and records
// whether the result was truncated plus the full available end so the caller
// can be told how to fetch the rest.
//
// Precedence:
//   - explicit start_line/end_line (either or both) override the entity span;
//   - context_lines padding is applied to the (derived) entity span only, not
//     to an explicit range (an explicit range is taken literally);
//   - max_lines (capped at the hard ceiling) bounds the emitted line count;
//   - the hard ceiling always applies last.
func computeSourceSpan(e *graph.Entity, opts sourceWindowOpts) sourceSpan {
	contextLines := opts.contextLines

	// Derive the base entity span, clamping a degenerate record.
	startLine := e.StartLine
	endLine := e.EndLine
	if startLine < 1 {
		startLine = 1
	}
	if endLine <= startLine || e.StartLine == 0 || e.EndLine == 0 {
		endLine = startLine + getSourceFallbackSpan
	}

	var start, end int
	explicit := opts.startLine > 0 || opts.endLine > 0
	if explicit {
		// Explicit range: taken literally (no context padding). Fill an omitted
		// bound from the entity span so start_line-only / end_line-only work.
		start = opts.startLine
		if start < 1 {
			start = startLine
		}
		end = opts.endLine
		if end < 1 {
			end = endLine
		}
		if end < start {
			end = start
		}
	} else {
		start = startLine - contextLines
		if start < 1 {
			start = 1
		}
		end = endLine + contextLines
	}

	// The full available span end, BEFORE any line-count cap — used to tell the
	// caller how to request the remainder.
	fullEnd := end

	// Effective per-call line cap: the smaller of the hard ceiling and a
	// caller-supplied max_lines (when positive).
	cap := getSourceHardMaxLines
	if opts.maxLines > 0 && opts.maxLines < cap {
		cap = opts.maxLines
	}
	truncated := false
	if end-start+1 > cap {
		end = start + cap - 1
		truncated = true
	}

	return sourceSpan{start: start, end: end, truncated: truncated, fullEnd: fullEnd}
}

// truncationMarker renders the continuation hint appended to a truncated
// get_source body. Returns "" when nothing was truncated (no marker on the
// common case, so the default payload is unchanged).
func (sp sourceSpan) truncationMarker(entityID string) string {
	if !sp.truncated {
		return ""
	}
	nextStart := sp.end + 1
	return fmt.Sprintf(
		"\n# archigraph: truncated — emitted lines %d-%d of %d-%d. "+
			"Request the rest with get_source(entity_id=%q, start_line=%d, end_line=%d).\n",
		sp.start, sp.end, sp.start, sp.fullEnd, entityID, nextStart, sp.fullEnd,
	)
}

// readSourceWindowOpts reads the #2828 opt-in slicing controls off the request
// map. They are intentionally undeclared in the tool schema per the #1639
// token-ceiling pattern (allow-listed in schema_contract_ast_test.go).
func readSourceWindowOpts(req mcpapi.CallToolRequest, contextLines int) sourceWindowOpts {
	return sourceWindowOpts{
		contextLines: contextLines,
		startLine:    argInt(req, "start_line", 0),
		endLine:      argInt(req, "end_line", 0),
		maxLines:     argInt(req, "max_lines", 0),
	}
}
