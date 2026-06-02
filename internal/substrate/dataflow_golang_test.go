package substrate

import "testing"

// findGoFlow returns the first flow whose sink name contains sinkSub, or nil.
func findGoFlow(flows []DataFlow, sinkSub string) *DataFlow {
	for i := range flows {
		if containsStr(flows[i].SinkName, sinkSub) {
			return &flows[i]
		}
	}
	return nil
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// --- POSITIVE: gin Query → gorm Create, field resolved -----------------

func TestGoDataFlow_GinQueryToDBCreate(t *testing.T) {
	src := `package h
func CreateItem(c *gin.Context) {
	q := c.Query("q")
	db.Create(&Item{Name: q})
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "db.Create")
	if f == nil {
		t.Fatalf("expected a db.Create flow, got %+v", flows)
	}
	if f.SinkKind != DataFlowSinkDBWrite {
		t.Errorf("sink kind = %q, want db_write", f.SinkKind)
	}
	if f.Function != "CreateItem" {
		t.Errorf("origin = %q, want CreateItem", f.Function)
	}
	if f.SourceField != "q" {
		t.Errorf("source field = %q, want q", f.SourceField)
	}
}

// --- POSITIVE: gin ShouldBindJSON(&dto) → db.Save(&User{Email:dto.Email}) ---

func TestGoDataFlow_ShouldBindJSONToDBSave(t *testing.T) {
	src := `package h
func UpdateUser(c *gin.Context) {
	var dto UserDTO
	c.ShouldBindJSON(&dto)
	db.Save(&User{Email: dto.Email})
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "db.Save")
	if f == nil {
		t.Fatalf("expected a db.Save flow, got %+v", flows)
	}
	if f.SinkKind != DataFlowSinkDBWrite {
		t.Errorf("sink kind = %q, want db_write", f.SinkKind)
	}
	if f.SourceField != "Email" {
		t.Errorf("source field = %q, want Email (lifted from dto.Email)", f.SourceField)
	}
}

// --- POSITIVE: net/http r.FormValue → w.Write (response flow) ----------

func TestGoDataFlow_NetHTTPFormValueToResponse(t *testing.T) {
	src := `package h
func handler(w http.ResponseWriter, r *http.Request) {
	x := r.FormValue("x")
	w.Write([]byte(x))
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "w.Write")
	if f == nil {
		t.Fatalf("expected a w.Write response flow, got %+v", flows)
	}
	if f.SinkKind != DataFlowSinkResponse {
		t.Errorf("sink kind = %q, want response", f.SinkKind)
	}
	if f.SourceField != "x" {
		t.Errorf("source field = %q, want x", f.SourceField)
	}
}

// --- POSITIVE: echo QueryParam → c.JSON response ----------------------

func TestGoDataFlow_EchoQueryParamToResponse(t *testing.T) {
	src := `package h
func search(c echo.Context) error {
	term := c.QueryParam("term")
	return c.JSON(200, term)
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "c.JSON")
	if f == nil {
		t.Fatalf("expected a c.JSON flow, got %+v", flows)
	}
	if f.SinkKind != DataFlowSinkResponse {
		t.Errorf("sink kind = %q, want response", f.SinkKind)
	}
	if f.SourceField != "term" {
		t.Errorf("source field = %q, want term", f.SourceField)
	}
}

// --- POSITIVE: chi URLParam → db.Exec (db_write) ----------------------

func TestGoDataFlow_ChiURLParamToExec(t *testing.T) {
	src := `package h
func del(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	db.Exec("DELETE FROM t WHERE id = " + id)
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "db.Exec")
	if f == nil {
		t.Fatalf("expected a db.Exec flow, got %+v", flows)
	}
	if f.SinkKind != DataFlowSinkDBWrite {
		t.Errorf("sink kind = %q, want db_write", f.SinkKind)
	}
	if f.SourceField != "id" {
		t.Errorf("source field = %q, want id", f.SourceField)
	}
}

// --- POSITIVE: net/http json.Decode(&dto) → http.Post (http_call) -----

func TestGoDataFlow_JSONDecodeToOutboundHTTP(t *testing.T) {
	src := `package h
func proxy(w http.ResponseWriter, r *http.Request) {
	var dto Payload
	json.NewDecoder(r.Body).Decode(&dto)
	http.Post("https://api", "application/json", dto.Body)
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "http.Post")
	if f == nil {
		t.Fatalf("expected an http.Post flow, got %+v", flows)
	}
	if f.SinkKind != DataFlowSinkHTTPCall {
		t.Errorf("sink kind = %q, want http_call", f.SinkKind)
	}
	if f.SourceField != "Body" {
		t.Errorf("source field = %q, want Body (lifted from dto.Body)", f.SourceField)
	}
}

// --- POSITIVE: one local hop, handler→helper→sink ---------------------

func TestGoDataFlow_OneLocalHop(t *testing.T) {
	src := `package h
func handler(c *gin.Context) {
	q := c.Query("name")
	persist(q)
}
func persist(val string) {
	db.Create(&Item{Name: val})
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "db.Create")
	if f == nil {
		t.Fatalf("expected a hopped db.Create flow, got %+v", flows)
	}
	if f.Function != "handler" {
		t.Errorf("origin = %q, want handler (flow attributed to originating handler)", f.Function)
	}
	if len(f.HopPath) != 1 || f.HopPath[0] != "persist" {
		t.Errorf("hop path = %v, want [persist]", f.HopPath)
	}
	if f.SourceField != "name" {
		t.Errorf("source field = %q, want name", f.SourceField)
	}
}

// --- NEGATIVE: static value → no flow ---------------------------------

func TestGoDataFlow_StaticValueNoFlow(t *testing.T) {
	src := `package h
func CreateItem(c *gin.Context) {
	name := "constant"
	db.Create(&Item{Name: name})
}`
	flows := sniffDataFlowGo(src)
	if f := findGoFlow(flows, "db.Create"); f != nil {
		t.Fatalf("expected no flow for a static value, got %+v", f)
	}
}

// --- NEGATIVE: non-request var → no source ----------------------------

func TestGoDataFlow_NonRequestVarNoSource(t *testing.T) {
	src := `package h
func CreateItem(c *gin.Context) {
	val := computeSomething()
	db.Create(&Item{Name: val})
}`
	flows := sniffDataFlowGo(src)
	if f := findGoFlow(flows, "db.Create"); f != nil {
		t.Fatalf("expected no flow for a non-request var, got %+v", f)
	}
}

// --- NEGATIVE: dynamic key → flow but NO field ------------------------

func TestGoDataFlow_DynamicKeyNoField(t *testing.T) {
	src := `package h
func CreateItem(c *gin.Context) {
	k := "x"
	q := c.Query(k)
	db.Create(&Item{Name: q})
}`
	flows := sniffDataFlowGo(src)
	f := findGoFlow(flows, "db.Create")
	if f == nil {
		t.Fatalf("expected a flow (the value IS request-derived), got %+v", flows)
	}
	if f.SourceField != "" {
		t.Errorf("source field = %q, want empty (dynamic key — honest-partial)", f.SourceField)
	}
}

// --- NEGATIVE: reassignment breaks the chain --------------------------

func TestGoDataFlow_ReassignmentBreaksChain(t *testing.T) {
	src := `package h
func CreateItem(c *gin.Context) {
	q := c.Query("q")
	q = "safe"
	db.Create(&Item{Name: q})
}`
	flows := sniffDataFlowGo(src)
	if f := findGoFlow(flows, "db.Create"); f != nil {
		t.Fatalf("expected no flow after reassignment to a constant, got %+v", f)
	}
}

// --- PARAM EXPANSION: shared trailing type ----------------------------

func TestGoExpandParams_SharedType(t *testing.T) {
	got := goExpandParams("a, b string, c int")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGoExpandParams_TypeOnlyUnnamed(t *testing.T) {
	got := goExpandParams("string, int")
	for i, n := range got {
		if n != "" {
			t.Errorf("slot %d = %q, want empty (unnamed type-only param)", i, n)
		}
	}
}
