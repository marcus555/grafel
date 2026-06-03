package kotlin_test

// javalin_routes_test.go — value-asserting tests for the
// custom_kotlin_javalin_routes extractor (Issue #4017, epic #3872).
//
// Asserts the SPECIFIC verb + path + handler on the SPECIFIC Javalin construct
// (direct fluent DSL + ApiBuilder DSL), plus the required negatives.

import (
	"testing"
)

// handlerOf returns the handler property of the first SCOPE.Operation endpoint
// whose name matches "<VERB> <path>", or "" if not found.
func handlerOf(ents []entitySummary, routeName string) (string, bool) {
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Name == routeName {
			return e.Props["handler"], true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Direct fluent DSL
// ---------------------------------------------------------------------------

func TestJavalinRoutes_DirectLambda(t *testing.T) {
	src := `
import io.javalin.Javalin

fun main() {
    val app = Javalin.create().start(7000)
    app.get("/users") { ctx -> ctx.json(users) }
    app.post("/users") { ctx -> ctx.status(201) }
}
`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("App.kt", "kotlin", src))
	names := routeNames(ents)
	if !names["GET /users"] {
		t.Fatalf("javalin: expected GET /users; got %v", names)
	}
	if !names["POST /users"] {
		t.Fatalf("javalin: expected POST /users; got %v", names)
	}
	// Handler attribution: trailing lambda.
	if h, _ := handlerOf(ents, "GET /users"); h != "lambda" {
		t.Errorf("javalin: GET /users handler = %q, want \"lambda\"", h)
	}
	if h, _ := handlerOf(ents, "POST /users"); h != "lambda" {
		t.Errorf("javalin: POST /users handler = %q, want \"lambda\"", h)
	}
}

func TestJavalinRoutes_DirectMethodRef(t *testing.T) {
	src := `
import io.javalin.Javalin

fun build(app: Javalin) {
    app.post("/users", ::createUser)
    app.get("/users/{id}", UserController::getOne)
    app.delete("/users/{id}", UserHandler())
}
`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("Routes.kt", "kotlin", src))

	if h, ok := handlerOf(ents, "POST /users"); !ok || h != "::createUser" {
		t.Errorf("javalin: POST /users handler = %q (found=%v), want \"::createUser\"", h, ok)
	}
	if h, ok := handlerOf(ents, "GET /users/{id}"); !ok || h != "UserController::getOne" {
		t.Errorf("javalin: GET /users/{id} handler = %q (found=%v), want \"UserController::getOne\"", h, ok)
	}
	if h, ok := handlerOf(ents, "DELETE /users/{id}"); !ok || h != "UserHandler" {
		t.Errorf("javalin: DELETE /users/{id} handler = %q (found=%v), want \"UserHandler\"", h, ok)
	}
}

func TestJavalinRoutes_ChainedCreate(t *testing.T) {
	// Javalin.create().get("/x", handler) — receiver is a chained call tail.
	src := `
import io.javalin.Javalin
val app = Javalin.create().get("/health", ::healthCheck)
`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("Health.kt", "kotlin", src))
	if h, ok := handlerOf(ents, "GET /health"); !ok || h != "::healthCheck" {
		t.Errorf("javalin: GET /health handler = %q (found=%v), want \"::healthCheck\"", h, ok)
	}
}

// ---------------------------------------------------------------------------
// ApiBuilder DSL — path() nesting
// ---------------------------------------------------------------------------

func TestJavalinRoutes_ApiBuilderNested(t *testing.T) {
	src := `
import io.javalin.Javalin
import io.javalin.apibuilder.ApiBuilder.*

fun routes(app: Javalin) {
    app.routes {
        path("users") {
            get(UserController::getAll)
            post(::create)
            path("{id}") {
                delete(::remove)
            }
        }
    }
}
`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("ApiRoutes.kt", "kotlin", src))
	names := routeNames(ents)

	want := map[string]string{
		"GET /users":         "UserController::getAll",
		"POST /users":        "::create",
		"DELETE /users/{id}": "::remove",
	}
	for route, wantHandler := range want {
		if !names[route] {
			t.Errorf("javalin ApiBuilder: expected composed route %q; got %v", route, names)
			continue
		}
		if h, _ := handlerOf(ents, route); h != wantHandler {
			t.Errorf("javalin ApiBuilder: %s handler = %q, want %q", route, h, wantHandler)
		}
	}
}

func TestJavalinRoutes_ApiBuilderSubPathArg(t *testing.T) {
	// ApiBuilder verb with an explicit sub-path string arg:
	//   get("active", ::listActive) inside path("orders") → GET /orders/active
	src := `
import io.javalin.Javalin
import io.javalin.apibuilder.ApiBuilder.*

fun build(app: Javalin) {
    app.routes {
        path("orders") {
            get("active", ::listActive)
        }
    }
}
`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("Orders.kt", "kotlin", src))
	if h, ok := handlerOf(ents, "GET /orders/active"); !ok || h != "::listActive" {
		t.Errorf("javalin ApiBuilder: GET /orders/active handler = %q (found=%v), want \"::listActive\"", h, ok)
	}
}

// ---------------------------------------------------------------------------
// Negatives
// ---------------------------------------------------------------------------

func TestJavalinRoutes_NonRouteCalls(t *testing.T) {
	// app.config(...) / app.start(...) / before / after must NOT yield routes.
	src := `
import io.javalin.Javalin

fun main() {
    val app = Javalin.create { config ->
        config.defaultContentType = "application/json"
    }
    app.config { c -> c.enableCors() }
    app.start(8080)
    app.before { ctx -> ctx.header("X-Trace", "1") }
    app.after("/admin/*") { ctx -> log(ctx) }
}
`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("Config.kt", "kotlin", src))
	if n := len(routeNames(ents)); n != 0 {
		t.Errorf("javalin: expected no routes from config/start/before/after, got %d: %v", n, routeNames(ents))
	}
}

func TestJavalinRoutes_NoJavalinSignal(t *testing.T) {
	// A file with get/post tokens but no Javalin reference must no-op (so this
	// extractor never poaches http4k / ktor routes).
	src := `
fun handler() {
    get("/ping")
    post("/users")
}
`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("NotJavalin.kt", "kotlin", src))
	if len(ents) != 0 {
		t.Errorf("javalin: expected no entities without Javalin signal, got %d", len(ents))
	}
}

func TestJavalinRoutes_WrongLanguage(t *testing.T) {
	src := `import io.javalin.Javalin
val app = Javalin.create()
app.get("/x") { ctx -> ctx.result("x") }`
	ents := extract(t, "custom_kotlin_javalin_routes", fi("App.java", "java", src))
	if len(ents) != 0 {
		t.Errorf("javalin: expected no entities for java language, got %d", len(ents))
	}
}

func TestJavalinRoutes_EmptyContent(t *testing.T) {
	ents := extract(t, "custom_kotlin_javalin_routes", fi("Empty.kt", "kotlin", ""))
	if len(ents) != 0 {
		t.Errorf("javalin: expected no entities for empty content, got %d", len(ents))
	}
}
