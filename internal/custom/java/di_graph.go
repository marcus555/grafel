// di_graph.go — Spring & Guice dependency-injection GRAPH extraction (#3699,
// parent #3628 area #5).
//
// spring_di_deepen.go already deepens Spring DI with @Qualifier/@Value/
// @ConditionalOnMissingBean DEPENDS_ON edges, but it never emits the actual DI
// GRAPH that the rewrite-parity oracle needs: "what provider satisfies this
// constructor param" (INJECTED_INTO) and "what implementation backs this
// token/interface" (BINDS). This file adds that graph for Spring and Guice,
// mirroring the NestJS DI work (#3649, javascript/nestjs_di.go) edge-for-edge:
//
//	INJECTED_INTO : provider/bean-type → the @Component/@Service/@Repository/
//	                @Controller (Spring) or @Inject-annotated (Guice) class that
//	                injects it via constructor or @Autowired/@Inject field.
//	                FromID = provider (the injected type), ToID = consumer class,
//	                matching nestjs_di.go and angular.go. Property
//	                via=spring_constructor|spring_field|guice_constructor|
//	                guice_field; qualifier recorded when @Qualifier is present.
//
//	BINDS         : a DI token/interface → its implementation, with the binding
//	                lifetime/scope. Two shapes:
//	                  Spring  @Bean methods in an @Configuration class:
//	                          return-type token BINDS the method (provider).
//	                  Guice   bind(Foo.class).to(FooImpl.class) in a Module:
//	                          Foo BINDS FooImpl, with .in(Scopes.SINGLETON) /
//	                          @Singleton lifetime when present.
//	                FromID = token/interface, ToID = impl, mirroring the NestJS
//	                {provide: TOKEN, useClass: Impl} and Helm token→impl BINDS.
//
// Honest-partial: a constructor-injected provider whose declaring bean lives in
// another file resolves cross-file only via the resolver pass (FromID is the
// bare type name); conditional @Bean methods (@ConditionalOnMissingBean) and
// Guice provider methods (@Provides) bind a type that may be supplied elsewhere.
// Those cross-file/conditional cases keep the di_binding cell at "partial".
//
// The edges are emitted through the SecondaryEntity/Relationship model the java
// custom dispatch (patterns_dispatch.go) converts: each edge hangs off a carrier
// entity whose Ref == Relationship.SourceRef, and ToID = Relationship.TargetRef.
// To match the NestJS FromID=provider / ToID=consumer convention, the carrier
// (SourceRef) is the provider/token and the TargetRef is the consumer/impl.
package java

import (
	"regexp"
	"strings"
)

// ── framework gates ───────────────────────────────────────────────────────────

// springDIGraphFrameworks gates the Spring INJECTED_INTO / @Bean BINDS graph.
// Same Spring family as spring_di_deepen, plus spring_mvc (controllers inject).
var springDIGraphFrameworks = map[string]bool{
	"spring_boot": true, "spring-boot": true, "springboot": true,
	"spring_webflux": true, "spring-webflux": true, "springwebflux": true,
	"spring_mvc": true, "spring-mvc": true, "springmvc": true,
}

// guiceFrameworks gates the Guice bind().to() / @Inject graph. Guice is not a
// distinct token in the dispatch markers, so the jakarta_inject signal
// (jakarta.inject / javax.inject — the @Inject annotation Guice reuses) and an
// explicit guice token are accepted; the extractor self-gates on its own regex
// signals (a bind(...).to(...) call or an extends AbstractModule) so a false
// candidate emits nothing.
var guiceFrameworks = map[string]bool{
	"guice": true, "google_guice": true,
	"jakarta_ee": true, // @Inject + AbstractModule signal still self-gated below
}

// ── Spring DI graph regexes ───────────────────────────────────────────────────

// dgSpringComponentRE detects a Spring-managed bean class declaration carrying
// a stereotype annotation, so we know its constructor params are injection
// points. Group 1 = stereotype, group 2 = class name.
var dgSpringComponentRE = regexp.MustCompile(
	`(?s)@(Component|Service|Repository|Controller|RestController|Configuration)\b` +
		`(?:\s*\([^)]*\))?\s*` +
		`(?:@\w+(?:\s*\([^()]*(?:\([^()]*\)[^()]*)*\))?\s*)*` +
		`(?:public\s+)?(?:(?:abstract|final)\s+)?class\s+(\w+)`)

