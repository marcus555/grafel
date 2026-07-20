package dashboard

// v2_graph_stream_gzip_test.go — the SSE graph stream is gzip-compressed for
// clients that send Accept-Encoding: gzip (#49). The stream is the exact path a
// large cold corpus uses, and its JSON is highly repetitive (repeated keys,
// kind strings, repo slugs) so gzip is ~8-12x here. Compression MUST preserve
// progressive delivery: each SSE event is gzip-flushed AND http-flushed so it
// reaches the client promptly rather than buffering to the end.

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// flushRecorder is an http.ResponseWriter + http.Flusher that records how many
// times Flush was called and lets the test snapshot the bytes written so far.
type flushRecorder struct {
	hdr     http.Header
	buf     bytes.Buffer
	flushes int
}

func newFlushRecorder() *flushRecorder { return &flushRecorder{hdr: http.Header{}} }

func (f *flushRecorder) Header() http.Header         { return f.hdr }
func (f *flushRecorder) Write(b []byte) (int, error) { return f.buf.Write(b) }
func (f *flushRecorder) WriteHeader(int)             {}
func (f *flushRecorder) Flush()                      { f.flushes++ }

func (f *flushRecorder) snapshot() []byte {
	b := f.buf.Bytes()
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gunzip read: %v", err)
	}
	return string(out)
}

// gunzipPartial decodes a gzip stream that has been sync-FLUSHED but not yet
// closed (no trailer). It returns whatever has been decoded so far, tolerating
// the expected io.ErrUnexpectedEOF at the flush boundary. This is exactly what a
// live client sees mid-stream, so it lets the test prove progressive delivery.
func gunzipPartial(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip.NewReader (partial): %v", err)
	}
	zr.Multistream(false)
	var acc bytes.Buffer
	buf := make([]byte, 4096)
	for {
		n, rerr := zr.Read(buf)
		acc.Write(buf[:n])
		if rerr != nil {
			// EOF / ErrUnexpectedEOF at the flush boundary is expected — the
			// bytes returned before it are the progressively-delivered payload.
			break
		}
	}
	return acc.String()
}

// TestSSEGzipWriter_FlushIsIncremental verifies the flush-compatible gzip
// wrapper: after each Flush() the bytes written so far already decompress to
// the events emitted so far (progressive delivery), and the underlying
// http.Flusher is flushed once per event.
func TestSSEGzipWriter_FlushIsIncremental(t *testing.T) {
	rec := newFlushRecorder()
	sw, cleanup := newSSEGzipWriter(rec, rec)

	// Event 1.
	writeV2SSEEvent(sw, "connected", `{"subscribed_at":1}`)
	sw.Flush()
	if rec.flushes != 1 {
		t.Fatalf("after event 1: underlying flushes = %d, want 1", rec.flushes)
	}
	afterFirst := rec.snapshot()
	if got := gunzipPartial(t, afterFirst); !bytesContains(got, "event: connected") {
		t.Fatalf("after first flush the stream does not yet decode the connected event; got %q", got)
	}

	// Event 2.
	writeV2SSEEvent(sw, "meta", `{"total_nodes":2}`)
	sw.Flush()
	if rec.flushes != 2 {
		t.Fatalf("after event 2: underlying flushes = %d, want 2 (per-event flush)", rec.flushes)
	}

	cleanup() // closes the gzip writer (trailer).

	final := gunzip(t, rec.snapshot())
	if !bytesContains(final, "event: connected") || !bytesContains(final, "event: meta") {
		t.Fatalf("final decoded stream missing events: %q", final)
	}
}

func bytesContains(hay, needle string) bool { return bytes.Contains([]byte(hay), []byte(needle)) }

// TestGraphStream_GzipCompressed verifies the end-to-end handler compresses the
// SSE stream when the client sends Accept-Encoding: gzip: the response carries
// Content-Encoding: gzip + Vary, and decompresses to the full connected → meta
// → chunk → done event sequence with the same shape as the uncompressed path.
func TestGraphStream_GzipCompressed(t *testing.T) {
	const n = 1800 // > 2 chunks
	grp := makeStreamTestGroup(n)
	ts := newV2GraphTestServerWithGroup(t, grp)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v2/graph/testgrp/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	// A transport that does NOT auto-add/-decode gzip, so we observe the raw
	// Content-Encoding and body exactly as the server sent them.
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ce := resp.Header.Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", ce)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" || ct[:len("text/event-stream")] != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if v := resp.Header.Get("Vary"); v == "" {
		t.Fatalf("Vary header missing, want Accept-Encoding")
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	events := readSSE(t, gunzip(t, raw))
	if len(events) < 4 {
		t.Fatalf("want connected+meta+chunk+done, got %d events", len(events))
	}
	if events[0].Type != "connected" {
		t.Errorf("first event = %q, want connected", events[0].Type)
	}
	if events[1].Type != "meta" {
		t.Errorf("second event = %q, want meta", events[1].Type)
	}
	if last := events[len(events)-1]; last.Type != "done" {
		t.Errorf("last event = %q, want done", last.Type)
	}

	var meta v2GraphStreamMeta
	if err := json.Unmarshal([]byte(events[1].Data), &meta); err != nil {
		t.Fatalf("meta unmarshal: %v", err)
	}
	if meta.TotalNodes != n {
		t.Errorf("meta.total_nodes = %d, want %d", meta.TotalNodes, n)
	}

	// Count streamed nodes to confirm the compressed stream carried the graph.
	streamed := 0
	for _, ev := range events {
		if ev.Type != "chunk" {
			continue
		}
		var ch v2GraphStreamChunk
		if err := json.Unmarshal([]byte(ev.Data), &ch); err != nil {
			t.Fatalf("chunk unmarshal: %v", err)
		}
		streamed += len(ch.Nodes)
	}
	if streamed != n {
		t.Errorf("streamed %d nodes over gzip, want %d", streamed, n)
	}
}

// TestGraphStream_UncompressedWithoutGzipHeader verifies a client that does NOT
// accept gzip gets the uncompressed stream exactly as before (no
// Content-Encoding), so the change is opt-in and never regresses that path.
func TestGraphStream_UncompressedWithoutGzipHeader(t *testing.T) {
	grp := makeStreamTestGroup(50)
	ts := newV2GraphTestServerWithGroup(t, grp)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v2/graph/testgrp/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Accept-Encoding", "identity")
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if ce := resp.Header.Get("Content-Encoding"); ce != "" {
		t.Fatalf("Content-Encoding = %q, want empty (uncompressed)", ce)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	// Body must be plain SSE text (not gzip magic bytes 0x1f 0x8b).
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		t.Fatalf("body appears gzip-compressed despite Accept-Encoding: identity")
	}
	events := readSSE(t, string(raw))
	if len(events) < 4 || events[0].Type != "connected" {
		t.Fatalf("uncompressed stream malformed: %d events", len(events))
	}
}
