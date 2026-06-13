// F# effect-sink sniffer (#4941 — db_effect for the F# data stack).
//
// Builds on the F# coverage from #4906 (base extractor + Giraffe/Saturn
// routing + testmap). #4906 shipped NO db_effect coverage; this sniffer
// adds data-access classification for the high-value F# data drivers.
//
// Recognises F# data-access sink primitives:
//
//   - db_read   :
//   - EF Core (F#): DbSet LINQ reads — ctx.Users.Find/Where/Single/
//     First/FirstOrDefault/Any/Count/ToList/ToListAsync/AsNoTracking
//     (...Async variants), and the F# `query { for x in ctx.T ... }`
//     computation expression (fsharpEFQueryCERe).
//   - Dapper / Dapper.FSharp: conn.Query/QueryAsync/QueryFirst*/
//     QuerySingle*/QueryMultiple* and Dapper.FSharp `select { ... }`
//     CE + conn.SelectAsync<T>.
//   - Npgsql.FSharp: a `Sql.query "SELECT ..."` literal (classified by
//     the leading SQL verb — fsharpNpgsqlReadRe).
//   - SQLProvider (#4999): the `query { for x in ctx.Dbo.T ... }` CE
//     (shared with fsharpEFQueryCERe) plus a direct table enumeration
//     `ctx.Dbo.T |> Seq.toList` (fsharpSQLProviderReadRe).
//   - db_write  :
//   - EF Core (F#): ctx.SaveChanges()/SaveChangesAsync(),
//     ctx.Users.Add/AddAsync/AddRange/Update/UpdateRange/Remove/
//     RemoveRange/ExecuteUpdate/ExecuteDelete.
//   - Dapper / Dapper.FSharp: conn.Execute/ExecuteAsync/ExecuteScalar*
//     and Dapper.FSharp `insert`/`update`/`delete` CEs +
//     conn.InsertAsync/UpdateAsync/DeleteAsync.
//   - Npgsql.FSharp: `Sql.query "INSERT|UPDATE|DELETE|..."` literal
//     (write SQL verb — fsharpNpgsqlWriteRe).
//   - SQLProvider (#4999): ctx.SubmitUpdates()/SubmitUpdatesAsync(),
//     the table “.Create“(...) row factory, and row.Delete()
//     (fsharpSQLProviderWriteRe). Best-effort `ctx.Schema.Table`
//     attribution is folded into the Sink tag.
//   - http_out  : System.Net.Http HttpClient.GetAsync/PostAsync/PutAsync/
//     PatchAsync/DeleteAsync/SendAsync/GetStringAsync/GetByteArrayAsync,
//     and FsHttp `http { GET ... }` / Http.get|post helpers.
//
// Function attribution uses F# `let [rec] name` / `member [this|_|x].Name`
// declaration headers (the same shapes the #4906 base extractor names as
// SCOPE.Operation). F# is off-side-rule; nearestHeader binds each sink to
// its nearest preceding declaration by line, matching the Crystal/Dart
// precedent. Table attribution (ACCESSES_TABLE) is out of scope for the
// sink sniffer — it emits the standard db_read/db_write/http_out effects,
// consumed downstream by internal/links/effect_propagation.go (mirrors
// every other language's effect sink).
package substrate

import "regexp"

func init() { RegisterEffectSniffer("fsharp", sniffEffectsFSharp) }

// fsharpFuncHeaderRe matches F# `let`/`member` declaration headers.
// Capture group 1 is the binding/member name. Covers:
//
//	let name ...        / let rec name ...   / let inline name ...
//	let private name    / let mutable name
//	member this.Name    / member _.Name      / member x.Name
//	member val Name     / override this.Name / abstract member Name
var fsharpFuncHeaderRe = regexp.MustCompile(
	`(?m)^\s*(?:(?:override|abstract|default|static|private|internal|public)\s+)*` +
		`(?:let(?:\s+(?:rec|inline|mutable|private))*\s+([A-Za-z_][\w']*)` +
		`|member(?:\s+val)?\s+(?:[A-Za-z_][\w']*\s*\.\s*)?([A-Za-z_][\w']*))`,
)

// fsharpHTTPRe matches outbound HTTP primitives (System.Net.Http + FsHttp).
var fsharpHTTPRe = regexp.MustCompile(
	`\b(?:client|_client|httpClient|http)\s*\.\s*` +
		`(?:GetAsync|PostAsync|PutAsync|PatchAsync|DeleteAsync|SendAsync|` +
		`GetStringAsync|GetByteArrayAsync|GetStreamAsync)\s*\(` +
		`|\bHttp\s*\.\s*(?:get|post|put|patch|delete|request)\b` +
		`|\bhttp\s*\{\s*(?:GET|POST|PUT|PATCH|DELETE)\b`,
)

