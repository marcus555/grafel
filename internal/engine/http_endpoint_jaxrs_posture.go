// JAX-RS / Jakarta-family endpoint posture stamping (#3857, epic #3854).
//
// Spring was built first, so the language-agnostic posture passes
// (response_codes / pagination — see http_endpoint_response_codes.go and
// http_endpoint_pagination.go) only recognised SPRING idioms in their `java`
// branch (ResponseEntity / @ResponseStatus / ResponseStatusException;
// Pageable / Page<…>). The rest of the JVM REST family
// — JAX-RS / Jakarta REST, Quarkus, Micronaut, MicroProfile, Helidon,
// Dropwizard, Jakarta EE — uses a DIFFERENT response/pagination API and so
// carried no response_codes / pagination posture even though their endpoints
// were emitted (Quarkus/JAX-RS via synthesizeJAXRS as http_endpoint_definition).
//
// This file adds the JAX-RS/Jakarta-family resolvers, merged into the existing
// `java` branch of resolveEndpointResponseCodes / resolveEndpointPagination so
// the SAME flat property contract Spring uses is stamped — no new properties, no
// new entities.
//
// Recognised response-code signals (jakarta.ws.rs + Micronaut):
//
//   - JAX-RS `Response.status(404)` / `Response.status(Response.Status.CREATED)`
//     / `Response.status(Status.NOT_FOUND)`.
//   - JAX-RS `Response.ok()` (200) / `.created(uri)` (201) / `.accepted()` (202)
//     / `.noContent()` (204) builder helpers.
//   - JAX-RS `throw new WebApplicationException(404)` and the typed exception
//     subclasses (`NotFoundException` → 404, `BadRequestException` → 400, …).
//   - Micronaut `HttpResponse.status(HttpStatus.CREATED)` / `.created(…)` (201)
//     / `.notFound()` (404) / `.ok()` (200) / `.badRequest()` (400) / `.noContent()`
//     (204) / `.unprocessableEntity()` (422) / `.serverError()` (500); and the
//     Micronaut `@Status(HttpStatus.CREATED)` handler annotation.
//
// Recognised pagination signals:
//
//   - Micronaut `Pageable` handler param / `Page<…>` / `Slice<…>` return type
//     (data-micronaut; binds `page`/`size`/`sort`) — page style.
//   - JAX-RS `@QueryParam("limit")` + `@QueryParam("offset")` (or cursor params)
//     classified by the shared classifyParamShape vocabulary.
//
// HONEST-PARTIAL: a dynamic status variable (`Response.status(code)`) yields no
// literal and is skipped; a lone `@QueryParam("limit")` with no offset/page/cursor
// companion is ambiguous and is NOT stamped.
//
// Refs #3857.
package engine

