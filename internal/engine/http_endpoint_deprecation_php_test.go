package engine

import "testing"

// ---------------------------------------------------------------------------
// PHP (Laravel / Symfony) deprecation + api_version port (epic #3628).
//
// Mirrors the flagship property contract exactly: deprecated / deprecated_since
// / deprecated_replacement / deprecation_source / api_version. Uses the shared
// deprecProps/mustEndpoint harness from http_endpoint_deprecation_test.go.
// ---------------------------------------------------------------------------

// A Laravel route under a `Route::group(['prefix' => 'api/v1'], ...)` group with
// a `@deprecated use /api/v2/users` PHPDoc above the registration →
// deprecated=true + replacement + source + api_version=1 (path-derived from the
// composed prefix). The canonical PHP idiom.
func TestDeprecation_PHPLaravelPHPDocDeprecated(t *testing.T) {
	src := `<?php
use Illuminate\Support\Facades\Route;

Route::group(['prefix' => 'api/v1'], function () {
    /**
     * @deprecated use /api/v2/users instead
     */
    Route::get('/users', [UserController::class, 'index']);

    Route::get('/health', [HealthController::class, 'index']);
});
`
	eps := deprecProps(t, "php", "routes/api.php", src)

	dep := mustEndpoint(t, eps, "GET /api/v1/users")
	if dep.Properties["deprecated"] != "true" {
		t.Fatalf("GET /api/v1/users deprecated=%q, want true (props: %v)", dep.Properties["deprecated"], dep.Properties)
	}
	if got := dep.Properties["deprecation_source"]; got != "@deprecated" {
		t.Errorf("deprecation_source=%q, want '@deprecated'", got)
	}
	if got := dep.Properties["deprecated_replacement"]; got != "/api/v2/users" {
		t.Errorf("deprecated_replacement=%q, want /api/v2/users", got)
	}
	if got := dep.Properties["api_version"]; got != "1" {
		t.Errorf("api_version=%q, want 1 (path-derived from group prefix)", got)
	}

	// Negative: the non-deprecated sibling carries no deprecation (the PHPDoc
	// above /users does NOT leak onto /health).
	live := mustEndpoint(t, eps, "GET /api/v1/health")
	if _, ok := live.Properties["deprecated"]; ok {
		t.Fatalf("GET /api/v1/health deprecation fabricated, want absent (props: %v)", live.Properties)
	}
	// But the version segment still pins api_version on the live endpoint.
	if got := live.Properties["api_version"]; got != "1" {
		t.Errorf("GET /api/v1/health api_version=%q, want 1", got)
	}
}

// `@deprecated since 2.0 use /reports/v2 instead` resolves BOTH the since-version
// and the replacement out of the PHPDoc message via the shared parser.
func TestDeprecation_PHPSinceAndReplacement(t *testing.T) {
	src := `<?php
use Illuminate\Support\Facades\Route;

/**
 * @deprecated since 2.0 use /reports/v2 instead
 */
Route::get('/reports', [ReportController::class, 'index']);
`
	eps := deprecProps(t, "php", "routes/web.php", src)
	dep := mustEndpoint(t, eps, "GET /reports")
	if dep.Properties["deprecated"] != "true" {
		t.Fatalf("GET /reports deprecated=%q, want true (props: %v)", dep.Properties["deprecated"], dep.Properties)
	}
	if got := dep.Properties["deprecated_since"]; got != "2.0" {
		t.Errorf("deprecated_since=%q, want 2.0", got)
	}
	if got := dep.Properties["deprecated_replacement"]; got != "/reports/v2" {
		t.Errorf("deprecated_replacement=%q, want /reports/v2", got)
	}
}