// fsharpEFReadRe matches EF Core (F#) DbSet LINQ read primitives.
// dbSet is the DbSet member access `ctx.Users.` / `db.Orders.` etc.
var fsharpEFReadRe = regexp.MustCompile(
	`\b[A-Za-z_][\w']*\s*\.\s*[A-Z][\w']*\s*\.\s*` +
		`(?:Find|FindAsync|Where|Single|SingleAsync|SingleOrDefault|SingleOrDefaultAsync|` +
		`First|FirstAsync|FirstOrDefault|FirstOrDefaultAsync|Any|AnyAsync|All|` +
		`Count|CountAsync|LongCount|ToList|ToListAsync|ToArray|ToArrayAsync|` +
		`AsNoTracking|Include|Select|OrderBy|FromSqlRaw|FromSqlInterpolated)\s*\(`,
)

// fsharpEFQueryCERe matches the F# `query { for x in ctx.Table ... }` CE.
var fsharpEFQueryCERe = regexp.MustCompile(
	`\bquery\s*\{\s*for\b`,
)

// fsharpEFWriteRe matches EF Core (F#) write primitives.
var fsharpEFWriteRe = regexp.MustCompile(
	`\b[A-Za-z_][\w']*\s*\.\s*(?:SaveChanges|SaveChangesAsync)\s*\(` +
		`|\b[A-Za-z_][\w']*\s*\.\s*[A-Z][\w']*\s*\.\s*` +
		`(?:Add|AddAsync|AddRange|AddRangeAsync|Update|UpdateRange|` +
		`Remove|RemoveRange|ExecuteUpdate|ExecuteUpdateAsync|` +
		`ExecuteDelete|ExecuteDeleteAsync)\s*\(`,
)

// fsharpDapperReceiverNames is the static receiver-name heuristic for Dapper
// connections (the conventional names an IDbConnection binding carries). It is
// the FALLBACK; receiver-type resolution (fsharpDapperTypedReceivers) augments
// it at sniff time with any binding statically typed/constructed as a
// (System.)Data IDbConnection so differently-named connections are also caught.
const fsharpDapperReceiverNames = `conn|connection|db|_conn|_db|cnn|dbConn|dbConnection|_connection`

// fsharpDapperUnambiguousReadVerbs are the Dapper methods whose name alone
// fixes them as a read (Query*/Select*/Get*). These never run write SQL.
const fsharpDapperUnambiguousReadVerbs = `Query|QueryAsync|QueryFirst|QueryFirstAsync|QueryFirstOrDefault|` +
	`QueryFirstOrDefaultAsync|QuerySingle|QuerySingleAsync|` +
	`QuerySingleOrDefault|QuerySingleOrDefaultAsync|QueryMultiple|` +
	`QueryMultipleAsync|SelectAsync|GetAsync|GetListAsync`

// fsharpDapperUnambiguousWriteVerbs are the Dapper.FSharp CRUD helpers whose
// name alone fixes them as a write (Insert/Update/Delete extension methods).
const fsharpDapperUnambiguousWriteVerbs = `InsertAsync|UpdateAsync|DeleteAsync`

// fsharpDapperAmbiguousVerbs are the Dapper methods that run ARBITRARY SQL and
// so are read-or-write depending on the verb of the SQL string argument:
// Execute/ExecuteAsync (stored-proc reads AND DML writes), and ExecuteScalar/
// ExecuteReader (often a SELECT count/aggregate, but can wrap any statement).
const fsharpDapperAmbiguousVerbs = `Execute|ExecuteAsync|ExecuteScalar|ExecuteScalarAsync|` +
	`ExecuteReader|ExecuteReaderAsync`

// fsharpDapperReadRe matches Dapper / Dapper.FSharp NAME-UNAMBIGUOUS read
// primitives (the Query*/Select*/Get* family) plus the Dapper.FSharp
// `select { ... }` CE. The receiver alternation is rendered at sniff time so
// type-resolved receivers can be folded in (see sniffEffectsFSharp).
var fsharpDapperReadRe = regexp.MustCompile(
	`\b(?:` + fsharpDapperReceiverNames + `)\s*\.\s*` +
		`(?:` + fsharpDapperUnambiguousReadVerbs + `)\b` +
		`|\bselect\s*\{\s*(?:for\b|table\b)`,
)

// fsharpDapperWriteRe matches Dapper / Dapper.FSharp NAME-UNAMBIGUOUS write
// primitives (Insert/Update/Delete extension methods) plus the Dapper.FSharp
// `insert`/`update`/`delete` CEs. The ambiguous Execute* family is handled
// separately by fsharpDapperExecuteRe + leading-verb inspection.
var fsharpDapperWriteRe = regexp.MustCompile(
	`\b(?:` + fsharpDapperReceiverNames + `)\s*\.\s*` +
		`(?:` + fsharpDapperUnambiguousWriteVerbs + `)\b` +
		`|\b(?:insert|update|delete)\s*\{\s*(?:for\b|into\b|table\b)`,
)

