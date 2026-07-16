// messaging_related_5782.go — cross-repo message-topic query layer (#5782).
//
// MessageTopic entities are per-repo SCOPE.MessageTopic nodes keyed by a broker-
// prefixed Name (e.g. "kafka:orders.placed"); the SAME Name appears as a
// separate entity in every repo that touches the topic. Intra-repo edges:
//
//	producer --PUBLISHES_TO--> topic      (inbound to the topic)
//	consumer --SUBSCRIBES_TO--> topic     (inbound to the topic)
//	topic    --DELIVERS_TO--> handler     (outbound; async-trigger inverse)
//
// The cross-repo publisher↔subscriber join is NOT in any single repo's
// adjacency — it lives in lg.Links as CrossRepoLink{Method:"topic"} rows written
// by internal/links/topic_pass.go (P7), whose Identifier is the topic Name.
//
// Two query-layer surfaces consumed this data one repo at a time and so returned
// nothing / an intra-repo-only view for a topic whose counterparts live in
// sibling repos:
//   - grafel_related  (handleCoreRelated) — the generic caller/callee handlers
//     dead-end on the first repo holding the resolved entity.
//   - grafel_impact_radius (handleImpactRadius) — walked one resolved repo's
//     adjacency only.
//
// This file adds the framework-agnostic, group-wide topic view both need. No
// re-index; purely a read over graph data that already exists.
package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cajasmota/grafel/internal/graph"
	"github.com/cajasmota/grafel/internal/types"
	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// isMessageTopicEntity reports whether e is a broker message-topic node.
func isMessageTopicEntity(e *graph.Entity) bool {
	return e != nil && e.Kind == string(types.EntityKindMessageTopic)
}

// topicSeed is a resolved SCOPE.MessageTopic entity: the repo it was found in,
// its local id, and its broker-prefixed Name (the group-wide join key).
type topicSeed struct {
	repo *LoadedRepo
	id   string
	name string
}

// resolveTopicSeed resolves entity_id to a SCOPE.MessageTopic across the group,
// by prefixed/local id first (honouring a repo hint) then by exact Name /
// QualifiedName. Returns nil when entity_id does not name a message topic.
func resolveTopicSeed(lg *LoadedGroup, entityID string) *topicSeed {
	repoHint, local := splitPrefixed(entityID)
	probe := local
	if probe == "" {
		probe = entityID
	}
	repos := reposToConsider(lg, nil)

	// Pass 1: exact local-id match (respect a repo hint when present).
	for _, r := range repos {
		if r == nil || r.Doc == nil {
			continue
		}
		if repoHint != "" && r.Repo != repoHint {
			continue
		}
		if e := r.getByID()[probe]; isMessageTopicEntity(e) {
			return &topicSeed{repo: r, id: e.ID, name: e.Name}
		}
	}

	// Pass 2: exact Name / QualifiedName match on a message topic. The first
	// repo wins as the "seed" repo; collectTopicNeighbors then unions every
	// same-Name topic in the group, so the choice of seed repo only affects the
	// cross_repo attribution flag, not which neighbors are returned.
	for _, r := range repos {
		if r == nil || r.Doc == nil {
			continue
		}
		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			if isMessageTopicEntity(e) && (e.Name == probe || e.QualifiedName == probe || e.Name == entityID) {
				return &topicSeed{repo: r, id: e.ID, name: e.Name}
			}
		}
	}
	return nil
}

// topicNeighbor is one producer / consumer / delivery-handler of a topic.
type topicNeighbor struct {
	EntityID  string `json:"entity_id"`
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`
	Repo      string `json:"repo"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
	EdgeKind  string `json:"edge_kind"`
	CrossRepo bool   `json:"cross_repo"`
}

