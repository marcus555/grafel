package golang_test

import (
	"testing"
)

// observability-attribution-sweep (issue #3613): gqlgen resolver modules emit
// the same framework-agnostic observability signals as the HTTP-server
// frameworks, but previously lacked an import marker in obsFrameworkMarkers, so
// they attributed to framework="" and were dropped — leaving gqlgen's
// log/metric/trace coverage cells stale-`missing` while its siblings are
// partial/full. These tests prove the gqlgen marker now credits those signals,
// asserting the EXACT entity id, observability props, and framework=gqlgen.
//
// Non-vacuousness: without the gqlgen marker in obsFrameworkMarkers, the file
// has no recognised framework, detectObsFramework returns "" and the extractor
// returns nil — so every assertion below fails. (Verified by reverting the
// marker addition.)

// gqlgen resolver file using the canonical generated receiver type, with all
// three observability families present. This is the value-asserting case.
func TestObservabilityGqlgenAttribution(t *testing.T) {
	src := `package resolvers

import "go.uber.org/zap"

type mutationResolver struct{ *Resolver }

var requestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{Name: "graphql_resolver_requests_total"},
	[]string{"field"},
)

func (r *mutationResolver) CreateUser(ctx context.Context) error {
	logger := zap.NewProduction()
	tracer := otel.Tracer("graphql")
	ctx, span := tracer.Start(ctx, "resolver.createUser")
	_ = logger
	_ = span
	return nil
}`
	ents := extractFull(t, "custom_go_observability", fi("schema.resolvers.go", "go", src))

	got := obsEntities(ents)
	if len(got) == 0 {
		t.Fatalf("expected gqlgen observability entities, got none")
	}

	// logging: zap setup attributed to gqlgen.
	logE := findObs(ents, "logging", "zap")
	if logE == nil {
		t.Fatalf("missing logging/zap entity; got %+v", got)
	}
	if logE.Props["framework"] != "gqlgen" {
		t.Errorf("logging framework=%q want gqlgen", logE.Props["framework"])
	}

	// metrics: prometheus collector with EXACT metric name + entity name + provenance.
	m := findObs(ents, "metrics", "prometheus")
	if m == nil {
		t.Fatalf("missing metrics/prometheus entity; got %+v", got)
	}
	if m.Name != "obs:metrics:prometheus:graphql_resolver_requests_total" {
		t.Errorf("metric entity name=%q want obs:metrics:prometheus:graphql_resolver_requests_total", m.Name)
	}
	if m.Props["metric_name"] != "graphql_resolver_requests_total" {
		t.Errorf("metric_name=%q want graphql_resolver_requests_total", m.Props["metric_name"])
	}
	if m.Props["framework"] != "gqlgen" {
		t.Errorf("metric framework=%q want gqlgen", m.Props["framework"])
	}
	if m.Props["provenance"] != "INFERRED_FROM_GQLGEN_METRICS" {
		t.Errorf("metric provenance=%q want INFERRED_FROM_GQLGEN_METRICS", m.Props["provenance"])
	}

	// tracing: span start with EXACT span name + entity name + framework.
	span := findObs(ents, "tracing", "span_start")
	if span == nil {
		t.Fatalf("missing tracing/span_start entity; got %+v", got)
	}
	if span.Name != "obs:tracing:span_start:resolver.createUser" {
		t.Errorf("span entity name=%q want obs:tracing:span_start:resolver.createUser", span.Name)
	}
	if span.Props["span_name"] != "resolver.createUser" {
		t.Errorf("span_name=%q want resolver.createUser", span.Props["span_name"])
	}
	if span.Props["framework"] != "gqlgen" {
		t.Errorf("span framework=%q want gqlgen", span.Props["framework"])
	}

	// every emitted entity must be attributed to gqlgen.
	for _, e := range got {
		if e.Props["framework"] != "gqlgen" {
			t.Errorf("entity %q framework=%q want gqlgen", e.Name, e.Props["framework"])
		}
	}
}

// The canonical gqlgen import alone (no generated receiver type) is sufficient
// to attribute a resolver/server-wiring file.
func TestObservabilityGqlgenImportMarker(t *testing.T) {
	src := `package graph

import "github.com/99designs/gqlgen/graphql/handler"

func newServer() {
	tracer := otel.Tracer("graphql")
	_, span := tracer.Start(ctx, "graphql.exec")
	_ = span
}`
	ents := extractFull(t, "custom_go_observability", fi("server.go", "go", src))
	span := findObs(ents, "tracing", "span_start")
	if span == nil {
		t.Fatalf("missing tracing/span_start; got %+v", obsEntities(ents))
	}
	if span.Props["framework"] != "gqlgen" {
		t.Errorf("framework=%q want gqlgen", span.Props["framework"])
	}
}

// SELF-AUDIT: marker ordering must not let gqlgen steal attribution from a
// concrete HTTP-server framework. A file that imports a gqlgen receiver type
// AND uses a gin request context must attribute to gin (gin's marker precedes
// gqlgen in obsFrameworkMarkers declaration order — mirrors the rust utoipa
// LAST-placement fix).
func TestObservabilityGqlgenDoesNotStealFromServerFramework(t *testing.T) {
	src := `package main

type queryResolver struct{}

func run() {
	r := gin.Default()
	tracer := otel.Tracer("svc")
	_, span := tracer.Start(ctx, "http.handle")
	_ = r
	_ = span
}`
	ents := extractFull(t, "custom_go_observability", fi("main.go", "go", src))
	span := findObs(ents, "tracing", "span_start")
	if span == nil {
		t.Fatalf("missing tracing/span_start; got %+v", obsEntities(ents))
	}
	if span.Props["framework"] != "gin" {
		t.Errorf("framework=%q want gin (server framework must win over gqlgen)", span.Props["framework"])
	}
}
