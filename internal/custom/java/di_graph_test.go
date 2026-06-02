package java

import "testing"

// di_graph_test.go — value-asserting tests for the Spring & Guice DI GRAPH
// extractors (#3699). Assertions check the SEMANTIC edge (provider → consumer,
// token → impl, lifetime), never len>0.

func guiceCtx(source string) PatternContext {
	return PatternContext{Source: source, Language: "java", Framework: "guice", FilePath: "GuiceTest.java"}
}

// refName maps a SecondaryEntity Ref back to its bare entity Name, so an edge's
// SourceRef/TargetRef can be asserted by the symbol they denote. When no emitted
// entity owns the ref (a synthetic cross-file `scope:dependency:...:Name` ref),
// the trailing colon-segment is the denoted type name.
func refName(entities []SecondaryEntity, ref string) string {
	for _, e := range entities {
		if e.Ref == ref {
			return e.Name
		}
	}
	if i := lastColon(ref); i >= 0 {
		return ref[i+1:]
	}
	return ""
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// findEdge returns the first relationship of the given kind whose resolved
// FromID(name) and ToID(name) match want, or nil. The TargetRef may name an
// entity present in the result OR a synthetic cross-file ref whose trailing
// segment is the type name; both are handled.
func findDIEdge(r PatternResult, kind, fromName, toName string) *Relationship {
	for i := range r.Relationships {
		rel := &r.Relationships[i]
		if rel.RelationshipType != kind {
			continue
		}
		from := refName(r.Entities, rel.SourceRef)
		to := refName(r.Entities, rel.TargetRef)
		if from == fromName && to == toName {
			return rel
		}
	}
	return nil
}

// ── Spring: constructor injection → INJECTED_INTO ───────────────────────────────

func TestSpringDI_ConstructorInjection_InjectedInto(t *testing.T) {
	src := `
package com.example;

@Service
public class UserService {
    private final UserRepo repo;
    public UserService(UserRepo repo, int maxRetries) {
        this.repo = repo;
    }
}
`
	r := ExtractSpringDIGraph(sbCtxFw(src, "spring_boot"))

	edge := findDIEdge(r, "INJECTED_INTO", "UserRepo", "UserService")
	if edge == nil {
		t.Fatalf("expected UserRepo INJECTED_INTO UserService; rels=%+v", r.Relationships)
	}
	if edge.Properties["via"] != "spring_constructor" {
		t.Errorf("via = %q, want spring_constructor", edge.Properties["via"])
	}
	// Negative: the primitive `int maxRetries` parameter must NOT yield an edge.
	if findDIEdge(r, "INJECTED_INTO", "int", "UserService") != nil {
		t.Error("primitive ctor param int produced a spurious INJECTED_INTO edge")
	}
	if len(r.Relationships) != 1 {
		t.Errorf("expected exactly 1 injection edge (only UserRepo), got %d: %+v",
			len(r.Relationships), r.Relationships)
	}
}

func TestSpringDI_AutowiredFieldInjection_InjectedInto(t *testing.T) {
	src := `
package com.example;

@Repository
public class OrderRepo {
    @Autowired
    @Qualifier("primaryDataSource")
    private DataSource dataSource;
}
`
	r := ExtractSpringDIGraph(sbCtxFw(src, "spring_boot"))

	edge := findDIEdge(r, "INJECTED_INTO", "DataSource", "OrderRepo")
	if edge == nil {
		t.Fatalf("expected DataSource INJECTED_INTO OrderRepo; rels=%+v", r.Relationships)
	}
	if edge.Properties["via"] != "spring_field" {
		t.Errorf("via = %q, want spring_field", edge.Properties["via"])
	}
	if edge.Properties["qualifier"] != "primaryDataSource" {
		t.Errorf("qualifier = %q, want primaryDataSource", edge.Properties["qualifier"])
	}
}

// ── Spring: @Bean method → BINDS (token ← method) ───────────────────────────────

func TestSpringDI_BeanMethod_Binds(t *testing.T) {
	src := `
package com.example;

@Configuration
public class AppConfig {
    @Bean
    @Scope("singleton")
    public DataSource dataSource() {
        return new HikariDataSource();
    }
}
`
	r := ExtractSpringDIGraph(sbCtxFw(src, "spring_boot"))

	edge := findDIEdge(r, "BINDS", "DataSource", "dataSource")
	if edge == nil {
		t.Fatalf("expected DataSource BINDS dataSource (the @Bean method); rels=%+v", r.Relationships)
	}
	if edge.Properties["binding_kind"] != "bean_method" {
		t.Errorf("binding_kind = %q, want bean_method", edge.Properties["binding_kind"])
	}
	if edge.Properties["lifetime"] != "singleton" {
		t.Errorf("lifetime = %q, want singleton", edge.Properties["lifetime"])
	}
}

// ── Guice: bind().to() → BINDS, and @Inject ctor → INJECTED_INTO ────────────────

func TestGuiceDI_BindTo_Binds(t *testing.T) {
	src := `
package com.example;

import com.google.inject.AbstractModule;

public class BillingModule extends AbstractModule {
    @Override
    protected void configure() {
        bind(IFoo.class).to(Foo.class).in(Scopes.SINGLETON);
    }
}
`
	r := ExtractGuiceDI(guiceCtx(src))

	edge := findDIEdge(r, "BINDS", "IFoo", "Foo")
	if edge == nil {
		t.Fatalf("expected IFoo BINDS Foo; rels=%+v", r.Relationships)
	}
	if edge.Properties["binding_kind"] != "bind_to" {
		t.Errorf("binding_kind = %q, want bind_to", edge.Properties["binding_kind"])
	}
	if edge.Properties["lifetime"] != "singleton" {
		t.Errorf("lifetime = %q, want singleton", edge.Properties["lifetime"])
	}
}

func TestGuiceDI_InjectConstructor_InjectedInto(t *testing.T) {
	src := `
package com.example;

import javax.inject.Inject;

public class BillingService {
    private final PaymentGateway gateway;

    @Inject
    public BillingService(PaymentGateway gateway, String currency) {
        this.gateway = gateway;
    }
}
`
	r := ExtractGuiceDI(guiceCtx(src))

	edge := findDIEdge(r, "INJECTED_INTO", "PaymentGateway", "BillingService")
	if edge == nil {
		t.Fatalf("expected PaymentGateway INJECTED_INTO BillingService; rels=%+v", r.Relationships)
	}
	if edge.Properties["via"] != "guice_constructor" {
		t.Errorf("via = %q, want guice_constructor", edge.Properties["via"])
	}
	// Negative: String ctor param must NOT inject.
	if findDIEdge(r, "INJECTED_INTO", "String", "BillingService") != nil {
		t.Error("String ctor param produced a spurious INJECTED_INTO edge")
	}
}

// ── Framework gating ────────────────────────────────────────────────────────────

func TestSpringDIGraph_FrameworkGate(t *testing.T) {
	src := `@Service public class X { public X(Repo r){} }`
	if r := ExtractSpringDIGraph(sbCtxFw(src, "django")); len(r.Relationships) != 0 {
		t.Errorf("non-Spring framework produced edges: %+v", r.Relationships)
	}
}

func TestGuiceDI_SelfGate_NoBindNoModule(t *testing.T) {
	// jakarta_ee candidate, but no bind() and no AbstractModule → emit nothing.
	src := `package com.example; public class Plain { public Plain(){} }`
	if r := ExtractGuiceDI(PatternContext{Source: src, Language: "java", Framework: "jakarta_ee", FilePath: "P.java"}); len(r.Relationships) != 0 {
		t.Errorf("non-Guice file produced edges: %+v", r.Relationships)
	}
}
