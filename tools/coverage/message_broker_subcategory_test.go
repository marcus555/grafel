package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// msgBrokerReg builds an in-memory registry covering the message_broker
// subcategory lanes (#4008, B5 epic #3871). Before the split, the 44
// records rendered one 7-wide union pivot in which room_channel_grouping
// (4 records) and signature_verification (1 record) were near-empty
// columns — 104 "—" cells the #4007 column-strand guard could not touch
// (every column carried ≥1 real cell category-wide). The fixture includes
// one representative per lane plus the two columns that were sparse so the
// tests can assert each carved lane declares only its own dense columns.
func msgBrokerReg() *Registry {
	rec := func(id, sub, label string, caps map[string]string) Record {
		c := map[string]Capability{}
		for k, s := range caps {
			c[k] = Capability{Status: s}
		}
		return Record{
			ID: id, Category: "message_broker", Subcategory: sub,
			Language: "multi", Label: label, Capabilities: c,
		}
	}
	return &Registry{
		SchemaVersion: SchemaVersion,
		Records: []Record{
			// Schedulers (consumer-only)
			rec("msg.whenever", "schedulers", "whenever (Ruby cron)",
				map[string]string{"consumer_extraction": "full"}),
			rec("msg.apscheduler", "schedulers", "APScheduler",
				map[string]string{"consumer_extraction": "full"}),
			// Task Queues (consumer + producer [+ topic])
			rec("msg.sidekiq", "task_queues", "Sidekiq (Ruby task queue)",
				map[string]string{"consumer_extraction": "partial", "producer_extraction": "partial"}),
			rec("msg.celery", "task_queues", "Celery (Python task queue)",
				map[string]string{"consumer_extraction": "full", "producer_extraction": "full", "topic_attribution": "full"}),
			// Brokers (consumer + producer + topic)
			rec("msg.broker.kafka", "brokers", "Apache Kafka",
				map[string]string{"consumer_extraction": "full", "producer_extraction": "full", "topic_attribution": "full"}),
			rec("msg.broker.rabbitmq", "brokers", "RabbitMQ",
				map[string]string{"consumer_extraction": "full", "producer_extraction": "full", "topic_attribution": "full"}),
			// Realtime Channels (room/channel grouping; some lack it)
			rec("msg.actioncable", "realtime_channels", "Rails ActionCable",
				map[string]string{"room_channel_grouping": "full"}),
			rec("msg.websocket", "realtime_channels", "WebSocket",
				map[string]string{"consumer_extraction": "full", "producer_extraction": "full", "room_channel_grouping": "full", "topic_attribution": "partial"}),
			rec("msg.signalr", "realtime_channels", "SignalR",
				map[string]string{"consumer_extraction": "missing", "producer_extraction": "full"}),
			// Webhooks (signature verification)
			rec("msg.webhook", "webhooks", "Webhooks",
				map[string]string{"consumer_extraction": "full", "producer_extraction": "full", "signature_verification": "full", "topic_attribution": "partial"}),
		},
	}
}

