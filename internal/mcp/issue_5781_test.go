package mcp

// issue_5781_test.go — regression coverage for issue #5781:
// the Kafka topic kind `SCOPE.MessageTopic` was not recognised where
// topic/kind matching happens.
//
//	Bug 1a — isTopic() did not match `SCOPE.MessageTopic`, so the topology
//	         orphan-publisher/subscriber scans returned empty.
//	Bug 1b — `grafel_orient view=topology` defaulted to the orphan-publisher
//	         scan; it now defaults to a channel listing with pub/consumer counts.
//	Bug 2  — the bm25 `grafel_find` path ignored kind_filter, so
//	         `kind_filter=MessageTopic cross_repo=true` returned count:0.

import (
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// --- Bug 1a: isTopic must recognise SCOPE.MessageTopic ----------------------

func TestTopology_MessageTopicKind_OrphanScans(t *testing.T) {
	entities := []graph.Entity{
		{ID: "t1", Name: "orders.created", Kind: "SCOPE.MessageTopic"},   // published, not subscribed → orphan publisher
		{ID: "t2", Name: "ghost.events", Kind: "SCOPE.MessageTopic"},     // subscribed, not published → orphan subscriber
		{ID: "svc", Name: "OrderService", Kind: "SCOPE.Service"},
	}
	rels := []graph.Relationship{
		{ID: "r1", FromID: "svc", ToID: "t1", Kind: "PUBLISHES_TO"},
		{ID: "r2", FromID: "svc", ToID: "t2", Kind: "SUBSCRIBES_TO"},
	}
	srv := newTestServer(t, minDoc(entities, rels))

	pub := callDashboardTool(t, srv.handleTopologyOrphanPublishers, map[string]any{"group": "test"})
	if got := int(pub["count"].(float64)); got != 1 {
		t.Fatalf("orphan_publishers: expected 1 SCOPE.MessageTopic orphan, got %d", got)
	}
	sub := callDashboardTool(t, srv.handleTopologyOrphanSubscribers, map[string]any{"group": "test"})
	if got := int(sub["count"].(float64)); got != 1 {
		t.Fatalf("orphan_subscribers: expected 1 SCOPE.MessageTopic orphan, got %d", got)
	}
}

// --- Bug 1b: orient view=topology defaults to a channel listing -------------

func TestOrient_TopologyDefaultsToChannelListing(t *testing.T) {
	entities := []graph.Entity{
		{ID: "t1", Name: "orders.created", Kind: "SCOPE.MessageTopic"},
		{ID: "t2", Name: "users.registered", Kind: "SCOPE.MessageTopic"},
		{ID: "pub", Name: "OrderService", Kind: "SCOPE.Service"},
		{ID: "sub", Name: "MailService", Kind: "SCOPE.Service"},
	}
	rels := []graph.Relationship{
		{ID: "r1", FromID: "pub", ToID: "t1", Kind: "PUBLISHES_TO"},
		{ID: "r2", FromID: "sub", ToID: "t1", Kind: "SUBSCRIBES_TO"},
		{ID: "r3", FromID: "pub", ToID: "t2", Kind: "PUBLISHES_TO"},
	}
	srv := newTestServer(t, minDoc(entities, rels))

	// No action → must list the channels, NOT run the orphan-publisher scan.
	out := callDashboardTool(t, srv.handleCoreOrient, map[string]any{
		"group": "test",
		"view":  "topology",
	})
	chAny, ok := out["channels"]
	if !ok {
		t.Fatalf("expected 'channels' listing key in orient view=topology output, got keys %v", keysOf(out))
	}
	channels := chAny.([]any)
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels listed, got %d", len(channels))
	}
	// Verify publisher/consumer counts are exposed per channel.
	byName := map[string]map[string]any{}
	for _, c := range channels {
		m := c.(map[string]any)
		byName[m["topic_name"].(string)] = m
	}
	oc := byName["orders.created"]
	if oc == nil {
		t.Fatal("orders.created missing from channel listing")
	}
	if int(oc["publisher_count"].(float64)) != 1 || int(oc["consumer_count"].(float64)) != 1 {
		t.Errorf("orders.created: want pub=1 consumer=1, got pub=%v consumer=%v",
			oc["publisher_count"], oc["consumer_count"])
	}
	ur := byName["users.registered"]
	if ur == nil {
		t.Fatal("users.registered missing from channel listing")
	}
	if int(ur["publisher_count"].(float64)) != 1 || int(ur["consumer_count"].(float64)) != 0 {
		t.Errorf("users.registered: want pub=1 consumer=0, got pub=%v consumer=%v",
			ur["publisher_count"], ur["consumer_count"])
	}
}

