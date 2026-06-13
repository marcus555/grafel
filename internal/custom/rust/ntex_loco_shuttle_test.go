package rust_test

// Tests for the ntex / Loco.rs / Shuttle extractors (#5008).
// Each framework gets: happy-path endpoint/substrate assertions, a
// wrong-language no-op (file.Language != "rust"), and a no-match no-op
// (rust file without the framework signal).

import "testing"

// ---------------------------------------------------------------------------
// ntex
// ---------------------------------------------------------------------------

func TestNtexMacroRoute(t *testing.T) {
	src := `
use ntex::web;

#[web::get("/users")]
async fn list_users() -> impl web::Responder { web::HttpResponse::Ok() }

#[web::post("/users/{id}")]
async fn update_user() -> impl web::Responder { web::HttpResponse::Ok() }
`
	ents := extract(t, "custom_rust_ntex", fi("handlers.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Operation", "GET /users") {
		t.Error("expected GET /users")
	}
	if !containsEntity(ents, "SCOPE.Operation", "POST /users/{id}") {
		t.Error("expected POST /users/{id}")
	}
}

func TestNtexResourceScopeComposed(t *testing.T) {
	src := `
use ntex::web;
let app = web::scope("/api")
    .service(web::resource("/users/{id}").route(web::get().to(get_user)));
`
	ents := extract(t, "custom_rust_ntex", fi("app.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Operation", "GET /api/users/{id}") {
		t.Error("expected scope-composed GET /api/users/{id}")
	}
	if p, _ := entityProp(ents, "SCOPE.Operation", "GET /api/users/{id}", "handler_name"); p != "get_user" {
		t.Errorf("handler_name = %q, want get_user", p)
	}
	if !containsEntity(ents, "SCOPE.Component", "/api") {
		t.Error("expected /api scope component")
	}
}

func TestNtexServerAndMiddleware(t *testing.T) {
	src := `
use ntex::web;
let server = web::HttpServer::new(|| {
    web::App::new().wrap(Logger::default())
}).bind("127.0.0.1:8080")?.run();
`
	ents := extract(t, "custom_rust_ntex", fi("main.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Service", "ntex::HttpServer") {
		t.Error("expected ntex::HttpServer service")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "middleware:Logger::default") {
		t.Error("expected middleware:Logger::default pattern")
	}
}

// Wrong-language no-op: an ntex-looking source tagged as a non-rust file.
func TestNtexWrongLanguageNoOp(t *testing.T) {
	src := `use ntex::web; #[web::get("/users")] async fn h() {}`
	ents := extract(t, "custom_rust_ntex", fi("handlers.go", "go", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities for non-rust file, got %d", len(ents))
	}
}

// No-match no-op: a rust file with no ntex signal yields nothing (and must not
// false-fire on a real actix file — which has no `ntex` token).
func TestNtexNoMatchNoOp(t *testing.T) {
	src := `
use actix_web::web;
#[get("/users")]
async fn list_users() -> impl Responder { HttpResponse::Ok() }
`
	ents := extract(t, "custom_rust_ntex", fi("handlers.rs", "rust", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities (no ntex signal), got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// Loco.rs
// ---------------------------------------------------------------------------

func TestLocoRouteChain(t *testing.T) {
	src := `
use loco_rs::prelude::*;

pub fn routes() -> Routes {
    Routes::new()
        .prefix("/api")
        .add("/users", get(list).post(create))
        .add("/users/:id", delete(remove));
}
`
	ents := extract(t, "custom_rust_loco", fi("controller.rs", "rust", src))
	for _, want := range []string{
		"GET /api/users", "POST /api/users", "DELETE /api/users/{id}",
	} {
		if !containsEntity(ents, "SCOPE.Operation", want) {
			t.Errorf("expected %q", want)
		}
	}
	if p, _ := entityProp(ents, "SCOPE.Operation", "POST /api/users", "handler_name"); p != "create" {
		t.Errorf("handler_name = %q, want create", p)
	}
	if !containsEntity(ents, "SCOPE.Component", "loco-prefix:/api") {
		t.Error("expected loco-prefix:/api component")
	}
}

func TestLocoWrongLanguageNoOp(t *testing.T) {
	src := `use loco_rs::prelude::*; Routes::new().add("/u", get(h));`
	ents := extract(t, "custom_rust_loco", fi("c.py", "python", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities for non-rust file, got %d", len(ents))
	}
}

func TestLocoNoMatchNoOp(t *testing.T) {
	// axum-style Routes::new().add but no loco_rs signal -> no-op.
	src := `let r = Routes::new().add("/users", get(list));`
	ents := extract(t, "custom_rust_loco", fi("r.rs", "rust", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities (no loco_rs signal), got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// Shuttle (deploy substrate)
// ---------------------------------------------------------------------------

func TestShuttleRuntimeAndResources(t *testing.T) {
	src := `
use shuttle_axum::ShuttleAxum;

#[shuttle_runtime::main]
async fn main(
    #[shuttle_shared_db::Postgres] pool: PgPool,
    #[shuttle_secrets::Secrets] secrets: SecretStore,
) -> ShuttleAxum {
    Ok(router.into())
}
`
	ents := extract(t, "custom_rust_shuttle", fi("main.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Service", "shuttle::main") {
		t.Error("expected shuttle::main deploy entrypoint service")
	}
	if dr, _ := entityProp(ents, "SCOPE.Service", "shuttle::main", "deploy_runtime"); dr != "shuttle" {
		t.Errorf("deploy_runtime = %q, want shuttle", dr)
	}
	if !containsEntity(ents, "SCOPE.Component", "shuttle-resource:shuttle_shared_db::Postgres") {
		t.Error("expected shuttle_shared_db::Postgres managed resource")
	}
	if !containsEntity(ents, "SCOPE.Component", "shuttle-resource:shuttle_secrets::Secrets") {
		t.Error("expected shuttle_secrets::Secrets managed resource")
	}
	// The runtime entrypoint macro must NOT also be emitted as a resource.
	if containsEntity(ents, "SCOPE.Component", "shuttle-resource:shuttle_runtime::main") {
		t.Error("shuttle_runtime::main must not be a managed-resource component")
	}
}

func TestShuttleWrongLanguageNoOp(t *testing.T) {
	src := `#[shuttle_runtime::main] async fn main() {}`
	ents := extract(t, "custom_rust_shuttle", fi("main.ts", "typescript", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities for non-rust file, got %d", len(ents))
	}
}

func TestShuttleNoMatchNoOp(t *testing.T) {
	src := `#[tokio::main] async fn main() { run().await; }`
	ents := extract(t, "custom_rust_shuttle", fi("main.rs", "rust", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities (no shuttle_ signal), got %d", len(ents))
	}
}
