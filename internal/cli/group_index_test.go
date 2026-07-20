package cli

// group_index_test.go — `grafel group add --index` completion honesty (#5790).
//
// The bug: in split mode the daemon's Rebuild RPC returns the instant the
// rebuild is ENQUEUED, so the CLI reported "indexed": true before the engine had
// built anything. The fix makes indexGroup request WaitForCompletion and only
// report indexed after the daemon confirms real completion (err==nil); on a
// non-completing rebuild it reports an honest pending state, never a false
// indexed:true. These tests drive a stub daemon whose Rebuild handler stands in
// for BOTH modes (the completion contract is entirely serve-side), so the CLI
// behavior is identical whether the real daemon is split or monolith.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/proto"
	"github.com/cajasmota/grafel/internal/daemon/transport"
)

// mockRebuildService is a minimal net/rpc handler exposing only Rebuild. rErr
// models the daemon's WaitForCompletion outcome: nil == the rebuild finished;
// non-nil == it did not confirm completion (timeout / engine-death).
type mockRebuildService struct {
	gotArgs proto.RebuildArgs
	reply   proto.RebuildReply
	rErr    error
}

func (m *mockRebuildService) Rebuild(args *proto.RebuildArgs, reply *proto.RebuildReply) error {
	m.gotArgs = *args
	*reply = m.reply
	return m.rErr
}

