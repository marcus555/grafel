// C# effect-sink sniffer (#2765 Phase 1A T2).
//
// Recognises C# sink primitives:
//
//   - http_out  : HttpClient.GetAsync / PostAsync / PutAsync / DeleteAsync
//     / SendAsync / GetStringAsync / GetStreamAsync, WebClient
//     .Download* / .Upload*, RestClient (RestSharp) .Execute*,
//     HttpWebRequest.Create
//   - db_read   : Entity Framework — distinctive terminals bare-matched
//     (`.FindAsync / .FirstOrDefaultAsync / .Include /
//     .AsNoTracking / .AsQueryable / .FromSqlRaw / ...`); the
//     ambiguous synchronous LINQ verbs (`.Where / .First /
//     .FirstOrDefault / .Single / .ToList / .ToArray / .Any /
//     .All / .Count / .Find`) credited ONLY on a DbSet/
//     IQueryable-typed receiver (#4692, so in-memory LINQ on a
//     plain List stays pure); raw ADO.NET `SqlCommand
//     .ExecuteReader / ExecuteScalar`, Dapper `Query / QueryAsync
//     / QueryFirst / QuerySingle`
//   - db_write  : EF DbSet `.Add / .AddAsync / .AddRange / .Update /
//     .UpdateRange / .Remove / .RemoveRange / .Attach`, EF
//     `SaveChanges / SaveChangesAsync / ExecuteSqlRaw /
//     ExecuteSqlInterpolated`, ADO.NET SqlCommand
//     `.ExecuteNonQuery`, Dapper `Execute / ExecuteAsync`
//   - fs_read   : File.ReadAllText / .ReadAllBytes / .ReadAllLines /
//     .ReadLines / .OpenRead / .OpenText, Directory.Get*,
//     new FileStream(..., FileMode.Open, FileAccess.Read),
//     StreamReader ctor
//   - fs_write  : File.WriteAllText / .WriteAllBytes / .WriteAllLines /
//     .AppendAllText / .AppendAllLines / .Delete / .Move /
//     .Copy / .Create / .CreateText, Directory.CreateDirectory
//     / .Delete / .Move, FileStream with FileMode.Create or
//     FileAccess.Write, StreamWriter ctor
//   - mutation  : `this.<Property|field> = ...` assignment inside a method
//
// Function attribution uses the nearest preceding method header — same
// shape as the Java sniffer with C#-specific keyword set.
package substrate

import "regexp"

func init() { RegisterEffectSniffer("csharp", sniffEffectsCSharp) }

// csharpMethodHeaderRe matches a C# method signature header. Conservative
// — requires a modifier (public/private/protected/internal/static/...) or
// async, the return type, the method name, and the opening parenthesis.
// Constructors are captured by csharpCtorHeaderRe.
var csharpMethodHeaderRe = regexp.MustCompile(
	`(?m)^\s*` +
		`(?:(?:public|private|protected|internal|static|virtual|override|abstract|sealed|async|unsafe|new|extern|partial|readonly|\[[^\]]+\])\s+)+` +
		`(?:<[^>]+>\s+)?` +
		`[A-Za-z_][\w<>\[\],?.\s]*\s+` +
		`([A-Za-z_][\w]*)\s*\(`,
)

// csharpCtorHeaderRe matches constructor declarations.
var csharpCtorHeaderRe = regexp.MustCompile(
	`(?m)^\s*(?:public|private|protected|internal)\s+([A-Z][\w]*)\s*\(`,
)

// csharpHTTPRe matches outbound HTTP primitives.
var csharpHTTPRe = regexp.MustCompile(
	`\b(?:httpClient|_httpClient|client|_client|http)\s*\.\s*(?:GetAsync|PostAsync|PutAsync|PatchAsync|DeleteAsync|HeadAsync|SendAsync|GetStringAsync|GetStreamAsync|GetByteArrayAsync|PostAsJsonAsync|PutAsJsonAsync|PatchAsJsonAsync)\s*\(` +
		`|\bnew\s+HttpClient\s*\(` +
		`|\b(?:HttpClient|HttpRequestMessage|HttpWebRequest|WebClient|RestClient)\s*\.\s*(?:Create|Default)\b` +
		`|\bnew\s+(?:RestClient|RestRequest|WebClient|HttpWebRequest)\s*\(` +
		`|\.\s*Execute(?:Async)?\s*<[^>]+>\s*\(` +
		`|\b(?:WebClient|HttpClient)\s*\.\s*(?:DownloadString|DownloadData|DownloadFile|UploadString|UploadData|UploadFile)\b`,
)

