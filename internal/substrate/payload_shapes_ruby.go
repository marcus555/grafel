// Ruby payload-shape sniffer (#2771 Phase 2A T2; REST-sibling credit #3951).
//
// Producer-side shapes (Rails / Sinatra / Grape / Hanami handlers):
//
//   - `params.require(:foo).permit(:a, :b, :c)` — the permit list is
//     the canonical Rails strong-parameter request shape.
//   - `params[:x]`, `params.fetch(:x)`, `params.fetch("x")` — bare
//     symbol / string indexed reads bind to the enclosing action.
//   - `render json: { x: ..., y: ... }` — inline hash returned as the
//     response body. The hash literal is captured single-line.
//   - `json({ x: ..., y: ... })` / `json x: ..., y: ...` — Sinatra /
//     Grape / Padrino JSON-helper response body. Same single-line hash
//     capture as `render json:`.
//   - Grape `requires :name` / `optional :age` declarations inside a
//     `params do … end` block become the request shape (#3951). Each
//     leading symbol is one field.
//   - Grape `expose :name` declarations inside an `Entity` class become
//     the response shape for any endpoint that `presents` the entity
//     (the present/expose pairing is recognised conservatively per
//     enclosing block via nearestHeader).
//
// Header attribution (#3951): Rails actions and Hanami actions declare
// their handler as `def name` / `def call`, so the stock `def` headers
// bind their shapes. Sinatra / Grape / Padrino / Roda instead declare
// the handler as a routing DSL block (`post '/users' do … end`) with no
// `def`, and Grape entities/params live in bare `class … < Grape::*` /
// `params do` blocks. scanRubyShapeHeaders augments the `def` header set
// with these DSL anchors so a sibling framework's request/response shape
// binds to a stable handler name (`POST /users`, the API/Entity class,
// or `params`) instead of being silently dropped at module scope.
//
// Consumer-side shapes (HTTParty / Faraday / Net::HTTP / RestClient):
//
//   - `HTTParty.<verb>(url, body: { x: ... }.to_json)`,
//     `HTTParty.<verb>(url, query: { x: ... })`
//   - `Faraday.<verb>(url) { |req| req.body = { x: ... }.to_json }` —
//     we match the inline hash regardless of the block wrapper.
//   - `Net::HTTP.post_form(URI(url), { "x" => ... })` — the form-hash
//     literal contributes the field set.
//
// Optional/required: Ruby doesn't statically annotate. Phase 2A leaves
// Optional default-false on every shape.
package substrate

import (
	"regexp"
	"strings"
)

func init() { RegisterPayloadShapeSniffer("ruby", sniffPayloadShapesRuby) }

// rubyPermitListRe matches the canonical Rails strong-parameter pattern
// `params.require(:foo).permit(:a, :b, :c)`. Capture group 1 = the
// permit arg list between parens (we then split on commas to extract
// the bare symbols).
var rubyPermitListRe = regexp.MustCompile(
	`\bparams\s*(?:\.\s*require\s*\([^)]*\))?\s*\.\s*permit\s*\(([^)]*)\)`,
)

// rubyParamsIndexRe matches `params[:x]` and `params["x"]` and
// `params.fetch(:x)` / `params.fetch("x")`. Capture groups 1/2/3/4 each
// hold the bare name (first non-empty wins).
var rubyParamsIndexRe = regexp.MustCompile(
	`\bparams\s*` +
		`(?:\[\s*:([A-Za-z_][\w]*)\s*\]` +
		`|\[\s*['"]([A-Za-z_][\w]*)['"]\s*\]` +
		`|\.\s*fetch\s*\(\s*:([A-Za-z_][\w]*)` +
		`|\.\s*fetch\s*\(\s*['"]([A-Za-z_][\w]*)['"])`,
)

// rubyRenderJSONRe matches `render json: { ... }`. Capture group 1 is
// the hash body between { and the first balanced } on the same line.
var rubyRenderJSONRe = regexp.MustCompile(
	`\brender\s+json\s*:\s*\{([^{}]*)\}`,
)

// rubyGrapeExposeRe matches Grape `expose :name` (the entity pattern).
// Capture group 1 = the field name.
var rubyGrapeExposeRe = regexp.MustCompile(
	`(?m)^\s*expose\s+:([A-Za-z_][\w]*)`,
)

