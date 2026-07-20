package dashboard

// handlers_index_status.go — GET /api/v2/groups/{group}/index-status (#47
// phase 1).
//
// The web dashboard wizard has no visibility into the `enhancing`
// (background enrichment) state or engine CPU/RSS, because internal/dashboard
// never read the status plane (internal/statusfile) — the TUI already reads
// it directly. This is a small, read-only poll endpoint the frontend can hit
// during and after an index job to drive a secondary "enhancing" progress bar
// + CPU/RSS badges, matching the TUI (see internal/statusfile.File.Enhancing
// and the RSSMB/CPUPct doc comment on the same type).
//
// Response shape (wrapped in the standard v2 envelope, see v2_helpers.go):
//
//	{
//	  "ok": true,
//	  "data": {
//	    "engine": { "cpu_pct": <float>, "rss_mb": <int64> },
//	    "repos": [
//	      { "repo_slug": "<slug>", "indexing": <bool>, "enhancing": <bool>,
//	        "entities": <int64>, "relationships": <int64>,
//	        "graph_fb_mtime": <int64-ns> }
//	    ]
//	  }
//	}
//
// repo_slug matches registry.Repo.Slug — the SAME slug progress.Event.RepoSlug
// carries on the SSE /api/index-progress stream (see cmd/grafel/daemon.go's
// indexing job, which stamps RepoSlug: rw.r.Slug), so the frontend can join
// rows from this endpoint against SSE progress rows by slug.
//
// A repo with no status-plane sidecar yet (never indexed by this engine) is
// reported as a zero/false row, not an error — RepoStatusFile's ok=false is
// exactly that "unknown" case. Missing/stale engine-liveness similarly
// degrades to a zero engine block rather than failing the whole response.
// This endpoint is read-only: no writes, no index trigger.

import (
	"net/http"

	"github.com/cajasmota/grafel/internal/daemon"
)

// indexStatusEngine is the engine-global CPU/RSS block.
type indexStatusEngine struct {
	CPUPct float64 `json:"cpu_pct"`
	RSSMB  int64   `json:"rss_mb"`
}

// indexStatusRepo is one repo's status-plane row.
type indexStatusRepo struct {
	RepoSlug      string `json:"repo_slug"`
	Indexing      bool   `json:"indexing"`
	Enhancing     bool   `json:"enhancing"`
	Entities      int64  `json:"entities"`
	Relationships int64  `json:"relationships"`
	GraphFBMtime  int64  `json:"graph_fb_mtime"`
}

// indexStatusReply is the payload for GET /api/v2/groups/{group}/index-status.
type indexStatusReply struct {
	Engine indexStatusEngine `json:"engine"`
	Repos  []indexStatusRepo `json:"repos"`
}

// handleV2IndexStatus — GET /api/v2/groups/{group}/index-status. Read-only
// poll surface exposing per-repo indexing/enhancing state (from the
// statusfile status plane) and engine CPU/RSS (from the engine-liveness
// sidecar) so the web wizard can drive a secondary "enhancing" bar and
// CPU/RAM badges, mirroring the TUI (#47 phase 1).
func (s *Server) handleV2IndexStatus(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if group == "" {
		writeV2Err(w, http.StatusBadRequest, "bad_request", "group required")
		return
	}

	cfg, err := loadGroupConfigBySlug(group)
	if err != nil {
		writeV2Err(w, http.StatusNotFound, "not_found", "group not registered: "+group)
		return
	}

	reply := indexStatusReply{Repos: []indexStatusRepo{}}

	if layout, lerr := daemon.DefaultLayout(); lerr == nil {
		if ef, fresh := daemon.EngineLivenessStatus(layout.Root); fresh && ef != nil {
			reply.Engine = indexStatusEngine{CPUPct: ef.CPUPct, RSSMB: ef.RSSMB}
		}
	}

	for _, repo := range cfg.Repos {
		row := indexStatusRepo{RepoSlug: repo.Slug}
		if f, ok := daemon.RepoStatusFile(repo.Path); ok && f != nil {
			row.Indexing = f.Indexing
			row.Enhancing = f.Enhancing
			row.Entities = f.Entities
			row.Relationships = f.Relationships
			row.GraphFBMtime = f.GraphFBMtime
		}
		reply.Repos = append(reply.Repos, row)
	}

	writeV2JSON(w, http.StatusOK, v2OK(reply))
}