// csharpDBReadRe matches the DISTINCTIVE EF / Dapper / ADO.NET read
// primitives — names that do NOT collide with in-memory LINQ-to-Objects on a
// plain List/array/IEnumerable, so they are safe to bare-match on any receiver
// (#4692). This covers the async EF terminals (`...Async`), the EF-only query
// shapers (`Include`/`AsNoTracking`/`AsQueryable`/`FromSqlRaw`/...), the ADO.NET
// readers, and the Dapper `Query*` family.
//
// The AMBIGUOUS synchronous LINQ verbs (`Where`/`First`/`FirstOrDefault`/
// `Single`/`SingleOrDefault`/`ToList`/`ToArray`/`Any`/`All`/`Count`/`Find`) are
// NOT here — they fire on in-memory collections too (`list.Where(...).ToList()`,
// `names.Any()`, `items.Find(p)`), which would be false db_read. They are
// credited ONLY on a DbSet/IQueryable-typed receiver by csharpDBSetReadMatches
// (#4692 receiver-typed read credit, mirroring the Python #4691 model).
var csharpDBReadRe = regexp.MustCompile(
	`\.\s*(?:FindAsync|FirstAsync|FirstOrDefaultAsync|SingleAsync|SingleOrDefaultAsync|AnyAsync|CountAsync|LongCountAsync|ToListAsync|ToArrayAsync|LongCount|ToDictionary|Include|ThenInclude|AsQueryable|AsNoTracking|FromSqlRaw|FromSqlInterpolated|SqlQuery)\s*[\(<]` +
		`|\.\s*(?:ExecuteReader|ExecuteReaderAsync|ExecuteScalar|ExecuteScalarAsync)\s*\(` +
		`|\.\s*(?:Query|QueryAsync|QueryFirst|QueryFirstAsync|QueryFirstOrDefault|QueryFirstOrDefaultAsync|QuerySingle|QuerySingleAsync|QuerySingleOrDefault|QuerySingleOrDefaultAsync|QueryMultiple|QueryMultipleAsync)\s*[\(<]`,
)

// --- #4692 EF receiver-typed read credit (ambiguous LINQ terminals) ---
//
// The synchronous LINQ verbs below collide with LINQ-to-Objects on plain
// collections, so they are credited db_read ONLY when the receiver is known to
// be an EF `DbSet<T>` / `IQueryable<T>` (the layered-repository read shape —
// `_context.Users.Where(...).FirstOrDefault()`, `_db.Set<Order>().ToList()`).
// On any other receiver they stay pure, preserving the false-positive guard.
const csharpAmbiguousLinqVerbs = `Where|First|FirstOrDefault|Single|SingleOrDefault|ToList|ToArray|Any|All|Count|Find`

// csharpDBSetTypedRe seeds the set of DbSet/IQueryable-typed receiver names.
// Group 1 captures the typed name from the recurring EF declarations:
//   - `public DbSet<User> Users { get; set; }`     (DbContext property)
//   - `DbSet<User> users = _context.Set<User>();`  (local)
//   - `IQueryable<User> q = ...;` / `var q = _ctx.Users.Where(...)` chains
//   - field/property `_context`/`_db`/`Context`/`db` of a DbContext are typed
//     as roots so `_context.Users` is reachable — handled by csharpDBSetRootRe.
var csharpDBSetTypedRe = regexp.MustCompile(
	`(?:DbSet|IQueryable|IOrderedQueryable|DbQuery)\s*<[^>]+>\s+([A-Za-z_]\w*)\b`,
)

// csharpDBSetLocalRe types a local assigned from a DbSet-producing RHS:
//   - `... = <root>.Set<T>()`     (DbContext.Set<T>())
//   - `... = <root>.<Prop>.Where|Include|AsNoTracking|...`  (queryable chain)
//
// Group 1 = assigned name, group 2 = the EF root receiver (so chains compose).
var csharpDBSetLocalRe = regexp.MustCompile(
	`(?m)\b(?:var|DbSet<[^>]+>|IQueryable<[^>]+>)\s+([A-Za-z_]\w*)\s*=\s*([A-Za-z_]\w*)\s*\.\s*` +
		`(?:Set\s*<[^>]+>\s*\(|[A-Za-z_]\w*\s*\.\s*(?:Where|Include|AsNoTracking|AsQueryable|OrderBy|OrderByDescending|Select)\b)`,
)

