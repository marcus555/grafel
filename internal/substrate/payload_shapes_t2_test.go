// Tests for the Phase 2A payload-shape sniffers — T2 languages (#2771).
// One canonical test per language verifying the inline-literal cases the
// drift detector relies on. Mirrors payload_shapes_test.go (T1).
package substrate

import (
	"reflect"
	"testing"
)

func TestPayloadShapesRuby_PermitAndRender(t *testing.T) {
	const src = `
class UsersController < ApplicationController
  def create
    user_params = params.require(:user).permit(:name, :email, :phone)
    render json: { id: 1, name: user_params[:name] }
  end
end
`
	shapes := sniffPayloadShapesRuby(src)
	req := findShape(shapes, "create", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected ruby permit request shape; got %+v", shapes)
	}
	wantReq := []string{"email", "name", "phone"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, wantReq) {
		t.Errorf("ruby permit fields: want %v got %v", wantReq, got)
	}
	resp := findShape(shapes, "create", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected ruby render response shape; got %+v", shapes)
	}
	wantResp := []string{"id", "name"}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, wantResp) {
		t.Errorf("ruby render fields: want %v got %v", wantResp, got)
	}
}

// TestPayloadShapesRuby_SinatraRoute verifies that a Sinatra routing-DSL
// handler (no `def`) gets BOTH its request shape (bare `params[:x]`
// reads) and its response shape (`json({...})` helper) bound to the
// route header `POST /users` (#3951).
func TestPayloadShapesRuby_SinatraRoute(t *testing.T) {
	const src = `
require 'sinatra'
post '/users' do
  name = params[:name]
  email = params[:email]
  json({ id: 1, name: name })
end
`
	shapes := sniffPayloadShapesRuby(src)
	req := findShape(shapes, "POST /users", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected sinatra request shape on POST /users; got %+v", shapes)
	}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, []string{"email", "name"}) {
		t.Errorf("sinatra request fields: want [email name] got %v", got)
	}
	resp := findShape(shapes, "POST /users", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected sinatra response shape on POST /users; got %+v", shapes)
	}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, []string{"id", "name"}) {
		t.Errorf("sinatra response fields: want [id name] got %v", got)
	}
}

// TestPayloadShapesRuby_GrapeRequiresAndExpose verifies a Grape API: the
// `params do; requires/optional` block yields the request shape, and the
// Grape::Entity `expose` declarations yield the response shape — neither
// of which uses `def` (#3951).
func TestPayloadShapesRuby_GrapeRequiresAndExpose(t *testing.T) {
	const src = `
class Users < Grape::API
  params do
    requires :name, type: String
    optional :age, type: Integer
  end
  post '/users' do
    present User.create!(declared(params)), with: Entities::User
  end
end

module Entities
  class User < Grape::Entity
    expose :id
    expose :name
  end
end
`
	shapes := sniffPayloadShapesRuby(src)
	req := findShape(shapes, "params", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected grape requires request shape; got %+v", shapes)
	}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, []string{"age", "name"}) {
		t.Errorf("grape request fields: want [age name] got %v", got)
	}
	resp := findShape(shapes, "User", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected grape expose response shape; got %+v", shapes)
	}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, []string{"id", "name"}) {
		t.Errorf("grape response fields: want [id name] got %v", got)
	}
}

// TestPayloadShapesRuby_HanamiAction verifies a Hanami action: `def call`
// binds the request shape (`params[:x]` reads) and the
// `JSON.generate({...})` response body binds the response shape (#3951).
func TestPayloadShapesRuby_HanamiAction(t *testing.T) {
	const src = `
module Web::Controllers::Users
  class Create
    include Web::Action
    def call(params)
      email = params[:email]
      name = params[:name]
      self.body = JSON.generate({ id: 1, email: email })
    end
  end
end
`
	shapes := sniffPayloadShapesRuby(src)
	req := findShape(shapes, "call", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected hanami request shape; got %+v", shapes)
	}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, []string{"email", "name"}) {
		t.Errorf("hanami request fields: want [email name] got %v", got)
	}
	resp := findShape(shapes, "call", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected hanami response shape; got %+v", shapes)
	}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, []string{"email", "id"}) {
		t.Errorf("hanami response fields: want [email id] got %v", got)
	}
}

