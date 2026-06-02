// Tests for the ORM model lifecycle-hook / signal → handler TRIGGERS pass
// (#3628 area). Each framework asserts the concrete event-node ID
// (SCOPE.ModelEvent:<Model>.<event>) AND the TRIGGERS edge to the named
// handler — never a bare len>0. Negative cases assert honest-partial skips.
package engine

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

func runORMHookDetect(t *testing.T, lang, path, src string) ([]types.EntityRecord, []types.RelationshipRecord) {
	t.Helper()
	res := applyORMLifecycleHookEdges(DetectorPassArgs{Lang: lang, Path: path, Content: []byte(src)})
	return res.Entities, res.Relationships
}

// hasModelEvent reports whether a SCOPE.ModelEvent node with the given stub ID
// and name exists.
func hasModelEvent(ents []types.EntityRecord, id, name string) bool {
	for _, e := range ents {
		if e.Kind == modelEventKind && e.ID == id && e.Name == name {
			return true
		}
	}
	return false
}

// hasTriggers reports whether a TRIGGERS edge from the given model-event stub
// to Function:<handler> exists.
func hasTriggers(rels []types.RelationshipRecord, fromStub, handler string) bool {
	for _, r := range rels {
		if r.Kind == string(types.RelationshipKindTriggers) &&
			r.FromID == fromStub && r.ToID == "Function:"+handler {
			return true
		}
	}
	return false
}

