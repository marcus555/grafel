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
