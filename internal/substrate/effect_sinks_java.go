// Java effect-sink sniffer (#2764 Phase 1A T1).
//
// Recognises Java sink primitives:
//
//   - http_out  : RestTemplate.<verb>, WebClient.method, HttpClient.send,
//     OkHttpClient.newCall, Feign / Retrofit interface calls
//     (heuristic — caught by the method-on-receiver shapes),
//     java.net.URL.openConnection / openStream
//   - db_read   : EntityManager.find / createQuery, JpaRepository.find*
//     / get* / count* / exists*, JdbcTemplate.queryFor*,
//     Statement.executeQuery, Spring Data Mongo .findBy*
//   - db_write  : EntityManager.persist / merge / remove,
//     JpaRepository.save* / delete*, JdbcTemplate.update /
//     batchUpdate, Statement.executeUpdate
//   - fs_read   : Files.readAllBytes / readAllLines / lines / newInputStream,
//     FileInputStream(<...>), new BufferedReader(new FileReader),
//     Paths.get(...).toFile().exists() style → modeled by Files.*
//   - fs_write  : Files.write / writeString / newOutputStream / createFile
//     / createDirectories / delete / move / copy,
//     FileOutputStream(<...>), FileWriter(<...>)
//   - mutation  : `this.<field> = ...` assignment inside a method body
//   - message_publish : SmallRye reactive-messaging publish sites —
//     `Emitter.send(...)` / `MutinyEmitter.send(...)` / `.sendMessage(...)`
//     on a receiver whose identifier contains "emitter", PLUS
//     `<field>.send(...)` / `<field>.sendMessage(...)` where `<field>` is
//     pre-scanned from the file as an `Emitter<...>` / `MutinyEmitter<...>`
//     typed field declaration — idiomatic SmallRye names the field after
//     its `@Channel`, e.g. `@Channel("orders-out") Emitter<T> ordersOut`,
//     not "emitter" — and any `@Outgoing("...")`-annotated method (the
//     method itself is a publisher via its return value, even with no
//     explicit `.send` call). ADR-0025 §2.
//
// Function attribution uses the same nearest-header heuristic as the
// other T1 sniffers.
package substrate

import (
	"regexp"
	"strings"
)

func init() { RegisterEffectSniffer("java", sniffEffectsJava) }

// javaMethodHeaderRe matches a Java method signature header. Conservative
// matcher — requires a modifier (public/private/protected/static/...) or
// a return-type token, the method name, parens, optional `throws ...`,
// and the opening brace on the same logical line. Lambdas are not captured
// (Phase 4 will handle them via the real parser).
//
// Capture group 1 is the method name.
var javaMethodHeaderRe = regexp.MustCompile(
	`(?m)^\s*` +
		`(?:(?:public|private|protected|static|final|abstract|synchronized|native|default|@[A-Za-z_][\w]*(?:\([^)]*\))?)\s+)+` +
		`(?:<[^>]+>\s+)?` +
		`[A-Za-z_$][\w$<>\[\],?.\s]*\s+` +
		`([A-Za-z_$][\w$]*)\s*\(`,
)

// javaCtorHeaderRe captures constructor declarations: `public Foo(...)`.
var javaCtorHeaderRe = regexp.MustCompile(
	`(?m)^\s*(?:public|private|protected)\s+([A-Z][\w$]*)\s*\(`,
)

// javaHTTPRe matches outbound HTTP sites.
var javaHTTPRe = regexp.MustCompile(
	`\b(?:restTemplate|webClient|httpClient|okHttpClient|client)\s*\.\s*(?:getForObject|getForEntity|postForObject|postForEntity|put|patch|delete|exchange|execute|send|sendAsync|newCall|method|head|options|get|post)\s*\(` +
		`|\bnew\s+URL\s*\([^)]*\)\s*\.\s*(?:openConnection|openStream)\s*\(` +
		`|\bHttpRequest\s*\.\s*newBuilder\b`,
)

// javaDBReadRe matches JPA / Spring Data / JDBC read primitives.
var javaDBReadRe = regexp.MustCompile(
	`\b(?:entityManager|em)\s*\.\s*(?:find|getReference|createQuery|createNamedQuery|createNativeQuery)\s*\(` +
		`|\.\s*(?:findById|findAll|findBy[A-Za-z_]\w*|findOne|getOne|count(?:By[A-Za-z_]\w*)?|exists(?:ById|By[A-Za-z_]\w*)?)\s*\(` +
		`|\b(?:jdbcTemplate|namedJdbcTemplate)\s*\.\s*(?:queryFor[A-Za-z]+|query|queryForList|queryForMap|queryForObject)\s*\(` +
		`|\.\s*executeQuery\s*\(` +
		`|\bcriteriaBuilder\s*\.\s*createQuery\s*\(`,
)

