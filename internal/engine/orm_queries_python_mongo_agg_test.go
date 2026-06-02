package engine

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// runMongoAggPy drives scanPythonMongoAggregation over `src` and collects the
// emitted stage entities + join edges.
func runMongoAggPy(t *testing.T, src string) ([]types.EntityRecord, []types.RelationshipRecord) {
	t.Helper()
	funcs := indexEnclosingFunctions("python", src)
	var ents []types.EntityRecord
	var rels []types.RelationshipRecord
	scanPythonMongoAggregation(src, funcs, "svc/agg.py", "python",
		func(e types.EntityRecord) { ents = append(ents, e) },
		func(r types.RelationshipRecord) { rels = append(rels, r) },
	)
	return ents, rels
}

func pyStageSubtypesInOrder(ents []types.EntityRecord) []string {
	var out []string
	for _, e := range ents {
		out = append(out, e.Subtype)
	}
	return out
}

func pyFindStage(ents []types.EntityRecord, subtype string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Subtype == subtype {
			return &ents[i]
		}
	}
	return nil
}

func pyFindJoinTo(rels []types.RelationshipRecord, toClass string) *types.RelationshipRecord {
	for i := range rels {
		if rels[i].Kind == string(types.RelationshipKindJoinsCollection) &&
			rels[i].ToID == "Class:"+toClass {
			return &rels[i]
		}
	}
	return nil
}

