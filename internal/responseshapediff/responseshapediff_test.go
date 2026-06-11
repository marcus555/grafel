package responseshapediff

import (
	"reflect"
	"testing"
)

// resolvedBranch is a test helper: a branch with a field set is resolved.
func resolvedBranch(status int, fields ...Field) Branch {
	return Branch{Status: status, Fields: fields, Resolved: true}
}

func f(name string) Field { return Field{Name: name} }

// TestDiff_StatusSetDrift_409MissingInV3 is the headline #4424 fixture: the
// oracle has a 409 conflict branch carrying an existing_user field that v3 lacks
// entirely. The diff must report status_set_drift for the 409, not silently
// equate the pair.
func TestDiff_StatusSetDrift_409MissingInV3(t *testing.T) {
	oracle := Contract{
		ResolvedAny: true,
		Branches: []Branch{
			resolvedBranch(200, f("id"), f("email")),
			resolvedBranch(409, f("existing_user")),
		},
	}
	v3 := Contract{
		ResolvedAny: true,
		Branches: []Branch{
			resolvedBranch(200, f("id"), f("email")),
		},
	}
	res := Diff(oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift", res.Verdict)
	}
	if len(res.StatusSetDrift) != 1 || res.StatusSetDrift[0].Status != 409 || res.StatusSetDrift[0].Side != "oracle" {
		t.Fatalf("status_set_drift = %+v, want one {409, oracle}", res.StatusSetDrift)
	}
	// 200 matched on both sides → no field_drift.
	if len(res.FieldDrift) != 0 {
		t.Fatalf("field_drift = %+v, want none (200 matches)", res.FieldDrift)
	}
}

// TestDiff_Equivalent_Reconciled is the reconciled pair: identical branch set,
// identical field sets (modulo casing) → equivalent.
func TestDiff_Equivalent_Reconciled(t *testing.T) {
	oracle := Contract{
		ResolvedAny: true,
		Branches: []Branch{
			resolvedBranch(200, f("user_id"), f("email_address")),
			resolvedBranch(400, f("detail")),
		},
	}
	v3 := Contract{
		ResolvedAny: true,
		Branches: []Branch{
			// camelCase — must align with the oracle's snake_case via CanonicalKey.
			resolvedBranch(200, f("userId"), f("emailAddress")),
			resolvedBranch(400, f("detail")),
		},
	}
	res := Diff(oracle, v3)
	if res.Verdict != VerdictEquivalent {
		t.Fatalf("verdict = %q, want equivalent (got drift=%+v / status=%+v)",
			res.Verdict, res.FieldDrift, res.StatusSetDrift)
	}
}

// TestDiff_Unresolved_OneSideBlind: v3 resolved no branch at all → unresolved,
// NOT a false drift/equivalent.
func TestDiff_Unresolved_OneSideBlind(t *testing.T) {
	oracle := Contract{
		ResolvedAny: true,
		Branches:    []Branch{resolvedBranch(200, f("id"))},
	}
	v3 := Contract{ResolvedAny: false}
	res := Diff(oracle, v3)
	if res.Verdict != VerdictUnresolved {
		t.Fatalf("verdict = %q, want unresolved", res.Verdict)
	}
	if res.Note == "" {
		t.Fatalf("unresolved result should carry an explanatory note")
	}
}