// rubyConsumerHTTPRe matches inline outbound calls with a hash body.
// Capture groups:
//
//	1 = HTTParty verb when present
//	2 = inline URL for HTTParty
//	3 = body hash (between { and })
//	4 = Faraday verb when present
//	5 = inline URL for Faraday
//	6 = body hash for Faraday (req.body = {...})
var rubyConsumerHTTPRe = regexp.MustCompile(
	`\bHTTParty\s*\.\s*(get|post|put|patch|delete|head)\s*\(\s*['"]([^'"]*)['"][^)]*?(?:body|query|json)\s*:\s*\{([^{}]*)\}` +
		`|\bFaraday\s*\.\s*(get|post|put|patch|delete|head)\s*\(\s*['"]([^'"]*)['"][^)]*?\.\s*body\s*=\s*\{([^{}]*)\}`,
)

// rubyHashKeyRe matches a key in a Ruby hash literal: bare `name:`,
// symbol `:name =>`, or string `"name" =>`. Capture groups 1/2/3 hold
// the name (first non-empty wins).
var rubyHashKeyRe = regexp.MustCompile(
	`([A-Za-z_][\w]*)\s*:` +
		`|:([A-Za-z_][\w]*)\s*=>` +
		`|['"]([A-Za-z_][\w]*)['"]\s*=>`,
)

// rubySymbolListRe matches a single `:name` symbol inside the permit
// argument list. Bare identifiers with no colon are ignored — Rails
// permit only accepts symbols / strings / hashes.
var rubySymbolListRe = regexp.MustCompile(`:([A-Za-z_][\w]*)`)

// rubyJSONHelperRe matches the Sinatra / Grape / Padrino JSON-response
// helper `json({ ... })` or `json(...)` carrying an inline hash literal.
// Capture group 1 is the hash body between the first `{` and its
// matching single-line `}`. The leading `json` must be a statement /
// expression head (start-of-line or after `=`, `(`, `do`, `;`) so it
// does NOT fire on `to_json`, `render json:` (handled separately), or a
// `:json` symbol. (#3951)
var rubyJSONHelperRe = regexp.MustCompile(
	`(?m)(?:^|[=(]|\bdo\b|;)\s*json\s*\(?\s*\{([^{}]*)\}`,
)

// rubyJSONGenerateRe matches a Hanami / plain-Ruby response body built
// from an inline hash via `JSON.generate({ ... })` or `JSON.dump({ ... })`
// (commonly `self.body = JSON.generate({...})` in a Hanami action).
// Capture group 1 is the hash body. (#3951)
var rubyJSONGenerateRe = regexp.MustCompile(
	`\bJSON\s*\.\s*(?:generate|dump)\s*\(\s*\{([^{}]*)\}`,
)

// rubyGrapeRequiresRe matches Grape `requires :name, …` / `optional
// :age, …` declarations inside a `params do … end` block. Capture group
// 1 is the field symbol. Bare `requires`/`optional` keep this scoped to
// the Grape params DSL (Hanami/dry-validation use a different surface).
// (#3951)
var rubyGrapeRequiresRe = regexp.MustCompile(
	`(?m)^\s*(?:requires|optional)\s+:([A-Za-z_][\w]*)`,
)

// rubyDefHeaderRe matches a `def name` / `def self.name` method header.
// Capture group 1 = the bare method name. (Mirrors rubyFuncHeaderRe in
// effect_sinks_ruby.go; duplicated here so the payload-shape header set
// can be extended with DSL anchors without disturbing the effect pass.)
var rubyDefHeaderRe = regexp.MustCompile(
	`(?m)^\s*def\s+(?:self\s*\.\s*)?([A-Za-z_][\w]*[!?=]?)\b`,
)

// rubyRouteBlockRe matches a Sinatra / Grape / Padrino / Roda routing
// DSL block header: `get '/path' do`, `post "/users" do`, optionally
// `do |params|`. Capture group 1 = the verb, group 2 = the route path.
// The trailing `do` (with optional block args) is required so a bare
// method call like `get('/x')` without a block body is not mistaken for
// a handler header. (#3951)
var rubyRouteBlockRe = regexp.MustCompile(
	`(?m)^\s*(get|post|put|patch|delete|head|options)\s+['"]([^'"]*)['"][^\n]*\bdo\b`,
)

// rubyGrapeClassHeaderRe matches a Grape API or Entity class header
// (`class Users < Grape::API`, `class User < Grape::Entity`). Capture
// group 1 = the class name; it anchors `requires`/`optional`/`expose`
// declarations that live in the class body but outside any `def`. (#3951)
var rubyGrapeClassHeaderRe = regexp.MustCompile(
	`(?m)^\s*class\s+([A-Za-z_][\w]*)\s*<\s*Grape::(?:API|Entity)\b`,
)