// DELIVERS_TO is synthesised topic→handler (topic is FromID). It must be
// counted toward the topic's consumer_count by FromID, not ToID (#5781 review).
func TestTopology_ChannelsConsumerCount_DeliversTo(t *testing.T) {
	entities := []graph.Entity{
		{ID: "t1", Name: "async.only", Kind: "SCOPE.MessageTopic"},   // only a DELIVERS_TO edge
		{ID: "t2", Name: "both.edges", Kind: "SCOPE.MessageTopic"},   // SUBSCRIBES_TO + DELIVERS_TO, same handler → dedupe
		{ID: "h1", Name: "AsyncHandler", Kind: "SCOPE.Service"},
		{ID: "h2", Name: "BothHandler", Kind: "SCOPE.Service"},
	}
	rels := []graph.Relationship{
		// t1: topic → handler (DELIVERS_TO), no SUBSCRIBES_TO
		{ID: "d1", FromID: "t1", ToID: "h1", Kind: "DELIVERS_TO"},
		// t2: both edges for the SAME handler (DELIVERS_TO is the inverse of SUBSCRIBES_TO)
		{ID: "s2", FromID: "h2", ToID: "t2", Kind: "SUBSCRIBES_TO"},
		{ID: "d2", FromID: "t2", ToID: "h2", Kind: "DELIVERS_TO"},
	}
	srv := newTestServer(t, minDoc(entities, rels))
	out := callDashboardTool(t, srv.handleTopologyChannels, map[string]any{"group": "test"})
	channels := out["channels"].([]any)
	byName := map[string]map[string]any{}
	for _, c := range channels {
		m := c.(map[string]any)
		byName[m["topic_name"].(string)] = m
	}
	if got := int(byName["async.only"]["consumer_count"].(float64)); got != 1 {
		t.Errorf("async.only (DELIVERS_TO only): want consumer_count=1, got %d", got)
	}
	if got := int(byName["both.edges"]["consumer_count"].(float64)); got != 1 {
		t.Errorf("both.edges (SUBSCRIBES_TO+DELIVERS_TO same handler): want deduped consumer_count=1, got %d", got)
	}
}

// Explicit action=channels also works via the topology dispatcher.
func TestTopology_ChannelsAction(t *testing.T) {
	entities := []graph.Entity{
		{ID: "t1", Name: "orders.created", Kind: "SCOPE.MessageTopic"},
	}
	srv := newTestServer(t, minDoc(entities, nil))
	out := callDashboardTool(t, srv.handleTopology, map[string]any{
		"group":  "test",
		"action": "channels",
	})
	if _, ok := out["channels"]; !ok {
		t.Fatalf("action=channels: expected 'channels' key, got %v", keysOf(out))
	}
}

// --- Bug 2: bm25 find honours kind_filter across repos ----------------------