// fsharpDapperExecuteRe matches the AMBIGUOUS Dapper Execute* family. Group 1
// captures the first string-literal argument (`@?"""..."""` / `@?"..."`) when
// present, so the leading SQL verb can be inspected to classify read vs write.
// The receiver alternation is rendered at sniff time (type-resolved receivers
// folded in). When no inspectable literal is present the call defaults to a
// write (conservative — DML is the common Execute use and over-recording a
// write is safer than missing one).
var fsharpDapperExecuteRe = regexp.MustCompile(
	`\b(?:` + fsharpDapperReceiverNames + `)\s*\.\s*` +
		`(?:` + fsharpDapperAmbiguousVerbs + `)\s*` +
		`(?:<[^>]*>)?\s*\(\s*(?:@?"""(?s:(.*?))"""|@?"((?:[^"\\]|\\.)*)")?`,
)

// fsharpSQLReadVerbRe matches a SQL statement whose leading verb is a read
// (SELECT / WITH ... SELECT). Leading whitespace/comments are tolerated.
var fsharpSQLReadVerbRe = regexp.MustCompile(
	`(?is)^\s*(?:--[^\n]*\n\s*|/\*.*?\*/\s*)*(?:SELECT|WITH)\b`,
)

// fsharpSQLWriteVerbRe matches a SQL statement whose leading verb is a write
// (DML/DDL). Leading whitespace/comments are tolerated. Used by the assembled-
// SQL verb inference (#5115) to positively confirm a write head fragment so a
// no-literal Execute can be promoted from the conservative `write?` guess to a
// confident `write`.
var fsharpSQLWriteVerbRe = regexp.MustCompile(
	`(?is)^\s*(?:--[^\n]*\n\s*|/\*.*?\*/\s*)*` +
		`(?:INSERT|UPDATE|DELETE|CREATE|DROP|ALTER|TRUNCATE|MERGE|UPSERT|REPLACE|GRANT|REVOKE)\b`,
)

// fsharpSQLLiteralBindingRe matches an intra-function `let <name> = "<sql>"`
// SQL-string binding, INCLUDING interpolated heads (`$"SELECT ..."`,
// `$"""..."""`) and verbatim (`@"..."`). Only the LEADING literal fragment is
// captured (an interpolation hole truncates the body at the first `{`), which
// is sufficient to read the SQL verb. Group 1 = bound name; groups 2/3 = the
// triple-/single-quoted literal head.
var fsharpSQLLiteralBindingRe = regexp.MustCompile(
	`(?m)^\s*let\s+(?:mutable\s+)?([A-Za-z_][\w']*)\s*=\s*` +
		`[$@]*(?:"""(?s:((?:[^"{]|"[^"]|""[^"])*?))"""|"((?:[^"\\{]|\\.)*)")`,
)

// fsharpInterpHeadRe pulls the literal HEAD of an interpolated/concatenated SQL
// expression — the first quoted fragment of a `$"SELECT ..." + cols + ...` or
// `"INSERT INTO " + tbl` assembly. Group 1/2 = the head literal. This recovers
// the verb even when the FULL string is not a single literal (the verb lives in
// the head fragment, which IS literal).
var fsharpInterpHeadRe = regexp.MustCompile(
	`^\s*[$@]*(?:"""(?s:(.*?))"""|"((?:[^"\\{]|\\.)*))`,
)

// fsharpDapperReceiverTypeRe resolves receiver-type bindings: an F# parameter
// or value statically annotated as a (System.Data) IDbConnection-family type,
// or constructed via `new SqlConnection(...)` / `new NpgsqlConnection(...)`.
// Group 1 / group 2 capture the bound NAME so differently-named connections
// (e.g. `database`, `pg`, `sqlite`) are recognised regardless of the static
// name heuristic. Recognised types: IDbConnection and the concrete ADO.NET
// connection classes (SqlConnection, NpgsqlConnection, SqliteConnection,
// MySqlConnection, OracleConnection, DbConnection, SqliteConnection).
var fsharpDapperReceiverTypeRe = regexp.MustCompile(
	`\(\s*([A-Za-z_][\w']*)\s*:\s*(?:[\w.]+\.)?` + fsharpDapperConnTypes + `\b` +
		`|\b(?:let|use)\s+(?:mutable\s+)?([A-Za-z_][\w']*)\s*` +
		`(?::\s*(?:[\w.]+\.)?` + fsharpDapperConnTypes + `\b[^=]*)?` +
		`=\s*(?:new\s+)?(?:[\w.]+\.)?` + fsharpDapperConnTypes + `\s*\(`,
)

