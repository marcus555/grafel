// Go effect-sink sniffer (#2764 Phase 1A T1).
//
// Recognises Go sink primitives:
//
//   - http_out  : http.Get/Post/Head/PostForm, (*http.Client).Do/Get/Post,
//     http.NewRequest + a subsequent Do is captured by the
//     receiver method form
//   - db_read   : db.Query / QueryContext / QueryRow / QueryRowContext,
//     (*sql.Stmt).Query*, GORM Find / First / Take / Last /
//     Count / Pluck / Scan, sqlx Get / Select
//   - db_write  : db.Exec / ExecContext, GORM Create / Save / Updates /
//     Update / Delete / Insert, sqlx NamedExec / MustExec
//   - fs_read   : os.Open / os.ReadFile / ioutil.ReadFile / os.ReadDir,
//     ioutil.ReadAll on a file reader (heuristic — not caught
//     without taint; covered later by Phase 2)
//   - fs_write  : os.Create / os.WriteFile / os.MkdirAll / os.Mkdir /
//     os.Remove / os.RemoveAll / os.Rename / os.Chmod,
//     ioutil.WriteFile
//   - mutation  : `<receiver>.<field> = ...` assignment inside a method
//     body. We attribute mutation by looking for any
//     identifier followed by `.field = ` and rely on the
//     nearest-header heuristic to bind the match to a method;
//     false positives on package-level struct-field writes
//     are tagged to the synthetic top-level scope (empty
//     function name) and elided by the propagation pass.
//
// Function attribution uses the same nearest-header heuristic as the
// other T1 sniffers; Go's gofmt indentation makes the heuristic very
// reliable in practice.
package substrate

import "regexp"

func init() { RegisterEffectSniffer("go", sniffEffectsGo) }

// goFuncHeaderRe matches `func name(` or `func (recv T) name(`. Capture
// group 1 is the bare name (method-receiver-stripped).
var goFuncHeaderRe = regexp.MustCompile(
	`(?m)^func\s+(?:\(\s*[A-Za-z_][\w]*\s+\*?[A-Za-z_][\w]*\s*\)\s+)?([A-Za-z_][\w]*)\s*\(`,
)

// goHTTPRe matches net/http client primitives.
var goHTTPRe = regexp.MustCompile(
	`\bhttp\s*\.\s*(?:Get|Post|Head|PostForm)\s*\(` +
		`|\.\s*(?:Do|Get|Post|Head|PostForm)\s*\(\s*(?:req|request|httpReq|r)\b` +
		`|\b(?:client|httpClient|c)\s*\.\s*(?:Do|Get|Post|Head|PostForm)\s*\(`,
)

// goDBReadRe matches the DISTINCTIVE database/sql + GORM + sqlx read
// primitives — names that don't collide with ordinary method calls on a
// non-DB receiver, so they're safe to bare-match anywhere. GORM `.Where` is a
// distinctive query-builder refinement (the layered-repo read entry point —
// `r.db.Where(...).Find(&xs)`); it's bare-matched here too. The sqlx `.Get(&` /
// `.Select(&` forms are kept for the canonical destination-pointer shape.
//
// The ambiguous sqlx terminals `.Get` / `.Select` / `.Query*` WITHOUT a `&`
// destination (e.g. a fluent `r.db.Get(ctx, id)` wrapper, or `.Select(cols)` as
// a column projection) collide with map getters / generic selectors, so they
// are credited db_read ONLY on a DB-typed receiver by goDBTypedReadMatches
// (#4692 receiver-typed read credit, mirroring the Python #4691 model).
var goDBReadRe = regexp.MustCompile(
	`\.\s*(?:Query|QueryContext|QueryRow|QueryRowContext)\s*\(` +
		`|\.\s*(?:Find|First|Last|Take|Pluck|Count|Scan|FindInBatches|Distinct|Where)\s*\(` +
		`|\.\s*Get\s*\(\s*&` + // sqlx convention: db.Get(&dest, ...)
		`|\.\s*Select\s*\(\s*&`, // sqlx convention: db.Select(&dest, ...)
)