// TestDiff_FieldDrift_PerStatus: shared 200 status, but v3 dropped a field and
// added one. Reported as per-status field_drift, casing-tolerant.
func TestDiff_FieldDrift_PerStatus(t *testing.T) {
	oracle := Contract{
		ResolvedAny: true,
		Branches:    []Branch{resolvedBranch(200, f("id"), f("created_at"), f("status"))},
	}
	v3 := Contract{
		ResolvedAny: true,
		Branches:    []Branch{resolvedBranch(200, f("id"), f("createdAt"), f("extra"))},
	}
	res := Diff(oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift", res.Verdict)
	}
	if len(res.FieldDrift) != 1 {
		t.Fatalf("field_drift = %+v, want one record", res.FieldDrift)
	}
	fd := res.FieldDrift[0]
	if fd.Status != 200 {
		t.Fatalf("field_drift status = %d, want 200", fd.Status)
	}
	// created_at ↔ createdAt fold → only the truly-different fields drift.
	if !reflect.DeepEqual(fd.OnlyInOracle, []string{"status"}) {
		t.Fatalf("only_in_oracle = %v, want [status]", fd.OnlyInOracle)
	}
	if !reflect.DeepEqual(fd.OnlyInV3, []string{"extra"}) {
		t.Fatalf("only_in_v3 = %v, want [extra]", fd.OnlyInV3)
	}
}

// TestDiff_TypeAndOptionalityMismatch: same field key on both sides but type and
// optionality differ.
func TestDiff_TypeAndOptionalityMismatch(t *testing.T) {
	oracle := Contract{
		ResolvedAny: true,
		Branches: []Branch{{Status: 200, Resolved: true, Fields: []Field{
			{Name: "id", Type: "int"},
			{Name: "name", Type: "string", Optional: false},
		}}},
	}
	v3 := Contract{
		ResolvedAny: true,
		Branches: []Branch{{Status: 200, Resolved: true, Fields: []Field{
			{Name: "id", Type: "string"},                // type mismatch
			{Name: "name", Type: "str", Optional: true}, // optionality mismatch (str==string folds)
		}}},
	}
	res := Diff(oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift", res.Verdict)
	}
	fd := res.FieldDrift[0]
	if len(fd.TypeMismatches) != 1 || fd.TypeMismatches[0].Field != "id" {
		t.Fatalf("type_mismatches = %+v, want one for id", fd.TypeMismatches)
	}
	if len(fd.OptionalityMismatch) != 1 || fd.OptionalityMismatch[0].Field != "name" {
		t.Fatalf("optionality_mismatches = %+v, want one for name", fd.OptionalityMismatch)
	}
}

// TestDiff_SharedStatus_OneBranchUnresolved: a status fires on both sides but
// one carried no field set → honest-partial (Unresolved:true), not a phantom
// all-fields-missing drift.
func TestDiff_SharedStatus_OneBranchUnresolved(t *testing.T) {
	oracle := Contract{
		ResolvedAny: true,
		Branches:    []Branch{resolvedBranch(200, f("id"), f("email"))},
	}
	v3 := Contract{
		ResolvedAny: true,
		Branches:    []Branch{{Status: 200, Resolved: false}},
	}
	res := Diff(oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift", res.Verdict)
	}
	if len(res.FieldDrift) != 1 || !res.FieldDrift[0].Unresolved {
		t.Fatalf("field_drift = %+v, want one Unresolved record", res.FieldDrift)
	}
	if len(res.FieldDrift[0].OnlyInOracle) != 0 {
		t.Fatalf("unresolved field_drift must not list phantom missing fields, got %v",
			res.FieldDrift[0].OnlyInOracle)
	}
}

// TestDiff_V3AddedStatus: v3 added a 422 the oracle never returns → status drift
// attributed to v3.
func TestDiff_V3AddedStatus(t *testing.T) {
	oracle := Contract{ResolvedAny: true, Branches: []Branch{resolvedBranch(200, f("id"))}}
	v3 := Contract{ResolvedAny: true, Branches: []Branch{
		resolvedBranch(200, f("id")),
		resolvedBranch(422, f("errors")),
	}}
	res := Diff(oracle, v3)
	if len(res.StatusSetDrift) != 1 || res.StatusSetDrift[0].Status != 422 || res.StatusSetDrift[0].Side != "v3" {
		t.Fatalf("status_set_drift = %+v, want one {422, v3}", res.StatusSetDrift)
	}
}