// fsharpDapperConnTypes is the IDbConnection-family type alternation.
const fsharpDapperConnTypes = `(?:IDbConnection|DbConnection|SqlConnection|NpgsqlConnection|` +
	`SqliteConnection|SQLiteConnection|MySqlConnection|MySqlConnector\.MySqlConnection|` +
	`OracleConnection|OdbcConnection|OleDbConnection|FbConnection)`

// fsharpDIConnFieldRe resolves a DI / cross-binding receiver: an
// IDbConnection-family value reached NOT through a param annotation or a
// `new`-construction (those are fsharpDapperReceiverTypeRe's job), but through
// (a) a class CONSTRUCTOR parameter that is then exposed as a member field, and
// (b) an intra-file ALIAS `let alias = <known conn>` that re-binds an existing
// connection under a new name (#5115). It is the cross-file/DI deepening of the
// #5001 single-statement receiver resolution.
//
//	Group 1 : `member [this|_|x].Name ... = <conn>` field exposure name.
//	Group 2 : the connection NAME the member exposes (must already be known).
//	Group 3 : `let alias` alias target name.
//	Group 4 : the connection NAME the alias re-binds (must already be known).
//
// Both forms only credit the new name when the SOURCE name is ALREADY a
// resolved connection (a typed param / construction / a prior alias), so an
// alias of an unrelated value never leaks a db effect.
var fsharpDIConnFieldRe = regexp.MustCompile(
	`(?m)^\s*member(?:\s+val)?\s+(?:[A-Za-z_][\w']*\s*\.\s*)?([A-Za-z_][\w']*)` +
		`(?:\s*:[^=]*)?\s*=\s*([A-Za-z_][\w']*)\s*$` +
		`|(?m)^\s*let\s+(?:mutable\s+)?([A-Za-z_][\w']*)\s*=\s*([A-Za-z_][\w']*)\s*$`,
)

// fsharpDIConnFactoryRe resolves a FACTORY-RETURNED receiver (#5115): a `let`
// binding (or `member`) whose RETURN TYPE is annotated as an IDbConnection-
// family type, e.g. `let openDb (cs: string) : IDbConnection = ...`. Group 1
// captures the factory NAME; a later `let x = factory(...)` then credits `x`.
var fsharpDIConnFactoryRe = regexp.MustCompile(
	`(?m)^\s*(?:(?:override|abstract|default|static|private|internal|public)\s+)*` +
		`(?:let(?:\s+(?:rec|inline|private))*\s+([A-Za-z_][\w']*)` +
		`|member(?:\s+val)?\s+(?:[A-Za-z_][\w']*\s*\.\s*)?([A-Za-z_][\w']*))` +
		`[^=\n]*:\s*(?:[\w.]+\.)?` + fsharpDapperConnTypes + `\b[^=\n]*=`,
)

// fsharpDIConnFromFactoryRe binds `let x = factory(...)` where factory is a
// known connection-returning factory. Group 1 = bound name, group 2 = factory
// callee. Only credited when the callee is in the resolved-factory set.
var fsharpDIConnFromFactoryRe = regexp.MustCompile(
	`(?m)^\s*(?:let|use)\s+(?:mutable\s+)?([A-Za-z_][\w']*)\s*=\s*` +
		`([A-Za-z_][\w']*)\s*(?:\(|<)`,
)

// SQLProvider (F# type provider) recognition (#4999, follow-up #4941).
//
// SQLProvider exposes an erased data context whose tables are reached via
// `ctx.Dbo.TableName` (the leading segment after the context is the SQL
// schema — Dbo/Public/Main/etc.). There is no stable static call shape on
// the provided types, so the provider's idiomatic surface is matched
// syntactically:
//
//   - db_read  : the generic F# `query { for x in ctx.Dbo.T ... }` CE is
//     already caught by fsharpEFQueryCERe. In addition a direct table
//     enumeration `ctx.Dbo.TableName |> Seq.toList` / `|> Seq.map` /
//     `|> List.ofSeq` (materialising the erased IQueryable) is a read
//     (fsharpSQLProviderReadRe).
//   - db_write : the provider commits via `ctx.SubmitUpdates()` /
//     `ctx.SubmitUpdatesAsync()`; rows are inserted with the table's
//     ``.Create``(...) / `.Create(...)` factory and removed with
//     `row.Delete()` (fsharpSQLProviderWriteRe).
//
// Table attribution is best-effort: fsharpSQLProviderTableRe extracts the
// `ctx.Schema.TableName` table segment so the matched read/write carries a
// candidate table in its Sink tag (`sqlprovider.read:Users`). ACCESSES_TABLE
// wiring stays a separate concern, as for every other sink language (see the
// package note above).