// dgCtorRE detects a constructor of a given class: `ClassName(` after a public/
// protected modifier. We build it per-class at runtime.

// dgQualifierInParamRE pulls a @Qualifier("x") out of a single parameter chunk.
var dgQualifierInParamRE = regexp.MustCompile(`@Qualifier\s*\(\s*"([^"]+)"\s*\)`)

// dgAutowiredFieldRE detects an @Autowired (or @Inject) field injection.
// Group 1 = field type, group 2 = field name. An optional @Qualifier may sit
// between @Autowired and the field; it is parsed separately from the match span.
var dgAutowiredFieldRE = regexp.MustCompile(
	`(?s)@(?:Autowired|Inject)\b\s*` +
		`(?:@\w+(?:\s*\([^()]*(?:\([^()]*\)[^()]*)*\))?\s*)*` +
		`(?:private|protected|public)\s+(?:final\s+)?` +
		`(\w+)(?:\s*<[^>]*>)?\s+(\w+)\s*[;=]`)

// dgBeanMethodRE detects a @Bean factory method in a @Configuration class.
// Group 1 = (optional) explicit bean name from @Bean("name"), group 2 = return
// type, group 3 = method name. The return type is the bound token.
var dgBeanMethodRE = regexp.MustCompile(
	`(?s)@Bean\b\s*(?:\(\s*(?:name\s*=\s*)?(?:\{\s*)?"?([^"){}]*)"?\s*\}?\s*\))?\s*` +
		`(?:@\w+(?:\s*\([^()]*(?:\([^()]*\)[^()]*)*\))?\s*)*` +
		`(?:public\s+|protected\s+|private\s+)?(?:static\s+)?` +
		`(?:<[^>]*>\s+)?(\w+)(?:\s*<[^>]*>)?\s+(\w+)\s*\(`)

// dgScopeOnBeanRE pulls a @Scope("singleton") that decorates a @Bean.
var dgScopeOnBeanRE = regexp.MustCompile(`@Scope\s*\(\s*(?:value\s*=\s*)?"([^"]+)"\s*\)`)

// ── Guice regexes ──────────────────────────────────────────────────────────────

// dgGuiceBindRE detects bind(Foo.class).to(FooImpl.class) and optional scope.
// Group 1 = bound type, group 2 = impl type. A trailing
// .in(Scopes.SINGLETON) / .asEagerSingleton() is captured separately.
var dgGuiceBindRE = regexp.MustCompile(
	`bind\s*\(\s*(\w+)\s*\.class\s*\)\s*\.\s*to\s*\(\s*(\w+)\s*\.class\s*\)([^;]*)`)

// dgGuiceBindProviderRE detects bind(Foo.class).toProvider(FooProvider.class).
var dgGuiceBindProviderRE = regexp.MustCompile(
	`bind\s*\(\s*(\w+)\s*\.class\s*\)\s*\.\s*toProvider\s*\(\s*(\w+)\s*\.class\s*\)([^;]*)`)

// dgGuiceModuleRE detects a Guice module (extends AbstractModule / implements
// Module). Group 1 = class name. Used only as a presence signal.
var dgGuiceModuleRE = regexp.MustCompile(
	`(?s)class\s+\w+[^{]*\b(?:extends\s+AbstractModule|implements\s+Module)\b`)

// dgGuiceScopeRE pulls the scope out of a Guice bind tail (.in(...) /
// .asEagerSingleton()).
var dgGuiceScopeRE = regexp.MustCompile(
	`\.\s*in\s*\(\s*(?:Scopes\.)?(\w+)|\.\s*(asEagerSingleton)\s*\(`)

// ── Spring extractor ───────────────────────────────────────────────────────────