// collectTopicNeighbors returns the producers, consumers and delivery handlers
// of a topic (matched by Name) across ALL repos in the group, folding BOTH:
//
//	(a) the local per-repo adjacency of every same-Name SCOPE.MessageTopic
//	    (PUBLISHES_TO / SUBSCRIBES_TO inbound; DELIVERS_TO outbound), and
//	(b) the cross-repo topic joins in lg.Links (method="topic", identifier==Name):
//	    Source is the publisher entity, Target the subscriber entity.
//
// seedRepo is the repo of the inspected topic entity; a neighbor in any other
// repo is flagged cross_repo. Results are de-duplicated by prefixed entity id
// (a counterpart present in both (a) and (b) appears once).
func collectTopicNeighbors(lg *LoadedGroup, topicName, seedRepo string) (producers, consumers, handlers []topicNeighbor) {
	producers, consumers, handlers = []topicNeighbor{}, []topicNeighbor{}, []topicNeighbor{}
	seenProd, seenCons, seenHand := map[string]bool{}, map[string]bool{}, map[string]bool{}

	add := func(list *[]topicNeighbor, seen map[string]bool, r *LoadedRepo, localID, edgeKind string) {
		if r == nil || r.Doc == nil {
			return
		}
		e := r.getByID()[localID]
		if e == nil {
			return
		}
		pid := prefixedID(r.Repo, e.ID)
		if seen[pid] {
			return
		}
		seen[pid] = true
		*list = append(*list, topicNeighbor{
			EntityID:  pid,
			Name:      e.Name,
			Kind:      stripScopePrefix(e.Kind),
			Repo:      r.Repo,
			File:      e.SourceFile,
			Line:      e.StartLine,
			EdgeKind:  edgeKind,
			CrossRepo: r.Repo != seedRepo,
		})
	}

	// (a) Local per-repo adjacency of every same-Name topic entity.
	for _, r := range reposToConsider(lg, nil) {
		if r == nil || r.Doc == nil {
			continue
		}
		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			if !isMessageTopicEntity(e) || e.Name != topicName {
				continue
			}
			adj := r.getAdjacency()
			for _, in := range adj.Incoming(e.ID) {
				switch strings.ToUpper(in.kind) {
				case string(types.RelationshipKindPublishesTo):
					add(&producers, seenProd, r, in.target, "PUBLISHES_TO")
				case string(types.RelationshipKindSubscribesTo):
					add(&consumers, seenCons, r, in.target, "SUBSCRIBES_TO")
				}
			}
			for _, out := range adj.Outgoing(e.ID) {
				if strings.ToUpper(out.kind) == string(types.RelationshipKindDeliversTo) {
					add(&handlers, seenHand, r, out.target, "DELIVERS_TO")
				}
			}
		}
	}

	// (b) Cross-repo topic joins. Matched to THIS topic by the link identifier
	// (the topic Name topic_pass.go stamps); a link without an identifier is
	// skipped rather than risk pulling in an unrelated topic's counterparts.
	for i := range lg.Links {
		l := &lg.Links[i]
		if !strings.EqualFold(l.Method, "topic") || l.Identifier != topicName {
			continue
		}
		if sr, sid := splitPrefixed(l.Source); sr != "" {
			add(&producers, seenProd, lg.Repos[sr], sid, "PUBLISHES_TO")
		}
		if tr, tid := splitPrefixed(l.Target); tr != "" {
			add(&consumers, seenCons, lg.Repos[tr], tid, "SUBSCRIBES_TO")
		}
	}

	sortTopicNeighbors(producers)
	sortTopicNeighbors(consumers)
	sortTopicNeighbors(handlers)
	return producers, consumers, handlers
}

// sortTopicNeighbors orders neighbors deterministically: local first, then by
// repo, then by name.
func sortTopicNeighbors(ns []topicNeighbor) {
	sort.Slice(ns, func(i, j int) bool {
		if ns[i].CrossRepo != ns[j].CrossRepo {
			return !ns[i].CrossRepo
		}
		if ns[i].Repo != ns[j].Repo {
			return ns[i].Repo < ns[j].Repo
		}
		return ns[i].Name < ns[j].Name
	})
}

