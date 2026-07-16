// Tests for the message_publish effect element (ADR-0025 §2, #5782 Phase 2
// Track B). Synthetic Java/Kotlin fixtures only — no live corpus.
package substrate

import "testing"

func TestAllEffects_HasEightSlotsIncludingMessagePublish(t *testing.T) {
	all := AllEffects()
	if got, want := len(all), 8; got != want {
		t.Fatalf("len(AllEffects()) = %d, want %d", got, want)
	}
	if got, want := len(all), effectSlots; got != want {
		t.Fatalf("len(AllEffects()) = %d, effectSlots = %d — must match", got, want)
	}
	if all[len(all)-1] != EffectMessagePublish {
		t.Fatalf("EffectMessagePublish must be appended at the END of AllEffects() (bit-position order is load-bearing); got last=%q", all[len(all)-1])
	}
}

const javaMsgPublishFixture = `
package com.example.orders;

import org.eclipse.microprofile.reactive.messaging.Emitter;
import org.eclipse.microprofile.reactive.messaging.Outgoing;
import org.eclipse.microprofile.reactive.messaging.Channel;
import javax.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class OrderPublisher {

    @Channel("orders-out")
    Emitter<String> emitter;

    public void publishOrder(String payload) {
        emitter.send(payload);
    }

    @Outgoing("orders-out")
    public String publishViaReturn() {
        return "order-payload";
    }

    public int addTwo(int a, int b) {
        return a + b;
    }

    // Non-SmallRye: Android Handler.sendMessage / JavaMail transport.sendMessage
    // etc. A bare unscoped sendMessage receiver is NOT reactive-messaging and
    // must not be flagged (ADR-0025 section 2 is SmallRye-scoped).
    public void notifyHandler(android.os.Handler handler, Object m) {
        handler.sendMessage(m);
    }
}
`

func TestSniffEffectsJava_MessagePublish_EmitterSend(t *testing.T) {
	matches := sniffEffectsJava(javaMsgPublishFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishOrder" && m.Effect == EffectMessagePublish {
			found = true
			if m.Confidence <= 0 || m.Confidence > 1 {
				t.Errorf("publishOrder message_publish confidence out of range: %v", m.Confidence)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishOrder (Emitter.send), matches=%+v", matches)
	}
}

func TestSniffEffectsJava_MessagePublish_OutgoingAnnotatedMethod(t *testing.T) {
	matches := sniffEffectsJava(javaMsgPublishFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishViaReturn" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishViaReturn (@Outgoing-annotated method), matches=%+v", matches)
	}
}

func TestSniffEffectsJava_MessagePublish_NonMessagingMethodNotFlagged(t *testing.T) {
	matches := sniffEffectsJava(javaMsgPublishFixture)
	for _, m := range matches {
		if m.Function == "addTwo" && m.Effect == EffectMessagePublish {
			t.Fatalf("addTwo has no messaging call and must NOT be flagged message_publish; matches=%+v", matches)
		}
	}
}

// A bare `handler.sendMessage(m)` (Android Handler / JavaMail transport /
// chat-SDK) on a non-emitter receiver is NOT SmallRye reactive messaging and
// must NOT be flagged message_publish. Guards against re-introducing the
// unscoped `.sendMessage(` alternative (coordinator review nit).
func TestSniffEffectsJava_MessagePublish_BareSendMessageNotFlagged(t *testing.T) {
	matches := sniffEffectsJava(javaMsgPublishFixture)
	for _, m := range matches {
		if m.Function == "notifyHandler" && m.Effect == EffectMessagePublish {
			t.Fatalf("notifyHandler uses a non-emitter handler.sendMessage and must NOT be flagged message_publish; matches=%+v", matches)
		}
	}
}

// Realistic SmallRye fixture modeled on the public event-driven-ai corpus
// (ai-triage-service/TriageTools.java, feedback-ingest-service/FeedbackResource.java):
// the Emitter/MutinyEmitter field is named after its CHANNEL, not "emitter".
// This is the field-aware precision fix for #5782 ask #4.
const javaChannelNamedEmitterFixture = `
package io.triage.ai;

import org.eclipse.microprofile.reactive.messaging.Channel;
import org.eclipse.microprofile.reactive.messaging.Emitter;
import io.smallrye.reactive.messaging.MutinyEmitter;
import javax.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class TriageTools {

    @Channel("offer-assign-out")
    Emitter<TriageActionEvent> offerAssignOut;

    @Channel("feedback-out")
    MutinyEmitter<FeedbackReceivedEvent> feedbackOut;

    // Not an emitter at all — some other messaging-shaped field. Must NOT
    // be treated as a publisher receiver.
    java.util.List<String> handler;

    public void offerAssign(TriageActionEvent action) {
        offerAssignOut.send(action);
    }

    public void publishFeedback(FeedbackReceivedEvent event) {
        feedbackOut.sendMessage(event);
    }

    // handler is a List, not an Emitter/MutinyEmitter field — must NOT be
    // flagged even though the call shape ".sendMessage(" matches.
    public void notAPublisher(String m) {
        handler.sendMessage(m);
    }
}
`

