package main

// daemon_rebuild_cancel_test.go — FINDING 2 backstop: a stale rebuild whose
// group was deleted must abort at the registry existence check in
// daemonRebuildFuncCore (no tombstone), while a recreated same-name group's
// rebuild runs to completion (no false-cancel).

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/proto"
	"github.com/cajasmota/grafel/internal/registry"
)

// TestRebuild_AbortsWhenGroupDeleted is FINDING 2 test (b): the deterministic
// split-mode close. A KindRebuild dispatched to the async worker for a group
// that is then DELETED (removed from the registry) must abort BEFORE any repo is
// indexed — the registry existence check in daemonRebuildFuncCore returns
// "unknown group" and indexFn is never called. This replaces the removed
// tombstone as the split-mode cancel-before-register backstop.
func TestRebuild_AbortsWhenGroupDeleted(t *testing.T) {
	group := setupTestGroup(t, "stale", []string{"r1", "r2"})

	// Simulate the delete that raced ahead of the async rebuild worker: the group
	// is gone from the registry by the time the rebuild starts.
	if err := registry.RemoveGroup(group); err != nil {
		t.Fatalf("RemoveGroup: %v", err)
	}

	var indexed int32
	mockIndexFn := func(_, _, _ string, _ []string, _, _ bool, _ ...IndexOption) error {
		atomic.AddInt32(&indexed, 1)
		return nil
	}

	_, _, err := daemonRebuildFuncCore(1, proto.RebuildArgs{Group: group}, mockIndexFn, noopLinksFn)
	if err == nil || !strings.Contains(err.Error(), "unknown group") {
		t.Fatalf("rebuild of a deleted group should abort with 'unknown group'; got err=%v", err)
	}
	if got := atomic.LoadInt32(&indexed); got != 0 {
		t.Fatalf("deleted-group rebuild indexed %d repo(s); want 0 — the heavy pass must not run", got)
	}
}

// TestRebuild_RecreatedGroupRunsToCompletion is FINDING 2 test (a): a
// delete→recreate of the same group name ("delete api; recreate api") must let
// the recreate's rebuild run to completion — the registry existence check passes
// because the group was re-registered, and there is no tombstone to spuriously
// cancel it.
func TestRebuild_RecreatedGroupRunsToCompletion(t *testing.T) {
	group := setupTestGroup(t, "api", []string{"r1", "r2"})

	// Delete, then immediately recreate the SAME group name.
	if err := registry.RemoveGroup(group); err != nil {
		t.Fatalf("RemoveGroup: %v", err)
	}
	_ = setupTestGroup(t, "api", []string{"r1", "r2"})

	var indexed int32
	mockIndexFn := func(_, _, _ string, _ []string, _, _ bool, _ ...IndexOption) error {
		atomic.AddInt32(&indexed, 1)
		return nil
	}
	linkCalled := int32(0)
	linksFn := func(_ context.Context, _ string) error {
		atomic.AddInt32(&linkCalled, 1)
		return nil
	}

	rebuilt, _, err := daemonRebuildFuncCore(1, proto.RebuildArgs{Group: group}, mockIndexFn, linksFn)
	if err != nil {
		t.Fatalf("recreate's rebuild should run to completion, not be cancelled; got err=%v", err)
	}
	if len(rebuilt) != 2 || atomic.LoadInt32(&indexed) != 2 {
		t.Fatalf("recreate rebuild indexed %d repo(s) (rebuilt=%d); want 2 — it was wrongly aborted",
			atomic.LoadInt32(&indexed), len(rebuilt))
	}
	if atomic.LoadInt32(&linkCalled) != 1 {
		t.Fatalf("recreate rebuild link pass ran %d times; want 1", atomic.LoadInt32(&linkCalled))
	}
}