// topicReposTouched returns the sorted unique set of repos across all neighbor
// groups.
func topicReposTouched(groups ...[]topicNeighbor) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range groups {
		for _, n := range g {
			if !seen[n.Repo] {
				seen[n.Repo] = true
				out = append(out, n.Repo)
			}
		}
	}
	sort.Strings(out)
	return out
}

// tryMessagingNeighbors implements the #5782 ask #3 default-direction fix: when
// grafel_related is called with direction=neighbors/both (or no direction at
// all — the tool's own default), an agent has no reason to know about the
// messaging discriminator value. If entity_id resolves to a SCOPE.MessageTopic
// or SCOPE.ChannelBinding, return the messaging-aware view instead of letting
// the generic CALLS-only neighbors handler return an empty result. Returns nil
// (meaning: fall through to the generic handler) when entity_id is missing,
// the group can't be resolved, or the entity is neither kind — callers of this
// function must NOT surface resolveAndGroup errors themselves, since a nil
// return here is expected to be silently followed by the normal neighbors
// path, which will produce its own, better-contextualized error.
func (s *Server) tryMessagingNeighbors(req mcpapi.CallToolRequest) *mcpapi.CallToolResult {
	entityID := argString(req, "entity_id", "")
	if entityID == "" {
		return nil
	}
	_, lg, errRes := s.resolveAndGroup(req)
	if errRes != nil || lg == nil {
		return nil
	}
	if seed := resolveTopicSeed(lg, entityID); seed != nil {
		out := messagingRelatedStructured(lg, entityID)
		out["direction"] = "neighbors"
		return jsonResult(out)
	}
	if seed := resolveChannelBindingSeed(lg, entityID); seed != nil {
		return jsonResult(channelBindingNeighborsStructured(seed))
	}
	return nil
}

// handleMessagingRelated implements grafel_related direction=messaging (#5782):
// a topic's producers / consumers / delivery handlers across the whole group.
func (s *Server) handleMessagingRelated(_ context.Context, req mcpapi.CallToolRequest) (*mcpapi.CallToolResult, error) {
	entityID, err := req.RequireString("entity_id")
	if err != nil {
		return mcpapi.NewToolResultError(err.Error()), nil
	}
	_, lg, errRes := s.resolveAndGroup(req)
	if errRes != nil {
		return errRes, nil
	}
	return jsonResult(messagingRelatedStructured(lg, entityID)), nil
}

// messagingRelatedStructured is the non-wire variant of handleMessagingRelated
// (mirrors findCallersStructured, #2325 pattern). Factored out so the
// direction=neighbors default path (handleCoreRelated, #5782 ask #3) can reuse
// the identical topic traversal without round-tripping through wire bytes, and
// so it can override the `direction` field on the way out (the caller asked
// for neighbors, not explicitly for messaging).
func messagingRelatedStructured(lg *LoadedGroup, entityID string) map[string]any {
	seed := resolveTopicSeed(lg, entityID)
	if seed == nil {
		return map[string]any{
			"entity_id": entityID,
			"direction": "messaging",
			"resolved":  false,
			"producers": []any{},
			"consumers": []any{},
			"handlers":  []any{},
			"count":     0,
			"reason": fmt.Sprintf("no SCOPE.MessageTopic matched %q by id or name. "+
				"direction=messaging expects a message-topic entity; "+
				"use grafel_orient view=topology to list the group's topics.", entityID),
		}
	}

	producers, consumers, handlers := collectTopicNeighbors(lg, seed.name, seed.repo.Repo)
	return map[string]any{
		"entity_id": prefixedID(seed.repo.Repo, seed.id),
		"topic":     seed.name,
		"kind":      "message_topic",
		"direction": "messaging",
		"resolved":  true,
		"repo":      seed.repo.Repo,
		"producers": producers,
		"consumers": consumers,
		"handlers":  handlers,
		"count":     len(producers) + len(consumers) + len(handlers),
		"repos":     topicReposTouched(producers, consumers, handlers),
		"tip": "producers PUBLISH_TO the topic; consumers SUBSCRIBE_TO it; handlers receive DELIVERS_TO. " +
			"cross_repo=true marks a counterpart in a sibling repo.",
	}
}