func TestSniffEffectsJava_MessagePublish_ChannelNamedEmitterField(t *testing.T) {
	matches := sniffEffectsJava(javaChannelNamedEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "offerAssign" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on offerAssign (channel-named Emitter field offerAssignOut.send), matches=%+v", matches)
	}
}

func TestSniffEffectsJava_MessagePublish_ChannelNamedMutinyEmitterField(t *testing.T) {
	matches := sniffEffectsJava(javaChannelNamedEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishFeedback" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishFeedback (channel-named MutinyEmitter field feedbackOut.sendMessage), matches=%+v", matches)
	}
}

func TestSniffEffectsJava_MessagePublish_NonEmitterFieldSendMessageNotFlagged(t *testing.T) {
	matches := sniffEffectsJava(javaChannelNamedEmitterFixture)
	for _, m := range matches {
		if m.Function == "notAPublisher" && m.Effect == EffectMessagePublish {
			t.Fatalf("notAPublisher calls handler.sendMessage where handler is a List field (not Emitter/MutinyEmitter) and must NOT be flagged; matches=%+v", matches)
		}
	}
}

const kotlinChannelNamedEmitterFixture = `
package io.triage.ai

import org.eclipse.microprofile.reactive.messaging.Channel
import org.eclipse.microprofile.reactive.messaging.Emitter
import io.smallrye.reactive.messaging.MutinyEmitter
import javax.enterprise.context.ApplicationScoped

@ApplicationScoped
class TriageTools {

    @Channel("offer-assign-out")
    lateinit var offerAssignOut: Emitter<TriageActionEvent>

    @Channel("feedback-out")
    lateinit var feedbackOut: MutinyEmitter<FeedbackReceivedEvent>

    // Not an emitter — a List field. Must NOT be treated as a publisher.
    var handler: MutableList<String> = mutableListOf()

    fun offerAssign(action: TriageActionEvent) {
        offerAssignOut.send(action)
    }

    fun publishFeedback(event: FeedbackReceivedEvent) {
        feedbackOut.sendMessage(event)
    }

    fun notAPublisher(m: String) {
        handler.sendMessage(m)
    }
}
`

func TestSniffEffectsKotlin_MessagePublish_ChannelNamedEmitterField(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinChannelNamedEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "offerAssign" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on offerAssign (channel-named Emitter field offerAssignOut.send), matches=%+v", matches)
	}
}

func TestSniffEffectsKotlin_MessagePublish_ChannelNamedMutinyEmitterField(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinChannelNamedEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishFeedback" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishFeedback (channel-named MutinyEmitter field feedbackOut.sendMessage), matches=%+v", matches)
	}
}

func TestSniffEffectsKotlin_MessagePublish_NonEmitterFieldSendMessageNotFlagged(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinChannelNamedEmitterFixture)
	for _, m := range matches {
		if m.Function == "notAPublisher" && m.Effect == EffectMessagePublish {
			t.Fatalf("notAPublisher calls handler.sendMessage where handler is a List field (not Emitter/MutinyEmitter) and must NOT be flagged; matches=%+v", matches)
		}
	}
}

const kotlinMsgPublishFixture = `
package com.example.orders

import org.eclipse.microprofile.reactive.messaging.Emitter
import org.eclipse.microprofile.reactive.messaging.Outgoing
import org.eclipse.microprofile.reactive.messaging.Channel
import javax.enterprise.context.ApplicationScoped

@ApplicationScoped
class OrderPublisher {

    @Channel("orders-out")
    lateinit var emitter: Emitter<String>

    fun publishOrder(payload: String) {
        emitter.send(payload)
    }

    @Outgoing("orders-out")
    fun publishViaReturn(): String {
        return "order-payload"
    }

    fun addTwo(a: Int, b: Int): Int {
        return a + b
    }

    // Non-SmallRye: bare .sendMessage on a non-emitter receiver (Android
    // Handler / chat SDK) must NOT be flagged.
    fun notifyHandler(handler: android.os.Handler, m: Any) {
        handler.sendMessage(m)
    }
}
`

func TestSniffEffectsKotlin_MessagePublish_EmitterSend(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinMsgPublishFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishOrder" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishOrder (Emitter.send), matches=%+v", matches)
	}
}

func TestSniffEffectsKotlin_MessagePublish_OutgoingAnnotatedMethod(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinMsgPublishFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishViaReturn" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishViaReturn (@Outgoing-annotated method), matches=%+v", matches)
	}
}

func TestSniffEffectsKotlin_MessagePublish_NonMessagingMethodNotFlagged(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinMsgPublishFixture)
	for _, m := range matches {
		if m.Function == "addTwo" && m.Effect == EffectMessagePublish {
			t.Fatalf("addTwo has no messaging call and must NOT be flagged message_publish; matches=%+v", matches)
		}
	}
}