func TestFind_KindFilterMessageTopic_CrossRepo(t *testing.T) {
	alpha := &graph.Document{Version: 1, Repo: "alpha", Entities: []graph.Entity{
		{ID: "t1", Name: "orders.created", Kind: "SCOPE.MessageTopic"},
		{ID: "c1", Name: "ChannelService", Kind: "SCOPE.Service"}, // bm25 matches "channel" but is NOT a topic
	}}
	beta := &graph.Document{Version: 1, Repo: "beta", Entities: []graph.Entity{
		{ID: "t2", Name: "users.registered", Kind: "SCOPE.MessageTopic"},
	}}
	srv := newTestServer(t, alpha, beta)

	check := func(t *testing.T, kindFilter string) {
		out := callDashboardTool(t, srv.handleQueryGraph, map[string]any{
			"group":       "test",
			"query":       "channel",
			"kind_filter": kindFilter,
			"cross_repo":  true,
			"full":        true,
		})
		matches, _ := out["matches"].([]any)
		names := map[string]bool{}
		for _, m := range matches {
			names[m.(map[string]any)["name"].(string)] = true
		}
		if !names["orders.created"] || !names["users.registered"] {
			t.Fatalf("kind_filter=%s: expected both topics across repos, got %v", kindFilter, names)
		}
		if names["ChannelService"] {
			t.Errorf("kind_filter=%s: non-topic ChannelService should be excluded", kindFilter)
		}
	}
	t.Run("leaf", func(t *testing.T) { check(t, "MessageTopic") })
	t.Run("qualified", func(t *testing.T) { check(t, "SCOPE.MessageTopic") })
}

// Substring path: kind_filter=MessageTopic must match the SCOPE.MessageTopic
// stored kind (display-stripped leaf) and exclude non-topic hits, across repos.
func TestFindSubstring_KindFilterMessageTopic_CrossRepo(t *testing.T) {
	alpha := &graph.Document{Version: 1, Repo: "alpha", Entities: []graph.Entity{
		{ID: "t1", Name: "kafka:feedback-topic", Kind: "SCOPE.MessageTopic"},
		{ID: "s1", Name: "feedback-topic-schema", Kind: "SCOPE.Schema"}, // name contains query but wrong kind
	}}
	beta := &graph.Document{Version: 1, Repo: "beta", Entities: []graph.Entity{
		{ID: "t2", Name: "kafka:orders-topic", Kind: "SCOPE.MessageTopic"},
	}}
	srv := newTestServer(t, alpha, beta)

	check := func(t *testing.T, kindFilter string) {
		out := callDashboardTool(t, srv.handleSearchEntities, map[string]any{
			"group":       "test",
			"query":       "topic", // substring-matches all three names
			"kind_filter": kindFilter,
			"cross_repo":  true,
		})
		results, _ := out["results"].([]any)
		names := map[string]bool{}
		for _, r := range results {
			names[r.(map[string]any)["name"].(string)] = true
		}
		if !names["kafka:feedback-topic"] || !names["kafka:orders-topic"] {
			t.Fatalf("kind_filter=%s: expected both topics across repos, got %v", kindFilter, names)
		}
		if names["feedback-topic-schema"] {
			t.Errorf("kind_filter=%s: SCOPE.Schema entity should be excluded", kindFilter)
		}
	}
	t.Run("leaf", func(t *testing.T) { check(t, "MessageTopic") })
	t.Run("qualified", func(t *testing.T) { check(t, "SCOPE.MessageTopic") })
}

// --- kind-normalization helper unit test ------------------------------------

func TestMatchesKindFilter_ScopeNormalization(t *testing.T) {
	e := &graph.Entity{Kind: "SCOPE.MessageTopic"}
	cases := []struct {
		filter string
		want   bool
	}{
		{"MessageTopic", true},        // leaf, exact case
		{"messagetopic", true},        // leaf, lower case
		{"SCOPE.MessageTopic", true},  // fully qualified
		{"scope.messagetopic", true},  // fully qualified, lower case
		{"DataAccess", false},         // unrelated kind
		{"", true},                    // empty = no filter
	}
	for _, c := range cases {
		if got := matchesKindFilter(e, c.filter); got != c.want {
			t.Errorf("matchesKindFilter(SCOPE.MessageTopic, %q) = %v, want %v", c.filter, got, c.want)
		}
	}
	// Generality: any SCOPE.X matches leaf X.
	da := &graph.Entity{Kind: "SCOPE.DataAccess"}
	if !matchesKindFilter(da, "DataAccess") {
		t.Error("SCOPE.DataAccess should match leaf filter DataAccess")
	}
}