// ExtractSpringDIGraph emits the Spring DI graph: INJECTED_INTO for constructor
// and @Autowired-field injection into stereotype-annotated beans, and BINDS for
// @Bean factory methods in @Configuration classes.
func ExtractSpringDIGraph(ctx PatternContext) PatternResult {
	var result PatternResult
	if (ctx.Language != "java" && ctx.Language != "kotlin") || !springDIGraphFrameworks[ctx.Framework] {
		return result
	}
	src := ctx.Source
	fp := ctx.FilePath
	seenRefs := make(map[string]bool)
	seenRels := make(map[relKey]bool)

	// ── 1. Constructor injection: provider INJECTED_INTO the bean class ──────
	for _, m := range dgSpringComponentRE.FindAllStringSubmatchIndex(src, -1) {
		stereotype := src[m[2]:m[3]]
		cls := src[m[4]:m[5]]
		clsBodyStart := indexClassBody(src, m[1])
		if clsBodyStart < 0 {
			continue
		}
		// Constructor of the bean: `ClassName(` within the body.
		params, paramsOK := classConstructorParams(src, cls, clsBodyStart)
		if paramsOK {
			for _, p := range splitJavaParams(params) {
				injType, qualifier := javaParamInjectedType(p)
				if injType == "" {
					continue
				}
				emitInjectedInto(&result, seenRefs, seenRels, fp, injType, cls,
					"spring_constructor", qualifier, ctx.Framework, lineOf(src, m[0]))
			}
		}
		// ── 2. @Autowired / @Inject field injection on this bean ─────────────
		clsEnd := classBodyEnd(src, clsBodyStart)
		clsBody := src[clsBodyStart:clsEnd]
		_ = stereotype
		for _, fm := range dgAutowiredFieldRE.FindAllStringSubmatchIndex(clsBody, -1) {
			fieldType := clsBody[fm[2]:fm[3]]
			if primitiveTypes[fieldType] {
				continue
			}
			qualifier := ""
			if qm := dgQualifierInParamRE.FindStringSubmatch(clsBody[fm[0]:fm[1]]); qm != nil {
				qualifier = qm[1]
			}
			emitInjectedInto(&result, seenRefs, seenRels, fp, fieldType, cls,
				"spring_field", qualifier, ctx.Framework, lineOf(src, clsBodyStart+fm[0]))
		}
	}

	// ── 3. @Bean factory methods: return-type token BINDS the @Bean method ───
	for _, m := range dgBeanMethodRE.FindAllStringSubmatchIndex(src, -1) {
		explicitName := ""
		if m[2] >= 0 {
			explicitName = strings.TrimSpace(src[m[2]:m[3]])
		}
		retType := src[m[4]:m[5]]
		methodName := src[m[6]:m[7]]
		if primitiveTypes[retType] || retType == "void" {
			continue
		}
		// Bean scope: a @Scope on the same method (look back up to 200 bytes).
		scope := "singleton"
		lookBack := m[0] - 200
		if lookBack < 0 {
			lookBack = 0
		}
		if sm := dgScopeOnBeanRE.FindStringSubmatch(src[lookBack:m[0]]); sm != nil {
			scope = sm[1]
		}
		token := retType
		if explicitName != "" {
			token = explicitName
		}
		emitBeanBinds(&result, seenRefs, seenRels, fp, token, retType, methodName,
			scope, ctx.Framework, lineOf(src, m[0]))
	}

	return result
}

// ── Guice extractor ────────────────────────────────────────────────────────────

