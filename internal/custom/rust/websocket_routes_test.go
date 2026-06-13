package rust_test

import (
	"testing"
)

const wsExtractor = "custom_rust_websocket_routes"

// --- axum: .route("/ws", get(handler)) where handler upgrades -----------------

func TestRustWS_AxumRoutedUpgrade(t *testing.T) {
	src := `
async fn ws_handler(ws: WebSocketUpgrade) -> impl IntoResponse {
    ws.on_upgrade(|socket| handle_socket(socket))
}

async fn list_users() -> impl IntoResponse { StatusCode::OK }

fn app() -> Router {
    Router::new()
        .route("/ws", get(ws_handler))
        .route("/users", get(list_users))
}
`
	ents := extract(t, wsExtractor, fi("ws.rs", "rust", src))

	op := findRustDep(ents, "SCOPE.Operation", "WS /ws")
	if op == nil {
		t.Fatalf("expected WS /ws, got %+v", ents)
	}
	propEq(t, op, "framework", "axum")
	propEq(t, op, "websocket", "true")
	propEq(t, op, "route_path", "/ws")
	propEq(t, op, "http_method", "GET")
	propEq(t, op, "handler_name", "ws_handler")
	propEq(t, op, "upgrade_mechanism", "WebSocketUpgrade")
	propEq(t, op, "provenance", "INFERRED_FROM_AXUM_WEBSOCKET")

	// The plain HTTP route must NOT be mis-stamped as WS.
	if findRustDep(ents, "SCOPE.Operation", "WS /users") != nil {
		t.Errorf("plain /users route mis-classified as WS: %+v", ents)
	}
}

func TestRustWS_AxumNestComposesPath(t *testing.T) {
	src := `
async fn chat(ws: WebSocketUpgrade) -> impl IntoResponse { ws.on_upgrade(handle) }
fn build() -> Router {
    let rt = Router::new().route("/chat", get(chat));
    Router::new().nest("/api/v1", rt)
}
`
	ents := extract(t, wsExtractor, fi("nest.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "WS /api/v1/chat")
	if op == nil {
		t.Fatalf("expected nest-composed WS /api/v1/chat, got %+v", ents)
	}
	propEq(t, op, "nest_prefix", "/api/v1")
	propEq(t, op, "handler_name", "chat")
}

// Honest-partial: a bare WebSocketUpgrade with no resolvable route still emits an
// unrouted evidence entity (route_path omitted, never fabricated).
func TestRustWS_AxumUnroutedUpgradeEvidence(t *testing.T) {
	src := `
async fn handler(ws: WebSocketUpgrade) -> impl IntoResponse {
    ws.on_upgrade(do_stuff)
}
`
	ents := extract(t, wsExtractor, fi("bare.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "WS upgrade (axum)")
	if op == nil {
		t.Fatalf("expected unrouted WS evidence, got %+v", ents)
	}
	propEq(t, op, "websocket", "true")
	propAbsent(t, op, "route_path")
}

// --- actix: ws::start(Actor{...}) and WebsocketContext<H> ---------------------

func TestRustWS_ActixActorStart(t *testing.T) {
	src := `
struct MyWs;
impl Actor for MyWs { type Context = ws::WebsocketContext<MyWs>; }

async fn ws_index(req: HttpRequest, stream: web::Payload) -> Result<HttpResponse, Error> {
    ws::start(MyWs {}, &req, stream)
}
`
	ents := extract(t, wsExtractor, fi("actix.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "WS MyWs (actix)")
	if op == nil {
		t.Fatalf("expected WS MyWs (actix), got %+v", ents)
	}
	propEq(t, op, "framework", "actix_web")
	propEq(t, op, "websocket", "true")
	propEq(t, op, "handler_name", "MyWs")
	propEq(t, op, "upgrade_mechanism", "ws::start")
}

func TestRustWS_ActixWsHandle(t *testing.T) {
	src := `
async fn ws(req: HttpRequest, body: web::Payload) -> Result<HttpResponse, Error> {
    let (res, session, stream) = actix_ws::handle(&req, body)?;
    Ok(res)
}
`
	ents := extract(t, wsExtractor, fi("actixws.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "WS handshake (actix-ws)")
	if op == nil {
		t.Fatalf("expected WS handshake (actix-ws), got %+v", ents)
	}
	propEq(t, op, "upgrade_mechanism", "actix_ws::handle")
}

// --- warp: warp::ws() filter chain --------------------------------------------

func TestRustWS_WarpFilterChain(t *testing.T) {
	src := `
fn routes() {
    let chat = warp::path("chat")
        .and(warp::ws())
        .map(|ws: warp::ws::Ws| ws.on_upgrade(handle_ws));
}
`
	ents := extract(t, wsExtractor, fi("warp.rs", "rust", src))
	// path precedes warp::ws() in the chain — recovered from the pre-chain window.
	op := findRustDep(ents, "SCOPE.Operation", "WS /chat")
	if op == nil {
		// fall back: handler-named form
		op = findRustDep(ents, "SCOPE.Operation", "WS handle_ws (warp)")
	}
	if op == nil {
		t.Fatalf("expected a warp WS op, got %+v", ents)
	}
	propEq(t, op, "framework", "warp")
	propEq(t, op, "websocket", "true")
	propEq(t, op, "handler_name", "handle_ws")
	propEq(t, op, "upgrade_mechanism", "warp::ws")
}

// --- tokio-tungstenite: accept_async ------------------------------------------

func TestRustWS_TungsteniteAccept(t *testing.T) {
	src := `
async fn run(stream: TcpStream) {
    let ws_stream = tokio_tungstenite::accept_async(stream).await.unwrap();
}
`
	ents := extract(t, wsExtractor, fi("tung.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "WS accept (tokio-tungstenite)")
	if op == nil {
		t.Fatalf("expected WS accept entity, got %+v", ents)
	}
	propEq(t, op, "framework", "tokio_tungstenite")
	propEq(t, op, "upgrade_mechanism", "accept_async")
}

// --- wrong-language no-op -----------------------------------------------------

func TestRustWS_WrongLanguageNoOp(t *testing.T) {
	// Same WS idioms but language is not rust → no entities.
	src := `func ws(ws WebSocketUpgrade) { ws.on_upgrade(h); accept_async(s); }`
	ents := extract(t, wsExtractor, fi("ws.go", "go", src))
	if len(ents) != 0 {
		t.Fatalf("expected no entities for non-rust file, got %+v", ents)
	}
}

// --- no-match no-op -----------------------------------------------------------

func TestRustWS_NoMatchNoOp(t *testing.T) {
	src := `
async fn list_users() -> impl IntoResponse { StatusCode::OK }
fn app() -> Router { Router::new().route("/users", get(list_users)) }
`
	ents := extract(t, wsExtractor, fi("plain.rs", "rust", src))
	if len(ents) != 0 {
		t.Fatalf("expected no WS entities for plain HTTP file, got %+v", ents)
	}
}
