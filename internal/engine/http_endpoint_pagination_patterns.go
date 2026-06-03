package engine

import "regexp"

// Compiled patterns for endpoint pagination detection (see
// http_endpoint_pagination.go). Kept in one file so the (cheap) compile cost is
// paid once at package init.
var (
	// pyParamDeclRe matches a Python parameter declaration `name: Type = ...` or
	// bare `name,` in a (possibly multi-line) function signature. Group 1 is the
	// identifier. We only later keep the ones that are pagination-shaped.
	pyParamDeclRe = regexp.MustCompile(`(?m)[\(,]\s*([A-Za-z_][A-Za-z0-9_]*)\s*[:=,\)]`)

	// pyRequestQueryGetRe matches a query-param read whose key is a STRING
	// LITERAL on the request object used by the ASGI/WSGI micro-frameworks:
	//
	//   sanic / quart / flask: request.args.get("limit") / request.args["limit"]
	//   starlette / litestar:  request.query_params.get("offset") /
	//                          request.query_params["cursor"]
	//
	// Group 1 is the param name. HONEST-PARTIAL: a dynamically-named read
	// (`request.args.get(key)`) has no string literal and does not match.
	pyRequestQueryGetRe = regexp.MustCompile(`\.\s*(?:args|query_params|GET)\s*(?:\.\s*get\s*\(|\[)\s*["']([A-Za-z_][A-Za-z0-9_]*)["']`)

	// pyBottleQueryRe matches Bottle's request-query reads:
	//
	//   request.query.limit               (FormsDict attribute access)
	//   request.query.get("limit")        (.get with string literal)
	//   request.query["limit"]            (bracket index)
	//
	// Group 1 is the attribute name, group 2 the .get/bracket literal name
	// (exactly one of the two is non-empty per match). HONEST-PARTIAL: a
	// dynamically-named read (`request.query.get(key)`) has no literal and does
	// not match.
	pyBottleQueryRe = regexp.MustCompile(`\.\s*query\s*(?:\.\s*get\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']|\[\s*["']([A-Za-z_][A-Za-z0-9_]*)["']|\.\s*([A-Za-z_][A-Za-z0-9_]*))`)

	// pyFalconGetParamRe matches Falcon's `req.get_param("limit")` /
	// `req.get_param_as_int("offset")` request-query reads. Group 1 is the name.
	pyFalconGetParamRe = regexp.MustCompile(`\.\s*get_param(?:_as_\w+)?\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']`)

	// pyTornadoArgRe matches Tornado's `self.get_query_argument("limit")` /
	// `self.get_argument("offset")` request-query reads. Group 1 is the name.
	pyTornadoArgRe = regexp.MustCompile(`\.\s*get_(?:query_)?argument\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']`)

	// djangoPaginatorRe matches `Paginator(<qs>, <n>)` — the canonical Django
	// core paginator constructor.
	djangoPaginatorRe = regexp.MustCompile(`\bPaginator\s*\(`)

	// fastapiPaginateRe matches a fastapi-pagination `paginate(` call (the
	// library's page-style helper).
	fastapiPaginateRe = regexp.MustCompile(`\bpaginate\s*\(`)

	// springPageableParamRe matches a `Pageable <name>` handler parameter
	// (optionally annotated). Anchored on the word boundary so it does not match
	// `PageableXxx`.
	springPageableParamRe = regexp.MustCompile(`\bPageable\b\s+[A-Za-z_]`)

	// springPageReturnRe matches a `Page<...>` or `Slice<...>` return type
	// (Spring Data's paginated result wrappers).
	springPageReturnRe = regexp.MustCompile(`\b(?:Page|Slice)\s*<`)

	// jsQueryDotRe matches `req.query.<name>` / `request.query.<name>` /
	// `ctx.query.<name>` reads. Group 1 is the param name.
	jsQueryDotRe = regexp.MustCompile(`(?:req|request|ctx)\.query\.([A-Za-z_][A-Za-z0-9_]*)`)

	// jsQueryBracketRe matches `req.query["<name>"]` / `req.query['<name>']`.
	jsQueryBracketRe = regexp.MustCompile(`(?:req|request|ctx)\.query\[\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\]`)

	// jsQueryDestructureRe matches `const { a, b } = req.query`. Group 1 is the
	// brace contents.
	jsQueryDestructureRe = regexp.MustCompile(`\{\s*([^}]*?)\s*\}\s*=\s*(?:req|request|ctx)\.query\b`)

	// jsHonoQueryRe matches Hono's request-query reads:
	//   c.req.query("limit") / c.req.query('offset')
	// Group 1 is the param name. A bare `c.req.query()` (all params, no literal)
	// has no name and does not match — honest-partial.
	jsHonoQueryRe = regexp.MustCompile(`\.\s*req\s*\.\s*query\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']`)

	// jsAdonisInputRe matches AdonisJS's request reads:
	//   request.input("limit") / request.qs().limit / request.qs()["offset"]
	// Group 1 is the input() literal, group 2 the qs() attribute name, group 3
	// the qs() bracket literal (exactly one non-empty per match).
	jsAdonisInputRe = regexp.MustCompile(`\.\s*(?:input\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']|qs\s*\(\s*\)\s*(?:\.\s*([A-Za-z_][A-Za-z0-9_]*)|\[\s*["']([A-Za-z_][A-Za-z0-9_]*)["']))`)

	// sequelizeOrPrismaTakeRe / sequelizeOrPrismaSkipRe match Prisma `take:` /
	// `skip:` keys (also used by some query builders).
	sequelizeOrPrismaTakeRe = regexp.MustCompile(`\btake\s*:`)
	sequelizeOrPrismaSkipRe = regexp.MustCompile(`\bskip\s*:`)

	// sequelizeLimitRe / sequelizeOffsetRe match Sequelize `limit:` / `offset:`
	// option keys (findAll({ limit, offset })).
	sequelizeLimitRe  = regexp.MustCompile(`\blimit\s*:`)
	sequelizeOffsetRe = regexp.MustCompile(`\boffset\s*:`)

	// prismaCursorRe matches a Prisma `.cursor(` / `cursor:` keyset selector.
	prismaCursorRe = regexp.MustCompile(`\bcursor\s*[:\(]`)

	// schemaNameRe pulls `"name"` keys out of a JSON parameter_schema blob.
	schemaNameRe = regexp.MustCompile(`"([A-Za-z_][A-Za-z0-9_]*)"\s*:`)
)