// rubyParamsBlockRe matches a Grape `params do` block opener so a
// `requires`/`optional` set binds to a stable `params` handler name when
// it is not enclosed by a Grape class header. (#3951)
var rubyParamsBlockRe = regexp.MustCompile(
	`(?m)^\s*params\s+do\b`,
)

// scanRubyShapeHeaders builds the payload-shape header set: stock `def`
// methods (Rails/Hanami actions) PLUS the routing-DSL / Grape-class /
// params-block anchors (#3951) so Sinatra/Grape/Padrino/Roda handlers —
// which never use `def` — get a stable handler name to bind shapes to.
// Headers are returned in ascending source-line order, as nearestHeader
// requires.
func scanRubyShapeHeaders(content string) []funcHeader {
	var hs []funcHeader
	add := func(line int, name string) { hs = append(hs, funcHeader{Line: line, Name: name}) }

	for _, m := range rubyDefHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		add(lineOfOffset(content, m[0]), content[m[2]:m[3]])
	}
	for _, m := range rubyRouteBlockRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		verb := strings.ToUpper(content[m[2]:m[3]])
		path := content[m[4]:m[5]]
		add(lineOfOffset(content, m[0]), verb+" "+path)
	}
	for _, m := range rubyGrapeClassHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		add(lineOfOffset(content, m[0]), content[m[2]:m[3]])
	}
	for _, m := range rubyParamsBlockRe.FindAllStringIndex(content, -1) {
		add(lineOfOffset(content, m[0]), "params")
	}

	sortFuncHeadersByLine(hs)
	return hs
}

// sortFuncHeadersByLine sorts headers ascending by source line so
// nearestHeader's "greatest line ≤ target" scan is correct. Insertion
// sort — header counts per file are small.
func sortFuncHeadersByLine(hs []funcHeader) {
	for i := 1; i < len(hs); i++ {
		for j := i; j > 0 && hs[j-1].Line > hs[j].Line; j-- {
			hs[j-1], hs[j] = hs[j], hs[j-1]
		}
	}
}

