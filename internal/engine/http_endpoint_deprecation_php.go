// PHP (Laravel / Symfony) deprecation signals for the language-agnostic endpoint
// enrichment passes in http_endpoint_deprecation.go (epic #3628).
//
// The flagship pass (http_endpoint_deprecation.go) already gives PHP two things
// for free, because they are language-agnostic:
//
//   - api_version from an explicit `/api/v1/...` / `/v2/...` route segment
//     (applyEndpointAPIVersion). Laravel composes this from a
//     `Route::group(['prefix' => 'api/v1'], ...)` group prefix, which the Laravel
//     synthesizer folds into the canonical `path` property the path-derived
//     version reads — so prefix-versioned Laravel endpoints already carry
//     api_version with no PHP-specific code.
//   - deprecated=true from a `Sunset`/`Deprecation` response header written in a
//     Laravel route closure body (the cross-language RFC-8594 signal), and from a
//     leading `// DEPRECATED` banner comment.
//
// What PHP additionally needs — and what this file adds — is the PHP-idiomatic
// deprecation marker the generic passes do not recognise, in the decorator
// region the flagship anchors on the route's declaration line:
//
//   - a `@deprecated <message>` PHPDoc tag (the canonical PHP marker, rendered by
//     phpDocumentor / PhpStorm and flagged by phpstan/psalm) above the route
//     registration — the message commonly names the replacement
//     (`@deprecated use /api/v2/users`) and may carry a `since X` hint, both
//     pulled by the shared message parser;
//   - the `#[\Deprecated]` PHP 8.4 attribute / a `@Deprecated` annotation;
//   - a `deprecated: true` flag in the route attribute region — the Symfony
//     `#[Route(..., deprecated: true)]` shape, recognised here for any Laravel
//     route whose synthesised endpoint carries it in its decorator region.
//
// HONEST-PARTIAL — what is intentionally NOT credited by THIS engine pass:
//
//   - Symfony `#[Route]` and API Platform `#[ApiResource]` endpoints. These are
//     extracted as SCOPE.Operation `endpoint` entities by the custom PHP
//     extractors (internal/custom/php/symfony.go, apiplatform.go), NOT as the
//     http_endpoint_definition producers this pass stamps. Their own idioms
//     (Symfony `deprecated: true`, API Platform `deprecationReason`) are stamped
//     at the source in those extractors with the SAME property contract; this
//     engine pass leaves them untouched (no double-stamp, no cross-Kind leak).
//   - A Laravel controller-action `@deprecated` PHPDoc that lives in a SEPARATE
//     app/Http/Controllers/<name>.php file the routes/api.php per-file pass never
//     sees (mirrors the Rails honest-partial). The path-derived api_version still
//     lights up; only `deprecated` is honestly absent.
//
// Property contract is IDENTICAL to the flagship (deprecated / deprecated_since /
// deprecated_replacement / deprecation_source / api_version).
//
// Refs #3628.
package engine

import "regexp"

// phpDocDeprecatedRe matches a `@deprecated` PHPDoc / annotation tag with its
// optional trailing message (up to end of line / comment). This is the canonical
// PHP deprecation marker. The leading `*`/`//`/`#` comment-line prefixes are not
// required by the pattern (the decorator region already bounds it to comment /
// attribute lines), and the `\b` keeps `@deprecatedness` from matching.
var phpDocDeprecatedRe = regexp.MustCompile(`@[dD]eprecated\b([^\n*]{0,200})`)

// phpRouteDeprecatedFlagRe matches a `deprecated: true` named argument in a
// route attribute region — the Symfony `#[Route(..., deprecated: true)]` flag.
// Scoped to the decorator region so a `deprecated: true` elsewhere in the file
// cannot leak onto an unrelated route.
var phpRouteDeprecatedFlagRe = regexp.MustCompile(`(?i)\bdeprecated\s*:\s*true\b`)

// phpDeprecationVerdict resolves a PHP deprecation marker from the comment /
// attribute region that decorates the endpoint's route declaration (the
// contiguous run of PHPDoc `*` / `//` / `#[...]` lines immediately above the
// `Route::get('/x', ...)` line, captured by handlerDecoratorRegion). Returns
// (verdict, true) on the first marker found; (zero, false) when none apply so the
// cross-language Sunset-header / generic `// DEPRECATED` fallbacks still get their
// chance.
//
// Honest-partial: a since/replacement the marker does not name is left empty
// (never fabricated); both are pulled from the free-text message by the shared
// parser (`use /api/v2/users` → replacement, `since 2.0` → since).
func phpDeprecationVerdict(region string) (deprecationVerdict, bool) {
	// `@deprecated <msg>` / `@Deprecated` — the canonical PHP PHPDoc tag.
	if m := phpDocDeprecatedRe.FindStringSubmatch(region); m != nil {
		v := deprecationVerdict{deprecated: true, source: "@deprecated"}
		v.since, v.replacement = parseDeprecationMessage(m[1])
		return v, true
	}
	// `deprecated: true` route-attribute flag (Symfony `#[Route(...)]` shape).
	if phpRouteDeprecatedFlagRe.MatchString(region) {
		return deprecationVerdict{deprecated: true, source: "deprecated: true"}, true
	}
	return deprecationVerdict{}, false
}