// javaDBWriteRe matches JPA / Spring Data / JDBC write primitives.
var javaDBWriteRe = regexp.MustCompile(
	`\b(?:entityManager|em)\s*\.\s*(?:persist|merge|remove|refresh|flush)\s*\(` +
		`|\.\s*(?:save|saveAll|saveAndFlush|delete(?:All|ById|InBatch)?|deleteBy[A-Za-z_]\w*|updateBy[A-Za-z_]\w*|insert)\s*\(` +
		`|\b(?:jdbcTemplate|namedJdbcTemplate)\s*\.\s*(?:update|batchUpdate|execute)\s*\(` +
		`|\.\s*executeUpdate\s*\(`,
)

// javaFSReadRe matches NIO / classic-IO read primitives.
var javaFSReadRe = regexp.MustCompile(
	`\bFiles\s*\.\s*(?:readAllBytes|readAllLines|readString|lines|newInputStream|newBufferedReader|exists|isReadable|size|getAttribute|readAttributes|isDirectory|isRegularFile|list|walk|find)\s*\(` +
		`|\bnew\s+(?:FileInputStream|FileReader|BufferedReader\s*\(\s*new\s+FileReader|Scanner\s*\(\s*new\s+File)\s*\(`,
)

// javaFSWriteRe matches NIO / classic-IO write primitives.
var javaFSWriteRe = regexp.MustCompile(
	`\bFiles\s*\.\s*(?:write|writeString|newOutputStream|newBufferedWriter|createFile|createDirectory|createDirectories|delete|deleteIfExists|move|copy|setAttribute|setPosixFilePermissions)\s*\(` +
		`|\bnew\s+(?:FileOutputStream|FileWriter|PrintWriter\s*\(\s*new\s+File)\s*\(`,
)

// javaMutationRe matches `this.<field> = ...` inside a method body.
var javaMutationRe = regexp.MustCompile(
	`\bthis\s*\.\s*[A-Za-z_$][\w$]*\s*=(?:[^=])`,
)

// javaMsgPublishRe matches SmallRye reactive-messaging publish sites
// (ADR-0025 §2):
//
//   - Emitter.send(...) / Emitter.sendMessage(...) — matched via the
//     receiver-identifier shape (any identifier containing "emitter",
//     case-insensitive on the leading letter, mirroring the other T1
//     sniffers' receiver-name heuristic since there is no type table).
//     This covers MutinyEmitter too (name contains "Emitter").
//   - @Outgoing("channel") — the annotated method is itself a publisher
//     via its return value, even with no explicit .send call in the
//     body. javaMethodHeaderRe's modifier group already treats a leading
//     annotation as part of the method header (its \s+ absorbs the
//     newline to "public ... name("), so the header's attributed Line is
//     the annotation's line — nearestHeader therefore binds this match to
//     the annotated method itself, not the preceding one.
//
// This is a fallback heuristic for the common `emitter`-named case. It is
// deliberately scoped to "emitter"-named identifiers: an unscoped
// `.sendMessage(` also matches Android Handler.sendMessage, JavaMail
// transport.sendMessage, and chat-SDK publishers, none of which are
// SmallRye reactive messaging. Idiomatic SmallRye code instead names the
// field after its `@Channel` (e.g. `Emitter<T> offerAssignOut`), which
// this regex alone would miss — see javaEmitterFieldDeclRe /
// javaMsgPublishFieldRe below, which close that gap in a type-aware way
// (only fields the file actually declares as Emitter/MutinyEmitter are
// treated as publisher receivers, preserving precision).
var javaMsgPublishRe = regexp.MustCompile(
	`\b\w*[Ee]mitter\w*\s*\.\s*send(?:Message)?\s*\(` +
		`|@Outgoing\s*\(\s*"[^"]*"\s*\)`,
)

// javaEmitterFieldDeclRe matches a field declaration typed as
// `Emitter<...>` or `MutinyEmitter<...>` (optionally preceded elsewhere by
// a `@Channel("...")` annotation — the annotation isn't required here since
// the TYPE alone is what makes the field a SmallRye publisher). Capture
// group 1 is the field name, e.g. `Emitter<TriageActionEvent> offerAssignOut;`
// or `MutinyEmitter<FeedbackReceivedEvent> feedbackOut;`.
//
// The floating `\b(?:Emitter|MutinyEmitter)` tolerates a fully-qualified
// type name (`org.eclipse.microprofile.reactive.messaging.Emitter<T> x;`) —
// the boundary just needs the simple name to appear. The generic argument
// allows ONE level of nesting via `<[^<>]*(?:<[^<>]*>[^<>]*)*>`, so the most
// idiomatic SmallRye shape `Emitter<Message<T>>` (Message-wrapped payload for
// metadata/ack), plus `Emitter<List<Foo>>` / `Emitter<Map<K,V>>`, are
// collected. A RAW non-generic field (`Emitter ordersOut;`, no `<`) is NOT
// collected — the `<` is required to keep the match precise; raw-type
// Emitter fields are rare enough that this is an accepted limit.
var javaEmitterFieldDeclRe = regexp.MustCompile(
	`\b(?:Emitter|MutinyEmitter)\s*<[^<>]*(?:<[^<>]*>[^<>]*)*>\s+([A-Za-z_$][\w$]*)\s*[;=]`,
)

