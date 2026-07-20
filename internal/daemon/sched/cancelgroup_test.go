package sched

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestCancelGroup_CancelsInFlightLinkPass verifies the v0.1.8 delete-cancel fix
// at the scheduler layer: when a group is deleted, Scheduler.CancelGroup cancels
// the context of that group's IN-FLIGHT link pass (the phantom-edge/link
// enrichment that kept a core pinned after the group was gone), and CancelGroup
// itself returns promptly rather than blocking behind the pass.
//
// The Links callback signals when it has started, then blocks on ctx.Done — so
// the assertion is purely on cancellation propagation, not wall-clock timing.
func TestCancelGroup_CancelsInFlightLinkPass(t *testing.T) {
	linkStarted := make(chan struct{})
	linkCancelled := make(chan struct{})
	s := New(Config{
		Workers:      1,
		LinkDebounce: time.Millisecond,
		Links: func(ctx context.Context, _ string) error {
			close(linkStarted)
			<-ctx.Done() // block until CancelGroup cancels this group's pass
			close(linkCancelled)
			return ctx.Err()
		},
	})
	s.Start()
	defer s.Stop()

	s.scheduleLinks("g1")

	select {
	case <-linkStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("link pass never started")
	}

	// CancelGroup must not block behind the (blocked) link pass.
	done := make(chan struct{})
	go func() { s.CancelGroup("g1"); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CancelGroup blocked — it must signal-and-return, not await the pass")
	}

	select {
	case <-linkCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight link pass ctx was NOT cancelled by CancelGroup")
	}
}

// TestCancelGroup_SupersededLinkPassCancelledThenNoneSurvives covers FINDING 1:
// link passes run on the timer AfterFunc goroutine (not the worker pool), so a
// pass that outlasts LinkDebounce runs CONCURRENTLY with the next re-armed pass.
// A newer scheduleLinks must therefore CANCEL the superseded predecessor (not
// merely overwrite its map entry) — otherwise only the newest token is
// reachable and the older long betweenness/phantom pass runs to completion for a
// deleted group.
//
// Asserts both halves: (1) pass-0's ctx is cancelled the moment pass-1
// supersedes it, and (2) after the supersede a CancelGroup cancels the surviving
// pass-1 — so NO link pass is left running for the deleted group.
func TestCancelGroup_SupersededLinkPassCancelledThenNoneSurvives(t *testing.T) {
	started := []chan struct{}{make(chan struct{}), make(chan struct{})}
	cancelled := []chan struct{}{make(chan struct{}), make(chan struct{})}
	var callN int32

	s := New(Config{
		Workers:      1,
		LinkDebounce: time.Millisecond,
		Links: func(ctx context.Context, _ string) error {
			n := int(atomic.AddInt32(&callN, 1) - 1)
			if n > 1 {
				return nil // ignore any extra re-arms
			}
			close(started[n])
			<-ctx.Done() // block until cancelled (supersede for n=0, CancelGroup for n=1)
			close(cancelled[n])
			return ctx.Err()
		},
	})
	s.Start()
	defer s.Stop()

	// Arm + start pass-0.
	s.scheduleLinks("g")
	<-started[0]

	// Arm pass-1 while pass-0 is still in flight — its AfterFunc must cancel
	// pass-0's token before registering its own.
	s.scheduleLinks("g")
	<-started[1]

	// (1) The superseded pass-0 must be cancelled by the arming of pass-1.
	select {
	case <-cancelled[0]:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded pass-0 was NOT cancelled by the newer scheduleLinks — it runs concurrently to completion for a deleted group (leak)")
	}

	// (2) A group delete now cancels the surviving pass-1 — no pass survives.
	s.CancelGroup("g")
	select {
	case <-cancelled[1]:
	case <-time.After(2 * time.Second):
		t.Fatal("surviving link pass was NOT cancelled by CancelGroup — a link pass survives for the deleted group")
	}
}

// TestCancelGroup_ScopedToGroup verifies cancellation is scoped: cancelling
// group g1 does NOT cancel a different group g2's in-flight link pass.
func TestCancelGroup_ScopedToGroup(t *testing.T) {
	started := map[string]chan struct{}{
		"g1": make(chan struct{}),
		"g2": make(chan struct{}),
	}
	cancelled := map[string]chan struct{}{
		"g1": make(chan struct{}),
		"g2": make(chan struct{}),
	}
	release := make(chan struct{})
	s := New(Config{
		Workers:      1,
		LinkDebounce: time.Millisecond,
		Links: func(ctx context.Context, group string) error {
			close(started[group])
			select {
			case <-ctx.Done():
				close(cancelled[group])
				return ctx.Err()
			case <-release:
				return nil
			}
		},
	})
	s.Start()
	defer s.Stop()
	defer close(release)

	s.scheduleLinks("g1")
	s.scheduleLinks("g2")
	<-started["g1"]
	<-started["g2"]

	s.CancelGroup("g1")

	select {
	case <-cancelled["g1"]:
	case <-time.After(2 * time.Second):
		t.Fatal("g1 pass should have been cancelled")
	}

	// g2 must still be running (not cancelled). Give the cancel a moment to
	// (wrongly) propagate; if g2 were cancelled it would close its channel.
	select {
	case <-cancelled["g2"]:
		t.Fatal("g2 pass was cancelled by a delete of g1 — cancellation leaked across groups")
	case <-time.After(200 * time.Millisecond):
		// expected: g2 still running
	}
}
