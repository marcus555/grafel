// Tests for AWS SDK publish-effect detection (#5797 PR1). Mirrors the
// SmallRye message_publish sniffer pattern (see
// effect_sinks_message_publish_test.go) but targets AWS SNS/SQS/Kinesis/
// EventBridge publish call-sites, which currently get no message_publish
// effect at all (#5782 only covers SmallRye). Synthetic fixtures only — no
// live corpus.
package substrate

import "testing"

const javaAWSSnsV2Fixture = `
package com.example.orders;

import software.amazon.awssdk.services.sns.SnsClient;
import software.amazon.awssdk.services.sns.model.PublishRequest;

public class OrderNotifier {

    private final SnsClient snsClient;
    private static final String TOPIC_ARN = "arn:aws:sns:us-east-1:123456789012:orders";

    public void publishOrder(Order order) {
        snsClient.publish(PublishRequest.builder().topicArn(TOPIC_ARN).message(order.toString()).build());
    }

    public int addTwo(int a, int b) {
        return a + b;
    }
}
`

func TestSniffEffectsJava_AWS_SNSPublishV2(t *testing.T) {
	matches := sniffEffectsJava(javaAWSSnsV2Fixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishOrder" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sns.publish" {
				t.Errorf("publishOrder sink = %q, want aws.sns.publish", m.Sink)
			}
			if m.Confidence != 0.9 {
				t.Errorf("publishOrder confidence = %v, want 0.9", m.Confidence)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishOrder (SNS v2 client.publish), matches=%+v", matches)
	}
}

const javaAWSSnsV1Fixture = `
package com.example.orders;

import com.amazonaws.services.sns.AmazonSNSClient;
import com.amazonaws.services.sns.model.PublishRequest;

public class OrderNotifier {

    public void publishOrder(String arn, String msg) {
        new AmazonSNSClient().publish(new PublishRequest(arn, msg, "subject"));
    }
}
`

func TestSniffEffectsJava_AWS_SNSPublishV1(t *testing.T) {
	matches := sniffEffectsJava(javaAWSSnsV1Fixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishOrder" && m.Effect == EffectMessagePublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishOrder (SNS v1 AmazonSNSClient.publish(new PublishRequest(...))), matches=%+v", matches)
	}
}

const javaAWSSqsFixture = `
package com.example.orders;

import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.SendMessageRequest;
import software.amazon.awssdk.services.sqs.model.SendMessageBatchRequest;

public class QueueNotifier {

    private final SqsClient sqsClient;

    public void enqueueOrder(String body) {
        sqsClient.sendMessage(SendMessageRequest.builder().queueUrl(QUEUE_URL).messageBody(body).build());
    }

    public void enqueueOrders(SendMessageBatchRequest req) {
        sqsClient.sendMessageBatch(req);
    }
}
`

func TestSniffEffectsJava_AWS_SQSSendMessage(t *testing.T) {
	matches := sniffEffectsJava(javaAWSSqsFixture)
	found := false
	for _, m := range matches {
		if m.Function == "enqueueOrder" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("enqueueOrder sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on enqueueOrder (SQS sendMessage), matches=%+v", matches)
	}
}

func TestSniffEffectsJava_AWS_SQSSendMessageBatch(t *testing.T) {
	matches := sniffEffectsJava(javaAWSSqsFixture)
	found := false
	for _, m := range matches {
		if m.Function == "enqueueOrders" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("enqueueOrders sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on enqueueOrders (SQS sendMessageBatch), matches=%+v", matches)
	}
}

const javaAWSKinesisFixture = `
package com.example.orders;

import software.amazon.awssdk.services.kinesis.KinesisClient;
import software.amazon.awssdk.services.kinesis.model.PutRecordRequest;
import software.amazon.awssdk.services.kinesis.model.PutRecordsRequest;

public class StreamPublisher {

    private final KinesisClient kinesisClient;

    public void putOne(PutRecordRequest req) {
        kinesisClient.putRecord(req);
    }

    public void putMany(PutRecordsRequest req) {
        kinesisClient.putRecords(req);
    }
}
`

