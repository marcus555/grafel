package substrate

// effect_sinks_cross_orm_read_4692_test.go — cross-ORM receiver-typed read
// reach for the layered-repository pattern (#4692, generalization of the Python
// #4691 / #4668 / #4694 fixes).
//
// The recurring bug: write verbs (.save/.create) bare-match and propagate
// db_write, but READ verbs that collide with builtins / in-memory collection
// methods (.get/.first/.find/.all/.where/.count) were gated, so layered-repo
// reads resolved PURE → GET/list handlers falsely looked like stubs.
//
// Each language gets three assertions:
//   A — a repository read on a TYPED receiver  → db_read (RED before #4692)
//   B — the same ambiguous verb on a plain collection/dict/map → STAYS pure
//   C — a thin controller→service→repo read chain whose repo method's effects
//       include db_read (the per-method sink that the CALLS-union propagates up)
//
// The propagation union itself (controller→service→repo) is exercised by the
// link-layer tests; here we assert the SINK is stamped on the repo read method,
// which is the precondition that was missing.

import "testing"

// ---------------------------------------------------------------- C# / EF Core

// A: ambiguous LINQ terminals on a DbSet/IQueryable-typed receiver are db_read.
func TestCSharpEFTypedRead_4692(t *testing.T) {
	src := `
public class UserRepository {
    private readonly AppDbContext _context;
    public User GetById(int id) {
        return _context.Users.Where(u => u.Id == id).FirstOrDefault();
    }
    public List<User> List() {
        var q = _context.Users.AsNoTracking();
        return q.ToList();
    }
    public User Find(int id) {
        return _db.Set<User>().Find(id);
    }
}`
	by := groupByEffect(sniffEffectsCSharp(src))
	mustHave(t, by, EffectDBRead, "GetById")
	mustHave(t, by, EffectDBRead, "List")
	mustHave(t, by, EffectDBRead, "Find")
}

// B (negative): the same LINQ verbs on an in-memory List/array stay pure.
func TestCSharpInMemoryLinqNoFalsePositive_4692(t *testing.T) {
	src := `
public class Calc {
    public Item Pick(List<Item> items, string[] names) {
        var x = items.Where(i => i.Active).ToList();
        var f = items.Find(i => i.Id == 1);
        var b = names.Any();
        var n = items.Count;
        return f;
    }
}`
	by := groupByEffect(sniffEffectsCSharp(src))
	mustNotHave(t, by, EffectDBRead, "Pick")
}

// C: thin repo read method carries the db_read sink for the CALLS-union to lift.
func TestCSharpRepoReadChainSink_4692(t *testing.T) {
	src := `
public class OrderRepository {
    private readonly OrderContext _context;
    public Order GetOrder(int id) {
        return _context.Orders.SingleOrDefault(o => o.Id == id);
    }
}`
	by := groupByEffect(sniffEffectsCSharp(src))
	mustHave(t, by, EffectDBRead, "GetOrder")
}

// ---------------------------------------------------------- Ruby / ActiveRecord

// A: ambiguous AR terminals on a Model class / relation-typed receiver.
func TestRubyARTypedRead_4692(t *testing.T) {
	src := `
class UserRepository
  def get(id)
    User.find(id)
  end

  def active
    rel = User.where(active: true)
    rel.all
  end

  def newest
    User.first
  end
end
`
	by := groupByEffect(sniffEffectsRuby(src))
	mustHave(t, by, EffectDBRead, "get")
	mustHave(t, by, EffectDBRead, "active")
	mustHave(t, by, EffectDBRead, "newest")
}

// B (negative): the same verbs on a plain Array/Hash stay pure.
func TestRubyArrayNoFalsePositive_4692(t *testing.T) {
	src := `
class Calc
  def pick(items, h)
    head = items.first(3)
    n = items.count { |x| x.odd? }
    found = h.find { |k, v| v }
    head
  end
end
`
	by := groupByEffect(sniffEffectsRuby(src))
	mustNotHave(t, by, EffectDBRead, "pick")
}