// fsharpSQLProviderReadRe matches a direct SQLProvider table enumeration
// (`ctx.Dbo.Users |> Seq.toList` and friends). The `query { for ... }` read
// path is already covered by fsharpEFQueryCERe, so this only adds the direct
// pipe-to-collection-combinator materialisation shape.
var fsharpSQLProviderReadRe = regexp.MustCompile(
	`\b[A-Za-z_][\w']*\s*\.\s*(?:Dbo|Public|Main|dbo|public|main)\s*\.\s*[A-Z][\w']*\s*` +
		`\|>\s*(?:Seq|List|Array)\s*\.\s*` +
		`(?:toList|toArray|ofSeq|map|filter|tryHead|head|find|tryFind|length|isEmpty|iter|fold|sortBy)\b`,
)

// fsharpSQLProviderWriteRe matches SQLProvider commit / row-mutation
// primitives: ctx.SubmitUpdates()/SubmitUpdatesAsync(), the table
// “.Create“(...)/.Create(...) row factory, and `row.Delete()`.
var fsharpSQLProviderWriteRe = regexp.MustCompile(
	`\b[A-Za-z_][\w']*\s*\.\s*(?:SubmitUpdates|SubmitUpdatesAsync)\s*\(` +
		"|\\b[A-Za-z_][\\w']*\\s*\\.\\s*(?:Dbo|Public|Main|dbo|public|main)\\s*\\.\\s*[A-Z][\\w']*\\s*\\.\\s*(?:`Create`|Create)\\s*\\(" +
		`|\b[A-Za-z_][\w']*\s*\.\s*Delete\s*\(\s*\)`,
)

// fsharpSQLProviderTableRe extracts the `ctx.Schema.TableName` table
// segment for best-effort table attribution. Group 1 = schema, group 2 =
// table. Schema is restricted to the common SQLProvider schema segments so
// arbitrary `A.B.C` member chains do not false-match.
var fsharpSQLProviderTableRe = regexp.MustCompile(
	`\b[A-Za-z_][\w']*\s*\.\s*(?:Dbo|Public|Main|dbo|public|main)\s*\.\s*([A-Z][\w']*)`,
)

// fsharpNpgsqlReadRe matches Npgsql.FSharp `Sql.query "SELECT|WITH ..."`.
var fsharpNpgsqlReadRe = regexp.MustCompile(
	`\bSql\s*\.\s*query\s+(?:@?"""|@?")\s*(?i:SELECT|WITH)\b`,
)

// fsharpNpgsqlWriteRe matches Npgsql.FSharp `Sql.query "INSERT|UPDATE|..."`.
var fsharpNpgsqlWriteRe = regexp.MustCompile(
	`\bSql\s*\.\s*query\s+(?:@?"""|@?")\s*(?i:INSERT|UPDATE|DELETE|CREATE|DROP|ALTER|TRUNCATE|MERGE|UPSERT)\b`,
)

// fsharpResolveDapperReceivers returns the per-call regexes for the Dapper
// read / write / Execute primitives, with the receiver-name alternation
// EXTENDED by any binding statically resolved to an IDbConnection-family type
// (#5001). When no typed receivers are present the package-level regexes are
// reused (no allocation). This drops the reliance on the bare name heuristic
// so differently-named connections are also classified.
func fsharpResolveDapperReceivers(content string) (readRe, writeRe, execRe *regexp.Regexp) {
	extra := fsharpTypedDapperReceiverNames(content)
	if len(extra) == 0 {
		return fsharpDapperReadRe, fsharpDapperWriteRe, fsharpDapperExecuteRe
	}
	recv := fsharpDapperReceiverNames
	for _, n := range extra {
		recv += `|` + regexp.QuoteMeta(n)
	}
	readRe = regexp.MustCompile(
		`\b(?:` + recv + `)\s*\.\s*` +
			`(?:` + fsharpDapperUnambiguousReadVerbs + `)\b` +
			`|\bselect\s*\{\s*(?:for\b|table\b)`,
	)
	writeRe = regexp.MustCompile(
		`\b(?:` + recv + `)\s*\.\s*` +
			`(?:` + fsharpDapperUnambiguousWriteVerbs + `)\b` +
			`|\b(?:insert|update|delete)\s*\{\s*(?:for\b|into\b|table\b)`,
	)
	execRe = regexp.MustCompile(
		`\b(?:` + recv + `)\s*\.\s*` +
			`(?:` + fsharpDapperAmbiguousVerbs + `)\s*` +
			`(?:<[^>]*>)?\s*\(\s*(?:@?"""(?s:(.*?))"""|@?"((?:[^"\\]|\\.)*)")?`,
	)
	return readRe, writeRe, execRe
}