// stubRebuildDaemon starts a JSON-RPC server exposing the Rebuild method over
// the platform IPC transport and returns its address (mirrors
// stubLifecycleDaemon in remove_test.go).
func stubRebuildDaemon(t *testing.T, svc *mockRebuildService) string {
	t.Helper()
	var addr string
	if runtime.GOOS == "windows" {
		addr = fmt.Sprintf(`\\.\pipe\ag-rebuild-%d`, stubPipeSeq(t))
	} else {
		dir, err := os.MkdirTemp("", "ag-rb-")
		if err != nil {
			t.Fatalf("mkdirtemp: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		addr = filepath.Join(dir, "d.sock")
	}
	ln, err := transport.Listen(addr)
	if err != nil {
		t.Fatalf("listen %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	srv := rpc.NewServer()
	if err := srv.RegisterName(proto.ServiceName, svc); err != nil {
		t.Fatalf("register: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
	}()
	return addr
}

// TestIndexGroup_ForwardsWaitForCompletion asserts the CLI always asks the
// daemon to block for real completion — without it the daemon would fire-and-
// forget and the "indexed" claim would be premature again.
func TestIndexGroup_ForwardsWaitForCompletion(t *testing.T) {
	svc := &mockRebuildService{}
	sock := stubRebuildDaemon(t, svc)

	completed, _, err := indexGroup("g", sock)
	if err != nil {
		t.Fatalf("indexGroup: %v", err)
	}
	if !completed {
		t.Fatal("want completed=true when the daemon returns a nil Rebuild error")
	}
	if !svc.gotArgs.WaitForCompletion {
		t.Fatal("indexGroup must set WaitForCompletion=true so err==nil means done")
	}
	if !svc.gotArgs.Interactive {
		t.Fatal("an explicit CLI group rebuild must be Interactive (foreground)")
	}
}

// TestIndexGroup_NonCompletionReportsHonestPending asserts a daemon Rebuild
// error (timeout / engine-death) is NOT a hard CLI failure but an honest
// not-completed state carrying the reason — never a false completed=true.
func TestIndexGroup_NonCompletionReportsHonestPending(t *testing.T) {
	svc := &mockRebuildService{rErr: fmt.Errorf("rebuild group=g timed out after 2h0m0s")}
	sock := stubRebuildDaemon(t, svc)

	completed, note, err := indexGroup("g", sock)
	if err != nil {
		t.Fatalf("indexGroup should not hard-error on a non-completing rebuild: %v", err)
	}
	if completed {
		t.Fatal("must NOT report completed=true when the daemon did not confirm completion")
	}
	if !strings.Contains(note, "timed out") {
		t.Fatalf("note = %q, want the daemon's reason", note)
	}
}

// TestGroupAdd_IndexCompletion_ReportsIndexedTrue: the JSON result reports
// indexed:true ONLY when the rebuild really completed (nil daemon error).
func TestGroupAdd_IndexCompletion_ReportsIndexedTrue(t *testing.T) {
	home := withSandboxHome(t)
	repoA := filepath.Join(home, "repos", "alpha")
	makeRepo(t, repoA)

	svc := &mockRebuildService{}
	sock := stubRebuildDaemon(t, svc)

	var buf bytes.Buffer
	cmd := newTestCmd(&buf)
	err := runGroupAddImpl(cmd, "demo", groupAddFlags{
		repoArgs: []string{repoA}, rules: false, mcp: false, runInst: true, doIndex: true, jsonOut: true,
	}, sock)
	if err != nil {
		t.Fatalf("group add: %v\n%s", err, buf.String())
	}

	var res groupAddResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("json output: %v\n%s", err, buf.String())
	}
	if !res.Indexed {
		t.Fatalf("want indexed=true after real completion, got %+v", res)
	}
	if res.IndexPending {
		t.Fatalf("completed rebuild must not be flagged pending, got %+v", res)
	}
}

// TestGroupAdd_IndexTimeout_ReportsHonestNotIndexed: on a non-completing rebuild
// the command still succeeds (group registered) but the result is honest —
// indexed:false + index_pending:true + a note — never a false indexed:true.
func TestGroupAdd_IndexTimeout_ReportsHonestNotIndexed(t *testing.T) {
	home := withSandboxHome(t)
	repoA := filepath.Join(home, "repos", "alpha")
	makeRepo(t, repoA)

	svc := &mockRebuildService{rErr: fmt.Errorf("index engine stopped responding before the group rebuild finished")}
	sock := stubRebuildDaemon(t, svc)

	var buf bytes.Buffer
	cmd := newTestCmd(&buf)
	err := runGroupAddImpl(cmd, "demo", groupAddFlags{
		repoArgs: []string{repoA}, rules: false, mcp: false, runInst: true, doIndex: true, jsonOut: true,
	}, sock)
	if err != nil {
		t.Fatalf("group add must not hard-fail when only the index did not confirm: %v\n%s", err, buf.String())
	}

	var res groupAddResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("json output: %v\n%s", err, buf.String())
	}
	if res.Indexed {
		t.Fatal("must NOT report indexed:true off a rebuild that did not confirm completion (#5790)")
	}
	if !res.IndexPending || res.IndexNote == "" {
		t.Fatalf("want an honest pending state with a note, got %+v", res)
	}
}

// TestGroupAdd_FailedRebuild_ReportsHonestNotIndexed is the end-to-end honesty
// guard for the MUST-FIX (#5790): when the daemon reports the rebuild FAILED
// (StatusError/dead-letter ack surfaced as a "group rebuild failed" error), the
// CLI must NOT claim indexed:true — it reports the honest pending state with the
// failure reason. Companion to the daemon-side ack-status tests.
func TestGroupAdd_FailedRebuild_ReportsHonestNotIndexed(t *testing.T) {
	home := withSandboxHome(t)
	repoA := filepath.Join(home, "repos", "alpha")
	makeRepo(t, repoA)

	svc := &mockRebuildService{rErr: fmt.Errorf("group rebuild failed: boom: OOM-reaped mid-rebuild")}
	sock := stubRebuildDaemon(t, svc)

	var buf bytes.Buffer
	cmd := newTestCmd(&buf)
	err := runGroupAddImpl(cmd, "demo", groupAddFlags{
		repoArgs: []string{repoA}, rules: false, mcp: false, runInst: true, doIndex: true, jsonOut: true,
	}, sock)
	if err != nil {
		t.Fatalf("group add should not hard-fail when only the rebuild failed: %v\n%s", err, buf.String())
	}

	var res groupAddResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("json output: %v\n%s", err, buf.String())
	}
	if res.Indexed {
		t.Fatal("must NOT report indexed:true off a FAILED rebuild (#5790)")
	}
	if !res.IndexPending || !strings.Contains(res.IndexNote, "group rebuild failed") {
		t.Fatalf("want honest pending state carrying the failure reason, got %+v", res)
	}
}

// TestGroupAdd_IndexParityAcrossModes: the CLI result shape is identical for a
// completing rebuild regardless of daemon mode — the completion contract lives
// entirely serve-side, so the CLI just trusts err==nil. Modeled by two stub
// daemons (one "monolith-synchronous", one "split-then-acked") that both return
// nil; the CLI must produce the same indexed:true / not-pending result.
func TestGroupAdd_IndexParityAcrossModes(t *testing.T) {
	run := func(t *testing.T) groupAddResult {
		home := withSandboxHome(t)
		repoA := filepath.Join(home, "repos", "alpha")
		makeRepo(t, repoA)
		sock := stubRebuildDaemon(t, &mockRebuildService{})
		var buf bytes.Buffer
		cmd := newTestCmd(&buf)
		if err := runGroupAddImpl(cmd, "demo", groupAddFlags{
			repoArgs: []string{repoA}, rules: false, mcp: false, runInst: true, doIndex: true, jsonOut: true,
		}, sock); err != nil {
			t.Fatalf("group add: %v\n%s", err, buf.String())
		}
		var res groupAddResult
		if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
			t.Fatalf("json: %v", err)
		}
		return res
	}

	var monolith, split groupAddResult
	t.Run("monolith", func(t *testing.T) { monolith = run(t) })
	t.Run("split", func(t *testing.T) { split = run(t) })

	if monolith.Indexed != split.Indexed || monolith.IndexPending != split.IndexPending {
		t.Fatalf("CLI result must be identical across modes: monolith=%+v split=%+v", monolith, split)
	}
	if !monolith.Indexed {
		t.Fatalf("both modes should report indexed:true on completion, got %+v", monolith)
	}
}