// csharpDBSetRootMemberRe credits an ambiguous verb invoked DIRECTLY off an EF
// root's navigation property — `_context.Users.Where(...)`, `_db.Orders.ToList()`,
// `this.Context.Set<T>().FirstOrDefault()`. The root token must look like a
// DbContext handle (`_context`/`_ctx`/`_db`/`context`/`db`/`Context`/`Db` or a
// `*Context` name), and the verb must be the ambiguous set; distinctive verbs
// are already covered bare by csharpDBReadRe.
var csharpDBSetRootMemberRe = regexp.MustCompile(
	`\b(?:_?[Cc]ontext|_?[Cc]tx|_?[Dd]b|[A-Za-z_]\w*Context)\s*\.\s*` +
		`(?:Set\s*<[^>]+>\s*\(\s*\)|[A-Za-z_]\w*)\s*\.\s*(?:` + csharpAmbiguousLinqVerbs + `)\s*[\(<]`,
)

// csharpDBWriteRe matches EF / Dapper / ADO.NET write primitives.
var csharpDBWriteRe = regexp.MustCompile(
	`\.\s*(?:Add|AddAsync|AddRange|AddRangeAsync|Update|UpdateRange|Remove|RemoveRange|Attach|AttachRange|Entry)\s*\(` +
		`|\.\s*(?:SaveChanges|SaveChangesAsync|ExecuteSqlRaw|ExecuteSqlRawAsync|ExecuteSqlInterpolated|ExecuteSqlInterpolatedAsync|ExecuteUpdate|ExecuteUpdateAsync|ExecuteDelete|ExecuteDeleteAsync)\s*\(` +
		`|\.\s*(?:ExecuteNonQuery|ExecuteNonQueryAsync)\s*\(` +
		`|\.\s*(?:Execute|ExecuteAsync|ExecuteScalar)\s*\(\s*["@$]`,
)

// csharpFSReadRe matches read-only filesystem primitives.
var csharpFSReadRe = regexp.MustCompile(
	`\bFile\s*\.\s*(?:ReadAllText|ReadAllTextAsync|ReadAllBytes|ReadAllBytesAsync|ReadAllLines|ReadAllLinesAsync|ReadLines|ReadLinesAsync|OpenRead|OpenText|Exists)\s*\(` +
		`|\bDirectory\s*\.\s*(?:GetFiles|GetDirectories|GetFileSystemEntries|EnumerateFiles|EnumerateDirectories|EnumerateFileSystemEntries|Exists)\s*\(` +
		`|\bnew\s+(?:StreamReader|FileStream\s*\([^)]*FileMode\.Open|BinaryReader)\s*\(`,
)

// csharpFSWriteRe matches write filesystem primitives.
var csharpFSWriteRe = regexp.MustCompile(
	`\bFile\s*\.\s*(?:WriteAllText|WriteAllTextAsync|WriteAllBytes|WriteAllBytesAsync|WriteAllLines|WriteAllLinesAsync|AppendAllText|AppendAllTextAsync|AppendAllLines|Delete|Move|Copy|Create|CreateText|OpenWrite|SetAttributes|SetCreationTime|SetLastWriteTime|Replace)\s*\(` +
		`|\bDirectory\s*\.\s*(?:CreateDirectory|Delete|Move)\s*\(` +
		`|\bnew\s+(?:StreamWriter|FileStream\s*\([^)]*FileMode\.(?:Create|CreateNew|Append)|BinaryWriter)\s*\(`,
)

// csharpProcessRe matches process-spawn primitives (modelled as fs_write).
var csharpProcessRe = regexp.MustCompile(
	`\bProcess\s*\.\s*Start\s*\(` +
		`|\bnew\s+Process\s*\(\s*\)\s*\.\s*Start\b` +
		`|\bProcessStartInfo\s*\(`,
)

// csharpMutationRe matches `this.<Member> = ...` assignment.
var csharpMutationRe = regexp.MustCompile(
	`\bthis\s*\.\s*[A-Za-z_][\w]*\s*=(?:[^=])`,
)

func sniffEffectsCSharp(content string) []EffectMatch {
	if content == "" {
		return nil
	}
	headers := scanCSharpFuncHeaders(content)
	var out []EffectMatch
	out = appendCSharpMatches(out, content, headers, csharpHTTPRe, EffectHTTPOut, "HttpClient/WebClient/RestSharp", 1.0)
	out = appendCSharpMatches(out, content, headers, csharpDBReadRe, EffectDBRead, "ef/dapper/ado.read", 0.85)
	out = append(out, csharpDBSetReadMatches(content, headers)...)
	out = appendCSharpMatches(out, content, headers, csharpDBWriteRe, EffectDBWrite, "ef/dapper/ado.write", 0.85)
	out = appendCSharpMatches(out, content, headers, csharpFSReadRe, EffectFSRead, "File.Read/Directory.Get", 1.0)
	out = appendCSharpMatches(out, content, headers, csharpFSWriteRe, EffectFSWrite, "File.Write/Directory.Create", 1.0)
	out = appendCSharpMatches(out, content, headers, csharpProcessRe, EffectFSWrite, "Process.Start", 0.9)
	out = appendCSharpMatches(out, content, headers, csharpMutationRe, EffectMutation, "this.field=", 0.7)
	return out
}

