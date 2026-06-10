package substrate

// effect_sinks_python_repo_read_4668_test.go — read/write attribution symmetry
// regression for the Python effect sniffer (#4668).
//
// The write sniffer matches the write verbs (.save/.create/.delete/...) on ANY
// receiver, so a layered repository's `self.queryset.save(obj)` resolves
// db_write and propagates up the controller→service→repo CALLS chain. The read
// sniffer historically only matched the Django `.objects.<verb>` MANAGER form,
// so a repository/query-builder read held on an attribute or variable —
// `self.queryset.filter(...)`, `qs.exclude(...)`, `self.get_queryset()` — was
// MISSED and the GET/list handler that delegates to it resolved PURE (the ~40
// false-pure read endpoints stub_detector flagged). pyDBReadRe now also matches
// the distinctive Django queryset read terminals as BARE verbs, mirroring the
// write side, WITHOUT flagging dict/list builtins (.get on a dict, etc.).

import "testing"

func TestPythonRepoQuerysetReads_4668(t *testing.T) {
	src := `
class ContractRepository:
    def list_active(self):
        return self.queryset.filter(active=True)
    def list_excluding(self, ids):
        return self.queryset.exclude(id__in=ids)
    def list_annotated(self):
        return self.queryset.annotate(n=Count("x"))
    def list_related(self):
        return self.queryset.select_related("client").prefetch_related("devices")
    def get_qs(self):
        return self.get_queryset()
    def save_one(self, obj):
        return self.queryset.save(obj)
`
	by := groupByEffect(sniffEffectsPython(src))
	// Every bare-receiver queryset read must now be attributed db_read.
	mustHave(t, by, EffectDBRead, "list_active")
	mustHave(t, by, EffectDBRead, "list_excluding")
	mustHave(t, by, EffectDBRead, "list_annotated")
	mustHave(t, by, EffectDBRead, "list_related")
	mustHave(t, by, EffectDBRead, "get_qs")
	// The write side stays correct (already matched bare verbs).
	mustHave(t, by, EffectDBWrite, "save_one")
}

// TestPythonReadVerbsNoDictFalsePositive_4668 guards the conservative boundary:
// builtin .get on a dict (the single most common collision) must NOT be read as
// db_read, so we keep ambiguous names (.get/.all/.first/.count/.values) gated to
// the `.objects.` manager form rather than bare-matching them.
func TestPythonReadVerbsNoDictFalsePositive_4668(t *testing.T) {
	src := `
def lookup(d, key):
    return d.get(key)

def first_item(items):
    return items.count("x")
`
	by := groupByEffect(sniffEffectsPython(src))
	mustNotHave(t, by, EffectDBRead, "lookup")
	mustNotHave(t, by, EffectDBRead, "first_item")
}