// ExtractGuiceDI emits the Guice DI graph: bind(Foo.class).to(Impl.class) →
// Foo BINDS Impl (with lifetime), and @Inject constructor/field → provider
// INJECTED_INTO the injecting class. Self-gates on the bind() / AbstractModule
// signal so a non-Guice file under the jakarta_ee candidate emits nothing.
func ExtractGuiceDI(ctx PatternContext) PatternResult {
	var result PatternResult
	if (ctx.Language != "java" && ctx.Language != "kotlin") || !guiceFrameworks[ctx.Framework] {
		return result
	}
	src := ctx.Source
	// Self-gate. Under the shared jakarta_ee fallback token (where @Inject is
	// ambiguous with Spring/CDI) we require a concrete Guice signal — a
	// bind(...).to(...) call or an AbstractModule — before emitting anything, so
	// a plain @Inject file is never a false positive. When the framework is the
	// explicit guice token, the caller has already classified the source as
	// Guice, so @Inject injection points are emitted directly.
	explicitGuice := ctx.Framework == "guice" || ctx.Framework == "google_guice"
	hasBind := dgGuiceBindRE.MatchString(src) || dgGuiceBindProviderRE.MatchString(src)
	isModule := dgGuiceModuleRE.MatchString(src)
	if !explicitGuice && !hasBind && !isModule {
		return result
	}
	fp := ctx.FilePath
	seenRefs := make(map[string]bool)
	seenRels := make(map[relKey]bool)

	// ── 1. bind(Foo.class).to(FooImpl.class) → Foo BINDS FooImpl ─────────────
	for _, m := range dgGuiceBindRE.FindAllStringSubmatchIndex(src, -1) {
		token := src[m[2]:m[3]]
		impl := src[m[4]:m[5]]
		tail := src[m[6]:m[7]]
		scope := guiceScope(tail)
		emitGuiceBinds(&result, seenRefs, seenRels, fp, token, impl, scope, "bind_to",
			ctx.Framework, lineOf(src, m[0]))
	}
	// bind(Foo.class).toProvider(FooProvider.class).
	for _, m := range dgGuiceBindProviderRE.FindAllStringSubmatchIndex(src, -1) {
		token := src[m[2]:m[3]]
		provider := src[m[4]:m[5]]
		tail := src[m[6]:m[7]]
		scope := guiceScope(tail)
		emitGuiceBinds(&result, seenRefs, seenRels, fp, token, provider, scope, "bind_provider",
			ctx.Framework, lineOf(src, m[0]))
	}

	// ── 2. @Inject constructor / field injection in any class in the file ────
	for _, c := range allClassDecls(src) {
		// @Inject constructor: scan the class body for an @Inject-decorated
		// constructor and treat its params as injection points.
		params, ok := injectConstructorParams(src, c.name, c.bodyStart, c.bodyEnd)
		if ok {
			for _, p := range splitJavaParams(params) {
				injType, qualifier := javaParamInjectedType(p)
				if injType == "" {
					continue
				}
				emitInjectedInto(&result, seenRefs, seenRels, fp, injType, c.name,
					"guice_constructor", qualifier, ctx.Framework, lineOf(src, c.bodyStart))
			}
		}
		// @Inject field injection.
		body := src[c.bodyStart:c.bodyEnd]
		for _, fm := range dgAutowiredFieldRE.FindAllStringSubmatchIndex(body, -1) {
			fieldType := body[fm[2]:fm[3]]
			if primitiveTypes[fieldType] {
				continue
			}
			qualifier := ""
			if qm := dgQualifierInParamRE.FindStringSubmatch(body[fm[0]:fm[1]]); qm != nil {
				qualifier = qm[1]
			}
			emitInjectedInto(&result, seenRefs, seenRels, fp, fieldType, c.name,
				"guice_field", qualifier, ctx.Framework, lineOf(src, c.bodyStart+fm[0]))
		}
	}

	return result
}

// ── shared emit helpers ────────────────────────────────────────────────────────

// emitInjectedInto records `provider INJECTED_INTO consumer`. The carrier
// entity is the provider (SourceRef), matching the NestJS FromID=provider
// convention; the consumer is the TargetRef. Both refs use the bare type name
// so the resolver can bind them to the declaring entity.
func emitInjectedInto(result *PatternResult, seenRefs map[string]bool, seenRels map[relKey]bool,
	fp, provider, consumer, via, qualifier, framework string, line int) {
	if provider == "" || consumer == "" || provider == consumer {
		return
	}
	providerRef := findRefForType(provider, fp, "di_provider", result)
	addEntity(result, seenRefs, SecondaryEntity{
		Name: provider, Kind: "SCOPE.Class", SourceFile: fp,
		LineStart: line, LineEnd: line,
		Provenance: "INFERRED_FROM_DI_PROVIDER",
		Ref:        providerRef,
		Properties: map[string]any{
			"di_role":   "provider",
			"framework": framework,
		},
	})
	consumerRef := findRefForType(consumer, fp, "di_consumer", result)
	props := map[string]string{
		"provider":  provider,
		"consumer":  consumer,
		"via":       via,
		"framework": framework,
	}
	if qualifier != "" {
		props["qualifier"] = qualifier
	}
	addRel(result, seenRels, Relationship{
		SourceRef:        providerRef,
		TargetRef:        consumerRef,
		RelationshipType: "INJECTED_INTO",
		Properties:       props,
	})
}