// csharpDBSetReadMatches implements the #4692 receiver-typed read credit for
// C#. It (1) collects the set of DbSet/IQueryable-typed receiver names
// (typed declarations, locals assigned from `.Set<T>()`/queryable chains,
// propagated to a fixpoint), then (2) emits db_read for each ambiguous LINQ
// terminal invoked on one of those typed names, AND for ambiguous terminals
// chained directly off an EF root's navigation property (`_context.Users.Where`).
// An ambiguous LINQ verb on an UNTYPED receiver (a plain List/array) earns no
// credit — the in-memory-LINQ false-positive guard is preserved.
func csharpDBSetReadMatches(content string, headers []funcHeader) []EffectMatch {
	typed := collectCSharpDBSetNames(content)
	var out []EffectMatch
	emit := func(off int) {
		line := lineOfOffset(content, off)
		out = append(out, EffectMatch{
			Function:   nearestHeader(headers, line),
			Line:       line,
			Effect:     EffectDBRead,
			Sink:       "ef.read.dbset",
			Confidence: 0.85,
		})
	}
	for name := range typed {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\.\s*(?:` + csharpAmbiguousLinqVerbs + `)\s*[\(<]`)
		for _, m := range re.FindAllStringIndex(content, -1) {
			emit(m[0])
		}
	}
	for _, m := range csharpDBSetRootMemberRe.FindAllStringIndex(content, -1) {
		emit(m[0])
	}
	return out
}

// collectCSharpDBSetNames returns the set of names known to hold a DbSet /
// IQueryable. Seeds from typed declarations and `.Set<T>()`/queryable-chain
// locals, then iterates the local-from-typed-receiver form to a fixpoint so
// `var q = users.Where(...)` types `q` when `users` is already a DbSet.
func collectCSharpDBSetNames(content string) map[string]bool {
	typed := map[string]bool{}
	for _, m := range csharpDBSetTypedRe.FindAllStringSubmatch(content, -1) {
		if len(m) >= 2 && m[1] != "" {
			typed[m[1]] = true
		}
	}
	for _, m := range csharpDBSetLocalRe.FindAllStringSubmatch(content, -1) {
		if len(m) >= 2 && m[1] != "" {
			typed[m[1]] = true
		}
	}
	// Fixpoint: `var <dst> = <src>.<queryable-op>(...)` where <src> is typed.
	chainRe := regexp.MustCompile(`(?m)\bvar\s+([A-Za-z_]\w*)\s*=\s*([A-Za-z_]\w*)\s*\.\s*(?:Where|Include|ThenInclude|AsNoTracking|AsQueryable|OrderBy|OrderByDescending|Select|Skip|Take)\b`)
	chains := chainRe.FindAllStringSubmatch(content, -1)
	for {
		changed := false
		for _, m := range chains {
			if len(m) < 3 {
				continue
			}
			if typed[m[2]] && !typed[m[1]] {
				typed[m[1]] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return typed
}

func scanCSharpFuncHeaders(content string) []funcHeader {
	var hs []funcHeader
	add := func(name string, off int) {
		if name == "" || csharpControlKeyword(name) {
			return
		}
		hs = append(hs, funcHeader{Line: lineOfOffset(content, off), Name: name})
	}
	for _, m := range csharpMethodHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		add(content[m[2]:m[3]], m[0])
	}
	for _, m := range csharpCtorHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		add(content[m[2]:m[3]], m[0])
	}
	return hs
}

// csharpControlKeyword rejects control-flow tokens that the conservative
// method regex can accidentally match.
func csharpControlKeyword(s string) bool {
	switch s {
	case "if", "else", "for", "foreach", "while", "switch", "catch", "try", "do", "return", "throw", "new", "lock", "using", "fixed", "checked", "unchecked", "yield", "await", "this", "base":
		return true
	}
	return false
}

func appendCSharpMatches(out []EffectMatch, content string, headers []funcHeader, re *regexp.Regexp, eff Effect, sink string, conf float64) []EffectMatch {
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