// THE important case — variable-bound pipeline, the legacy-Django/pymongo
// shape: three $lookups to specific `from` collections then a $match. Asserts
// the three JOINS_COLLECTION edges to the SPECIFIC collections, the stage
// entities and their order.
func TestMongoAggPy_VariableBound_ThreeLookups(t *testing.T) {
	src := `
import pymongo
from pymongo import MongoClient

def inspection_report(db):
    pipeline = [
        {"$lookup": {"from": "inspection_groups", "localField": "group_id", "foreignField": "_id", "as": "groups"}},
        {"$lookup": {"from": "m_devices", "localField": "device_id", "foreignField": "_id", "as": "devices"}},
        {"$lookup": {"from": "m_buildings", "localField": "building_id", "foreignField": "_id", "as": "buildings"}},
        {"$match": {"status": "active"}},
    ]
    return db.inspections.aggregate(pipeline)
`
	ents, rels := runMongoAggPy(t, src)

	// 3 join edges to the SPECIFIC from-collections.
	wantTo := []string{"inspection_groups", "m_devices", "m_buildings"}
	for _, coll := range wantTo {
		j := pyFindJoinTo(rels, capitalisedSingular(coll))
		if j == nil {
			t.Fatalf("expected JOINS_COLLECTION edge to %s; rels=%+v", coll, rels)
		}
		if j.FromID != "Class:"+capitalisedSingular("inspections") {
			t.Errorf("join from %s = %q, want aggregating collection inspections", coll, j.FromID)
		}
		if j.Properties["stage"] != "lookup" {
			t.Errorf("join to %s stage = %q, want lookup", coll, j.Properties["stage"])
		}
	}
	if n := len(rels); n != 3 {
		t.Fatalf("expected exactly 3 join edges, got %d: %+v", n, rels)
	}

	// Local/foreign field props captured on the first lookup edge.
	jg := pyFindJoinTo(rels, capitalisedSingular("inspection_groups"))
	if jg.Properties["local_field"] != "group_id" || jg.Properties["foreign_field"] != "_id" || jg.Properties["as"] != "groups" {
		t.Errorf("inspection_groups join props = %+v", jg.Properties)
	}

	// Stage entities + order.
	got := pyStageSubtypesInOrder(ents)
	want := []string{"$lookup", "$lookup", "$lookup", "$match"}
	if len(got) != len(want) {
		t.Fatalf("stage subtypes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stage[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
	// stage_index preserved on each entity.
	for i, e := range ents {
		if e.Properties["stage_index"] != itoa(i) {
			t.Errorf("stage[%d] stage_index = %q, want %d", i, e.Properties["stage_index"], i)
		}
		if e.Properties["collection"] != "inspections" {
			t.Errorf("stage[%d] collection = %q, want inspections", i, e.Properties["collection"])
		}
		if e.Kind != string(types.EntityKindDataAccess) {
			t.Errorf("stage[%d] kind = %q, want SCOPE.DataAccess", i, e.Kind)
		}
	}
	if c := pyFindStage(ents, "$lookup").Properties["from"]; c != "inspection_groups" {
		t.Errorf("first $lookup from = %q, want inspection_groups", c)
	}
}

// THE rewrite-BLOCKING case — variable-bound pipeline whose $lookup stages use
// the CORRELATED sub-pipeline form (`from` + `let` + `pipeline` + `as`, NO
// localField/foreignField). The nested `pipeline: [...]` array and `let: {...}`
// object inside each stage must not break the multiline RHS capture nor the
// stage split. Mirrors `_get_me_inspections` from the legacy repo.
func TestMongoAggPy_VariableBound_CorrelatedLookups(t *testing.T) {
	src := `
import pymongo
from pymongo import MongoClient

def _get_me_inspections():
    inspections_cls = MongoDBConnection.get_collection(INSPECTIONS)
    pipeline = [
        {"$project": {"_id": 1, "checklist_type": 1}},
        {"$match": {"checklist_type": 4, "status": "active"}},
        {"$lookup": {"from": "inspection_groups", "let": {"gid": "$group_id"}, "pipeline": [{"$match": {"$expr": {"$eq": ["$_id", "$$gid"]}}}], "as": "inspections_group"}},
        {"$unwind": {"path": "$inspections_group", "preserveNullAndEmptyArrays": True}},
        {"$lookup": {"from": "m_devices", "let": {"did": "$device_id"}, "pipeline": [{"$match": {"$expr": {"$eq": ["$_id", "$$did"]}}}], "as": "device"}},
        {"$unwind": {"path": "$device", "preserveNullAndEmptyArrays": True}},
        {"$lookup": {"from": "m_buildings", "let": {"bid": "$building_id"}, "pipeline": [{"$match": {"$expr": {"$eq": ["$_id", "$$bid"]}}}], "as": "building"}},
        {"$unwind": {"path": "$building"}},
        {"$project": {"name": 1}},
    ]
    return list(inspections_cls.aggregate(pipeline))
`
	ents, rels := runMongoAggPy(t, src)

	// 3 join edges to the SPECIFIC correlated from-collections.
	wantTo := []string{"inspection_groups", "m_devices", "m_buildings"}
	for _, coll := range wantTo {
		j := pyFindJoinTo(rels, capitalisedSingular(coll))
		if j == nil {
			t.Fatalf("expected JOINS_COLLECTION edge to %s; rels=%+v", coll, rels)
		}
		// The `INSPECTIONS` constant receiver (imported cross-module, no in-file
		// value) must resolve to the SAME canonical collection node as the
		// string-literal `"inspections"` viewset forms: Class:Inspection, NOT a
		// phantom Class:INSPECTIONS. This is the real-vs-fixture fix.
		if j.FromID != "Class:"+capitalisedSingular("inspections") {
			t.Errorf("join to %s from = %q, want aggregating collection Class:Inspection", coll, j.FromID)
		}
		if j.Properties["stage"] != "lookup" {
			t.Errorf("join to %s stage = %q, want lookup", coll, j.Properties["stage"])
		}
	}
	if n := len(rels); n != 3 {
		t.Fatalf("expected exactly 3 join edges, got %d: %+v", n, rels)
	}

	// The correlated `as` alias is captured even without local/foreign fields.
	jg := pyFindJoinTo(rels, capitalisedSingular("inspection_groups"))
	if jg.Properties["as"] != "inspections_group" {
		t.Errorf("inspection_groups join as = %q, want inspections_group", jg.Properties["as"])
	}

	// Stage entities + order: project, match, lookup, unwind, lookup, unwind,
	// lookup, unwind, project (9 stages, order preserved).
	got := pyStageSubtypesInOrder(ents)
	want := []string{"$project", "$match", "$lookup", "$unwind", "$lookup", "$unwind", "$lookup", "$unwind", "$project"}
	if len(got) != len(want) {
		t.Fatalf("stage subtypes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stage[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

// REAL-FORM regression — verbatim copy of the production
// `core.tasks.maintenance_evaluation_notifications._get_me_inspections`
// pipeline (multiline stage objects, nested `let`/`pipeline`/`$project` on
// their OWN lines, `$first` accessor, `True` Python literals). The earlier
// #3567 fixture collapsed each `$lookup` onto ONE line, which hid a stage-split
// failure that occurs on the real multiline form. Must emit the 3 correlated
// joins (inspection_groups, m_devices, m_buildings) and the full 8-stage order.
func TestMongoAggPy_RealForm_GetMeInspections(t *testing.T) {
	src := `
import logging
from core.helper.mongo_helper import MongoDBConnection
from core.mongodb_collections import INSPECTIONS


def _get_me_inspections():
    inspections_cls = MongoDBConnection.get_collection(INSPECTIONS)
    pipeline = [
        {
            "$project": {
                "device_id": 1,
                "inspectionGroupId": 1,
                "status": 1,
                "checklist_id": {"$first": "$inspection_checklists.id"},
                "checklist_type": {"$first": "$inspection_checklists.type"},
            }
        },
        {"$match": {"checklist_type": 4, "status": {"$in": ["Results Reviewed", "Assembling Report"]}}},
        {
            "$lookup": {
                "from": "inspection_groups",
                "let": {"inspections_group_id": "$inspectionGroupId"},
                "pipeline": [
                    {"$match": {"$expr": {"$eq": ["$_id", "$$inspections_group_id"]}}},
                    {"$project": {"_id": 1, "buildingId": 1, "groupId": 1}},
                ],
                "as": "inspections_group",
            }
        },
        {"$unwind": {"path": "$inspections_group", "preserveNullAndEmptyArrays": True}},
        {
            "$lookup": {
                "from": "m_devices",
                "let": {"device_id": "$device_id"},
                "pipeline": [
                    {"$match": {"$expr": {"$eq": ["$postgresql_id", "$$device_id"]}}},
                    {"$project": {"_id": 0, "id": "$postgresql_id", "name": 1}},
                ],
                "as": "device",
            }
        },
        {"$unwind": {"path": "$device", "preserveNullAndEmptyArrays": True}},
        {
            "$lookup": {
                "from": "m_buildings",
                "let": {"building_id": "$inspections_group.buildingId"},
                "pipeline": [
                    {"$match": {"$expr": {"$eq": ["$postgresql_id", "$$building_id"]}}},
                    {"$project": {"_id": 0, "id": "$postgresql_id", "name": 1}},
                ],
                "as": "building",
            }
        },
        {"$unwind": {"path": "$building", "preserveNullAndEmptyArrays": True}},
    ]
    return list(inspections_cls.aggregate(pipeline))
`
	ents, rels := runMongoAggPy(t, src)

	// The 3 correlated joins to the SPECIFIC from-collections.
	wantTo := []string{"inspection_groups", "m_devices", "m_buildings"}
	for _, coll := range wantTo {
		j := pyFindJoinTo(rels, capitalisedSingular(coll))
		if j == nil {
			t.Fatalf("expected JOINS_COLLECTION edge to %s; rels=%+v", coll, rels)
		}
		// Cross-module `INSPECTIONS` constant resolves to the canonical
		// Class:Inspection node — identical to the string-literal viewset forms.
		if j.FromID != "Class:"+capitalisedSingular("inspections") {
			t.Errorf("join to %s from = %q, want aggregating collection Class:Inspection", coll, j.FromID)
		}
		if j.Properties["stage"] != "lookup" {
			t.Errorf("join to %s stage = %q, want lookup", coll, j.Properties["stage"])
		}
	}
	if n := len(rels); n != 3 {
		t.Fatalf("expected exactly 3 join edges, got %d: %+v", n, rels)
	}

	jg := pyFindJoinTo(rels, capitalisedSingular("inspection_groups"))
	if jg.Properties["as"] != "inspections_group" {
		t.Errorf("inspection_groups join as = %q, want inspections_group", jg.Properties["as"])
	}

	// 8 stages, order preserved: project, match, lookup, unwind, lookup, unwind,
	// lookup, unwind.
	got := pyStageSubtypesInOrder(ents)
	want := []string{"$project", "$match", "$lookup", "$unwind", "$lookup", "$unwind", "$lookup", "$unwind"}
	if len(got) != len(want) {
		t.Fatalf("stage subtypes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stage[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

// Same-file constant definition: `INSPECTIONS = "inspections"` at module scope,
// then `get_collection(INSPECTIONS)`. The constant must resolve to its literal
// VALUE "inspections" → Class:Inspection, exactly as a quoted receiver would.
func TestMongoAggPy_CollConstResolvedToValue_SameFile(t *testing.T) {
	src := `
import pymongo
from core.helper.mongo_helper import MongoDBConnection

INSPECTIONS = "inspections"


def run():
    inspections_cls = MongoDBConnection.get_collection(INSPECTIONS)
    pipeline = [
        {"$lookup": {"from": "m_devices", "localField": "device_id", "foreignField": "_id", "as": "device"}},
    ]
    return list(inspections_cls.aggregate(pipeline))
`
	ents, rels := runMongoAggPy(t, src)
	if len(rels) != 1 {
		t.Fatalf("expected 1 join edge, got %d: %+v", len(rels), rels)
	}
	if rels[0].FromID != "Class:"+capitalisedSingular("inspections") {
		t.Errorf("join from = %q, want Class:Inspection (constant resolved to value)", rels[0].FromID)
	}
	if len(ents) == 0 || ents[0].Properties["collection"] != "inspections" {
		t.Errorf("stage collection = %q, want inspections", ents[0].Properties["collection"])
	}
}

// REGRESSION GUARD for the 57 string-literal viewset aggregations: a quoted
// `get_collection("inspections")` receiver via the same variable-binding follow
// must keep producing Class:Inspection. The constant-resolution change must not
// touch this path.
func TestMongoAggPy_ViewsetStringLiteralReceiver_Unchanged(t *testing.T) {
	src := `
from core.helper.mongo_helper import MongoDBConnection


def get_inspection_details(self):
    inspections_group_cln = MongoDBConnection.get_collection("inspection_groups")
    pipeline = [
        {"$lookup": {"from": "inspections", "localField": "_id", "foreignField": "inspectionGroupId", "as": "inspections"}},
        {"$lookup": {"from": "m_buildings", "localField": "buildingId", "foreignField": "postgresql_id", "as": "building"}},
    ]
    return inspections_group_cln.aggregate(pipeline)
`
	ents, rels := runMongoAggPy(t, src)
	if len(rels) != 2 {
		t.Fatalf("expected 2 join edges, got %d: %+v", len(rels), rels)
	}
	for _, r := range rels {
		if r.FromID != "Class:"+capitalisedSingular("inspection_groups") {
			t.Errorf("join from = %q, want aggregating collection Class:Inspection_group", r.FromID)
		}
	}
	if pyFindJoinTo(rels, capitalisedSingular("inspections")) == nil {
		t.Errorf("expected join to inspections; rels=%+v", rels)
	}
	if pyFindJoinTo(rels, capitalisedSingular("m_buildings")) == nil {
		t.Errorf("expected join to m_buildings; rels=%+v", rels)
	}
	if len(ents) != 2 {
		t.Fatalf("expected 2 stage entities, got %d", len(ents))
	}
}

// Inline list-literal pipeline with $lookup + $graphLookup + $group + $facet,
// receiver via db["collname"] subscript. Asserts both join kinds, the $group
// _id/accumulators, and the $facet keys.
func TestMongoAggPy_InlineLiteral_LookupGraphGroupFacet(t *testing.T) {
	src := `
from motor.motor_asyncio import AsyncIOMotorClient

async def stats(db):
    return await db["orders"].aggregate([
        {"$lookup": {"from": "customers", "localField": "cust_id", "foreignField": "_id", "as": "cust"}},
        {"$graphLookup": {"from": "categories", "startWith": "$cat", "connectFromField": "parent", "connectToField": "_id", "as": "tree"}},
        {"$group": {"_id": "$status", "total": {"$sum": "$amount"}, "count": {"$sum": 1}}},
        {"$facet": {"byStatus": [{"$match": {}}], "byMonth": [{"$match": {}}]}},
    ])
`
	ents, rels := runMongoAggPy(t, src)

	if j := pyFindJoinTo(rels, capitalisedSingular("customers")); j == nil {
		t.Fatalf("expected $lookup join to customers; rels=%+v", rels)
	} else if j.Properties["stage"] != "lookup" {
		t.Errorf("customers join stage = %q, want lookup", j.Properties["stage"])
	}
	if j := pyFindJoinTo(rels, capitalisedSingular("categories")); j == nil {
		t.Fatalf("expected $graphLookup join to categories; rels=%+v", rels)
	} else if j.Properties["stage"] != "graphLookup" {
		t.Errorf("categories join stage = %q, want graphLookup", j.Properties["stage"])
	}
	if len(rels) != 2 {
		t.Fatalf("expected 2 join edges, got %d: %+v", len(rels), rels)
	}

	if got := pyStageSubtypesInOrder(ents); len(got) != 4 ||
		got[0] != "$lookup" || got[1] != "$graphLookup" || got[2] != "$group" || got[3] != "$facet" {
		t.Fatalf("stage subtypes = %v, want [$lookup $graphLookup $group $facet]", got)
	}

	g := pyFindStage(ents, "$group")
	// _id value is captured as raw text; Python string literals keep quotes.
	if g.Properties["group_id"] != `"$status"` {
		t.Errorf("$group group_id = %q, want \"$status\"", g.Properties["group_id"])
	}
	if g.Properties["accumulators"] != "total,count" {
		t.Errorf("$group accumulators = %q, want total,count", g.Properties["accumulators"])
	}

	f := pyFindStage(ents, "$facet")
	if f.Properties["facets"] != "byStatus,byMonth" {
		t.Errorf("$facet facets = %q, want byStatus,byMonth", f.Properties["facets"])
	}

	// Receiver resolved from db["orders"] subscript.
	if ents[0].Properties["collection"] != "orders" {
		t.Errorf("collection = %q, want orders", ents[0].Properties["collection"])
	}
}

// get_collection("coll") receiver idiom.
func TestMongoAggPy_GetCollectionReceiver(t *testing.T) {
	src := `
import pymongo

def run(db):
    coll = db.get_collection("events")
    return db.get_collection("events").aggregate([
        {"$lookup": {"from": "users", "localField": "uid", "foreignField": "_id", "as": "u"}},
    ])
`
	ents, rels := runMongoAggPy(t, src)
	if j := pyFindJoinTo(rels, capitalisedSingular("users")); j == nil {
		t.Fatalf("expected join to users; rels=%+v", rels)
	} else if j.FromID != "Class:"+capitalisedSingular("events") {
		t.Errorf("join from = %q, want aggregating collection events", j.FromID)
	}
	if len(ents) != 1 || ents[0].Properties["collection"] != "events" {
		t.Errorf("collection = %q, want events", ents[0].Properties["collection"])
	}
}

// NEGATIVE: variable bound to a NON-literal (a builder call) must NOT fabricate
// any join or stage. Honest unresolved.
func TestMongoAggPy_NegativeBuilderBinding_NoFabrication(t *testing.T) {
	src := `
import pymongo

def run(db):
    pipeline = build_pipeline(db, status="active")
    return db.inspections.aggregate(pipeline)
`
	ents, rels := runMongoAggPy(t, src)
	if len(rels) != 0 {
		t.Fatalf("builder-bound pipeline must emit NO join edges, got %d: %+v", len(rels), rels)
	}
	if len(ents) != 0 {
		t.Fatalf("builder-bound pipeline must emit NO stage entities, got %d: %+v", len(ents), ents)
	}
}

// NEGATIVE: identifier argument that is an EXPRESSION (`pipeline + extra`) is
// not a single-binding follow → unresolved, no fabrication.
func TestMongoAggPy_NegativeExpressionArg_NoFabrication(t *testing.T) {
	src := `
import pymongo

def run(db):
    pipeline = [{"$lookup": {"from": "users", "localField": "u", "foreignField": "_id", "as": "x"}}]
    extra = []
    return db.inspections.aggregate(pipeline + extra)
`
	_, rels := runMongoAggPy(t, src)
	if len(rels) != 0 {
		t.Fatalf("expression-arg pipeline must emit NO join edges, got %d: %+v", len(rels), rels)
	}
}

// NEGATIVE: no pymongo/motor surface in the file → gate skips entirely even if
// some `.aggregate([...])` is present (e.g. pandas).
func TestMongoAggPy_NegativeGate_NoMongoSurface(t *testing.T) {
	src := `
import pandas as pd

def run(frame):
    return frame.aggregate([
        {"$lookup": {"from": "users", "localField": "u", "foreignField": "_id", "as": "x"}},
    ])
`
	ents, rels := runMongoAggPy(t, src)
	if len(ents) != 0 || len(rels) != 0 {
		t.Fatalf("non-mongo file must be skipped by gate, got ents=%d rels=%d", len(ents), len(rels))
	}
}

// Single-binding follow picks the NEAREST preceding assignment in the same
// function; an earlier same-name binding in a different function is out of scope.
func TestMongoAggPy_BindingScopedToFunction(t *testing.T) {
	src := `
import pymongo

def other(db):
    pipeline = [{"$lookup": {"from": "WRONG", "localField": "a", "foreignField": "_id", "as": "x"}}]
    return db.x.aggregate(pipeline)

def run(db):
    pipeline = [{"$lookup": {"from": "right_coll", "localField": "b", "foreignField": "_id", "as": "y"}}]
    return db.inspections.aggregate(pipeline)
`
	_, rels := runMongoAggPy(t, src)
	if pyFindJoinTo(rels, capitalisedSingular("right_coll")) == nil {
		t.Fatalf("expected join to right_coll (in-scope binding); rels=%+v", rels)
	}
	// The WRONG binding belongs to other(); run()'s aggregate must not use it.
	for _, r := range rels {
		if r.ToID == "Class:"+capitalisedSingular("WRONG") && r.FromID == "Class:"+capitalisedSingular("inspections") {
			t.Fatalf("run() aggregate wrongly resolved to other()'s binding: %+v", r)
		}
	}
}