// emitBeanBinds records a Spring @Bean: `token BINDS method` (the @Bean method
// is the provider of the bound type). FromID = token (return type / bean name),
// ToID = the factory method.
func emitBeanBinds(result *PatternResult, seenRefs map[string]bool, seenRels map[relKey]bool,
	fp, token, retType, methodName, scope, framework string, line int) {
	tokenRef := findRefForType(token, fp, "di_token", result)
	addEntity(result, seenRefs, SecondaryEntity{
		Name: token, Kind: "SCOPE.Class", SourceFile: fp,
		LineStart: line, LineEnd: line,
		Provenance: "INFERRED_FROM_SPRING_BEAN",
		Ref:        tokenRef,
		Properties: map[string]any{
			"di_role":   "token",
			"bean_type": retType,
			"framework": framework,
		},
	})
	implRef := "scope:operation:spring_bean:" + fp + ":" + methodName
	addEntity(result, seenRefs, SecondaryEntity{
		Name: methodName, Kind: "SCOPE.Operation", Subtype: "function", SourceFile: fp,
		LineStart: line, LineEnd: line,
		Provenance: "INFERRED_FROM_SPRING_BEAN_METHOD",
		Ref:        implRef,
		Properties: map[string]any{
			"di_role":     "bean_method",
			"bean_type":   retType,
			"bean_method": methodName,
			"framework":   framework,
		},
	})
	addRel(result, seenRels, Relationship{
		SourceRef:        tokenRef,
		TargetRef:        implRef,
		RelationshipType: "BINDS",
		Properties: map[string]string{
			"binding_kind": "bean_method",
			"token":        token,
			"bean_type":    retType,
			"lifetime":     scope,
			"framework":    framework,
		},
	})
}

// emitGuiceBinds records a Guice bind(): `token BINDS impl` with lifetime.
func emitGuiceBinds(result *PatternResult, seenRefs map[string]bool, seenRels map[relKey]bool,
	fp, token, impl, scope, kind, framework string, line int) {
	if token == "" || impl == "" {
		return
	}
	tokenRef := findRefForType(token, fp, "di_token", result)
	addEntity(result, seenRefs, SecondaryEntity{
		Name: token, Kind: "SCOPE.Class", SourceFile: fp,
		LineStart: line, LineEnd: line,
		Provenance: "INFERRED_FROM_GUICE_BIND",
		Ref:        tokenRef,
		Properties: map[string]any{
			"di_role":   "token",
			"framework": framework,
		},
	})
	implRef := findRefForType(impl, fp, "di_impl", result)
	addEntity(result, seenRefs, SecondaryEntity{
		Name: impl, Kind: "SCOPE.Class", SourceFile: fp,
		LineStart: line, LineEnd: line,
		Provenance: "INFERRED_FROM_GUICE_IMPL",
		Ref:        implRef,
		Properties: map[string]any{
			"di_role":   "implementation",
			"framework": framework,
		},
	})
	addRel(result, seenRels, Relationship{
		SourceRef:        tokenRef,
		TargetRef:        implRef,
		RelationshipType: "BINDS",
		Properties: map[string]string{
			"binding_kind": kind, // bind_to | bind_provider
			"token":        token,
			"lifetime":     scope,
			"framework":    framework,
		},
	})
}

// ── parsing helpers ────────────────────────────────────────────────────────────

// indexClassBody returns the index of the opening `{` of a class body whose
// declaration head ends at headEnd (the byte after `class Name`), or -1.
func indexClassBody(src string, headEnd int) int {
	for i := headEnd; i < len(src); i++ {
		switch src[i] {
		case '{':
			return i
		case ';':
			return -1 // forward decl / no body on this line
		}
	}
	return -1
}

// classBodyEnd returns the index just past the matching `}` for the class body
// that opens at bodyStart (src[bodyStart]=='{'), or len(src) on imbalance.
func classBodyEnd(src string, bodyStart int) int {
	depth := 0
	for i := bodyStart; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(src)
}

// classConstructorParams finds the parameter list of a constructor named cls
// within the class body opening at bodyStart. Returns (params, true) when a
// constructor is found, ("", false) otherwise.
func classConstructorParams(src, cls string, bodyStart int) (string, bool) {
	end := classBodyEnd(src, bodyStart)
	body := src[bodyStart:end]
	re := regexp.MustCompile(`(?:public|protected|private)?\s*` + regexp.QuoteMeta(cls) + `\s*\(`)
	loc := re.FindStringIndex(body)
	if loc == nil {
		return "", false
	}
	open := bodyStart + loc[1] - 1
	params := balancedParens(src, open)
	return params, true
}

// injectConstructorParams finds an @Inject-annotated constructor of class cls
// within [bodyStart,bodyEnd) and returns its parameter list.
func injectConstructorParams(src, cls string, bodyStart, bodyEnd int) (string, bool) {
	body := src[bodyStart:bodyEnd]
	re := regexp.MustCompile(
		`(?s)@Inject\b\s*(?:@\w+(?:\([^)]*\))?\s*)*(?:public|protected|private)?\s*` +
			regexp.QuoteMeta(cls) + `\s*\(`)
	loc := re.FindStringIndex(body)
	if loc == nil {
		return "", false
	}
	open := bodyStart + loc[1] - 1
	params := balancedParens(src, open)
	return params, true
}

