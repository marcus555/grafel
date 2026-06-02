// C# payload-shape sniffer (#2771 Phase 2A T2).
//
// Producer-side shapes (ASP.NET Core / Carter / FastEndpoints):
//
//   - `[FromBody] T name` parameter — the wrapped DTO T's public
//     properties (declared in the same file) become the request shape.
//   - DTO classes / records: `public string Name { get; set; }` and
//     positional record `record CreateDto(string Name, int Age)`.
//     `[Required]` is treated as default (Optional=false); nullable
//     reference / `int?` flips Optional=true.
//   - Response shapes: `return Ok(new T { A = ..., B = ... })` and
//     `return new JsonResult(new { A = ..., B = ... })` — anonymous
//     object initializers contribute their property set.
//
// Consumer-side shapes (HttpClient + JsonContent / StringContent):
//
//   - `client.PostAsJsonAsync(url, new { A = ..., B = ... })`
//   - `JsonContent.Create(new { A = ..., B = ... })`
//   - `new StringContent(JsonSerializer.Serialize(new { A = ... }))` —
//     out of scope for the regex sniffer (multi-call); we recognise
//     PostAsJsonAsync as the canonical idiom.
//
// Optional/required: `?` (nullable) flips Optional=true; everything
// else stays default-false (the `[Required]` attribute is implied).
package substrate

import (
	"regexp"
	"strings"
)

func init() { RegisterPayloadShapeSniffer("csharp", sniffPayloadShapesCSharp) }

// csClassHeaderRe matches a C# class or record declaration.
// Capture group 1 = the type name.
var csClassHeaderRe = regexp.MustCompile(
	`(?m)^\s*(?:public\s+|internal\s+|private\s+|protected\s+|sealed\s+|abstract\s+|static\s+|partial\s+)*(?:class|record)\s+([A-Z][\w]*)\b`,
)

// csPropertyRe matches a public property declaration:
// `public string Name { get; set; }`. Capture group 1 = type;
// group 2 = name.
var csPropertyRe = regexp.MustCompile(
	`(?m)^\s*public\s+([A-Za-z_][\w<>?\[\],\s.]*?)\s+([A-Z][\w]*)\s*\{\s*get\s*;`,
)

// csPositionalRecordRe matches `record Name(string A, int B, ...)`.
// Capture group 1 = type name; group 2 = the parameter list body.
var csPositionalRecordRe = regexp.MustCompile(
	`(?m)^\s*(?:public\s+|internal\s+)?record\s+([A-Z][\w]*)\s*\(([^)]*)\)`,
)

// csFromBodyParamRe matches `[FromBody] T name`. Capture group 1 = T;
// group 2 = parameter name.
var csFromBodyParamRe = regexp.MustCompile(
	`\[FromBody\]\s+([A-Z][\w<>,?\s.]*)\s+([A-Za-z_][\w]*)`,
)

// csAnonObjectRe matches `new { A = ..., B = ... }`. Capture group 1 =
// body between `{` and `}`. Bounded single-level.
var csAnonObjectRe = regexp.MustCompile(
	`\bnew\s*\{([^{}]*)\}`,
)

// csPostAsJsonRe matches `client.PostAsJsonAsync(url, ...)`. Capture
// group 1 = inline URL when present, group 2 = verb (POST/PUT etc).
var csPostAsJsonRe = regexp.MustCompile(
	`\.\s*(Post|Put|Patch|Get|Delete)AsJsonAsync\s*\(\s*[$@]?"([^"]*)"`,
)

// csAnonAssignRe matches `Identifier =` inside an anonymous-object
// body. Capture group 1 = the property name. C# property casing is
// conventionally PascalCase but anonymous-object members can be any
// identifier shape; we accept both. The trailing `(?:[^=])` excludes
// `==` comparisons.
var csAnonAssignRe = regexp.MustCompile(
	`\b([A-Za-z_][\w]*)\s*=(?:[^=])`,
)

func sniffPayloadShapesCSharp(content string) []PayloadShape {
	if content == "" {
		return nil
	}
	headers := scanCSharpFuncHeaders(content)
	classFields := scanCSharpClassFields(content)

	var out []PayloadShape

	// Producer-side: [FromBody] T name → request shape from T.
	for _, m := range csFromBodyParamRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		typ := strings.TrimSpace(content[m[2]:m[3]])
		typ = stripGenericSuffix(typ)
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		if fn == "" {
			continue
		}
		fields := classFields[typ]
		conf := 0.9
		if len(fields) == 0 {
			conf = 0.4
		}
		out = append(out, PayloadShape{
			Function:   fn,
			Line:       line,
			Direction:  PayloadDirectionRequest,
			Side:       PayloadSideProducer,
			Fields:     fields,
			Confidence: conf,
		})
	}

	// Producer-side: anonymous-object initializers as response shape.
	// We must also tag PostAsJsonAsync usages as consumer; detect those
	// by checking whether the surrounding line contains the marker.
	clientLines := scanCSharpClientLines(content)
	for _, m := range csAnonObjectRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		body := content[m[2]:m[3]]
		fields := extractCSharpAnonKeys(body)
		if len(fields) == 0 {
			continue
		}
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		if hint, ok := clientLines[line]; ok {
			out = append(out, PayloadShape{
				Function:     fn,
				Line:         line,
				Direction:    PayloadDirectionRequest,
				Side:         PayloadSideConsumer,
				Fields:       fields,
				Confidence:   1.0,
				EndpointHint: hint.url,
				VerbHint:     hint.verb,
			})
			continue
		}
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

	// Producer-side: HotChocolate GraphQL resolver methods (#3961). A resolver's
	// typed argument list is the request shape; its typed return is the response
	// shape. Gated on a HotChocolate file-signal so plain ASP.NET methods don't
	// double-emit.
	out = append(out, sniffHotChocolateResolverShapes(content, classFields)...)

	return out
}

