package substrate

import (
	"reflect"
	"testing"
)

// TestPayloadShapesHotChocolate_TypedResolver asserts that a HotChocolate
// resolver method contributes a producer REQUEST shape from its typed argument
// list (DTO-typed args expanded to the DTO's fields) and a producer RESPONSE
// shape from its typed return type (#3961).
func TestPayloadShapesHotChocolate_TypedResolver(t *testing.T) {
	const src = `
using HotChocolate;
using HotChocolate.Types;

public class GetUserInput {
  public int Id { get; set; }
  public string Tenant { get; set; }
}
public class User {
  public int Id { get; set; }
  public string Name { get; set; }
  public string Email { get; set; }
}

[QueryType]
public class Query
{
    public User GetUser(GetUserInput input) => _repo.Find(input.Id);
}
`
	shapes := sniffPayloadShapesCSharp(src)

	req := findShape(shapes, "GetUser", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected HotChocolate request shape for GetUser; got %+v", shapes)
	}
	wantReq := []string{"Id", "Tenant"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, wantReq) {
		t.Errorf("HC request fields: want %v got %v", wantReq, got)
	}

	resp := findShape(shapes, "GetUser", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected HotChocolate response shape for GetUser; got %+v", shapes)
	}
	wantResp := []string{"Email", "Id", "Name"}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, wantResp) {
		t.Errorf("HC response fields: want %v got %v", wantResp, got)
	}
}

// TestPayloadShapesHotChocolate_ScalarArgsAndWrappedReturn asserts scalar
// resolver arguments contribute one field each (ambient framework params
// skipped) and a wrapped return type (Task<IEnumerable<User>>) is unwrapped to
// the leaf DTO for the response shape.
func TestPayloadShapesHotChocolate_ScalarArgsAndWrappedReturn(t *testing.T) {
	const src = `
using HotChocolate;

public class User {
  public int Id { get; set; }
  public string Name { get; set; }
}

[QueryType]
public class Query
{
    public async Task<IEnumerable<User>> GetUsers(int teamId, string filter, CancellationToken ct)
        => await _repo.ByTeam(teamId);
}
`
	shapes := sniffPayloadShapesCSharp(src)

	req := findShape(shapes, "GetUsers", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected HC scalar request shape; got %+v", shapes)
	}
	// CancellationToken is an ambient framework param → excluded.
	wantReq := []string{"filter", "teamId"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, wantReq) {
		t.Errorf("HC scalar request fields: want %v got %v", wantReq, got)
	}

	resp := findShape(shapes, "GetUsers", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected HC wrapped-return response shape; got %+v", shapes)
	}
	wantResp := []string{"Id", "Name"}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, wantResp) {
		t.Errorf("HC wrapped response fields: want %v got %v", wantResp, got)
	}
}

// TestPayloadShapesHotChocolate_NoSignalNoEmit asserts the HotChocolate shape
// sniffer is gated on a HotChocolate file-signal — a plain C# class with the
// same method shape but NO HotChocolate import produces no HC resolver shapes.
func TestPayloadShapesHotChocolate_NoSignalNoEmit(t *testing.T) {
	const src = `
public class User {
  public int Id { get; set; }
}
public class Service
{
    public User Lookup(int id) => _repo.Find(id);
}
`
	shapes := sniffPayloadShapesCSharp(src)
	if s := findShape(shapes, "Lookup", PayloadDirectionResponse, PayloadSideProducer); s != nil {
		t.Errorf("HC sniffer fired on a non-HotChocolate file: %+v", s)
	}
}