// javaEmitterFieldNames pre-scans the file for SmallRye Emitter/MutinyEmitter
// typed field declarations and returns their declared names. Only fields
// whose declared TYPE is Emitter/MutinyEmitter are collected — this is what
// keeps the field-aware match precise (ADR-0025 §2 field-aware fix, #5782
// ask #4): a `.send(`/`.sendMessage(` call on a receiver of any other type
// is never matched by javaMsgPublishFieldRe below.
func javaEmitterFieldNames(content string) []string {
	var names []string
	seen := map[string]bool{}
	for _, m := range javaEmitterFieldDeclRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 || m[1] == "" || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		names = append(names, m[1])
	}
	return names
}

// javaMsgPublishFieldRe builds a regex matching `<field>.send(...)` /
// `<field>.sendMessage(...)` for the given set of pre-scanned SmallRye
// emitter field names. Returns nil if names is empty (no dynamic regex to
// run).
func javaMsgPublishFieldRe(names []string) *regexp.Regexp {
	if len(names) == 0 {
		return nil
	}
	alt := make([]string, len(names))
	for i, n := range names {
		alt[i] = regexp.QuoteMeta(n)
	}
	pattern := `\b(?:` + strings.Join(alt, "|") + `)\s*\.\s*send(?:Message)?\s*\(`
	return regexp.MustCompile(pattern)
}

func sniffEffectsJava(content string) []EffectMatch {
	if content == "" {
		return nil
	}
	headers := scanJavaFuncHeaders(content)
	var out []EffectMatch
	out = appendJavaMatches(out, content, headers, javaHTTPRe, EffectHTTPOut, "RestTemplate/WebClient/HttpClient", 1.0)
	out = appendJavaMatches(out, content, headers, javaDBReadRe, EffectDBRead, "jpa/jdbc.read", 0.85)
	out = appendJavaMatches(out, content, headers, javaDBWriteRe, EffectDBWrite, "jpa/jdbc.write", 0.85)
	out = appendJavaMatches(out, content, headers, javaFSReadRe, EffectFSRead, "Files.read/FileInputStream", 1.0)
	out = appendJavaMatches(out, content, headers, javaFSWriteRe, EffectFSWrite, "Files.write/FileOutputStream", 1.0)
	out = appendJavaMatches(out, content, headers, javaMutationRe, EffectMutation, "this.field=", 0.7)
	out = appendJavaMatches(out, content, headers, javaMsgPublishRe, EffectMessagePublish, "smallrye.Emitter.send/@Outgoing", 0.9)
	if fieldRe := javaMsgPublishFieldRe(javaEmitterFieldNames(content)); fieldRe != nil {
		out = appendJavaMatches(out, content, headers, fieldRe, EffectMessagePublish, "smallrye.@Channel-field.send", 0.9)
	}
	return out
}

func scanJavaFuncHeaders(content string) []funcHeader {
	var hs []funcHeader
	add := func(name string, off int) {
		if name == "" || javaControlKeyword(name) {
			return
		}
		hs = append(hs, funcHeader{Line: lineOfOffset(content, off), Name: name})
	}
	for _, m := range javaMethodHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		add(content[m[2]:m[3]], m[0])
	}
	for _, m := range javaCtorHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		add(content[m[2]:m[3]], m[0])
	}
	return hs
}

// javaControlKeyword rejects control-flow tokens the conservative method
// regex can accidentally match (`if (...) {`, `while (...) {`, etc.).
func javaControlKeyword(s string) bool {
	switch s {
	case "if", "else", "for", "while", "switch", "catch", "try", "do", "return", "throw", "new", "synchronized", "super", "this":
		return true
	}
	return false
}

func appendJavaMatches(out []EffectMatch, content string, headers []funcHeader, re *regexp.Regexp, eff Effect, sink string, conf float64) []EffectMatch {
	for _, m := range re.FindAllStringIndex(content, -1) {
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		out = append(out, EffectMatch{
			Function:   fn,
			Line:       line,
			Effect:     eff,
			Sink:       sink,
			Confidence: conf,
		})
	}
	return out
}
