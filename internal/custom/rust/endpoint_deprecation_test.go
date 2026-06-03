package rust_test

import (
	"testing"
)

// findEntity returns the first entity matching kind+name (nil when absent).
func findRustDep(ents []entitySummary, kind, name string) *entitySummary {
	for i := range ents {
		if ents[i].Kind == kind && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

func propEq(t *testing.T, e *entitySummary, key, want string) {
	t.Helper()
	if e == nil {
		t.Fatalf("entity is nil, cannot check %s=%s", key, want)
	}
	if got := e.Props[key]; got != want {
		t.Errorf("prop %s: got %q want %q (props=%v)", key, got, want, e.Props)
	}
}

func propAbsent(t *testing.T, e *entitySummary, key string) {
	t.Helper()
	if e == nil {
		return
	}
	if got, ok := e.Props[key]; ok {
		t.Errorf("prop %s expected ABSENT, got %q", key, got)
	}
}

// --- axum: #[deprecated(since, note)] on the handler fn ----------------------

func TestRustDep_AxumDeprecatedHandler(t *testing.T) {
	src := `
async fn list_users() -> impl IntoResponse { "ok" }

#[deprecated(since = "2.0", note = "use /api/v2/users")]
async fn legacy_users() -> impl IntoResponse { "old" }

fn app() -> Router {
    Router::new()
        .route("/api/v1/users", get(legacy_users))
        .route("/api/v2/users", get(list_users))
}
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("routes.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /api/v1/users")
	if dep == nil {
		t.Fatalf("expected stamped GET /api/v1/users, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propEq(t, dep, "deprecated_since", "2.0")
	propEq(t, dep, "deprecated_replacement", "/api/v2/users")
	propEq(t, dep, "api_version", "1")
	propEq(t, dep, "framework", "axum")
	if dep.Props["deprecation_source"] != "#[deprecated]" {
		t.Errorf("deprecation_source: got %q", dep.Props["deprecation_source"])
	}

	// The non-deprecated v2 route is versioned, so it carries api_version but NOT
	// deprecated.
	v2 := findRustDep(ents, "SCOPE.Operation", "GET /api/v2/users")
	if v2 == nil {
		t.Fatalf("expected GET /api/v2/users (versioned)")
	}
	propEq(t, v2, "api_version", "2")
	propAbsent(t, v2, "deprecated")
}

func TestRustDep_AxumNestComposesVersion(t *testing.T) {
	src := `
#[deprecated(since = "1.5")]
async fn old_handler() -> impl IntoResponse { "x" }

fn build() -> Router {
    let api = Router::new().route("/users", get(old_handler));
    let app = Router::new().nest("/api/v1", api);
    app
}
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("nest.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /api/v1/users")
	if dep == nil {
		t.Fatalf("expected nest-composed GET /api/v1/users, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propEq(t, dep, "deprecated_since", "1.5")
	propEq(t, dep, "api_version", "1")
	propEq(t, dep, "nest_prefix", "/api/v1")
}

// --- actix: macro + #[deprecated] in the attribute region -------------------

func TestRustDep_ActixDeprecatedMacro(t *testing.T) {
	src := `
#[deprecated(since = "3.0", note = "use /v2/items")]
#[get("/v1/items")]
async fn list_items() -> impl Responder { HttpResponse::Ok() }
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("actix.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /v1/items")
	if dep == nil {
		t.Fatalf("expected GET /v1/items, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propEq(t, dep, "deprecated_since", "3.0")
	propEq(t, dep, "deprecated_replacement", "/v2/items")
	propEq(t, dep, "api_version", "1")
	propEq(t, dep, "framework", "actix_web")
}

func TestRustDep_ActixDeprecatedBelowMacro(t *testing.T) {
	// #[deprecated] stacked BETWEEN the route macro and the fn.
	src := `
#[get("/v1/legacy")]
#[deprecated(since = "2.1", note = "use /v2/legacy")]
async fn legacy() -> impl Responder { HttpResponse::Ok() }
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("actix2.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /v1/legacy")
	if dep == nil {
		t.Fatalf("expected GET /v1/legacy, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propEq(t, dep, "deprecated_since", "2.1")
	propEq(t, dep, "deprecated_replacement", "/v2/legacy")
}

// --- rocket: macro + mount prefix + #[deprecated] ---------------------------

func TestRustDep_RocketDeprecatedMounted(t *testing.T) {
	src := `
#[deprecated(since = "4.0", note = "use /api/v2/posts")]
#[get("/posts")]
fn list_posts() -> &'static str { "posts" }

#[launch]
fn rocket() -> _ {
    rocket::build().mount("/api/v1", routes![list_posts])
}
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("rocket.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /api/v1/posts")
	if dep == nil {
		t.Fatalf("expected mount-composed GET /api/v1/posts, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propEq(t, dep, "deprecated_since", "4.0")
	propEq(t, dep, "deprecated_replacement", "/api/v2/posts")
	propEq(t, dep, "api_version", "1")
	propEq(t, dep, "framework", "rocket")
	propEq(t, dep, "mount_prefix", "/api/v1")
}

// --- rustdoc @deprecated + banner + Sunset header ---------------------------

func TestRustDep_RustdocTag(t *testing.T) {
	src := `
use rocket::routes;
/// @deprecated since 5.0 use /v2/orders
#[get("/v1/orders")]
fn list_orders() -> &'static str { "orders" }
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("doc.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /v1/orders")
	if dep == nil {
		t.Fatalf("expected GET /v1/orders, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propEq(t, dep, "deprecated_since", "5.0")
	propEq(t, dep, "deprecated_replacement", "/v2/orders")
	propEq(t, dep, "deprecation_source", "rustdoc @deprecated")
}

func TestRustDep_SunsetHeaderAxum(t *testing.T) {
	src := `
async fn handler() -> impl IntoResponse {
    let mut headers = HeaderMap::new();
    headers.insert("Sunset", HeaderValue::from_static("Sat, 31 Dec 2025 23:59:59 GMT"));
    (headers, "body")
}

fn app() -> Router {
    Router::new().route("/legacy", get(handler))
}
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("sunset.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /legacy")
	if dep == nil {
		t.Fatalf("expected GET /legacy stamped via Sunset header, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propEq(t, dep, "deprecation_source", "Sunset response header")
	// Versionless → no api_version.
	propAbsent(t, dep, "api_version")
}

func TestRustDep_BareDeprecated(t *testing.T) {
	// A bare #[deprecated] (no args) still credits deprecated=true; no since/repl.
	src := `
use rocket::routes;
#[deprecated]
#[get("/v1/thing")]
fn thing() -> &'static str { "t" }
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("bare.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /v1/thing")
	if dep == nil {
		t.Fatalf("expected GET /v1/thing, got %+v", ents)
	}
	propEq(t, dep, "deprecated", "true")
	propAbsent(t, dep, "deprecated_since")
	propAbsent(t, dep, "deprecated_replacement")
	propEq(t, dep, "api_version", "1")
}

// --- negatives ---------------------------------------------------------------

func TestRustDep_NonDeprecatedVersionlessNone(t *testing.T) {
	// A plain, non-deprecated, versionless route is NOT re-emitted.
	src := `
async fn list() -> impl IntoResponse { "ok" }
fn app() -> Router { Router::new().route("/users", get(list)) }
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("plain.rs", "rust", src))
	if findRustDep(ents, "SCOPE.Operation", "GET /users") != nil {
		t.Errorf("plain non-deprecated versionless route must NOT be stamped, got %+v", ents)
	}
}

func TestRustDep_NonRouteDeprecatedUnaffected(t *testing.T) {
	// A #[deprecated] on a non-route helper fn that no route names must NOT emit
	// any endpoint.
	src := `
#[deprecated(since = "1.0", note = "use new_helper")]
fn old_helper() -> i32 { 0 }

async fn list() -> impl IntoResponse { "ok" }
fn app() -> Router { Router::new().route("/users", get(list)) }
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("helper.rs", "rust", src))
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" {
			t.Errorf("no endpoint should be stamped from a non-route #[deprecated] helper, got %+v", e)
		}
	}
}

func TestRustDep_VersionlessNoApiVersion(t *testing.T) {
	src := `
#[deprecated(since = "2.0")]
async fn legacy() -> impl IntoResponse { "x" }
fn app() -> Router { Router::new().route("/legacy", get(legacy)) }
`
	ents := extract(t, "custom_rust_endpoint_deprecation", fi("nov.rs", "rust", src))
	dep := findRustDep(ents, "SCOPE.Operation", "GET /legacy")
	if dep == nil {
		t.Fatalf("expected GET /legacy")
	}
	propEq(t, dep, "deprecated", "true")
	propAbsent(t, dep, "api_version")
}