// hcShapeSignalRe detects a HotChocolate GraphQL server in the file (import or
// fluent server registration), mirroring hotChocolateHasSignal in the engine.
var hcShapeSignalRe = regexp.MustCompile(`HotChocolate|\.AddGraphQLServer\s*\(`)

// hcResolverSigRe matches a public HotChocolate resolver method declaration,
// capturing the return type, method name, and the parenthesised argument list.
//
//	public User GetUser(GetUserInput input) => ...
//	public async Task<User> GetUser(int id, CancellationToken ct) { ... }
//
// Group 1 = return type chunk; group 2 = method name; group 3 = arg list body.
var hcResolverSigRe = regexp.MustCompile(
	`(?m)^\s*(?:\[[^\]\r\n]+\]\s*)*\s*public\s+` +
		`(?:static\s+|virtual\s+|override\s+|async\s+|sealed\s+)*` +
		`([\w<>\[\],.\s?]+?)\s+([A-Za-z_]\w*)\s*\(([^)]*)\)`,
)

// hcAmbientArgTypes are framework-injected resolver parameters that are NOT part
// of the GraphQL request shape (HotChocolate service/context injection). Matched
// on the leaf type name, case-insensitively.
var hcAmbientArgTypes = map[string]bool{
	"cancellationtoken":    true,
	"iresolvercontext":     true,
	"claimsprincipal":      true,
	"httpcontext":          true,
	"ihttpcontextaccessor": true,
}

// sniffHotChocolateResolverShapes emits one producer request shape (from the
// typed argument list) and one producer response shape (from the return type)
// per HotChocolate resolver method. A request field whose static type resolves
// to a DTO class in this file expands to that DTO's fields; a scalar argument
// contributes a single field named after the parameter. The response shape is
// the return type's DTO fields (after stripping Task<>/IEnumerable<>/nullable
// wrappers).
func sniffHotChocolateResolverShapes(content string, classFields map[string][]PayloadField) []PayloadShape {
	if !hcShapeSignalRe.MatchString(content) {
		return nil
	}
	var out []PayloadShape
	for _, m := range hcResolverSigRe.FindAllStringSubmatchIndex(content, -1) {
		retType := strings.TrimSpace(content[m[2]:m[3]])
		method := content[m[4]:m[5]]
		argList := content[m[6]:m[7]]
		line := lineOfOffset(content, m[0])

		// Skip non-resolver return types: void / constructors have no return
		// type captured by the regex (it requires `<ret> <name>`), but guard
		// against `void` explicitly.
		if retType == "" || retType == "void" {
			continue
		}

		// Request shape from the typed argument list.
		if reqFields := hcRequestFields(argList, classFields); len(reqFields) > 0 {
			out = append(out, PayloadShape{
				Function:   method,
				Line:       line,
				Direction:  PayloadDirectionRequest,
				Side:       PayloadSideProducer,
				Fields:     DedupFields(reqFields),
				Confidence: 0.85,
			})
		}

		// Response shape from the (unwrapped) return type's DTO fields.
		retLeaf := hcUnwrapType(retType)
		if fields := classFields[retLeaf]; len(fields) > 0 {
			out = append(out, PayloadShape{
				Function:   method,
				Line:       line,
				Direction:  PayloadDirectionResponse,
				Side:       PayloadSideProducer,
				Fields:     fields,
				Confidence: 0.85,
			})
		}
	}
	return out
}

// hcRequestFields builds the request-shape fields from a resolver argument list.
// A DTO-typed argument (type resolves to a class in the file) expands to that
// DTO's fields; a scalar/unknown argument contributes one field named after the
// parameter. Framework-injected ambient parameters are skipped.
func hcRequestFields(argList string, classFields map[string][]PayloadField) []PayloadField {
	var fields []PayloadField
	for _, raw := range splitTopLevel(argList) {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		// Drop a leading parameter attribute (e.g. `[Service] X x`).
		for strings.HasPrefix(p, "[") {
			if j := strings.IndexByte(p, ']'); j >= 0 {
				p = strings.TrimSpace(p[j+1:])
			} else {
				break
			}
		}
		// Drop a default-value suffix (`int id = 0`).
		if eq := strings.IndexByte(p, '='); eq >= 0 {
			p = strings.TrimSpace(p[:eq])
		}
		// `Type Name` — split on last whitespace.
		i := strings.LastIndexAny(p, " \t")
		if i <= 0 {
			continue
		}
		typ := strings.TrimSpace(p[:i])
		name := strings.TrimSpace(p[i+1:])
		if !isPlainIdent(name) {
			continue
		}
		leaf := hcUnwrapType(typ)
		if hcAmbientArgTypes[strings.ToLower(leaf)] {
			continue
		}
		if dtoFields := classFields[leaf]; len(dtoFields) > 0 {
			fields = append(fields, dtoFields...)
			continue
		}
		optional := strings.HasSuffix(typ, "?")
		fields = append(fields, PayloadField{Name: name, Type: typ, Optional: optional})
	}
	return fields
}