// TestPayloadShapesRuby_SiblingNegatives guards the precision boundary:
// a param-less route yields no shape, `to_json` on a bare receiver does
// not fire a response shape, and a Rails `render json:` still yields
// exactly one response shape (no double-count from the json() helper).
func TestPayloadShapesRuby_SiblingNegatives(t *testing.T) {
	if s := sniffPayloadShapesRuby("get '/health' do\n  \"ok\"\nend\n"); len(s) != 0 {
		t.Errorf("param-less route must yield no shapes; got %+v", s)
	}
	for _, s := range sniffPayloadShapesRuby("post '/x' do\n  user.to_json\nend\n") {
		if s.Direction == PayloadDirectionResponse {
			t.Errorf("to_json must not fire a response shape; got %+v", s)
		}
	}
	const rails = "class C < ApplicationController\n  def show\n    render json: { a: 1, b: 2 }\n  end\nend\n"
	resp := 0
	for _, s := range sniffPayloadShapesRuby(rails) {
		if s.Direction == PayloadDirectionResponse {
			resp++
		}
	}
	if resp != 1 {
		t.Errorf("rails render json: want exactly 1 response shape; got %d", resp)
	}
}

func TestPayloadShapesRuby_ConsumerHTTParty(t *testing.T) {
	const src = `
def push
  HTTParty.post("/api/users", body: { name: "x", email: "y" }.to_json)
end
`
	shapes := sniffPayloadShapesRuby(src)
	cs := findShape(shapes, "push", PayloadDirectionRequest, PayloadSideConsumer)
	if cs == nil {
		t.Fatalf("expected ruby consumer shape; got %+v", shapes)
	}
	if cs.EndpointHint != "/api/users" || cs.VerbHint != "POST" {
		t.Errorf("ruby consumer hint: got %q %q", cs.EndpointHint, cs.VerbHint)
	}
	want := []string{"email", "name"}
	if got := sortedNames(cs.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("ruby consumer fields: want %v got %v", want, got)
	}
}

func TestPayloadShapesPHP_FormRequestAndGuzzle(t *testing.T) {
	const src = `
<?php
class CreateUserRequest extends FormRequest {
  public function rules() {
    return [ 'name' => 'required', 'email' => 'email', 'phone' => 'nullable' ];
  }
}
class UserClient {
  public function push() {
    return $this->client->request('POST', '/api/users', [ 'json' => [ 'name' => 'x', 'email' => 'y' ] ]);
  }
}
`
	shapes := sniffPayloadShapesPHP(src)
	req := findShape(shapes, "rules", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected php rules() shape; got %+v", shapes)
	}
	want := []string{"email", "name", "phone"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("php rules fields: want %v got %v", want, got)
	}
	cs := findShape(shapes, "push", PayloadDirectionRequest, PayloadSideConsumer)
	if cs == nil {
		t.Fatalf("expected php guzzle consumer shape; got %+v", shapes)
	}
	if cs.EndpointHint != "/api/users" || cs.VerbHint != "POST" {
		t.Errorf("php consumer hint: got %q %q", cs.EndpointHint, cs.VerbHint)
	}
}

func TestPayloadShapesRust_JsonExtractor(t *testing.T) {
	const src = `
#[derive(Deserialize)]
pub struct CreateUser {
    pub name: String,
    pub email: String,
    pub phone: Option<String>,
}

async fn create_user(Json(body): Json<CreateUser>) -> impl IntoResponse {
    let _ = body;
}
`
	shapes := sniffPayloadShapesRust(src)
	req := findShape(shapes, "create_user", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected rust Json<T> shape; got %+v", shapes)
	}
	want := []string{"email", "name", "phone"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("rust fields: want %v got %v", want, got)
	}
	// phone is Option<String> → Optional=true.
	for _, f := range req.Fields {
		if f.Name == "phone" && !f.Optional {
			t.Errorf("phone should be Optional=true; got %+v", f)
		}
	}
}

func TestPayloadShapesRust_ConsumerReqwest(t *testing.T) {
	const src = `
async fn push() {
    let _ = client.post("/api/users").json(&serde_json::json!({"name": "x", "email": "y"})).send().await;
}
`
	shapes := sniffPayloadShapesRust(src)
	cs := findShape(shapes, "push", PayloadDirectionRequest, PayloadSideConsumer)
	if cs == nil {
		t.Fatalf("expected rust consumer shape; got %+v", shapes)
	}
	if cs.EndpointHint != "/api/users" || cs.VerbHint != "POST" {
		t.Errorf("rust consumer hint: got %q %q", cs.EndpointHint, cs.VerbHint)
	}
}