import (
	"regexp"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Response codes — JAX-RS / Jakarta REST + Micronaut
// ---------------------------------------------------------------------------

// jaxrsStatusEnumRe matches `Response.status(Response.Status.CREATED)`,
// `Response.status(Status.NOT_FOUND)` and `HttpResponse.status(HttpStatus.OK)` —
// a status(...) call whose argument is a named status enum constant. Group 1 is
// the enum constant name.
var jaxrsStatusEnumRe = regexp.MustCompile(`\.\s*status\s*\(\s*(?:[A-Za-z_][A-Za-z0-9_]*\s*\.\s*)*([A-Z][A-Z0-9_]+)\s*[\),]`)

// jaxrsStatusNumericRe matches `Response.status(404)` / `HttpResponse.status(201)`
// — a status(...) call whose argument is a 3-digit literal. A non-literal
// argument (a variable) does not match — honest-partial.
var jaxrsStatusNumericRe = regexp.MustCompile(`\.\s*status\s*\(\s*(\d{3})\s*[\),]`)

// jaxrsBuilderFactoryRe matches the JAX-RS / Micronaut Response builder factory
// helpers that imply a fixed status: ok()→200, created(...)→201, accepted()→202,
// noContent()→204, notFound()→404, badRequest()→400, serverError()→500,
// unprocessableEntity()→422. Anchored on a leading `.` or `(` so it matches
// `Response.ok(` / `HttpResponse.created(` / `return ok(`.
var jaxrsBuilderFactoryRe = regexp.MustCompile(`(?:Response|HttpResponse)\s*\.\s*(ok|created|accepted|noContent|notFound|badRequest|serverError|unprocessableEntity|unauthorized)\b`)

// jaxrsStatusAnnotationRe matches the Micronaut `@Status(HttpStatus.CREATED)` /
// `@Status(HttpStatus.NO_CONTENT)` handler annotation. Group 1 is the enum name.
var jaxrsStatusAnnotationRe = regexp.MustCompile(`@Status\s*\(\s*(?:HttpStatus\s*\.\s*)?([A-Z][A-Z0-9_]+)\s*\)`)

// jaxrsStatusAnnotationNumericRe matches a numeric Micronaut `@Status(201)`.
var jaxrsStatusAnnotationNumericRe = regexp.MustCompile(`@Status\s*\(\s*(\d{3})\s*\)`)

// jaxrsThrowExceptionRe matches `new <Name>Exception(` for a JAX-RS exception
// subclass thrown in the handler body; the class name is mapped via
// jaxrsExceptionCodes. Also matches the generic
// `new WebApplicationException(404)` whose explicit numeric/enum status is read
// separately by jaxrsWebApplicationExceptionRe.
var jaxrsThrowExceptionRe = regexp.MustCompile(`new\s+([A-Z][A-Za-z]*Exception)\b`)

// jaxrsWebApplicationExceptionRe matches `new WebApplicationException(404)` and
// `new WebApplicationException(Response.Status.CONFLICT)` — the explicit-status
// form. Group 1 = numeric code (when present); group 2 = enum name (when present).
var jaxrsWebApplicationExceptionRe = regexp.MustCompile(`new\s+WebApplicationException\s*\(\s*(?:"[^"]*"\s*,\s*)?(?:(\d{3})|(?:[A-Za-z_][A-Za-z0-9_]*\s*\.\s*)*([A-Z][A-Z0-9_]+))`)

// jaxrsBuilderFactoryCodes maps a JAX-RS / Micronaut Response builder factory to
// its conventional status code.
var jaxrsBuilderFactoryCodes = map[string]int{
	"ok":                  200,
	"created":             201,
	"accepted":            202,
	"noContent":           204,
	"badRequest":          400,
	"unauthorized":        401,
	"notFound":            404,
	"unprocessableEntity": 422,
	"serverError":         500,
}

// jaxrsExceptionCodes maps the jakarta.ws.rs typed exception subclasses to the
// HTTP status each one carries by definition (jakarta.ws.rs.*Exception).
var jaxrsExceptionCodes = map[string]int{
	"BadRequestException":          400,
	"NotAuthorizedException":       401,
	"ForbiddenException":           403,
	"NotFoundException":            404,
	"NotAllowedException":          405,
	"NotAcceptableException":       406,
	"NotSupportedException":        415,
	"ClientErrorException":         400,
	"InternalServerErrorException": 500,
	"ServiceUnavailableException":  503,
	"ServerErrorException":         500,
	"RedirectionException":         303,
}

// jaxrsStatusEnumName maps the jakarta.ws.rs Response.Status enum names (also
// shared by Micronaut's io.micronaut.http.HttpStatus) to their numeric codes.
var jaxrsStatusEnumName = map[string]int{
	"OK":                     200,
	"CREATED":                201,
	"ACCEPTED":               202,
	"NO_CONTENT":             204,
	"RESET_CONTENT":          205,
	"PARTIAL_CONTENT":        206,
	"MOVED_PERMANENTLY":      301,
	"FOUND":                  302,
	"SEE_OTHER":              303,
	"NOT_MODIFIED":           304,
	"TEMPORARY_REDIRECT":     307,
	"BAD_REQUEST":            400,
	"UNAUTHORIZED":           401,
	"PAYMENT_REQUIRED":       402,
	"FORBIDDEN":              403,
	"NOT_FOUND":              404,
	"METHOD_NOT_ALLOWED":     405,
	"NOT_ACCEPTABLE":         406,
	"REQUEST_TIMEOUT":        408,
	"CONFLICT":               409,
	"GONE":                   410,
	"PRECONDITION_FAILED":    412,
	"UNSUPPORTED_MEDIA_TYPE": 415,
	"UNPROCESSABLE_ENTITY":   422,
	"TOO_MANY_REQUESTS":      429,
	"INTERNAL_SERVER_ERROR":  500,
	"NOT_IMPLEMENTED":        501,
	"BAD_GATEWAY":            502,
	"SERVICE_UNAVAILABLE":    503,
	"GATEWAY_TIMEOUT":        504,
}

// jaxrsStatusEnumCode resolves a JAX-RS / Micronaut status enum constant name to
// its code. A pure numeric token is rejected (handled by the numeric regexes).
func jaxrsStatusEnumCode(name string) (int, bool) {
	c, ok := jaxrsStatusEnumName[name]
	return c, ok
}

// jaxrsResponseCodes resolves the status-code set for a JAX-RS / Jakarta-family
// or Micronaut endpoint from the decorator/annotation region + handler body.
func jaxrsResponseCodes(region, body string) responseCodesVerdict {
	var v responseCodesVerdict

	// Micronaut @Status(...) in the annotation region.
	for _, m := range jaxrsStatusAnnotationRe.FindAllStringSubmatch(region, -1) {
		if c, ok := jaxrsStatusEnumCode(m[1]); ok {
			v.add(c)
			if v.source == "" {
				v.source = "@Status"
			}
		}
	}
	for _, m := range jaxrsStatusAnnotationNumericRe.FindAllStringSubmatch(region, -1) {
		if c, err := strconv.Atoi(m[1]); err == nil {
			v.add(c)
			if v.source == "" {
				v.source = "@Status"
			}
		}
	}

	// Response.status(...) / HttpResponse.status(...) — named enum form.
	for _, m := range jaxrsStatusEnumRe.FindAllStringSubmatch(body, -1) {
		if c, ok := jaxrsStatusEnumCode(m[1]); ok {
			v.add(c)
			if v.source == "" {
				v.source = "Response.status"
			}
		}
	}
	// Response.status(NNN) — numeric form.
	for _, m := range jaxrsStatusNumericRe.FindAllStringSubmatch(body, -1) {
		if c, err := strconv.Atoi(m[1]); err == nil {
			v.add(c)
			if v.source == "" {
				v.source = "Response.status"
			}
		}
	}

	// Response / HttpResponse builder factory helpers (ok / created / …).
	for _, m := range jaxrsBuilderFactoryRe.FindAllStringSubmatch(body, -1) {
		if c, ok := jaxrsBuilderFactoryCodes[m[1]]; ok {
			v.add(c)
			if v.source == "" {
				v.source = "Response." + m[1] + "()"
			}
		}
	}

	// new WebApplicationException(<status>) — explicit status form first.
	for _, m := range jaxrsWebApplicationExceptionRe.FindAllStringSubmatch(body, -1) {
		if m[1] != "" {
			if c, err := strconv.Atoi(m[1]); err == nil {
				v.add(c)
			}
		} else if m[2] != "" {
			if c, ok := jaxrsStatusEnumCode(m[2]); ok {
				v.add(c)
			}
		}
		if v.source == "" {
			v.source = "WebApplicationException"
		}
	}

	// Typed JAX-RS exception subclasses thrown in the body → mapped code.
	for _, m := range jaxrsThrowExceptionRe.FindAllStringSubmatch(body, -1) {
		if c, ok := jaxrsExceptionCodes[m[1]]; ok {
			v.add(c)
			if v.source == "" {
				v.source = "JAX-RS exception"
			}
		}
	}

	return v
}

// ---------------------------------------------------------------------------
// Pagination — JAX-RS @QueryParam pairs + Micronaut Pageable / Page<…>
// ---------------------------------------------------------------------------

// jaxrsQueryParamRe captures the query-param NAME from a JAX-RS
// `@QueryParam("limit")` annotation (group 1). Micronaut also uses
// `@QueryValue("limit")`, matched by jaxrsQueryValueRe.
var jaxrsQueryParamRe = regexp.MustCompile(`@QueryParam\s*\(\s*"([^"]+)"\s*\)`)

// jaxrsQueryValueRe captures the param NAME from a Micronaut
// `@QueryValue("limit")` / `@QueryValue(value = "offset")` annotation.
var jaxrsQueryValueRe = regexp.MustCompile(`@QueryValue\s*\(\s*(?:value\s*=\s*)?"([^"]+)"\s*\)`)

// jaxrsPaginationVerdict resolves pagination for a JAX-RS / Jakarta-family or
// Micronaut endpoint. Micronaut's Pageable param / Page<…> | Slice<…> return is
// reused from the Spring patterns (same data-layer types in the Micronaut Data
// world bind `page`/`size`); JAX-RS @QueryParam / Micronaut @QueryValue pairs are
// classified by the shared param-shape vocabulary.
func jaxrsPaginationVerdict(region string) (paginationVerdict, bool) {
	// Micronaut Data Pageable param / Page<…> | Slice<…> return (same shapes the
	// Spring resolver already recognises — Micronaut Data reuses the Spring Data
	// pagination types).
	if springPageableParamRe.MatchString(region) || springPageReturnRe.MatchString(region) {
		return paginationVerdict{
			paginated: true,
			style:     "page",
			params:    []string{"page", "size"},
			source:    "Micronaut Pageable",
		}, true
	}

	// JAX-RS @QueryParam / Micronaut @QueryValue param names → shared classifier.
	present := map[string]bool{}
	for _, re := range []*regexp.Regexp{jaxrsQueryParamRe, jaxrsQueryValueRe} {
		for _, m := range re.FindAllStringSubmatch(region, -1) {
			name := strings.ToLower(m[1])
			if isPaginationParam(name) {
				present[name] = true
			}
		}
	}
	if v, ok := classifyParamShape(present, "@QueryParam"); ok {
		return v, true
	}
	return paginationVerdict{}, false
}