// hcUnwrapType reduces a resolver type to its leaf DTO name by stripping common
// async / collection / nullable wrappers: `Task<User>` → `User`,
// `IEnumerable<User?>` → `User`, `List<User>` → `User`, `User?` → `User`.
// Namespace qualification is dropped (`Types.User` → `User`).
func hcUnwrapType(t string) string {
	t = strings.TrimSpace(t)
	// Peel one or more generic wrappers down to the innermost argument.
	for {
		lt := strings.IndexByte(t, '<')
		gt := strings.LastIndexByte(t, '>')
		if lt < 0 || gt < 0 || gt < lt {
			break
		}
		t = strings.TrimSpace(t[lt+1 : gt])
	}
	t = strings.TrimSuffix(t, "?")
	t = strings.TrimSuffix(t, "[]")
	t = strings.TrimSpace(t)
	if i := strings.LastIndexByte(t, '.'); i >= 0 {
		t = t[i+1:]
	}
	return t
}

// scanCSharpClassFields walks the file once and returns a map of
// className → []PayloadField. Recognises both property-style classes
// and positional records.
func scanCSharpClassFields(content string) map[string][]PayloadField {
	out := map[string][]PayloadField{}
	// Property-style: bucket between consecutive class headers.
	type block struct {
		name       string
		start, end int
	}
	var blocks []block
	matches := csClassHeaderRe.FindAllStringSubmatchIndex(content, -1)
	for i, m := range matches {
		if len(m) < 4 {
			continue
		}
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		blocks = append(blocks, block{name: content[m[2]:m[3]], start: m[1], end: end})
	}
	for _, b := range blocks {
		body := content[b.start:b.end]
		var fields []PayloadField
		for _, fm := range csPropertyRe.FindAllStringSubmatchIndex(body, -1) {
			if len(fm) < 6 {
				continue
			}
			typ := strings.TrimSpace(body[fm[2]:fm[3]])
			name := body[fm[4]:fm[5]]
			optional := strings.HasSuffix(typ, "?")
			fields = append(fields, PayloadField{Name: name, Type: typ, Optional: optional})
		}
		if len(fields) > 0 {
			out[b.name] = DedupFields(fields)
		}
	}
	// Positional records: shape = constructor parameters.
	for _, m := range csPositionalRecordRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		name := content[m[2]:m[3]]
		paramList := content[m[4]:m[5]]
		var fields []PayloadField
		for _, p := range splitTopLevel(paramList) {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// `Type Name` — split on last whitespace.
			i := strings.LastIndexAny(p, " \t")
			if i <= 0 {
				continue
			}
			typ := strings.TrimSpace(p[:i])
			fname := strings.TrimSpace(p[i+1:])
			if !isPlainIdent(fname) {
				continue
			}
			optional := strings.HasSuffix(typ, "?")
			fields = append(fields, PayloadField{Name: fname, Type: typ, Optional: optional})
		}
		if len(fields) > 0 {
			out[name] = DedupFields(fields)
		}
	}
	return out
}

// csharpClientHint mirrors the rust hint struct.
type csharpClientHint struct {
	url  string
	verb string
}

// scanCSharpClientLines returns a line-keyed map of HttpClient
// PostAsJsonAsync / PutAsJsonAsync call sites. The anonymous-object
// recognition uses this to flip side=consumer when the literal is the
// body argument on the same line.
func scanCSharpClientLines(content string) map[int]csharpClientHint {
	out := map[int]csharpClientHint{}
	for _, m := range csPostAsJsonRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		line := lineOfOffset(content, m[0])
		verb := strings.ToUpper(content[m[2]:m[3]])
		// HttpClient.{Verb}AsJsonAsync → HTTP verb is the prefix.
		out[line] = csharpClientHint{url: content[m[4]:m[5]], verb: verb}
	}
	return out
}

// extractCSharpAnonKeys lifts `Name =` property assignments out of a
// `new { ... }` body. Deduped, source order preserved.
func extractCSharpAnonKeys(body string) []PayloadField {
	var fields []PayloadField
	for _, m := range csAnonAssignRe.FindAllStringSubmatchIndex(body, -1) {
		if len(m) < 4 {
			continue
		}
		fields = append(fields, PayloadField{Name: body[m[2]:m[3]]})
	}
	return DedupFields(fields)
}