// fsharpTypedDapperReceiverNames scans for IDbConnection-family bindings whose
// name is NOT already in the static heuristic, returning the de-duplicated
// extra names to fold into the receiver alternation. Resolution proceeds in
// three layers (#5001 base + #5115 deepening):
//
//  1. DIRECT (#5001): a param annotation `(x: IDbConnection)` or a
//     `let/use x = new SqlConnection(...)` construction (fsharpDapperReceiverTypeRe).
//  2. FACTORY (#5115): a connection-returning factory `let f ... : IDbConnection = ...`
//     plus a `let x = f(...)` callsite credits `x` (fsharpDIConnFactoryRe +
//     fsharpDIConnFromFactoryRe).
//  3. DI / ALIAS (#5115): a member field or alias `let alias = <known conn>`
//     re-binds an already-resolved connection name (fsharpDIConnFieldRe). This
//     is iterated to a fixpoint so alias-of-alias chains resolve.
//
// Every layer only credits a NEW name when its source is ALREADY resolved, so
// an alias/field of an unrelated value never leaks a db effect.
func fsharpTypedDapperReceiverNames(content string) []string {
	known := map[string]bool{
		"conn": true, "connection": true, "db": true, "_conn": true,
		"_db": true, "cnn": true, "dbConn": true, "dbConnection": true,
		"_connection": true,
	}
	var out []string
	add := func(name string) {
		if name == "" || known[name] {
			return
		}
		known[name] = true
		out = append(out, name)
	}

	// Layer 1: direct param-annotated / new-constructed bindings.
	for _, m := range fsharpDapperReceiverTypeRe.FindAllStringSubmatch(content, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		add(name)
	}

	// Layer 2: factory-returned connections. First collect the names of
	// IDbConnection-returning factories, then credit every `let x = factory(...)`.
	factories := map[string]bool{}
	for _, m := range fsharpDIConnFactoryRe.FindAllStringSubmatch(content, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name != "" {
			factories[name] = true
		}
	}
	if len(factories) > 0 {
		for _, m := range fsharpDIConnFromFactoryRe.FindAllStringSubmatch(content, -1) {
			if factories[m[2]] {
				add(m[1])
			}
		}
	}

	// Layer 3: DI member-field exposure + alias chains, iterated to a fixpoint
	// so `let a = conn; let b = a` resolves both a and b.
	fieldMatches := fsharpDIConnFieldRe.FindAllStringSubmatch(content, -1)
	for {
		grew := false
		for _, m := range fieldMatches {
			// member-field form: group1=field, group2=source.
			if m[2] != "" && known[m[2]] {
				if !known[m[1]] {
					add(m[1])
					grew = true
				}
			}
			// alias form: group3=alias, group4=source.
			if m[4] != "" && known[m[4]] {
				if !known[m[3]] {
					add(m[3])
					grew = true
				}
			}
		}
		if !grew {
			break
		}
	}
	return out
}

func sniffEffectsFSharp(content string) []EffectMatch {
	if content == "" {
		return nil
	}
	headers := scanFSharpEffectHeaders(content)
	var out []EffectMatch
	out = appendFSharpMatches(out, content, headers, fsharpHTTPRe, EffectHTTPOut, "HttpClient/FsHttp", 0.95)
	out = appendFSharpMatches(out, content, headers, fsharpEFReadRe, EffectDBRead, "efcore.dbset.read", 0.85)
	out = appendFSharpMatches(out, content, headers, fsharpEFQueryCERe, EffectDBRead, "efcore.query-ce", 0.8)
	out = appendFSharpMatches(out, content, headers, fsharpEFWriteRe, EffectDBWrite, "efcore.write", 0.85)
	dapperReadRe, dapperWriteRe, dapperExecRe := fsharpResolveDapperReceivers(content)
	out = appendFSharpMatches(out, content, headers, dapperReadRe, EffectDBRead, "dapper.read", 0.85)
	out = appendFSharpMatches(out, content, headers, dapperWriteRe, EffectDBWrite, "dapper.write", 0.85)
	// Dapper ambiguous Execute* family (#5001): classify read vs write by the
	// leading SQL verb of the string-literal argument; type-resolved receivers
	// (#5001) extend the static name heuristic so differently-named
	// IDbConnection bindings (`database`, `pg`, ...) are also caught.
	out = appendFSharpDapperExecuteMatches(out, content, headers, dapperExecRe)
	out = appendFSharpMatches(out, content, headers, fsharpNpgsqlReadRe, EffectDBRead, "npgsql.fsharp.read", 0.9)
	out = appendFSharpMatches(out, content, headers, fsharpNpgsqlWriteRe, EffectDBWrite, "npgsql.fsharp.write", 0.9)
	// SQLProvider type-provider (#4999): direct table enumeration -> db_read,
	// SubmitUpdates/.Create/.Delete() -> db_write, each with best-effort
	// `ctx.Schema.Table` attribution folded into the Sink tag.
	out = appendFSharpSQLProviderMatches(out, content, headers, fsharpSQLProviderReadRe, EffectDBRead, "sqlprovider.read", 0.75)
	out = appendFSharpSQLProviderMatches(out, content, headers, fsharpSQLProviderWriteRe, EffectDBWrite, "sqlprovider.write", 0.75)
	return out
}