// --- #4692 Go DB receiver-typed read credit (ambiguous terminals) ---
//
// goDBAmbiguousVerbs collide with non-DB method names (a map/cache `.Get`, a
// column `.Select`, a builder `.First`), so they are credited db_read ONLY when
// the receiver is known to be a `*gorm.DB` / `*sqlx.DB` / `*sqlx.Tx` handle —
// the layered-repository read shape `r.db.First(&u, id)` / `r.db.Get(ctx,id)`.
const goDBAmbiguousVerbs = `Get|Select|Query|QueryRow|First|Find|Take|Where|Scan|Pluck`

// goDBHandleTypedRe seeds the set of DB-typed names. Group 1 captures the name
// from the recurring shapes:
//   - `db *gorm.DB` / `db *sqlx.DB` / `db *sqlx.Tx` / `db *sql.DB`  (params/fields)
//   - `db := gorm.Open(...)` / `sqlx.Connect(...)` / `sqlx.Open(...)`
//   - `db.Model(&X{})` chains return *gorm.DB — the receiver `db` is typed
var goDBHandleTypedRe = regexp.MustCompile(
	`\b([A-Za-z_]\w*)\s+\*?\s*(?:gorm\.DB|sqlx\.DB|sqlx\.Tx|sql\.DB|sql\.Tx|bun\.DB|ent\.Client)\b` +
		`|\b([A-Za-z_]\w*)\s*:?=\s*(?:gorm\s*\.\s*Open|sqlx\s*\.\s*(?:Connect|ConnectContext|Open|MustConnect|NewDb)|sql\s*\.\s*Open|bun\s*\.\s*NewDB)\s*\(`,
)

// goDBStructFieldRe types the struct field a repository holds its handle in —
// `db *gorm.DB` / `DB *sqlx.DB` inside a struct — so `r.db.First(...)` resolves
// (the receiver token is the field name). Group 1 = field name.
var goDBStructFieldRe = regexp.MustCompile(
	`(?m)^\s*([A-Za-z_]\w*)\s+\*?\s*(?:gorm\.DB|sqlx\.DB|sqlx\.Tx|sql\.DB|sql\.Tx|bun\.DB|ent\.Client)\b`,
)

// goDBWriteRe matches database/sql + GORM + sqlx write primitives.
var goDBWriteRe = regexp.MustCompile(
	`\.\s*(?:Exec|ExecContext)\s*\(` +
		`|\.\s*(?:Create|Save|Updates|Update|UpdateColumn|UpdateColumns|Delete|Insert|FirstOrCreate|Assign|Attrs|Begin|Commit|Rollback)\s*\(` +
		`|\.\s*(?:NamedExec|MustExec|NamedQuery)\s*\(`,
)

// goFSReadRe matches os / ioutil read primitives.
var goFSReadRe = regexp.MustCompile(
	`\b(?:os|ioutil)\s*\.\s*(?:Open|OpenFile|ReadFile|ReadDir|Stat|Lstat|ReadAll|Readlink)\s*\(`,
)

// goFSWriteRe matches os / ioutil write primitives.
var goFSWriteRe = regexp.MustCompile(
	`\b(?:os|ioutil)\s*\.\s*(?:Create|WriteFile|Mkdir|MkdirAll|Remove|RemoveAll|Rename|Chmod|Chown|Symlink|Link|Truncate)\s*\(`,
)

// goMutationRe matches `<recv>.<field> = ...` style assignment. The
// nearest-header attribution binds this to the enclosing method; if
// no method precedes it, the match falls to the module scope (synthetic
// "" function name) and the propagation pass drops it.
//
// We require a single-identifier receiver to avoid matching qualified
// constants (e.g. `pkg.Const`) — a single bare identifier followed by
// `.field = ` (with a trailing non-`=`).
var goMutationRe = regexp.MustCompile(
	`(?m)^\s*[A-Za-z_][\w]*\s*\.\s*[A-Za-z_][\w]*\s*=(?:[^=])`,
)