// Bare `handler.sendMessage(m)` on a non-emitter receiver must NOT be flagged
// (guards against re-introducing the unscoped `.sendMessage(` alternative).
func TestSniffEffectsKotlin_MessagePublish_BareSendMessageNotFlagged(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinMsgPublishFixture)
	for _, m := range matches {
		if m.Function == "notifyHandler" && m.Effect == EffectMessagePublish {
			t.Fatalf("notifyHandler uses a non-emitter handler.sendMessage and must NOT be flagged message_publish; matches=%+v", matches)
		}
	}
}

// ---------------------------------------------------------------------------
// Adversarial-review follow-ups (#5782 ask #4 review):
//   F1 nested generics Emitter<Message<T>>, F2 fully-qualified type name,
//   F6 empty-field-set guard. Realistic SmallRye shapes.
// ---------------------------------------------------------------------------

// Emitter<Message<T>> — the most idiomatic SmallRye publisher shape (Message
// wrapping for metadata/ack). Finding 1: the `<[^>]*>` regex stops at the
// first `>` and misses this. Both Java and Kotlin.
const javaNestedGenericEmitterFixture = `
package io.triage.ai;

import org.eclipse.microprofile.reactive.messaging.Channel;
import org.eclipse.microprofile.reactive.messaging.Emitter;
import org.eclipse.microprofile.reactive.messaging.Message;

public class TriageTools {

    @Channel("offer-assign-out")
    Emitter<Message<TriageActionEvent>> offerAssignOut;

    public void offerAssign(TriageActionEvent action) {
        offerAssignOut.send(Message.of(action));
    }
}
`

func TestSniffEffectsJava_MessagePublish_NestedGenericEmitterField(t *testing.T) {
	matches := sniffEffectsJava(javaNestedGenericEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "offerAssign" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish on offerAssign (Emitter<Message<T>> nested-generic field), matches=%+v", matches)
	}
}

const kotlinNestedGenericEmitterFixture = `
package io.triage.ai

import org.eclipse.microprofile.reactive.messaging.Channel
import org.eclipse.microprofile.reactive.messaging.Emitter
import org.eclipse.microprofile.reactive.messaging.Message

class TriageTools {

    @Channel("offer-assign-out")
    lateinit var offerAssignOut: Emitter<Message<TriageActionEvent>>

    fun offerAssign(action: TriageActionEvent) {
        offerAssignOut.send(Message.of(action))
    }
}
`

func TestSniffEffectsKotlin_MessagePublish_NestedGenericEmitterField(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinNestedGenericEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "offerAssign" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish on offerAssign (Emitter<Message<T>> nested-generic field), matches=%+v", matches)
	}
}

// Fully-qualified type name. Finding 2: Kotlin's regex anchored on
// `:\s*(?:Emitter|MutinyEmitter)` misses a qualified path; harmonize with
// Java (which already tolerates the floating type name).
const javaFQNEmitterFixture = `
package io.triage.ai;

public class TriageTools {

    @org.eclipse.microprofile.reactive.messaging.Channel("orders-out")
    org.eclipse.microprofile.reactive.messaging.Emitter<OrderEvent> ordersOut;

    public void publish(OrderEvent e) {
        ordersOut.send(e);
    }
}
`

func TestSniffEffectsJava_MessagePublish_FQNEmitterField(t *testing.T) {
	matches := sniffEffectsJava(javaFQNEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publish" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish on publish (FQN Emitter field), matches=%+v", matches)
	}
}

const kotlinFQNEmitterFixture = `
package io.triage.ai

class TriageTools {

    @org.eclipse.microprofile.reactive.messaging.Channel("orders-out")
    lateinit var ordersOut: org.eclipse.microprofile.reactive.messaging.Emitter<OrderEvent>

    fun publish(e: OrderEvent) {
        ordersOut.send(e)
    }
}
`

func TestSniffEffectsKotlin_MessagePublish_FQNEmitterField(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinFQNEmitterFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publish" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish on publish (FQN Emitter field), matches=%+v", matches)
	}
}

// Finding 6: empty-field-set guard. A `.send(` call with NO Emitter field
// declared anywhere in the file must NOT be flagged — proves the dynamic
// field regex is skipped when the pre-scan collects nothing.
const javaNoEmitterFieldFixture = `
package io.triage.ai;

public class BusPublisher {

    private final EventBus bus = new EventBus();

    public void publish(Object e) {
        bus.send(e);
    }
}
`

func TestSniffEffectsJava_MessagePublish_NoEmitterFieldNotFlagged(t *testing.T) {
	matches := sniffEffectsJava(javaNoEmitterFieldFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("no Emitter/MutinyEmitter field is declared — bus.send must NOT be flagged message_publish; matches=%+v", matches)
		}
	}
}

const kotlinNoEmitterFieldFixture = `
package io.triage.ai

class BusPublisher {

    private val bus = EventBus()

    fun publish(e: Any) {
        bus.send(e)
    }
}
`

func TestSniffEffectsKotlin_MessagePublish_NoEmitterFieldNotFlagged(t *testing.T) {
	matches := sniffEffectsKotlin(kotlinNoEmitterFieldFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("no Emitter/MutinyEmitter field is declared — bus.send must NOT be flagged message_publish; matches=%+v", matches)
		}
	}
}
