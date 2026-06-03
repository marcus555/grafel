package scala_test

import "testing"

// ---------------------------------------------------------------------------
// custom_scala_endpoint_deprecation — endpoint deprecation + api_version
// stamping (#4141, child of #3628). Value-asserting: each test pins the SPECIFIC
// deprecated/deprecated_since/deprecated_replacement/deprecation_source/
// api_version resolved on the SPECIFIC Scala idiom. A "≥1 entity" check is NEVER
// used. Negatives required (non-deprecated → none; versionless → no api_version).
// ---------------------------------------------------------------------------

const depKey = "custom_scala_endpoint_deprecation"

// findDep returns the first deprecation marker matching deprecation_source.
func findDep(ents []entitySummary, source string) (entitySummary, bool) {
	for _, e := range ents {
		if e.Subtype == "deprecation" && e.Props["deprecation_source"] == source {
			return e, true
		}
	}
	return entitySummary{}, false
}

// TestScalaDep_Http4sAnnotation: the flagship case from the ticket — a
// @deprecated("use /api/v2/users", "2.0") annotation on an http4s route in a v1
// path resolves the full contract (deprecated + since=2.0 + replacement +
// api_version=1 + source).
func TestScalaDep_Http4sAnnotation(t *testing.T) {
	src := `
import org.http4s._
import org.http4s.dsl.io._

val routes = HttpRoutes.of[IO] {
  @deprecated("use /api/v2/users", "2.0")
  case GET -> Root / "api" / "v1" / "users" => Ok("legacy")
}
`
	ents := extract(t, depKey, fi("Routes.scala", "scala", src))
	e, ok := findDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["deprecated_since"] != "2.0" {
		t.Errorf("deprecated_since = %q, want 2.0", e.Props["deprecated_since"])
	}
	if e.Props["deprecated_replacement"] != "/api/v2/users" {
		t.Errorf("deprecated_replacement = %q, want /api/v2/users", e.Props["deprecated_replacement"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
	if e.Props["framework"] != "http4s" {
		t.Errorf("framework = %q, want http4s", e.Props["framework"])
	}
}

// TestScalaDep_Http4sDSLVersion: api_version resolved from the http4s path-DSL
// `Root / "api" / "v2"` (no quoted /api/vN literal).
func TestScalaDep_Http4sDSLVersion(t *testing.T) {
	src := `
import org.http4s._

val routes = HttpRoutes.of[IO] {
  @deprecated("superseded", "3.1")
  case GET -> Root / "api" / "v2" / "orders" => Ok("o")
}
`
	ents := extract(t, depKey, fi("Orders.scala", "scala", src))
	e, ok := findDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["api_version"] != "2" {
		t.Errorf("api_version = %q, want 2", e.Props["api_version"])
	}
	if e.Props["deprecated_since"] != "3.1" {
		t.Errorf("deprecated_since = %q, want 3.1", e.Props["deprecated_since"])
	}
}

// TestScalaDep_AkkaAnnotationVerb: akka-http verb directive deprecated via the
// stdlib annotation, version from the path DSL.
func TestScalaDep_AkkaAnnotationVerb(t *testing.T) {
	src := `
import akka.http.scaladsl.server.Directives._

val route =
  path("v1" / "users") {
    @deprecated("use v2", "1.5")
    get {
      complete("legacy")
    }
  }
`
	ents := extract(t, depKey, fi("AkkaRoutes.scala", "scala", src))
	e, ok := findDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated_since"] != "1.5" {
		t.Errorf("deprecated_since = %q, want 1.5", e.Props["deprecated_since"])
	}
	if e.Props["deprecated_replacement"] != "v2" {
		t.Errorf("deprecated_replacement = %q, want v2", e.Props["deprecated_replacement"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
	if e.Props["framework"] != "akka-http" {
		t.Errorf("framework = %q, want akka-http", e.Props["framework"])
	}
}

// TestScalaDep_PekkoAnnotation: pekko-http (org.apache.pekko) detected as its own
// framework on a deprecated route.
func TestScalaDep_PekkoAnnotation(t *testing.T) {
	src := `
import org.apache.pekko.http.scaladsl.server.Directives._

val route =
  path("api" / "v1" / "items") {
    @deprecated("removed soon", "4.0")
    get { complete("x") }
  }
`
	ents := extract(t, depKey, fi("PekkoRoutes.scala", "scala", src))
	e, ok := findDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["framework"] != "pekko-http" {
		t.Errorf("framework = %q, want pekko-http", e.Props["framework"])
	}
	if e.Props["deprecated_since"] != "4.0" {
		t.Errorf("deprecated_since = %q, want 4.0", e.Props["deprecated_since"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// TestScalaDep_ScaladocTag: a Scaladoc @deprecated tag above an http4s route.
func TestScalaDep_ScaladocTag(t *testing.T) {
	src := `
import org.http4s._

/**
 * Legacy lookup endpoint.
 * @deprecated since 2.0 use /api/v2/lookup instead
 */
val route = HttpRoutes.of[IO] {
  case GET -> Root / "api" / "v1" / "lookup" => Ok("l")
}
`
	ents := extract(t, depKey, fi("Lookup.scala", "scala", src))
	e, ok := findDep(ents, "@deprecated scaladoc")
	if !ok {
		t.Fatalf("expected @deprecated scaladoc marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["deprecated_since"] != "2.0" {
		t.Errorf("deprecated_since = %q, want 2.0", e.Props["deprecated_since"])
	}
	if e.Props["deprecated_replacement"] != "/api/v2/lookup" {
		t.Errorf("deprecated_replacement = %q, want /api/v2/lookup", e.Props["deprecated_replacement"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// TestScalaDep_BannerComment: a `// DEPRECATED` banner at a route.
func TestScalaDep_BannerComment(t *testing.T) {
	src := `
import org.http4s._

val route = HttpRoutes.of[IO] {
  // DEPRECATED use /v2 instead
  case GET -> Root / "v1" / "ping" => Ok("pong")
}
`
	ents := extract(t, depKey, fi("Ping.scala", "scala", src))
	e, ok := findDep(ents, "comment // DEPRECATED")
	if !ok {
		t.Fatalf("expected banner-comment marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// TestScalaDep_SunsetHeader: a Sunset response header (RFC 8594) on a route.
func TestScalaDep_SunsetHeader(t *testing.T) {
	src := `
import org.http4s._
import org.http4s.headers._

val route = HttpRoutes.of[IO] {
  case GET -> Root / "api" / "v1" / "report" =>
    Ok("r").map(_.putHeaders(Header.Raw(ci"Sunset", "Wed, 11 Nov 2026 23:59:59 GMT")))
}
`
	ents := extract(t, depKey, fi("Report.scala", "scala", src))
	e, ok := findDep(ents, "Sunset response header")
	if !ok {
		t.Fatalf("expected Sunset header marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// TestScalaDep_NamedSinceArg: the named `@deprecated(message = .., since = ..)`
// form resolves the same as positional.
func TestScalaDep_NamedSinceArg(t *testing.T) {
	src := `
import org.http4s._

val route = HttpRoutes.of[IO] {
  @deprecated(message = "use the new search", since = "5.2")
  case GET -> Root / "api" / "v1" / "search" => Ok("s")
}
`
	ents := extract(t, depKey, fi("Search.scala", "scala", src))
	e, ok := findDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated_since"] != "5.2" {
		t.Errorf("deprecated_since = %q, want 5.2", e.Props["deprecated_since"])
	}
}

// --- Negatives -------------------------------------------------------------

// TestScalaDep_NonDeprecatedNone: a plain route with NO deprecation marker emits
// no deprecation entity.
func TestScalaDep_NonDeprecatedNone(t *testing.T) {
	src := `
import org.http4s._

val route = HttpRoutes.of[IO] {
  case GET -> Root / "api" / "v1" / "health" => Ok("ok")
}
`
	ents := extract(t, depKey, fi("Health.scala", "scala", src))
	for _, e := range ents {
		if e.Subtype == "deprecation" {
			t.Fatalf("expected NO deprecation marker on a non-deprecated route; got: %+v", e)
		}
	}
}

// TestScalaDep_VersionlessNoApiVersion: a deprecated route with no /vN segment
// carries deprecated=true but NO api_version (honest-partial).
func TestScalaDep_VersionlessNoApiVersion(t *testing.T) {
	src := `
import org.http4s._

val route = HttpRoutes.of[IO] {
  @deprecated("going away", "1.0")
  case GET -> Root / "users" => Ok("u")
}
`
	ents := extract(t, depKey, fi("Users.scala", "scala", src))
	e, ok := findDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if v, present := e.Props["api_version"]; present {
		t.Errorf("api_version should be ABSENT on a versionless route; got %q", v)
	}
}

// TestScalaDep_NonRouteDeprecatedUnaffected: a @deprecated on a non-route helper
// class (no route anchor nearby) does NOT emit a route-deprecation marker.
func TestScalaDep_NonRouteDeprecatedUnaffected(t *testing.T) {
	src := `
import org.http4s._

@deprecated("legacy model", "1.0")
case class OldUser(id: Long, name: String)

object Helpers {
  @deprecated("use newFormat", "2.0")
  def oldFormat(x: Int): String = x.toString
}
`
	ents := extract(t, depKey, fi("Models.scala", "scala", src))
	for _, e := range ents {
		if e.Subtype == "deprecation" {
			t.Fatalf("expected NO route-deprecation marker for non-route @deprecated; got: %+v", e)
		}
	}
}

// TestScalaDep_NonScalaSkipped: a non-scala file is skipped entirely.
func TestScalaDep_NonScalaSkipped(t *testing.T) {
	src := `@deprecated("x", "1.0") case GET -> Root / "v1" => Ok("x")`
	ents := extract(t, depKey, fi("Routes.kt", "kotlin", src))
	if len(ents) != 0 {
		t.Fatalf("expected no entities for non-scala file; got: %+v", ents)
	}
}
