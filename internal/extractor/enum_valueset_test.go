package extractor

import "testing"

// TestEnumEntity_ValuesAndID asserts the shared builder records a "Name=Value"
// pair for every member with a known literal, omits value-less members from the
// `values` property (but keeps them in `members`), and stamps a deterministic
// file-scoped QualifiedName + ID.
func TestEnumEntity_ValuesAndID(t *testing.T) {
	ent, ok := EnumEntity(
		"Status", "go", "go_iota", "pkg/order/status.go", 10, 14,
		[]EnumMember{
			{Name: "Active", Value: "0"},
			{Name: "Pending", Value: "1"},
			{Name: "Computed", Value: ""}, // value-less: kept in members, not values
		},
	)
	if !ok {
		t.Fatal("EnumEntity returned ok=false for a valid enum")
	}
	if ent.Kind != "SCOPE.Enum" {
		t.Fatalf("Kind = %q, want SCOPE.Enum", ent.Kind)
	}
	if got := ent.Properties["members"]; got != "Active, Pending, Computed" {
		t.Fatalf("members = %q", got)
	}
	if got := ent.Properties["values"]; got != "Active=0, Pending=1" {
		t.Fatalf("values = %q, want %q", got, "Active=0, Pending=1")
	}
	if got := ent.Properties["member_count"]; got != "3" {
		t.Fatalf("member_count = %q, want 3", got)
	}
	wantQN := "scope:enum:pkg/order/status.go:Status"
	if ent.QualifiedName != wantQN {
		t.Fatalf("QualifiedName = %q, want %q", ent.QualifiedName, wantQN)
	}
	if ent.ID == "" || ent.ID != ent.ComputeID() {
		t.Fatalf("ID not precomputed deterministically: %q", ent.ID)
	}
}

// TestEnumEntity_DistinctSameNameDifferentFile asserts two same-named enums in
// different files get DISTINCT IDs (file-scoped, unlike ExceptionType).
func TestEnumEntity_DistinctSameNameDifferentFile(t *testing.T) {
	a, _ := EnumEntity("Status", "python", "python_enum", "a.py", 1, 2,
		[]EnumMember{{Name: "X", Value: "1"}})
	b, _ := EnumEntity("Status", "python", "python_enum", "b.py", 1, 2,
		[]EnumMember{{Name: "X", Value: "1"}})
	if a.ID == b.ID {
		t.Fatalf("same-named enums in different files collided: %q", a.ID)
	}
}

// TestEnumEntity_EmptyRejected asserts an empty name or zero members yields no
// node (honest-partial: a dynamic/unnamed enum is not fabricated).
func TestEnumEntity_EmptyRejected(t *testing.T) {
	if _, ok := EnumEntity("", "ts", "ts_enum", "f.ts", 1, 1,
		[]EnumMember{{Name: "A"}}); ok {
		t.Fatal("empty name should yield ok=false")
	}
	if _, ok := EnumEntity("Role", "ts", "ts_literal_union", "f.ts", 1, 1, nil); ok {
		t.Fatal("zero members should yield ok=false")
	}
}

// TestStripLiteralQuotes covers the quote-normalisation used to record TS
// 'active' and Python "active" both as the literal `active`.
func TestStripLiteralQuotes(t *testing.T) {
	cases := map[string]string{
		`'active'`:   "active",
		`"active"`:   "active",
		"`active`":   "active",
		`42`:         "42",
		`UPPER`:      "UPPER",
		`'unmatched`: "'unmatched",
	}
	for in, want := range cases {
		if got := StripLiteralQuotes(in); got != want {
			t.Errorf("StripLiteralQuotes(%q) = %q, want %q", in, got, want)
		}
	}
}