// A `deprecated: true` route-attribute flag in the decorator region (the Symfony
// `#[Route(..., deprecated: true)]` shape) marks the route deprecated even with
// no PHPDoc message.
func TestDeprecation_PHPDeprecatedFlag(t *testing.T) {
	src := `<?php
use Illuminate\Support\Facades\Route;

#[Route(deprecated: true)]
Route::get('/legacy', [LegacyController::class, 'index']);
`
	eps := deprecProps(t, "php", "routes/web.php", src)
	dep := mustEndpoint(t, eps, "GET /legacy")
	if dep.Properties["deprecated"] != "true" {
		t.Fatalf("GET /legacy deprecated=%q, want true (props: %v)", dep.Properties["deprecated"], dep.Properties)
	}
	if got := dep.Properties["deprecation_source"]; got != "deprecated: true" {
		t.Errorf("deprecation_source=%q, want 'deprecated: true'", got)
	}
}

// A Sunset response header written in a Laravel route closure body is the
// cross-language runtime deprecation signal (flagship path), proven to fire for
// PHP.
func TestDeprecation_PHPSunsetResponseHeader(t *testing.T) {
	src := `<?php
use Illuminate\Support\Facades\Route;

Route::get('/payments', function () {
    return response('paid')->header('Sunset', 'Sat, 31 Dec 2025 23:59:59 GMT');
});
`
	eps := deprecProps(t, "php", "routes/api.php", src)
	dep := mustEndpoint(t, eps, "GET /payments")
	if dep.Properties["deprecated"] != "true" {
		t.Fatalf("GET /payments deprecated=%q, want true (props: %v)", dep.Properties["deprecated"], dep.Properties)
	}
	if got := dep.Properties["deprecation_source"]; got != "Sunset response header" {
		t.Errorf("deprecation_source=%q, want 'Sunset response header'", got)
	}
}

// Honest-partial: a versionless Laravel route with no deprecation marker carries
// NEITHER api_version NOR deprecated (never fabricated).
func TestDeprecation_PHPVersionlessNonDeprecated(t *testing.T) {
	src := `<?php
use Illuminate\Support\Facades\Route;

Route::get('/status', [StatusController::class, 'index']);
`
	eps := deprecProps(t, "php", "routes/web.php", src)
	e := mustEndpoint(t, eps, "GET /status")
	if got, ok := e.Properties["api_version"]; ok {
		t.Fatalf("api_version=%q fabricated on versionless route, want absent", got)
	}
	if got, ok := e.Properties["deprecated"]; ok {
		t.Fatalf("deprecated=%q fabricated on plain route, want absent", got)
	}
}

// ---------------------------------------------------------------------------
// unit-level: phpDeprecationVerdict
// ---------------------------------------------------------------------------

func TestPHPDeprecationVerdict(t *testing.T) {
	cases := []struct {
		name       string
		region     string
		wantDep    bool
		wantSource string
		wantRepl   string
		wantSince  string
	}{
		{"phpdoc bare", "* @deprecated", true, "@deprecated", "", ""},
		{"phpdoc msg replacement", "* @deprecated use /api/v2/x instead", true, "@deprecated", "/api/v2/x", ""},
		{"phpdoc since+repl", "* @deprecated since 1.5 use /v2/y", true, "@deprecated", "/v2/y", "1.5"},
		{"capital Deprecated annotation", "* @Deprecated", true, "@deprecated", "", ""},
		{"route deprecated flag", "#[Route('/x', deprecated: true)]", true, "deprecated: true", "", ""},
		{"no marker", "// just a regular comment\nRoute::get('/x', 'h');", false, "", "", ""},
		{"deprecated word in prose", "* this is not deprecated yet", false, "", "", ""},
	}
	for _, c := range cases {
		v, ok := phpDeprecationVerdict(c.region)
		if ok != c.wantDep {
			t.Errorf("%s: ok=%v, want %v", c.name, ok, c.wantDep)
			continue
		}
		if !c.wantDep {
			continue
		}
		if v.source != c.wantSource {
			t.Errorf("%s: source=%q, want %q", c.name, v.source, c.wantSource)
		}
		if v.replacement != c.wantRepl {
			t.Errorf("%s: replacement=%q, want %q", c.name, v.replacement, c.wantRepl)
		}
		if v.since != c.wantSince {
			t.Errorf("%s: since=%q, want %q", c.name, v.since, c.wantSince)
		}
	}
}