// ---------------------------------------------------------------------------
// ChannelBinding neighbors (#5782 ask #3 / #5) — grafel_related
// direction=neighbors on a SCOPE.ChannelBinding.
// ---------------------------------------------------------------------------

// isChannelBindingEntity reports whether e is a config-side channel-binding
// node (#5782/ADR-0025).
func isChannelBindingEntity(e *graph.Entity) bool {
	return e != nil && e.Kind == string(types.EntityKindChannelBinding)
}

// channelBindingSeed is a resolved SCOPE.ChannelBinding entity.
type channelBindingSeed struct {
	repo *LoadedRepo
	id   string
	name string
}

// resolveChannelBindingSeed resolves entity_id to a SCOPE.ChannelBinding,
// mirroring resolveTopicSeed's id-then-name resolution. ChannelBinding edges
// (BINDS_CHANNEL / BINDS_TOPIC) are intra-repo, so unlike topics there is no
// cross-repo Name union to perform here.
func resolveChannelBindingSeed(lg *LoadedGroup, entityID string) *channelBindingSeed {
	repoHint, local := splitPrefixed(entityID)
	probe := local
	if probe == "" {
		probe = entityID
	}
	repos := reposToConsider(lg, nil)

	for _, r := range repos {
		if r == nil || r.Doc == nil {
			continue
		}
		if repoHint != "" && r.Repo != repoHint {
			continue
		}
		if e := r.getByID()[probe]; isChannelBindingEntity(e) {
			return &channelBindingSeed{repo: r, id: e.ID, name: e.Name}
		}
	}
	for _, r := range repos {
		if r == nil || r.Doc == nil {
			continue
		}
		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			if isChannelBindingEntity(e) && (e.Name == probe || e.QualifiedName == probe || e.Name == entityID) {
				return &channelBindingSeed{repo: r, id: e.ID, name: e.Name}
			}
		}
	}
	return nil
}

// channelBindingNeighborsStructured walks seed's outbound BINDS_CHANNEL /
// BINDS_TOPIC edges and returns its bound operation(s) and topic(s). Both
// edges are intra-repo (the ChannelBinding config row lives in the same repo
// as the SCOPE.Operation / SCOPE.MessageTopic it binds), so a single
// per-repo adjacency walk is sufficient — no lg.Links join needed.
func channelBindingNeighborsStructured(seed *channelBindingSeed) map[string]any {
	r := seed.repo
	channels, topics := []topicNeighbor{}, []topicNeighbor{}
	for _, out := range r.getAdjacency().Outgoing(seed.id) {
		var list *[]topicNeighbor
		var edgeKind string
		switch strings.ToUpper(out.kind) {
		case string(types.RelationshipKindBindsChannel):
			list, edgeKind = &channels, "BINDS_CHANNEL"
		case string(types.RelationshipKindBindsTopic):
			list, edgeKind = &topics, "BINDS_TOPIC"
		default:
			continue
		}
		e := r.getByID()[out.target]
		if e == nil {
			continue
		}
		*list = append(*list, topicNeighbor{
			EntityID: prefixedID(r.Repo, e.ID),
			Name:     e.Name,
			Kind:     stripScopePrefix(e.Kind),
			Repo:     r.Repo,
			File:     e.SourceFile,
			Line:     e.StartLine,
			EdgeKind: edgeKind,
		})
	}
	sortTopicNeighbors(channels)
	sortTopicNeighbors(topics)

	return map[string]any{
		"entity_id": prefixedID(r.Repo, seed.id),
		"name":      seed.name,
		"kind":      "channel_binding",
		"direction": "neighbors",
		"resolved":  true,
		"repo":      r.Repo,
		"channels":  channels,
		"topics":    topics,
		"count":     len(channels) + len(topics),
		"tip": "channels are the bound SCOPE.Operation (BINDS_CHANNEL); topics are the bound " +
			"SCOPE.MessageTopic (BINDS_TOPIC).",
	}
}