// TestMessageBrokerSubcategoryLanes asserts the #4008 redesign: the
// message_broker by-category page is split into five behaviourally
// distinct subcategory lanes, each declaring ONLY the capability columns
// its records actually carry (no 5-wide union sprawl). This is the fix
// that kills the 104-em-dash flat pivot.
func TestMessageBrokerSubcategoryLanes(t *testing.T) {
	root := t.TempDir()
	if err := generate(msgBrokerReg(), root); err != nil {
		t.Fatalf("generate: %v", err)
	}
	page := readFile(t, filepath.Join(root, "docs/coverage/by-category/message_broker.md"))

	// Each lane renders as its own section heading, in the dictionary's
	// declared subcategory_order.
	wantOrder := []string{
		"## Schedulers",
		"## Task Queues",
		"## Brokers",
		"## Realtime Channels",
		"## Webhooks",
	}
	prev := -1
	for _, h := range wantOrder {
		i := strings.Index(page, h)
		if i < 0 {
			t.Errorf("message_broker.md missing subcategory section %q\n%s", h, page)
			continue
		}
		if i < prev {
			t.Errorf("subcategory section %q out of declared order", h)
		}
		prev = i
	}

	// Per-lane column sets: each lane header carries exactly the capability
	// columns its records declare, and NONE of the columns that belong only
	// to other lanes (the sparse-union leak the redesign removes).
	cases := []struct {
		lane    string
		want    []string // columns that MUST appear in this lane's header
		notWant []string // columns that MUST NOT appear (other lanes' vocab)
		rec     string
	}{
		{
			"## Schedulers",
			[]string{"Consumer extraction"},
			[]string{"Producer extraction", "Topic attribution", "Room channel grouping", "Signature verification"},
			"msg.whenever",
		},
		{
			"## Task Queues",
			[]string{"Consumer extraction", "Producer extraction", "Topic attribution"},
			[]string{"Room channel grouping", "Signature verification"},
			"msg.sidekiq",
		},
		{
			"## Brokers",
			[]string{"Consumer extraction", "Producer extraction", "Topic attribution"},
			[]string{"Room channel grouping", "Signature verification"},
			"msg.broker.kafka",
		},
		{
			"## Realtime Channels",
			[]string{"Consumer extraction", "Producer extraction", "Room channel grouping", "Topic attribution"},
			[]string{"Signature verification"},
			"msg.actioncable",
		},
		{
			"## Webhooks",
			[]string{"Consumer extraction", "Producer extraction", "Signature verification", "Topic attribution"},
			[]string{"Room channel grouping"},
			"msg.webhook",
		},
	}
	for _, c := range cases {
		body := sectionBody(page, c.lane)
		if !strings.Contains(body, c.rec) {
			t.Errorf("lane %q should contain record %s\n%s", c.lane, c.rec, body)
		}
		header := firstTableHeader(body)
		if header == "" {
			t.Errorf("lane %q has no table header\n%s", c.lane, body)
			continue
		}
		for _, col := range c.want {
			if !strings.Contains(header, col) {
				t.Errorf("lane %q header missing its own column %q: %s", c.lane, col, header)
			}
		}
		for _, col := range c.notWant {
			if strings.Contains(header, col) {
				t.Errorf("lane %q header carries foreign column %q (sparse-union leak): %s", c.lane, col, header)
			}
		}
	}

	// Signature verification lives ONLY on the Webhooks lane and Room
	// channel grouping ONLY on Realtime — the two columns that were
	// near-empty in the old flat union are now confined to where they apply.
	if strings.Count(page, "Signature verification") != 1 {
		t.Errorf("Signature verification column should appear in exactly one lane (Webhooks)")
	}
	if strings.Count(page, "Room channel grouping") != 1 {
		t.Errorf("Room channel grouping column should appear in exactly one lane (Realtime Channels)")
	}

	// No structurally-always-empty column: every declared lane column is
	// backed by ≥1 non-"—" cell. This is the core anti-sprawl invariant.
	assertNoAlwaysEmptyColumn(t, page)
}

// TestMessageBrokerSubcategoriesDeclared pins the dictionary taxonomy for
// message_broker so a future edit can't silently drop a lane or reorder it.
func TestMessageBrokerSubcategoriesDeclared(t *testing.T) {
	want := []string{"schedulers", "task_queues", "brokers", "realtime_channels", "webhooks"}
	got := dict().SubcategoriesByCategory("message_broker")
	if len(got) != len(want) {
		t.Fatalf("SubcategoriesByCategory(message_broker) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SubcategoriesByCategory(message_broker)[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
		if !dict().HasSubcategory("message_broker", want[i]) {
			t.Errorf("dictionary missing message_broker subcategory %q", want[i])
		}
	}

	// Per-lane allow-lists declare only the keys that lane's records carry.
	laneKeys := map[string][]string{
		"schedulers":        {"consumer_extraction"},
		"task_queues":       {"consumer_extraction", "producer_extraction", "topic_attribution"},
		"brokers":           {"consumer_extraction", "producer_extraction", "topic_attribution"},
		"realtime_channels": {"consumer_extraction", "producer_extraction", "room_channel_grouping", "topic_attribution"},
		"webhooks":          {"consumer_extraction", "producer_extraction", "signature_verification", "topic_attribution"},
	}
	for lane, wantKeys := range laneKeys {
		got := map[string]bool{}
		for _, k := range dict().SubcategoryCapabilities(lane) {
			got[k] = true
		}
		if len(got) != len(wantKeys) {
			t.Errorf("lane %q capabilities = %v, want %v", lane, dict().SubcategoryCapabilities(lane), wantKeys)
		}
		for _, k := range wantKeys {
			if !got[k] {
				t.Errorf("lane %q missing declared capability %q", lane, k)
			}
		}
	}
}