func sniffEffectsGo(content string) []EffectMatch {
	if content == "" {
		return nil
	}
	headers := scanGoFuncHeaders(content)
	var out []EffectMatch
	out = appendGoMatches(out, content, headers, goHTTPRe, EffectHTTPOut, "http.Client.Do/Get/Post", 1.0)
	out = appendGoMatches(out, content, headers, goDBReadRe, EffectDBRead, "sql.Query/GORM.Find/sqlx.Get", 0.85)
	out = append(out, goDBTypedReadMatches(content, headers)...)
	out = appendGoMatches(out, content, headers, goDBWriteRe, EffectDBWrite, "sql.Exec/GORM.Create/Save", 0.85)
	out = appendGoMatches(out, content, headers, goFSReadRe, EffectFSRead, "os.Open/ReadFile", 1.0)
	out = appendGoMatches(out, content, headers, goFSWriteRe, EffectFSWrite, "os.Create/WriteFile", 1.0)
	out = appendGoMatches(out, content, headers, goMutationRe, EffectMutation, "recv.field=", 0.6)
	out = appendAWSGoMatches(out, content, headers) // #5798: AWS SDK publish call-sites
	return out
}

// goDBTypedReadMatches implements the #4692 receiver-typed read credit for Go.
// It collects the set of DB-typed handle names (params/fields/locals declared or
// assigned as *gorm.DB / *sqlx.DB / sql.DB / ...), then emits db_read for each
// ambiguous read terminal invoked on one of those handles — either as a bare
// receiver (`db.Get(ctx,id)`) or as a struct field selector (`r.db.First(&u)`,
// `s.DB.Select(&xs)`). An ambiguous verb on an untyped receiver (a map's `.Get`,
// a builder's `.Select`) earns no credit — the false-positive guard is preserved.
func goDBTypedReadMatches(content string, headers []funcHeader) []EffectMatch {
	typed := collectGoDBHandleNames(content)
	if len(typed) == 0 {
		return nil
	}
	var out []EffectMatch
	seen := map[int]bool{}
	emit := func(off int) {
		if seen[off] {
			return
		}
		seen[off] = true
		line := lineOfOffset(content, off)
		out = append(out, EffectMatch{
			Function:   nearestHeader(headers, line),
			Line:       line,
			Effect:     EffectDBRead,
			Sink:       "sqlx/gorm.read.typed",
			Confidence: 0.85,
		})
	}
	for name := range typed {
		// Bare receiver `<name>.<verb>(` and field selector `.<name>.<verb>(`.
		re := regexp.MustCompile(`(?:\b|\.)` + regexp.QuoteMeta(name) + `\s*\.\s*(?:` + goDBAmbiguousVerbs + `)\s*\(`)
		for _, m := range re.FindAllStringIndex(content, -1) {
			emit(m[0])
		}
	}
	return out
}

// collectGoDBHandleNames returns the set of names (params, struct fields, locals)
// known to hold a *gorm.DB / *sqlx.DB / sql.DB / bun.DB / ent.Client handle.
func collectGoDBHandleNames(content string) map[string]bool {
	typed := map[string]bool{}
	add := func(s string) {
		if s != "" {
			typed[s] = true
		}
	}
	for _, m := range goDBHandleTypedRe.FindAllStringSubmatch(content, -1) {
		if len(m) >= 3 {
			add(m[1])
			add(m[2])
		}
	}
	for _, m := range goDBStructFieldRe.FindAllStringSubmatch(content, -1) {
		if len(m) >= 2 {
			add(m[1])
		}
	}
	return typed
}

func scanGoFuncHeaders(content string) []funcHeader {
	var hs []funcHeader
	for _, m := range goFuncHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		hs = append(hs, funcHeader{Line: lineOfOffset(content, m[0]), Name: content[m[2]:m[3]]})
	}
	return hs
}

func appendGoMatches(out []EffectMatch, content string, headers []funcHeader, re *regexp.Regexp, eff Effect, sink string, conf float64) []EffectMatch {
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