func TestPayloadShapesCSharp_FromBodyDTO(t *testing.T) {
	const src = `
public class CreateUserDto {
  public string Name { get; set; }
  public string Email { get; set; }
  public int? Age { get; set; }
}
public class UsersController {
  [HttpPost]
  public IActionResult Create([FromBody] CreateUserDto dto) {
    return Ok(new { id = 1, name = dto.Name });
  }
}
`
	shapes := sniffPayloadShapesCSharp(src)
	req := findShape(shapes, "Create", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected csharp [FromBody] shape; got %+v", shapes)
	}
	want := []string{"Age", "Email", "Name"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("csharp DTO fields: want %v got %v", want, got)
	}
	for _, f := range req.Fields {
		if f.Name == "Age" && !f.Optional {
			t.Errorf("Age should be Optional=true (int?); got %+v", f)
		}
	}
	resp := findShape(shapes, "Create", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected csharp anonymous response shape; got %+v", shapes)
	}
}

func TestPayloadShapesKotlin_RequestBodyAndMapOf(t *testing.T) {
	const src = `
data class CreateUser(val name: String, val email: String, val phone: String? = null)

class UsersController {
  @PostMapping("/users")
  fun create(@RequestBody dto: CreateUser): ResponseEntity<Map<String, Any>> {
    return ResponseEntity.ok(mapOf("id" to 1, "name" to dto.name))
  }
}
`
	shapes := sniffPayloadShapesKotlin(src)
	req := findShape(shapes, "create", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected kotlin @RequestBody shape; got %+v", shapes)
	}
	want := []string{"email", "name", "phone"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("kotlin data class fields: want %v got %v", want, got)
	}
	for _, f := range req.Fields {
		if f.Name == "phone" && !f.Optional {
			t.Errorf("phone should be Optional=true; got %+v", f)
		}
	}
	resp := findShape(shapes, "create", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected kotlin mapOf response shape; got %+v", shapes)
	}
	wantR := []string{"id", "name"}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, wantR) {
		t.Errorf("kotlin mapOf fields: want %v got %v", wantR, got)
	}
}

func TestPayloadShapesElixir_DestructureAndJSON(t *testing.T) {
	const src = `
defmodule UsersController do
  def create(conn, %{"name" => name, "email" => email}) do
    json(conn, %{"id" => 1, "name" => name})
  end
end
`
	shapes := sniffPayloadShapesElixir(src)
	req := findShape(shapes, "create", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected elixir destructure shape; got %+v", shapes)
	}
	want := []string{"email", "name"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("elixir destructure fields: want %v got %v", want, got)
	}
	resp := findShape(shapes, "create", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected elixir json response shape; got %+v", shapes)
	}
	wantR := []string{"id", "name"}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, wantR) {
		t.Errorf("elixir json fields: want %v got %v", wantR, got)
	}
}

func TestPayloadShapesScala_CaseClassAndJsonObj(t *testing.T) {
	const src = `
case class CreateUser(name: String, email: String, phone: Option[String])

class UsersController {
  def create(request: Request): Future[Response] = {
    val dto = request.decodeJson[CreateUser]
    Ok(Json.obj("id" -> 1, "name" -> "x"))
  }
}
`
	shapes := sniffPayloadShapesScala(src)
	req := findShape(shapes, "create", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected scala decodeJson shape; got %+v", shapes)
	}
	want := []string{"email", "name", "phone"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("scala case class fields: want %v got %v", want, got)
	}
	for _, f := range req.Fields {
		if f.Name == "phone" && !f.Optional {
			t.Errorf("phone should be Optional=true; got %+v", f)
		}
	}
	resp := findShape(shapes, "create", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected scala Json.obj response shape; got %+v", shapes)
	}
}

func TestPayloadShapesCCPP_BodyAccessAndCurl(t *testing.T) {
	const src = `
void handle_create(http_request request) {
    auto body = request.extract_json().get();
    auto name = body[U("name")].as_string();
    auto email = body[U("email")].as_string();
    json::value result;
    result[U("id")] = json::value::number(1);
    result[U("name")] = json::value::string(name);
}

void push() {
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, "name=x&email=y");
}
`
	shapes := sniffPayloadShapesCCPP(src)
	req := findShape(shapes, "handle_create", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected cpp body access shape; got %+v", shapes)
	}
	want := []string{"email", "name"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("cpp body fields: want %v got %v", want, got)
	}
	resp := findShape(shapes, "handle_create", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected cpp result assign shape; got %+v", shapes)
	}
	wantR := []string{"id", "name"}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, wantR) {
		t.Errorf("cpp result fields: want %v got %v", wantR, got)
	}
	cs := findShape(shapes, "push", PayloadDirectionRequest, PayloadSideConsumer)
	if cs == nil {
		t.Fatalf("expected cpp curl consumer shape; got %+v", shapes)
	}
	wantC := []string{"email", "name"}
	if got := sortedNames(cs.Fields); !reflect.DeepEqual(got, wantC) {
		t.Errorf("cpp curl fields: want %v got %v", wantC, got)
	}
}