// C: thin repo read method carries the db_read sink.
func TestRubyRepoReadChainSink_4692(t *testing.T) {
	src := `
class OrderRepository
  def get_order(id)
    Order.find(id)
  end
end
`
	by := groupByEffect(sniffEffectsRuby(src))
	mustHave(t, by, EffectDBRead, "get_order")
}

// ------------------------------------------------------------- PHP / Eloquent

// A: ambiguous Eloquent terminals on a Model/Builder-typed variable or the
// injected `$this->model->` collaborator.
func TestPHPEloquentTypedRead_4692(t *testing.T) {
	src := `
class UserRepository {
    public function active() {
        $q = User::query();
        return $q->where('active', 1)->get();
    }
    public function listAll() {
        return $this->model->where('a', 1)->get();
    }
    public function newest() {
        $base = DB::table('users');
        return $base->first();
    }
}`
	by := groupByEffect(sniffEffectsPHP(src))
	mustHave(t, by, EffectDBRead, "active")
	mustHave(t, by, EffectDBRead, "listAll")
	mustHave(t, by, EffectDBRead, "newest")
}

// B (negative): ambiguous terminals on a plain (untyped) collection variable
// stay pure.
func TestPHPCollectionNoFalsePositive_4692(t *testing.T) {
	src := `
class Calc {
    public function pick($rows) {
        $head = $rows->first();
        $n = $items->count();
        return $head;
    }
}`
	by := groupByEffect(sniffEffectsPHP(src))
	mustNotHave(t, by, EffectDBRead, "pick")
}

// C: thin repo read method carries the db_read sink.
func TestPHPRepoReadChainSink_4692(t *testing.T) {
	src := `
class OrderRepository {
    public function getOrder($id) {
        $q = Order::query();
        return $q->where('id', $id)->first();
    }
}`
	by := groupByEffect(sniffEffectsPHP(src))
	mustHave(t, by, EffectDBRead, "getOrder")
}

// ------------------------------------------------------- Go / GORM + sqlx

// A: ambiguous read terminals on a DB-typed handle (field / param).
func TestGoDBTypedRead_4692(t *testing.T) {
	src := `
type Repo struct {
	db *sqlx.DB
}

func (r *Repo) Get(ctx context.Context, id int) (User, error) {
	var u User
	err := r.db.Get(ctx, id)
	return u, err
}

func (r *Repo) List() ([]User, error) {
	var us []User
	r.db.Where("active = ?", true).Find(&us)
	return us, nil
}

func loadOne(db *gorm.DB, id int) User {
	var u User
	db.First(&u, id)
	return u
}
`
	by := groupByEffect(sniffEffectsGo(src))
	mustHave(t, by, EffectDBRead, "Get")
	mustHave(t, by, EffectDBRead, "List")
	mustHave(t, by, EffectDBRead, "loadOne")
}

// B (negative): an ambiguous verb on a non-DB receiver (a cache .Get) stays pure.
func TestGoNonDBReceiverNoFalsePositive_4692(t *testing.T) {
	src := `
func lookup(m Cache, key string) string {
	v, ok := m.Get(key)
	if !ok {
		return ""
	}
	return v
}
`
	by := groupByEffect(sniffEffectsGo(src))
	mustNotHave(t, by, EffectDBRead, "lookup")
}

// C: thin repo read method carries the db_read sink.
func TestGoRepoReadChainSink_4692(t *testing.T) {
	src := `
type OrderRepo struct {
	db *gorm.DB
}

func (r *OrderRepo) GetOrder(id int) Order {
	var o Order
	r.db.First(&o, id)
	return o
}
`
	by := groupByEffect(sniffEffectsGo(src))
	mustHave(t, by, EffectDBRead, "GetOrder")
}