func TestSniffEffectsJava_AWS_KinesisPutRecord(t *testing.T) {
	matches := sniffEffectsJava(javaAWSKinesisFixture)
	found := false
	for _, m := range matches {
		if m.Function == "putOne" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.kinesis.put" {
				t.Errorf("putOne sink = %q, want aws.kinesis.put", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on putOne (Kinesis putRecord), matches=%+v", matches)
	}
}

func TestSniffEffectsJava_AWS_KinesisPutRecords(t *testing.T) {
	matches := sniffEffectsJava(javaAWSKinesisFixture)
	found := false
	for _, m := range matches {
		if m.Function == "putMany" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.kinesis.put" {
				t.Errorf("putMany sink = %q, want aws.kinesis.put", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on putMany (Kinesis putRecords), matches=%+v", matches)
	}
}

const javaAWSEventBridgeFixture = `
package com.example.orders;

import software.amazon.awssdk.services.eventbridge.EventBridgeClient;
import software.amazon.awssdk.services.eventbridge.model.PutEventsRequest;

public class EventPublisher {

    private final EventBridgeClient eventBridgeClient;

    public void publishEvent(PutEventsRequest req) {
        eventBridgeClient.putEvents(req);
    }
}
`

func TestSniffEffectsJava_AWS_EventBridgePutEvents(t *testing.T) {
	matches := sniffEffectsJava(javaAWSEventBridgeFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishEvent" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.eventbridge.putevents" {
				t.Errorf("publishEvent sink = %q, want aws.eventbridge.putevents", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishEvent (EventBridge putEvents), matches=%+v", matches)
	}
}

const javaAWSAsyncFixture = `
package com.example.orders;

import software.amazon.awssdk.services.sns.SnsAsyncClient;
import software.amazon.awssdk.services.sqs.SqsAsyncClient;
import software.amazon.awssdk.services.kinesis.KinesisAsyncClient;
import software.amazon.awssdk.services.eventbridge.EventBridgeAsyncClient;

public class AsyncPublisher {

    private final SnsAsyncClient snsClient;
    private final SqsAsyncClient sqsClient;
    private final KinesisAsyncClient kinesisClient;
    private final EventBridgeAsyncClient eventBridgeClient;

    public void publishAsyncSns(PublishRequest req) {
        snsClient.publishAsync(req);
    }

    public void sendAsyncSqs(SendMessageRequest req) {
        sqsClient.sendMessageAsync(req);
    }

    public void putRecordsAsyncKinesis(PutRecordsRequest req) {
        kinesisClient.putRecordsAsync(req);
    }

    public void putEventsAsyncBridge(PutEventsRequest req) {
        eventBridgeClient.putEventsAsync(req);
    }
}
`

func TestSniffEffectsJava_AWS_AsyncVariants(t *testing.T) {
	matches := sniffEffectsJava(javaAWSAsyncFixture)
	want := map[string]string{
		"publishAsyncSns":        "aws.sns.publish",
		"sendAsyncSqs":           "aws.sqs.send",
		"putRecordsAsyncKinesis": "aws.kinesis.put",
		"putEventsAsyncBridge":   "aws.eventbridge.putevents",
	}
	got := map[string]bool{}
	for _, m := range matches {
		if m.Effect != EffectMessagePublish {
			continue
		}
		if wantSink, ok := want[m.Function]; ok {
			if m.Sink != wantSink {
				t.Errorf("%s sink = %q, want %q", m.Function, m.Sink, wantSink)
			}
			got[m.Function] = true
		}
	}
	for fn := range want {
		if !got[fn] {
			t.Errorf("expected message_publish effect on %s (async variant), matches=%+v", fn, matches)
		}
	}
}

// PRECISION: a file with NO AWS SDK token anywhere must short-circuit the
// AWS matchers entirely — a bare `.sendMessage(`/`.publish(` shape must not
// collide with SmallRye/JMS/Android/chat-SDK usages.
const javaNonAWSFixture = `
package com.example.orders;

public class NotificationHandler {

    private final MessageRepository repository;

    public void notify(Object m) {
        handler.sendMessage(m);
    }

    public void save(Object x) {
        repository.save(x);
    }
}
`

func TestSniffEffectsJava_AWS_NonAWSNotFlagged(t *testing.T) {
	matches := sniffEffectsJava(javaNonAWSFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("file has no AWS SDK token — AWS gate must short-circuit; must NOT be flagged message_publish; matches=%+v", matches)
		}
	}
}

// PRECISION (review FIX 1): a non-AWS file whose only "AWS-looking" tokens
// are DTOs that happen to share a request-class name (`SendMessageRequest`,
// `PublishRequest`) — with NO AWS SDK import or client type — must NOT pass
// the gate. The gate is independent AWS evidence, not the matcher's own
// constructor targets.
const javaFalseRequestNamesFixture = `
package com.example.chat;

public class ChatService {

    public void relay(String userId, String text) {
        SendMessageRequest r = new SendMessageRequest(userId, text);
        PublishRequest p = new PublishRequest(userId, text);
        bus.dispatch(r, p);
    }
}
`

func TestSniffEffectsJava_AWS_RequestClassNameWithoutSDKNotFlagged(t *testing.T) {
	matches := sniffEffectsJava(javaFalseRequestNamesFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("file has non-AWS DTOs named like AWS request classes but NO SDK import/client — gate must short-circuit; must NOT be flagged; matches=%+v", matches)
		}
	}
}

// And the flip side: a genuine AWS file (client type present) still flags
// the constructor form once the gate passes on independent evidence.
const javaGenuineAWSConstructorFixture = `
package com.example.orders;

import com.amazonaws.services.sqs.AmazonSQS;

public class QueueNotifier {

    private final AmazonSQS sqs;

    public void enqueue(String url, String body) {
        SendMessageRequest req = new SendMessageRequest(url, body);
        sqs.sendMessage(req);
    }
}
`

func TestSniffEffectsJava_AWS_GenuineConstructorStillFlagged(t *testing.T) {
	matches := sniffEffectsJava(javaGenuineAWSConstructorFixture)
	found := false
	for _, m := range matches {
		if m.Function == "enqueue" && m.Effect == EffectMessagePublish && m.Sink == "aws.sqs.send" {
			found = true
		}
	}
	if !found {
		t.Fatalf("genuine AWS file (AmazonSQS client present) must still flag SendMessageRequest/sendMessage; matches=%+v", matches)
	}
}

// SINGLE-EMISSION (review FIX 2): the idiomatic SNS v2 call-site
// `snsClient.publish(PublishRequest.builder()...)` matches both the receiver
// arm and the builder arm; dedup must collapse them to exactly ONE
// aws.sns.publish EffectMatch for that method+line.
func TestSniffEffectsJava_AWS_SNSPublishV2SingleEmission(t *testing.T) {
	matches := sniffEffectsJava(javaAWSSnsV2Fixture)
	count := 0
	for _, m := range matches {
		if m.Function == "publishOrder" && m.Effect == EffectMessagePublish && m.Sink == "aws.sns.publish" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly ONE aws.sns.publish effect on publishOrder (dedup receiver+builder arms), got %d; matches=%+v", count, matches)
	}
}

// MethodGranularity: two methods, only the one containing the publish call
// gets the effect.
const javaAWSMethodGranularityFixture = `
package com.example.orders;

import software.amazon.awssdk.services.sns.SnsClient;
import software.amazon.awssdk.services.sns.model.PublishRequest;

public class OrderNotifier {

    private final SnsClient snsClient;

    public void publishOrder(Order order) {
        snsClient.publish(PublishRequest.builder().topicArn(TOPIC_ARN).build());
    }

    public void computeTotal(Order order) {
        int total = order.getQuantity() * order.getPrice();
    }
}
`

func TestSniffEffectsJava_AWS_MethodGranularity(t *testing.T) {
	matches := sniffEffectsJava(javaAWSMethodGranularityFixture)
	sawPublish, sawCompute := false, false
	for _, m := range matches {
		if m.Effect != EffectMessagePublish {
			continue
		}
		switch m.Function {
		case "publishOrder":
			sawPublish = true
		case "computeTotal":
			sawCompute = true
		}
	}
	if !sawPublish {
		t.Fatalf("expected message_publish effect on publishOrder, matches=%+v", matches)
	}
	if sawCompute {
		t.Fatalf("computeTotal has no AWS publish call and must NOT be flagged message_publish; matches=%+v", matches)
	}
}

// REGRESSION: the existing SmallRye fixture's message_publish output must be
// unchanged by the AWS additions (append-only proof).
func TestSniffEffectsJava_AWS_SmallRyeStillWorks(t *testing.T) {
	matches := sniffEffectsJava(javaMsgPublishFixture)
	foundReceiverShape, foundFieldBased := false, false
	for _, m := range matches {
		if m.Function != "publishOrder" || m.Effect != EffectMessagePublish {
			continue
		}
		switch m.Sink {
		case "smallrye.Emitter.send/@Outgoing":
			foundReceiverShape = true
			if m.Confidence != 0.9 {
				t.Errorf("publishOrder (receiver-shape) confidence = %v, want 0.9 (unchanged)", m.Confidence)
			}
		case "smallrye.@Channel-field.send":
			foundFieldBased = true
			if m.Confidence != 0.9 {
				t.Errorf("publishOrder (field-based) confidence = %v, want 0.9 (unchanged)", m.Confidence)
			}
		}
	}
	if !foundReceiverShape {
		t.Fatalf("SmallRye regression: expected receiver-shape message_publish (smallrye.Emitter.send/@Outgoing) on publishOrder unchanged, matches=%+v", matches)
	}
	if !foundFieldBased {
		t.Fatalf("SmallRye regression: expected field-based message_publish (smallrye.@Channel-field.send) on publishOrder unchanged, matches=%+v", matches)
	}
	for _, m := range matches {
		if m.Function == "notifyHandler" && m.Effect == EffectMessagePublish {
			t.Fatalf("SmallRye regression: notifyHandler must still NOT be flagged, matches=%+v", matches)
		}
	}
}