func countModelEvents(ents []types.EntityRecord) int {
	n := 0
	for _, e := range ents {
		if e.Kind == modelEventKind {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Django signals
// ---------------------------------------------------------------------------

func TestORMHook_Django_ReceiverPostSaveWithSender(t *testing.T) {
	src := `from django.db.models.signals import post_save
from django.dispatch import receiver
from .models import User

@receiver(post_save, sender=User)
def notify(sender, instance, **kwargs):
    pass
`
	ents, rels := runORMHookDetect(t, "python", "signals.py", src)
	const stub = "SCOPE.ModelEvent:User.post_save"
	if !hasModelEvent(ents, stub, "User.post_save") {
		t.Fatalf("missing model-event node %s; ents=%+v", stub, ents)
	}
	if !hasTriggers(rels, stub, "notify") {
		t.Fatalf("missing TRIGGERS %s -> notify; rels=%+v", stub, rels)
	}
}

func TestORMHook_Django_DottedSignalAndSender(t *testing.T) {
	src := `from django.db.models import signals
import app.models as m

@signals.pre_delete(sender=m.Order)
def cleanup(sender, **kwargs):
    pass
`
	// Note: decorator form must be @receiver; the above is not a receiver, so
	// no edge. Validate the real dotted form below.
	src = `from django.db.models import signals

@receiver(signals.post_delete, sender=app.models.Order)
def purge(sender, instance, **kwargs):
    pass
`
	ents, rels := runORMHookDetect(t, "python", "signals.py", src)
	const stub = "SCOPE.ModelEvent:Order.post_delete"
	if !hasModelEvent(ents, stub, "Order.post_delete") {
		t.Fatalf("missing %s; ents=%+v", stub, ents)
	}
	if !hasTriggers(rels, stub, "purge") {
		t.Fatalf("missing TRIGGERS %s -> purge", stub)
	}
}

// Negative: @receiver(post_save) with NO sender → dynamic/all-models →
// honest-partial skip (no fabricated model).
func TestORMHook_Django_NoSender_Skipped(t *testing.T) {
	src := `from django.db.models.signals import post_save
from django.dispatch import receiver

@receiver(post_save)
def audit(sender, instance, **kwargs):
    pass
`
	ents, rels := runORMHookDetect(t, "python", "signals.py", src)
	if countModelEvents(ents) != 0 {
		t.Fatalf("expected no model-event nodes for senderless receiver, got %+v", ents)
	}
	if len(rels) != 0 {
		t.Fatalf("expected no TRIGGERS edges, got %+v", rels)
	}
}

// Negative: a non-signal decorator must not produce an edge.
func TestORMHook_Django_NonSignalDecorator_NoEdge(t *testing.T) {
	src := `@app.route("/x")
def view():
    pass
`
	ents, rels := runORMHookDetect(t, "python", "views.py", src)
	if countModelEvents(ents) != 0 || len(rels) != 0 {
		t.Fatalf("non-signal decorator produced output: ents=%+v rels=%+v", ents, rels)
	}
}

// ---------------------------------------------------------------------------
// SQLAlchemy events
// ---------------------------------------------------------------------------

func TestORMHook_SQLAlchemy_ListensFor(t *testing.T) {
	src := `from sqlalchemy import event
from .models import User

@event.listens_for(User, 'after_insert')
def set_default(mapper, connection, target):
    pass
`
	ents, rels := runORMHookDetect(t, "python", "events.py", src)
	const stub = "SCOPE.ModelEvent:User.after_insert"
	if !hasModelEvent(ents, stub, "User.after_insert") {
		t.Fatalf("missing %s; ents=%+v", stub, ents)
	}
	if !hasTriggers(rels, stub, "set_default") {
		t.Fatalf("missing TRIGGERS %s -> set_default", stub)
	}
}

// ---------------------------------------------------------------------------
// ActiveRecord callbacks
// ---------------------------------------------------------------------------

func TestORMHook_ActiveRecord_AfterCreate(t *testing.T) {
	src := `class User < ApplicationRecord
  after_create :send_welcome
  before_save :normalize_email

  def send_welcome
  end
end
`
	ents, rels := runORMHookDetect(t, "ruby", "user.rb", src)
	const ac = "SCOPE.ModelEvent:User.after_create"
	const bs = "SCOPE.ModelEvent:User.before_save"
	if !hasModelEvent(ents, ac, "User.after_create") {
		t.Fatalf("missing %s; ents=%+v", ac, ents)
	}
	if !hasTriggers(rels, ac, "send_welcome") {
		t.Fatalf("missing TRIGGERS %s -> send_welcome", ac)
	}
	if !hasModelEvent(ents, bs, "User.before_save") {
		t.Fatalf("missing %s", bs)
	}
	if !hasTriggers(rels, bs, "normalize_email") {
		t.Fatalf("missing TRIGGERS %s -> normalize_email", bs)
	}
}

func TestORMHook_ActiveRecord_MultipleHandlers(t *testing.T) {
	src := `class Order < ActiveRecord::Base
  after_save :recalc_total, :notify_warehouse
end
`
	_, rels := runORMHookDetect(t, "ruby", "order.rb", src)
	const stub = "SCOPE.ModelEvent:Order.after_save"
	if !hasTriggers(rels, stub, "recalc_total") {
		t.Fatalf("missing TRIGGERS %s -> recalc_total", stub)
	}
	if !hasTriggers(rels, stub, "notify_warehouse") {
		t.Fatalf("missing TRIGGERS %s -> notify_warehouse", stub)
	}
}

// Negative: a plain Ruby class (not an AR model) must not produce hooks.
func TestORMHook_ActiveRecord_NonModelClass_Skipped(t *testing.T) {
	src := `class PlainService
  after_create :foo
end
`
	ents, rels := runORMHookDetect(t, "ruby", "svc.rb", src)
	if countModelEvents(ents) != 0 || len(rels) != 0 {
		t.Fatalf("non-AR class produced output: ents=%+v rels=%+v", ents, rels)
	}
}

// ---------------------------------------------------------------------------
// TypeORM entity listeners
// ---------------------------------------------------------------------------

func TestORMHook_TypeORM_AfterInsert(t *testing.T) {
	src := `import { Entity, AfterInsert, BeforeUpdate } from 'typeorm';

@Entity()
export class Order {
  @AfterInsert()
  logInsert() {}

  @BeforeUpdate()
  touch() {}
}
`
	ents, rels := runORMHookDetect(t, "typescript", "order.entity.ts", src)
	const ai = "SCOPE.ModelEvent:Order.afterInsert"
	const bu = "SCOPE.ModelEvent:Order.beforeUpdate"
	if !hasModelEvent(ents, ai, "Order.afterInsert") {
		t.Fatalf("missing %s; ents=%+v", ai, ents)
	}
	if !hasTriggers(rels, ai, "logInsert") {
		t.Fatalf("missing TRIGGERS %s -> logInsert", ai)
	}
	if !hasTriggers(rels, bu, "touch") {
		t.Fatalf("missing TRIGGERS %s -> touch", bu)
	}
}

// ---------------------------------------------------------------------------
// Sequelize hooks
// ---------------------------------------------------------------------------

func TestORMHook_Sequelize_AfterCreate(t *testing.T) {
	src := `User.afterCreate(sendWelcomeEmail);
Order.beforeSave('lower', normalizeSku);
`
	ents, rels := runORMHookDetect(t, "javascript", "hooks.js", src)
	const uc = "SCOPE.ModelEvent:User.afterCreate"
	if !hasModelEvent(ents, uc, "User.afterCreate") {
		t.Fatalf("missing %s; ents=%+v", uc, ents)
	}
	if !hasTriggers(rels, uc, "sendWelcomeEmail") {
		t.Fatalf("missing TRIGGERS %s -> sendWelcomeEmail", uc)
	}
	if !hasTriggers(rels, "SCOPE.ModelEvent:Order.beforeSave", "normalizeSku") {
		t.Fatalf("missing Order.beforeSave -> normalizeSku")
	}
}

// Negative: anonymous arrow handler → no named symbol → skip.
func TestORMHook_Sequelize_AnonymousHandler_Skipped(t *testing.T) {
	src := `User.afterCreate((user) => { console.log(user); });`
	ents, rels := runORMHookDetect(t, "javascript", "hooks.js", src)
	if countModelEvents(ents) != 0 || len(rels) != 0 {
		t.Fatalf("anonymous handler produced output: ents=%+v rels=%+v", ents, rels)
	}
}

// ---------------------------------------------------------------------------
// Mongoose middleware
// ---------------------------------------------------------------------------

func TestORMHook_Mongoose_PostSave(t *testing.T) {
	src := `const userSchema = new mongoose.Schema({ name: String });
userSchema.post('save', sendEmail);
userSchema.pre('save', hashPassword);
const User = mongoose.model('User', userSchema);
`
	ents, rels := runORMHookDetect(t, "javascript", "user.model.js", src)
	// Schema resolves to model "User" via mongoose.model('User', userSchema).
	const post = "SCOPE.ModelEvent:User.post.save"
	const pre = "SCOPE.ModelEvent:User.pre.save"
	if !hasModelEvent(ents, post, "User.post.save") {
		t.Fatalf("missing %s; ents=%+v", post, ents)
	}
	if !hasTriggers(rels, post, "sendEmail") {
		t.Fatalf("missing TRIGGERS %s -> sendEmail", post)
	}
	if !hasTriggers(rels, pre, "hashPassword") {
		t.Fatalf("missing TRIGGERS %s -> hashPassword", pre)
	}
}

// Negative: anonymous mongoose middleware → skip.
func TestORMHook_Mongoose_AnonymousHandler_Skipped(t *testing.T) {
	src := `schema.post('save', function (doc) { /* ... */ });`
	_, rels := runORMHookDetect(t, "javascript", "x.js", src)
	if len(rels) != 0 {
		t.Fatalf("anonymous mongoose handler produced edges: %+v", rels)
	}
}
