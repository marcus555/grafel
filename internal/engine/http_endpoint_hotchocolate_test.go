package engine

import "testing"

// TestHotChocolate_MarkerRootTypes asserts that HotChocolate's annotation-based
// root types ([QueryType] / [MutationType] / [SubscriptionType]) map each
// public resolver method to the canonical cross-stack GraphQL endpoint shape
// `http:GRAPHQL:/graphql/<Root>/<field>` — Get-prefix stripped + lowerCamel,
// IDENTICAL to gqlgen (Go) / Strawberry (Python) / Apollo (JS) — with a
// HANDLES edge (source_handler) to the C# resolver method.
func TestHotChocolate_MarkerRootTypes(t *testing.T) {
	src := `
using HotChocolate;
using HotChocolate.Types;

namespace Demo.GraphQL;

[QueryType]
public class Query
{
    public User GetUser(int id) => _repo.Find(id);
    public IEnumerable<User> GetUsers() => _repo.All();
    private User Internal() => null;            // non-public → NOT a field
}

[MutationType]
public class Mutation
{
    public User CreateUser(string name) => _repo.Add(name);
}

[SubscriptionType]
public class Subscription
{
    public User OnUserAdded() => null;
}
`
	ids, res := runDetect(t, "csharp", "GraphQL/Types.cs", src)

	// EXACT shape parity with gqlgen/JS: `GetUser` → field `user`,
	// `GetUsers` → `users`, `CreateUser` → `createUser` (no Get strip),
	// `OnUserAdded` → `onUserAdded`.
	want := []string{
		"http:GRAPHQL:/graphql/Query/user",
		"http:GRAPHQL:/graphql/Query/users",
		"http:GRAPHQL:/graphql/Mutation/createUser",
		"http:GRAPHQL:/graphql/Subscription/onUserAdded",
	}
	requireContains(t, ids, want, "hotchocolate-marker")

	// The private method must NOT become an endpoint.
	for _, bad := range []string{
		"http:GRAPHQL:/graphql/Query/internal",
	} {
		for _, id := range ids {
			if id == bad {
				t.Errorf("hotchocolate: non-public method wrongly emitted as %s", bad)
			}
		}
	}

	// HANDLES edge: the `user` endpoint carries source_handler pointing at the
	// C# resolver method Query.GetUser (SCOPE.Operation:<Class>.<Method>), the
	// same convention the C# extractor's buildOperation produces — which the
	// resolver consumes to materialise the endpoint → method HANDLES edge.
	assertHandler(t, res, "http:GRAPHQL:/graphql/Query/user",
		"hotchocolate", "SCOPE.Operation:Query.GetUser")
	assertHandler(t, res, "http:GRAPHQL:/graphql/Mutation/createUser",
		"hotchocolate", "SCOPE.Operation:Mutation.CreateUser")
}

// TestHotChocolate_ExtendObjectType asserts that an [ExtendObjectType(typeof(
// Query))] type extension contributes its public methods as additional fields
// on the Query root, in the same canonical shape.
func TestHotChocolate_ExtendObjectType(t *testing.T) {
	src := `
using HotChocolate.Types;

[ExtendObjectType(typeof(Query))]
public class UserQueries
{
    public User GetUserByEmail(string email) => _repo.ByEmail(email);
}

[ExtendObjectType(OperationType.Mutation)]
public class UserMutations
{
    public bool DeleteUser(int id) => _repo.Remove(id);
}
`
	ids, res := runDetect(t, "csharp", "GraphQL/Extensions.cs", src)
	want := []string{
		"http:GRAPHQL:/graphql/Query/userByEmail",
		"http:GRAPHQL:/graphql/Mutation/deleteUser",
	}
	requireContains(t, ids, want, "hotchocolate-extend")

	assertHandler(t, res, "http:GRAPHQL:/graphql/Query/userByEmail",
		"hotchocolate", "SCOPE.Operation:UserQueries.GetUserByEmail")
}

// TestHotChocolate_FluentRegistration asserts that a class registered fluently
// via `.AddQueryType<Query>()` in the same file (Program.cs-style) — with NO
// marker attribute on the class — still has its resolver methods mapped.
func TestHotChocolate_FluentRegistration(t *testing.T) {
	src := `
using HotChocolate;

var builder = WebApplication.CreateBuilder(args);
builder.Services
    .AddGraphQLServer()
    .AddQueryType<Query>()
    .AddMutationType<Mutation>();

public class Query
{
    public Product GetProduct(int id) => _repo.Find(id);
}

public class Mutation
{
    public Product AddProduct(string sku) => _repo.Add(sku);
}
`
	ids, res := runDetect(t, "csharp", "Program.cs", src)
	want := []string{
		"http:GRAPHQL:/graphql/Query/product",
		"http:GRAPHQL:/graphql/Mutation/addProduct",
	}
	requireContains(t, ids, want, "hotchocolate-fluent")

	assertHandler(t, res, "http:GRAPHQL:/graphql/Query/product",
		"hotchocolate", "SCOPE.Operation:Query.GetProduct")
}

// TestHotChocolate_NoSignalNoOp asserts the synthesizer is a no-op on a plain
// ASP.NET Core C# file with no HotChocolate signal — proving the file-signal
// gate prevents false GraphQL endpoints on the rest of the C# index.
func TestHotChocolate_NoSignalNoOp(t *testing.T) {
	src := `
using Microsoft.AspNetCore.Mvc;

[ApiController]
[Route("/api/[controller]")]
public class WidgetsController : ControllerBase
{
    [HttpGet]
    public IActionResult GetWidget() => Ok();
}
`
	ids, _ := runDetect(t, "csharp", "WidgetsController.cs", src)
	for _, id := range ids {
		if len(id) >= 13 && id[:13] == "http:GRAPHQL:" {
			t.Errorf("hotchocolate: emitted GraphQL endpoint %q on a non-HotChocolate file", id)
		}
	}
}

// assertHandler asserts the synthetic endpoint with id `wantID` exists, is
// tagged framework=`wantFW`, and carries source_handler=`wantHandler`.
func assertHandler(t *testing.T, res *DetectResult, wantID, wantFW, wantHandler string) {
	t.Helper()
	for _, e := range res.Entities {
		if e.ID != wantID {
			continue
		}
		if e.Properties["framework"] != wantFW {
			t.Errorf("%s: framework=%q, want %q", wantID, e.Properties["framework"], wantFW)
		}
		if e.Properties["source_handler"] != wantHandler {
			t.Errorf("%s: source_handler=%q, want %q", wantID, e.Properties["source_handler"], wantHandler)
		}
		return
	}
	t.Errorf("missing synthetic endpoint %q", wantID)
}