func sniffPayloadShapesRuby(content string) []PayloadShape {
	if content == "" {
		return nil
	}
	headers := scanRubyShapeHeaders(content)
	var out []PayloadShape

	// Producer-side: permit list → request shape (high confidence).
	for _, m := range rubyPermitListRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		body := content[m[2]:m[3]]
		var fields []PayloadField
		for _, sm := range rubySymbolListRe.FindAllStringSubmatchIndex(body, -1) {
			if len(sm) < 4 {
				continue
			}
			fields = append(fields, PayloadField{Name: body[sm[2]:sm[3]]})
		}
		if len(fields) == 0 {
			continue
		}
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		if fn == "" {
			continue
		}
		out = append(out, PayloadShape{
			Function:   fn,
			Line:       line,
			Direction:  PayloadDirectionRequest,
			Side:       PayloadSideProducer,
			Fields:     DedupFields(fields),
			Confidence: 1.0,
		})
	}

	// Producer-side: Grape `requires :x` / `optional :y` declarations in
	// a `params do … end` block → request shape (#3951). Bucketed by the
	// enclosing header (the Grape API class or the `params` block).
	reqFields := map[string][]PayloadField{}
	reqFirstLine := map[string]int{}
	for _, m := range rubyGrapeRequiresRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		name := content[m[2]:m[3]]
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		if fn == "" {
			continue
		}
		reqFields[fn] = append(reqFields[fn], PayloadField{Name: name})
		if _, ok := reqFirstLine[fn]; !ok {
			reqFirstLine[fn] = line
		}
	}
	for fn, fields := range reqFields {
		out = append(out, PayloadShape{
			Function:   fn,
			Line:       reqFirstLine[fn],
			Direction:  PayloadDirectionRequest,
			Side:       PayloadSideProducer,
			Fields:     DedupFields(fields),
			Confidence: 0.9,
		})
	}

	// Producer-side: bare params[:x] / params.fetch reads — fallback
	// when the handler doesn't use strong parameters.
	idxFields := map[string][]PayloadField{}
	idxFirstLine := map[string]int{}
	for _, m := range rubyParamsIndexRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		var name string
		switch {
		case m[2] >= 0:
			name = content[m[2]:m[3]]
		case m[4] >= 0:
			name = content[m[4]:m[5]]
		case m[6] >= 0:
			name = content[m[6]:m[7]]
		case m[8] >= 0:
			name = content[m[8]:m[9]]
		}
		if name == "" {
			continue
		}
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		if fn == "" {
			continue
		}
		idxFields[fn] = append(idxFields[fn], PayloadField{Name: name})
		if _, ok := idxFirstLine[fn]; !ok {
			idxFirstLine[fn] = line
		}
	}
	for fn, fields := range idxFields {
		out = append(out, PayloadShape{
			Function:   fn,
			Line:       idxFirstLine[fn],
			Direction:  PayloadDirectionRequest,
			Side:       PayloadSideProducer,
			Fields:     DedupFields(fields),
			Confidence: 0.8,
		})
	}

	// Producer-side: render json: {...} response shape.
	for _, m := range rubyRenderJSONRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		body := content[m[2]:m[3]]
		fields := extractRubyHashKeys(body)
		if len(fields) == 0 {
			continue
		}
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		if fn == "" {
			continue
		}
		out = append(out, PayloadShape{
			Function:   fn,
			Line:       line,
			Direction:  PayloadDirectionResponse,
			Side:       PayloadSideProducer,
			Fields:     fields,
			Confidence: 1.0,
		})
	}

	// Producer-side: inline-hash response helpers (#3951) — Sinatra /
	// Grape / Padrino `json({ ... })` and Hanami / plain-Ruby
	// `JSON.generate({ ... })` / `JSON.dump({ ... })`. Same single-line
	// hash capture as `render json:`.
	for _, re := range []*regexp.Regexp{rubyJSONHelperRe, rubyJSONGenerateRe} {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			if len(m) < 4 {
				continue
			}
			body := content[m[2]:m[3]]
			fields := extractRubyHashKeys(body)
			if len(fields) == 0 {
				continue
			}
			line := lineOfOffset(content, m[0])
			fn := nearestHeader(headers, line)
			if fn == "" {
				continue
			}
			out = append(out, PayloadShape{
				Function:   fn,
				Line:       line,
				Direction:  PayloadDirectionResponse,
				Side:       PayloadSideProducer,
				Fields:     fields,
				Confidence: 1.0,
			})
		}
	}

	// Producer-side: Grape expose declarations bucket-by-enclosing-
	// header (the Entity class' header binds them).
	exposeFields := map[string][]PayloadField{}
	exposeFirstLine := map[string]int{}
	for _, m := range rubyGrapeExposeRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		name := content[m[2]:m[3]]
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		if fn == "" {
			continue
		}
		exposeFields[fn] = append(exposeFields[fn], PayloadField{Name: name})
		if _, ok := exposeFirstLine[fn]; !ok {
			exposeFirstLine[fn] = line
		}
	}
	for fn, fields := range exposeFields {
		out = append(out, PayloadShape{
			Function:   fn,
			Line:       exposeFirstLine[fn],
			Direction:  PayloadDirectionResponse,
			Side:       PayloadSideProducer,
			Fields:     DedupFields(fields),
			Confidence: 0.85,
		})
	}

	// Consumer-side: HTTParty / Faraday inline body.
	for _, m := range rubyConsumerHTTPRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 14 {
			continue
		}
		var verb, url, body string
		switch {
		case m[2] >= 0:
			verb = strings.ToUpper(content[m[2]:m[3]])
			url = content[m[4]:m[5]]
			body = content[m[6]:m[7]]
		case m[8] >= 0:
			verb = strings.ToUpper(content[m[8]:m[9]])
			url = content[m[10]:m[11]]
			body = content[m[12]:m[13]]
		}
		fields := extractRubyHashKeys(body)
		if len(fields) == 0 {
			continue
		}
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		out = append(out, PayloadShape{
			Function:     fn,
			Line:         line,
			Direction:    PayloadDirectionRequest,
			Side:         PayloadSideConsumer,
			Fields:       fields,
			Confidence:   1.0,
			EndpointHint: url,
			VerbHint:     verb,
		})
	}

	return out
}

// extractRubyHashKeys lifts the keys out of a Ruby hash-literal body,
// recognising both modern (`name: value`) and rocket (`:name => value`
// / `"name" => value`) forms. Deduped, source order preserved.
func extractRubyHashKeys(body string) []PayloadField {
	var fields []PayloadField
	for _, m := range rubyHashKeyRe.FindAllStringSubmatchIndex(body, -1) {
		if len(m) < 8 {
			continue
		}
		var name string
		switch {
		case m[2] >= 0:
			name = body[m[2]:m[3]]
		case m[4] >= 0:
			name = body[m[4]:m[5]]
		case m[6] >= 0:
			name = body[m[6]:m[7]]
		}
		if name == "" {
			continue
		}
		fields = append(fields, PayloadField{Name: name})
	}
	return DedupFields(fields)
}