// ---------------------------------------------------------------------------
// grafel_impact_radius — topic seed expansion (#5782 item #2)
// ---------------------------------------------------------------------------

// impactAffected mirrors the anonymous struct in handleImpactRadius so the
// topic path emits the identical wire shape.
type impactAffected struct {
	EntityID   string  `json:"entity_id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Repo       string  `json:"repo"`
	SourceFile string  `json:"source_file,omitempty"`
	HopCount   int     `json:"hop_count"`
	RiskScore  float64 `json:"risk_score"`
	RiskReason string  `json:"risk_reason,omitempty"`
}

// computeRepoImpact walks r's INBOUND adjacency from target up to `hops` and
// returns the affected entities in r (target excluded), scored exactly like the
// intra-repo handleImpactRadius path. Used by the topic expansion for each
// same-Name topic entity's home repo.
func computeRepoImpact(r *LoadedRepo, target string, hops int) []impactAffected {
	if r == nil || r.Doc == nil {
		return nil
	}
	byID := r.getByID()

	namedCallerMap := map[string]int{}
	moduleCallerMap := map[string]int{}
	totalDegreeMap := map[string]int{}
	inboundTestsMap := map[string]int{}
	for i := range r.Doc.Relationships {
		rel := &r.Doc.Relationships[i]
		totalDegreeMap[rel.ToID]++
		if rel.Kind == "TESTS" {
			inboundTestsMap[rel.ToID]++
		}
		if src := byID[rel.FromID]; src != nil {
			if isModuleFileEntity(src) {
				moduleCallerMap[rel.ToID]++
			} else {
				namedCallerMap[rel.ToID]++
			}
		} else {
			namedCallerMap[rel.ToID]++
		}
	}

	adj := r.getAdjacency()
	visited := map[string]int{target: 0}
	frontier := []string{target}
	for d := 0; d < hops; d++ {
		next := []string{}
		for _, n := range frontier {
			for _, e := range adj.in[n] {
				if _, seen := visited[e.target]; seen {
					continue
				}
				visited[e.target] = d + 1
				next = append(next, e.target)
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}

	results := make([]impactAffected, 0, len(visited))
	for id, d := range visited {
		if id == target {
			continue
		}
		e := byID[id]
		if e == nil {
			continue
		}
		hasTests := inboundTestsMap[id] > 0
		isSpec := isTestSpecEntity(e)
		results = append(results, impactAffected{
			EntityID:   prefixedID(r.Repo, e.ID),
			Name:       e.Name,
			Kind:       stripScopePrefix(e.Kind),
			Repo:       r.Repo,
			SourceFile: e.SourceFile,
			HopCount:   d,
			RiskScore:  impactRiskScore(e, totalDegreeMap[id], hasTests, isSpec),
			RiskReason: buildRiskReason(e, namedCallerMap[id], moduleCallerMap[id], totalDegreeMap[id], hasTests, isSpec),
		})
	}
	return results
}

// oneEntityImpact scores a single entity as directly affected at the given hop.
// Used to fold a cross-repo lg.Links counterpart that lacks a local topic edge;
// it recomputes only that entity's in-degree (one O(R) pass, run rarely).
func oneEntityImpact(r *LoadedRepo, e *graph.Entity, hop int) impactAffected {
	byID := r.getByID()
	named, module, total, tests := 0, 0, 0, 0
	for i := range r.Doc.Relationships {
		rel := &r.Doc.Relationships[i]
		if rel.ToID != e.ID {
			continue
		}
		total++
		if rel.Kind == "TESTS" {
			tests++
		}
		if src := byID[rel.FromID]; src != nil && isModuleFileEntity(src) {
			module++
		} else {
			named++
		}
	}
	hasTests := tests > 0
	isSpec := isTestSpecEntity(e)
	return impactAffected{
		EntityID:   prefixedID(r.Repo, e.ID),
		Name:       e.Name,
		Kind:       stripScopePrefix(e.Kind),
		Repo:       r.Repo,
		SourceFile: e.SourceFile,
		HopCount:   hop,
		RiskScore:  impactRiskScore(e, total, hasTests, isSpec),
		RiskReason: buildRiskReason(e, named, module, total, hasTests, isSpec),
	}
}

// impactRadiusForTopic computes a topic's blast radius across the whole group:
// the union of every same-Name topic entity's intra-repo inbound walk, plus any
// cross-repo counterparts from lg.Links (method="topic") not already reached.
// Respects impactRadiusMaxResults and the hop bound.
func (s *Server) impactRadiusForTopic(lg *LoadedGroup, seed *topicSeed, hops int) *mcpapi.CallToolResult {
	seen := map[string]bool{}
	var results []impactAffected

	// Seed expansion: every same-Name SCOPE.MessageTopic across the group.
	for _, r := range reposToConsider(lg, nil) {
		if r == nil || r.Doc == nil {
			continue
		}
		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			if !isMessageTopicEntity(e) || e.Name != seed.name {
				continue
			}
			for _, a := range computeRepoImpact(r, e.ID, hops) {
				if a.Kind == stripScopePrefix(string(types.EntityKindMessageTopic)) {
					// Never fold a sibling topic node itself into the radius.
					continue
				}
				if seen[a.EntityID] {
					continue
				}
				seen[a.EntityID] = true
				results = append(results, a)
			}
		}
	}

	// Fold cross-repo topic joins whose publisher/subscriber has no local topic
	// edge (so the walk above missed them). Matched by link identifier == Name.
	for i := range lg.Links {
		l := &lg.Links[i]
		if !strings.EqualFold(l.Method, "topic") || l.Identifier != seed.name {
			continue
		}
		for _, side := range []string{l.Source, l.Target} {
			rp, id := splitPrefixed(side)
			r := lg.Repos[rp]
			if r == nil || r.Doc == nil {
				continue
			}
			e := r.getByID()[id]
			if e == nil {
				continue
			}
			pid := prefixedID(r.Repo, e.ID)
			if seen[pid] {
				continue
			}
			seen[pid] = true
			results = append(results, oneEntityImpact(r, e, 1))
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].RiskScore != results[j].RiskScore {
			return results[i].RiskScore > results[j].RiskScore
		}
		if results[i].HopCount != results[j].HopCount {
			return results[i].HopCount < results[j].HopCount
		}
		if results[i].Repo != results[j].Repo {
			return results[i].Repo < results[j].Repo
		}
		return results[i].Name < results[j].Name
	})

	totalAffected := len(results)
	truncated := false
	if len(results) > impactRadiusMaxResults {
		results = results[:impactRadiusMaxResults]
		truncated = true
	}
	if results == nil {
		results = []impactAffected{}
	}

	out := map[string]any{
		"entity_id":   prefixedID(seed.repo.Repo, seed.id),
		"entity_name": seed.name,
		"repo":        seed.repo.Repo,
		"hops":        hops,
		"resolved":    true,
		"messaging":   true,
		"affected":    results,
		"count":       len(results),
		"repos":       impactReposTouched(results),
		"tip": "message-topic blast radius: publishers (PUBLISHES_TO) and subscribers (SUBSCRIBES_TO) " +
			"across every repo that touches this broker-prefixed topic. risk_score 0.0–1.0.",
	}
	if truncated {
		out["truncated"] = true
		out["total_affected"] = totalAffected
		out["truncation_note"] = fmt.Sprintf(
			"hub topic: %d entities are affected; returning the top %d by risk_score. "+
				"Narrow with a smaller `hops`.", totalAffected, impactRadiusMaxResults)
	}
	return jsonResult(out)
}

// impactReposTouched returns the sorted unique repos across an affected set.
func impactReposTouched(rs []impactAffected) []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range rs {
		if !seen[a.Repo] {
			seen[a.Repo] = true
			out = append(out, a.Repo)
		}
	}
	sort.Strings(out)
	return out
}