// appendFSharpSQLProviderMatches is appendFSharpMatches with best-effort
// SQLProvider table attribution: when the matched line carries a
// `ctx.Schema.TableName` segment, the resolved table name is appended to the
// Sink tag (`sqlprovider.read:Users`). SQLProvider provided types are erased,
// so the table is a best-effort hint, not a resolved entity (honest-partial).
func appendFSharpSQLProviderMatches(out []EffectMatch, content string, headers []funcHeader, re *regexp.Regexp, eff Effect, sink string, conf float64) []EffectMatch {
	for _, m := range re.FindAllStringIndex(content, -1) {
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		s := sink
		if tbl := fsharpSQLProviderTableOnLine(content, m[0]); tbl != "" {
			s = sink + ":" + tbl
		}
		out = append(out, EffectMatch{
			Function:   fn,
			Line:       line,
			Effect:     eff,
			Sink:       s,
			Confidence: conf,
		})
	}
	return out
}

// appendFSharpDapperExecuteMatches classifies the ambiguous Dapper Execute*
// family (#5001). The string-literal argument's leading SQL verb decides the
// effect: SELECT / WITH (... SELECT) -> db_read; any other (or no inspectable)
// literal -> db_write. A read carries higher confidence (the verb is explicit);
// a defaulted write (no literal to inspect — e.g. a SQL value bound elsewhere)
// drops confidence to reflect the heuristic fallback. The Sink tag records the
// classification basis (`dapper.execute.read` / `.write` / `.write?`).
func appendFSharpDapperExecuteMatches(out []EffectMatch, content string, headers []funcHeader, execRe *regexp.Regexp) []EffectMatch {
	for _, m := range execRe.FindAllStringSubmatchIndex(content, -1) {
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		// Group 1 = triple-quoted literal body, group 2 = quoted literal body.
		lit := ""
		if m[2] >= 0 {
			lit = content[m[2]:m[3]]
		} else if m[4] >= 0 {
			lit = content[m[4]:m[5]]
		}
		eff := EffectDBWrite
		sink := "dapper.execute.write"
		conf := 0.85
		switch {
		case lit == "":
			// No inspectable literal on the call. Before falling to the
			// conservative write default, attempt ASSEMBLED-SQL verb inference
			// (#5115): recover the leading SQL verb from (a) the first argument
			// when it is itself an interpolated/concatenated string whose HEAD
			// is literal, or (b) an intra-function `let sql = "VERB ..."`
			// binding the call passes by name. The verb lives in the literal
			// head fragment even when the full string is assembled.
			if v := fsharpInferAssembledVerb(content, m[1], line, headers); v != verbUnknown {
				if v == verbRead {
					eff = EffectDBRead
					sink = "dapper.execute.read~" // ~ = inferred from assembled SQL
					conf = 0.8
				} else {
					sink = "dapper.execute.write~"
					conf = 0.8
				}
				break
			}
			// Still no recoverable verb (SQL parameterised elsewhere / a stored
			// proc name): default to write, flagging the lower-confidence guess.
			sink = "dapper.execute.write?"
			conf = 0.7
		case fsharpSQLReadVerbRe.MatchString(lit):
			eff = EffectDBRead
			sink = "dapper.execute.read"
			conf = 0.9
		}
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

// verb classification for assembled-SQL inference (#5115).
type fsharpSQLVerb int

const (
	verbUnknown fsharpSQLVerb = iota
	verbRead
	verbWrite
)

// fsharpClassifyVerbHead classifies an SQL HEAD fragment by its leading verb,
// returning verbUnknown when neither a read nor a write verb leads (so a head
// of pure interpolation / whitespace does not force a guess).
func fsharpClassifyVerbHead(head string) fsharpSQLVerb {
	switch {
	case fsharpSQLReadVerbRe.MatchString(head):
		return verbRead
	case fsharpSQLWriteVerbRe.MatchString(head):
		return verbWrite
	}
	return verbUnknown
}

// fsharpInferAssembledVerb recovers the SQL verb for a no-literal Dapper
// Execute call (#5115). argStart is the offset of the FIRST ARGUMENT (the
// fsharpDapperExecuteRe match consumes through `(` + optional empty literal, so
// its match-end IS the argument start). Two recovery paths:
//
//  1. INLINE assembly — the first argument is itself an interpolated/
//     concatenated string (`$"SELECT ..." + cols`, `"INSERT INTO " + t`): the
//     verb is in the literal head fragment (fsharpInterpHeadRe).
//  2. NAMED assembly — the argument is an identifier bound earlier in the same
//     function to an SQL literal (`let sql = "SELECT ..."` / `$"UPDATE ..."`):
//     resolve the binding within the enclosing function body and read its head.
//
// Returns verbUnknown when no verb can be recovered.
func fsharpInferAssembledVerb(content string, argStart, callLine int, headers []funcHeader) fsharpSQLVerb {
	// Isolate the first argument text: from argStart to the delimiter (top-level
	// comma / close-paren / newline). Bracket depth is tracked so the literal
	// head's own parens/braces do not terminate the scan early.
	arg := argStart
	if arg >= len(content) {
		return verbUnknown
	}
	end := arg
	depth := 0
	for end < len(content) {
		c := content[end]
		if c == '(' || c == '[' || c == '{' {
			depth++
		} else if c == ')' || c == ']' || c == '}' {
			if depth == 0 {
				break
			}
			depth--
		} else if c == ',' && depth == 0 {
			break
		} else if c == '\n' {
			break
		}
		end++
	}
	argText := content[arg:end]

	// Path 1: inline interpolated/concatenated string head.
	if mm := fsharpInterpHeadRe.FindStringSubmatch(argText); mm != nil {
		head := mm[1]
		if head == "" {
			head = mm[2]
		}
		if v := fsharpClassifyVerbHead(head); v != verbUnknown {
			return v
		}
	}

	// Path 2: the argument is a bare identifier bound to an SQL literal earlier
	// in the enclosing function. Extract the leading identifier of the arg.
	id := fsharpLeadingIdent(argText)
	if id == "" {
		return verbUnknown
	}
	// Function body span: from the nearest header at/above callLine to the next
	// header strictly below it (or EOF). Binding must precede the call line.
	bodyStart, bodyEnd := fsharpFuncBodySpan(content, callLine, headers)
	body := content[bodyStart:bodyEnd]
	for _, bm := range fsharpSQLLiteralBindingRe.FindAllStringSubmatch(body, -1) {
		if bm[1] != id {
			continue
		}
		head := bm[2]
		if head == "" {
			head = bm[3]
		}
		if v := fsharpClassifyVerbHead(head); v != verbUnknown {
			return v
		}
	}
	return verbUnknown
}

// fsharpLeadingIdent returns the leading F# identifier of s (after optional
// whitespace), or "" when s does not start with an identifier.
func fsharpLeadingIdent(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	start := i
	for i < len(s) {
		c := s[i]
		if c == '_' || c == '\'' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			i++
			continue
		}
		break
	}
	if i == start {
		return ""
	}
	// Reject a member-access head (`x.Foo`) — that is not a local SQL binding.
	if i < len(s) && s[i] == '.' {
		return ""
	}
	return s[start:i]
}

// fsharpFuncBodySpan returns the [start,end) byte offsets of the function body
// enclosing callLine: from the nearest header at-or-above callLine to the next
// header strictly below it (or EOF). Used to scope assembled-SQL binding lookup
// to a single function so a `let sql` in another function never leaks.
func fsharpFuncBodySpan(content string, callLine int, headers []funcHeader) (int, int) {
	startLine, endLine := 1, 1<<30
	for _, h := range headers {
		if h.Line <= callLine && h.Line >= startLine {
			startLine = h.Line
		}
		if h.Line > callLine && h.Line < endLine {
			endLine = h.Line
		}
	}
	return fsharpLineOffset(content, startLine), fsharpLineOffset(content, endLine)
}

// fsharpLineOffset returns the byte offset of the 1-based line, or len(content)
// when line exceeds the file.
func fsharpLineOffset(content string, line int) int {
	if line <= 1 {
		return 0
	}
	cur := 1
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			cur++
			if cur == line {
				return i + 1
			}
		}
	}
	return len(content)
}

// fsharpSQLProviderTableOnLine returns the best-effort `ctx.Schema.Table`
// table name for the source line containing offset off, or "" when none is
// present (e.g. a `ctx.SubmitUpdates()` commit with no table on the line).
func fsharpSQLProviderTableOnLine(content string, off int) string {
	start := off
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	end := off
	for end < len(content) && content[end] != '\n' {
		end++
	}
	if mm := fsharpSQLProviderTableRe.FindStringSubmatch(content[start:end]); mm != nil {
		return mm[1]
	}
	return ""
}

func scanFSharpEffectHeaders(content string) []funcHeader {
	var hs []funcHeader
	for _, m := range fsharpFuncHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		// Group 1 = let-binding name, group 2 = member name; exactly one fires.
		name := ""
		if m[2] >= 0 {
			name = content[m[2]:m[3]]
		} else if m[4] >= 0 {
			name = content[m[4]:m[5]]
		}
		if name == "" {
			continue
		}
		hs = append(hs, funcHeader{Line: lineOfOffset(content, m[0]), Name: name})
	}
	return hs
}

func appendFSharpMatches(out []EffectMatch, content string, headers []funcHeader, re *regexp.Regexp, eff Effect, sink string, conf float64) []EffectMatch {
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
