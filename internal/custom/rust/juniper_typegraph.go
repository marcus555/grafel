package rust

// juniper_typegraph.go — code-first GraphQL schema type→type graph for juniper
// (#5007, follow-up from #4964). The juniper sibling of
// internal/custom/rust/graphql_codefirst_typegraph.go (async-graphql, #3983) and
// internal/custom/{python,javascript}/graphql_codefirst_typegraph.go.
//
// internal/custom/rust/juniper.go already emits the flat DTO catalog (a
// SCOPE.Schema/dto per #[derive(GraphQLObject/GraphQLInputObject/GraphQLEnum)]
// type) and the synthetic GRAPHQL endpoint per #[graphql_object] resolver
// method. What it does NOT emit is the typed field→type relationship graph (the
// #3804 lane): which object type references which other object type, with what
// cardinality. juniper's Schema.type_graph_extraction was HONESTLY missing.
//
// This extractor closes that gap for juniper:
//
//	#[derive(GraphQLObject)]
//	struct User {
//	    id: i32,                  // scalar -> no edge
//	    name: String,             // scalar -> no edge
//	    orders: Vec<Order>,       // GRAPH_RELATES User->Order  to_many
//	    account: Option<Account>, // GRAPH_RELATES User->Account nullable to_one
//	    manager: Option<User>,    // self_ref
//	}
//
//	struct Query;
//	#[graphql_object]
//	impl Query {
//	    // resolver return type -> GRAPH_RELATES Query->User
//	    fn user(&self, id: i32) -> FieldResult<User> { ... }
//	    fn orders(&self) -> Vec<Order> { ... }
//	}
//
// Each object type (a #[derive(GraphQLObject)] struct, the resolver root of a
// #[graphql_object] / #[graphql_subscription] impl) becomes a SCOPE.Schema/type
// node addressed with the SAME canonical structural ref the SDL pass and the
// async-graphql / py / jsts code-first passes use
// (BuildOperationStructuralRef("graphql", file, TypeName)), so identities
// converge on one node per type across passes/repos. Each object-typed struct
// field and each resolver return type becomes a GRAPH_RELATES edge carrying the
// identical cardinality property contract as the SDL / async-graphql / py / jsts
// emitters:
//
//	{field_name, list, nullable, item_nullable, cardinality:to_one|to_many,
//	 self_ref, graphql_field, framework:juniper}
//
// HONEST LIMITS (identical to the async-graphql extractor):
//   - Field-type resolution is heuristic and same-file: a field whose innermost
//     type identifier is a scalar (i32/String/...) or is not an object type
//     declared in the SAME file produces NO edge. Cross-module references and
//     custom scalars are not chased.
//   - GraphQLInputObject and GraphQLEnum types are intentionally NOT owners —
//     inputs/enums are the flat DTO catalog (already credited in #4964) and carry
//     no object→object data relations in the output type graph.
//   - A plain `struct` with no juniper derive emits no type node and no edge; a
//     non-GraphQL `impl` (no #[graphql_object]/#[graphql_subscription]) emits no
//     resolver edge. The #[graphql(name = "...")] field-rename attribute is not
//     yet honoured for the GraphQL field name (matches juniper.go).

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_rust_juniper_typegraph", &rustJuniperTypeGraphExtractor{})
}

// rustJuniperTypeGraphExtractor builds the object-type→type relationship graph
// for juniper code-first schemas.
type rustJuniperTypeGraphExtractor struct{}

func (e *rustJuniperTypeGraphExtractor) Language() string {
	return "custom_rust_juniper_typegraph"
}

var (
	// #[derive(... GraphQLObject ...)] struct Name { ... } — juniper output
	// object types. These are the owners of the field graph and edge targets.
	// Negative lookahead is not available in RE2; GraphQLInputObject is filtered
	// out below by requiring the derive token NOT to be the input/enum form.
	reJunTGObjectStruct = regexp.MustCompile(
		`#\[derive\s*\(([^)]*\bGraphQLObject\b[^)]*)\)\]\s*(?:#\[[^\]]*\]\s*)*(?:pub\s+)?struct\s+([A-Za-z_]\w*)`,
	)
	// #[graphql_object ...] / #[graphql_subscription ...] (resolver impl)
	// immediately preceding `impl <Root>`. Group 1 = the resolver root type name.
	reJunTGObjectImpl = regexp.MustCompile(
		`#\[graphql_(?:object|subscription)[^\]]*\]\s*(?:#\[[^\]]*\]\s*)*impl\s+(?:<[^>]*>\s*)?([A-Za-z_]\w*)`,
	)
)

func (e *rustJuniperTypeGraphExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("custom.rust_juniper_typegraph")
	_, span := tracer.Start(ctx, "custom.rust_juniper_typegraph")
	defer span.End()
	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 || file.Language != "rust" {
		return nil, nil
	}
	src := string(file.Content)

	// File-signal gate: require a juniper marker (mirrors juniper.go).
	if !strings.Contains(src, "#[graphql_object") &&
		!strings.Contains(src, "#[graphql_subscription") &&
		!strings.Contains(src, "GraphQLObject") {
		return nil, nil
	}

	type ownerBlock struct {
		name string
		kind string // "struct" | "resolver"
		line int
		body string
	}
	var owners []ownerBlock
	known := map[string]bool{}

	// #[derive(GraphQLObject)] structs are both owners and edge targets.
	// GraphQLInputObject also matches the substring GraphQLObject, so require the
	// derive group to NOT contain the input/enum forms.
	for _, m := range reJunTGObjectStruct.FindAllStringSubmatchIndex(src, -1) {
		derives := src[m[2]:m[3]]
		if strings.Contains(derives, "GraphQLInputObject") {
			continue
		}
		name := src[m[4]:m[5]]
		bodyStart, bodyEnd := agqlBlockBody(src, m[5])
		body := ""
		if bodyStart >= 0 {
			body = src[bodyStart:bodyEnd]
		}
		owners = append(owners, ownerBlock{name: name, kind: "struct", line: lineOf(src, m[0]), body: body})
		known[name] = true
	}
	// #[graphql_object]/#[graphql_subscription] impl <Root> resolver blocks are
	// owners; their return types are edge targets.
	for _, m := range reJunTGObjectImpl.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		bodyStart, bodyEnd := agqlBlockBody(src, m[1])
		body := ""
		if bodyStart >= 0 {
			body = src[bodyStart:bodyEnd]
		}
		owners = append(owners, ownerBlock{name: name, kind: "resolver", line: lineOf(src, m[0]), body: body})
	}

	if len(owners) == 0 {
		return nil, nil
	}

	nodes := map[string]int{}
	var out []types.EntityRecord
	nodeFor := func(name string, line int) {
		if _, ok := nodes[name]; ok {
			return
		}
		ent := makeEntity(name, "SCOPE.Schema", "type", file.Path, file.Language, line)
		setProps(&ent,
			"graphql_type", name,
			"framework", "juniper",
			"code_first", "true",
			"structural_ref", extractor.BuildOperationStructuralRef("graphql", file.Path, name),
			"provenance", "INFERRED_FROM_CODEFIRST_GRAPHQL_OBJECTTYPE",
		)
		out = append(out, ent)
		nodes[name] = len(out) - 1
	}

	seen := map[string]bool{}
	emit := func(owner, fieldName, target string, tc rustGqlCard) {
		if !known[target] || rustGqlScalars[target] {
			return
		}
		key := owner + "|" + fieldName + "|" + target
		if seen[key] {
			return
		}
		seen[key] = true
		ownerRef := extractor.BuildOperationStructuralRef("graphql", file.Path, owner)
		targetRef := extractor.BuildOperationStructuralRef("graphql", file.Path, target)
		props := map[string]string{
			"field_name":    fieldName,
			"list":          rustGqlBool(tc.list),
			"nullable":      rustGqlBool(tc.nullable),
			"cardinality":   rustGqlCardLabel(tc),
			"self_ref":      rustGqlBool(target == owner),
			"graphql_field": owner + "." + fieldName,
			"framework":     "juniper",
			"provenance":    "INFERRED_FROM_CODEFIRST_GRAPHQL_FIELD",
		}
		if tc.list {
			props["item_nullable"] = rustGqlBool(tc.itemNullable)
		}
		idx := nodes[owner]
		out[idx].Relationships = append(out[idx].Relationships,
			types.RelationshipRecord{
				FromID:     ownerRef,
				ToID:       targetRef,
				Kind:       string(types.RelationshipKindGraphRelates),
				Properties: props,
			})
	}

	for _, ow := range owners {
		nodeFor(ow.name, ow.line)
		switch ow.kind {
		case "struct":
			for _, fm := range reGqlTGStructField.FindAllStringSubmatch(ow.body, -1) {
				fieldName := fm[1]
				typeExpr := strings.TrimSpace(fm[2])
				base, tc := rustParseTypeExpr(typeExpr)
				if base == "" {
					continue
				}
				emit(ow.name, fieldName, base, tc)
			}
		case "resolver":
			for _, fm := range reGqlTGResolverFn.FindAllStringSubmatchIndex(ow.body, -1) {
				fieldName := ow.body[fm[2]:fm[3]]
				ret := rustResolverReturnType(ow.body, fm[0])
				if ret == "" {
					continue
				}
				base, tc := rustParseTypeExpr(ret)
				if base == "" {
					continue
				}
				emit(ow.name, fieldName, base, tc)
			}
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}