// balancedParens returns the substring strictly inside the balanced () pair that
// opens at index open (src[open]=='('), or "" on imbalance.
func balancedParens(src string, open int) string {
	if open < 0 || open >= len(src) || src[open] != '(' {
		return ""
	}
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return src[open+1 : i]
			}
		}
	}
	return ""
}

// splitJavaParams splits a Java parameter list on top-level commas, respecting
// nested (), <>, [], {} so generics and annotation args stay intact.
func splitJavaParams(params string) []string {
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case '(', '<', '[', '{':
			depth++
		case ')', '>', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, params[start:i])
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(params[start:]) != "" {
		out = append(out, params[start:])
	}
	return out
}

// javaParamInjectedType returns the injectable bean type of a single Java
// parameter chunk (e.g. "@Qualifier(\"db\") final UserRepo repo" → UserRepo)
// and any @Qualifier name. Primitives, String, and collections are rejected so
// a non-bean parameter yields no edge. The type is the last type token before
// the parameter name.
func javaParamInjectedType(param string) (typ, qualifier string) {
	p := strings.TrimSpace(param)
	if p == "" {
		return "", ""
	}
	if qm := dgQualifierInParamRE.FindStringSubmatch(p); qm != nil {
		qualifier = qm[1]
	}
	// Strip leading annotations (each @Name or @Name(...)).
	p = stripLeadingAnnotations(p)
	// Strip modifiers.
	for _, mod := range []string{"final ", "@Nullable "} {
		p = strings.TrimPrefix(strings.TrimSpace(p), mod)
	}
	p = strings.TrimSpace(p)
	// Tokens: <type> <name>. Take the type (first whitespace-delimited token),
	// then drop any generic parameters.
	fields := strings.Fields(p)
	if len(fields) < 2 {
		return "", qualifier
	}
	t := fields[0]
	if i := strings.IndexAny(t, "<[ "); i >= 0 {
		t = t[:i]
	}
	t = strings.TrimSpace(t)
	if t == "" || primitiveTypes[t] {
		return "", qualifier
	}
	// A bean type starts uppercase; reject lowercase (likely a primitive missed
	// by the set, or a keyword).
	if t[0] < 'A' || t[0] > 'Z' {
		return "", qualifier
	}
	return t, qualifier
}

// stripLeadingAnnotations removes a run of leading @Annotation / @Annotation(...)
// tokens from a parameter chunk, returning the remainder.
func stripLeadingAnnotations(p string) string {
	p = strings.TrimSpace(p)
	for strings.HasPrefix(p, "@") {
		// Skip the annotation name.
		i := 1
		for i < len(p) && (isIdentChar(p[i])) {
			i++
		}
		// Skip an optional (...) argument block.
		for i < len(p) && (p[i] == ' ' || p[i] == '\t') {
			i++
		}
		if i < len(p) && p[i] == '(' {
			depth := 0
			for i < len(p) {
				if p[i] == '(' {
					depth++
				} else if p[i] == ')' {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				i++
			}
		}
		p = strings.TrimSpace(p[i:])
	}
	return p
}

func isIdentChar(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// classDecl is a class declaration span.
type classDecl struct {
	name      string
	bodyStart int
	bodyEnd   int
}

// allClassDecls returns every top-level/nested class declaration in src with
// its body span. Used by the Guice extractor to scan each class for @Inject.
func allClassDecls(src string) []classDecl {
	var out []classDecl
	for _, m := range classDeclRE.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		bs := indexClassBody(src, m[1])
		if bs < 0 {
			continue
		}
		be := classBodyEnd(src, bs)
		out = append(out, classDecl{name: name, bodyStart: bs, bodyEnd: be})
	}
	return out
}

// guiceScope maps a Guice bind tail (.in(Scopes.SINGLETON) / .asEagerSingleton())
// to a lifetime string. Defaults to "no_scope" (a fresh instance per request,
// Guice's unscoped default).
func guiceScope(tail string) string {
	if m := dgGuiceScopeRE.FindStringSubmatch(tail); m != nil {
		if m[1] != "" {
			return strings.ToLower(m[1])
		}
		if m[2] != "" {
			return "eager_singleton"
		}
	}
	return "no_scope"
}
