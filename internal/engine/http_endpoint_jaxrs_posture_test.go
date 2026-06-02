package engine

import "testing"

// Value-asserting posture tests for the JAX-RS / Jakarta-family JVM frameworks
// (#3857, epic #3854). Each test asserts the SPECIFIC posture prop on the
// SPECIFIC endpoint for the named framework — not len>0. They reuse the
// deprecProps / mustEndpoint harness (the JAX-RS posture resolvers run in the
// same `java` synthesis-tail branch as the Spring resolvers).

// ---------------------------------------------------------------------------
// Response codes — JAX-RS / Jakarta REST (Quarkus, Jakarta EE, Helidon, …)
// ---------------------------------------------------------------------------

func TestResponseCodes_JAXRS_ResponseStatusNumericAndBuilder(t *testing.T) {
	// jakarta.ws.rs Response.status(404) + Response.ok() builder → {200,404}.
	src := `
@Path("/api/v1/items")
public class ItemResource {

    @GET
    @Path("/{id}")
    public Response get(@PathParam("id") Long id) {
        if (id == null) {
            return Response.status(404).build();
        }
        return Response.ok(item).build();
    }
}
`
	eps := deprecProps(t, "java", "src/ItemResource.java", src)
	e := mustEndpoint(t, eps, "GET /api/v1/items/{id}")
	if got := e.Properties["response_codes"]; got != "200,404" {
		t.Fatalf("response_codes=%q want 200,404 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "200" {
		t.Fatalf("success_code=%q want 200", got)
	}
}

func TestResponseCodes_JAXRS_StatusEnumCreated(t *testing.T) {
	// Response.status(Response.Status.CREATED) → 201.
	src := `
@Path("/widgets")
public class WidgetResource {

    @POST
    public Response create(Widget w) {
        return Response.status(Response.Status.CREATED).entity(w).build();
    }
}
`
	eps := deprecProps(t, "java", "src/WidgetResource.java", src)
	e := mustEndpoint(t, eps, "POST /widgets")
	if got := e.Properties["response_codes"]; got != "201" {
		t.Fatalf("response_codes=%q want 201 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "201" {
		t.Fatalf("success_code=%q want 201", got)
	}
}

func TestResponseCodes_JAXRS_WebApplicationExceptionNumeric(t *testing.T) {
	// throw new WebApplicationException(409) → 409 (no 2xx → no success_code).
	src := `
@Path("/accounts")
public class AccountResource {

    @POST
    public Response create(Account a) {
        if (exists) {
            throw new WebApplicationException(409);
        }
        return Response.ok().build();
    }
}
`
	eps := deprecProps(t, "java", "src/AccountResource.java", src)
	e := mustEndpoint(t, eps, "POST /accounts")
	if got := e.Properties["response_codes"]; got != "200,409" {
		t.Fatalf("response_codes=%q want 200,409 (props: %v)", got, e.Properties)
	}
}

func TestResponseCodes_JAXRS_TypedException(t *testing.T) {
	// throw new NotFoundException() → 404 (JAX-RS typed exception → code).
	src := `
@Path("/things")
public class ThingResource {

    @GET
    @Path("/{id}")
    public Thing get(@PathParam("id") Long id) {
        if (missing) {
            throw new NotFoundException();
        }
        return thing;
    }
}
`
	eps := deprecProps(t, "java", "src/ThingResource.java", src)
	e := mustEndpoint(t, eps, "GET /things/{id}")
	if got := e.Properties["response_codes"]; got != "404" {
		t.Fatalf("response_codes=%q want 404 (props: %v)", got, e.Properties)
	}
}

// ---------------------------------------------------------------------------
// Response codes — Micronaut
//
// Micronaut HTTP endpoints (@Controller + @Get/@Post) are not synthesised as
// http_endpoint_definition entities (they flow through the custom Micronaut
// extractor as SCOPE.Operation handlers), so the full-pipeline harness has no
// endpoint to stamp. The Micronaut RESOLVER below is correct and ready and is
// asserted directly; it will stamp posture the moment Micronaut endpoint
// synthesis lands. Honest-partial per framework.
// ---------------------------------------------------------------------------

func TestResponseCodes_Micronaut_HttpResponseBuilders_Resolver(t *testing.T) {
	// Micronaut HttpResponse.created(...) + .notFound() → {201,404}.
	body := `
        if (b == null) {
            return HttpResponse.notFound();
        }
        return HttpResponse.created(b);
`
	v := jaxrsResponseCodes("", body)
	if !v.codes[201] || !v.codes[404] {
		t.Fatalf("codes=%v want {201,404}", v.codes)
	}
	if len(v.codes) != 2 {
		t.Fatalf("codes=%v want exactly {201,404}", v.codes)
	}
}

func TestResponseCodes_Micronaut_StatusAnnotation_Resolver(t *testing.T) {
	// Micronaut @Status(HttpStatus.CREATED) → 201.
	region := `
    @Post
    @Status(HttpStatus.CREATED)
    public Note create(@Body Note n) {
`
	v := jaxrsResponseCodes(region, "")
	if !v.codes[201] || len(v.codes) != 1 {
		t.Fatalf("codes=%v want {201}", v.codes)
	}
}

// ---------------------------------------------------------------------------
// Pagination — JAX-RS @QueryParam pair + Micronaut Pageable / Page<…>
// ---------------------------------------------------------------------------

func TestPagination_JAXRS_LimitOffsetQueryParams(t *testing.T) {
	// @QueryParam("limit") + @QueryParam("offset") → offset style.
	src := `
@Path("/api/v1/products")
public class ProductResource {

    @GET
    public List<Product> list(@QueryParam("limit") int limit, @QueryParam("offset") int offset) {
        return repo.find(limit, offset);
    }
}
`
	eps := deprecProps(t, "java", "src/ProductResource.java", src)
	e := mustEndpoint(t, eps, "GET /api/v1/products")
	if got := e.Properties["paginated"]; got != "true" {
		t.Fatalf("paginated=%q want true (props: %v)", got, e.Properties)
	}
	if got := e.Properties["pagination_style"]; got != "offset" {
		t.Fatalf("pagination_style=%q want offset", got)
	}
	if got := e.Properties["pagination_params"]; got != "limit,offset" {
		t.Fatalf("pagination_params=%q want limit,offset", got)
	}
}

func TestPagination_Micronaut_Pageable_Resolver(t *testing.T) {
	// Micronaut Data Pageable param / Page<…> return → page style. Asserted at
	// the resolver level (see the Micronaut response-codes note above: Micronaut
	// endpoints are not yet synthesised as http_endpoint_definition entities).
	region := `    public Page<User> list(@QueryValue Pageable pageable) {`
	v, ok := jaxrsPaginationVerdict(region)
	if !ok || !v.paginated {
		t.Fatalf("verdict=%+v ok=%v want paginated", v, ok)
	}
	if v.style != "page" {
		t.Fatalf("style=%q want page", v.style)
	}
}

func TestPagination_JAXRS_LoneLimitNotStamped(t *testing.T) {
	// Negative: a lone @QueryParam("limit") with no offset/page/cursor companion
	// is ambiguous → NOT stamped (honest-partial).
	src := `
@Path("/feeds")
public class FeedResource {

    @GET
    public List<Feed> list(@QueryParam("limit") int limit) {
        return repo.recent(limit);
    }
}
`
	eps := deprecProps(t, "java", "src/FeedResource.java", src)
	e := mustEndpoint(t, eps, "GET /feeds")
	if got := e.Properties["paginated"]; got != "" {
		t.Fatalf("paginated=%q want empty (lone limit is ambiguous)", got)
	}
}

// ---------------------------------------------------------------------------
// Deprecation + api_version — JAX-RS (shared Java @Deprecated + path-derived)
// ---------------------------------------------------------------------------

func TestDeprecation_JAXRS_DeprecatedMethod(t *testing.T) {
	// @Deprecated on a JAX-RS @GET method → deprecated=true.
	src := `
@Path("/legacy")
public class LegacyResource {

    @Deprecated
    @GET
    @Path("/old")
    public Response old() {
        return Response.ok().build();
    }
}
`
	eps := deprecProps(t, "java", "src/LegacyResource.java", src)
	e := mustEndpoint(t, eps, "GET /legacy/old")
	if got := e.Properties["deprecated"]; got != "true" {
		t.Fatalf("deprecated=%q want true (props: %v)", got, e.Properties)
	}
}

func TestDeprecation_JAXRS_NonRouteDeprecatedUnaffected(t *testing.T) {
	// Negative: a @Deprecated on a NON-route helper method must not mark the
	// route endpoint as deprecated.
	src := `
@Path("/safe")
public class SafeResource {

    @GET
    public Response active() {
        return Response.ok(helper()).build();
    }

    @Deprecated
    private String helper() {
        return "x";
    }
}
`
	eps := deprecProps(t, "java", "src/SafeResource.java", src)
	e := mustEndpoint(t, eps, "GET /safe")
	if got := e.Properties["deprecated"]; got != "" {
		t.Fatalf("deprecated=%q want empty (non-route @Deprecated must not leak)", got)
	}
}

func TestAPIVersion_JAXRS_PathV2(t *testing.T) {
	// JAX-RS class @Path("/api/v2/...") → api_version=2 on the endpoint.
	src := `
@Path("/api/v2/orders")
public class OrderResource {

    @GET
    public List<Order> list() {
        return repo.all();
    }
}
`
	eps := deprecProps(t, "java", "src/OrderResource.java", src)
	e := mustEndpoint(t, eps, "GET /api/v2/orders")
	if got := e.Properties["api_version"]; got != "2" {
		t.Fatalf("api_version=%q want 2 (props: %v)", got, e.Properties)
	}
}
