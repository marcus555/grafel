// Tests for the Node/TS AWS SDK publish-effect sniffer (#5798 Tier 1).
// Mirrors effect_sinks_aws_test.go (Java pilot, #5797 PR1) but targets
// @aws-sdk v3 `client.send(new XCommand(...))` call-sites (and the v2
// `sns.publish(...)`/`sqs.sendMessage(...)` shape), keyed off an
// @aws-sdk/client-* or aws-sdk import. Synthetic fixtures only — no live
// corpus (mirrors gap7_node_v3_and_dotnet_producers.md).
package substrate

import "testing"

const jstsAWSSQSKinesisFixture = `
import { SQSClient, SendMessageCommand, SendMessageBatchCommand } from "@aws-sdk/client-sqs";
import { KinesisClient, PutRecordsCommand } from "@aws-sdk/client-kinesis";

const sqs = new SQSClient({});
export async function enqueue(queueUrl, body) {
  await sqs.send(new SendMessageCommand({ QueueUrl: queueUrl, MessageBody: body }));
}
export async function enqueueBatch(queueUrl, entries) {
  await sqs.send(new SendMessageBatchCommand({ QueueUrl: queueUrl, Entries: entries }));
}

const kinesis = new KinesisClient({});
export async function putBatch(streamName, records) {
  await kinesis.send(new PutRecordsCommand({ StreamName: streamName, Records: records }));
}
`

func TestSniffEffectsJSTS_AWS_SQSSendMessageCommand(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSSQSKinesisFixture)
	found := false
	for _, m := range matches {
		if m.Function == "enqueue" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("enqueue sink = %q, want aws.sqs.send", m.Sink)
			}
			if m.Confidence != 0.9 {
				t.Errorf("enqueue confidence = %v, want 0.9", m.Confidence)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on enqueue (SQS SendMessageCommand via send), matches=%+v", matches)
	}
}

func TestSniffEffectsJSTS_AWS_SQSSendMessageBatchCommand(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSSQSKinesisFixture)
	found := false
	for _, m := range matches {
		if m.Function == "enqueueBatch" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("enqueueBatch sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on enqueueBatch (SQS SendMessageBatchCommand), matches=%+v", matches)
	}
}

func TestSniffEffectsJSTS_AWS_KinesisPutRecordsCommand(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSSQSKinesisFixture)
	found := false
	for _, m := range matches {
		if m.Function == "putBatch" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.kinesis.put" {
				t.Errorf("putBatch sink = %q, want aws.kinesis.put", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on putBatch (Kinesis PutRecordsCommand), matches=%+v", matches)
	}
}

const jstsAWSSNSEventBridgeFixture = `
import { SNSClient, PublishCommand } from "@aws-sdk/client-sns";
import { EventBridgeClient, PutEventsCommand } from "@aws-sdk/client-eventbridge";

const sns = new SNSClient({});
export async function publishOrder(topicArn, message) {
  await sns.send(new PublishCommand({ TopicArn: topicArn, Message: message }));
}

const eventBridge = new EventBridgeClient({});
export async function publishEvent(busName, detail) {
  await eventBridge.send(new PutEventsCommand({ Entries: [{ EventBusName: busName, Detail: detail }] }));
}
`

func TestSniffEffectsJSTS_AWS_SNSPublishCommand(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSSNSEventBridgeFixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishOrder" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sns.publish" {
				t.Errorf("publishOrder sink = %q, want aws.sns.publish", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishOrder (SNS PublishCommand), matches=%+v", matches)
	}
}

func TestSniffEffectsJSTS_AWS_EventBridgePutEventsCommand(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSSNSEventBridgeFixture)
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
		t.Fatalf("expected message_publish effect on publishEvent (EventBridge PutEventsCommand), matches=%+v", matches)
	}
}

// v2 SDK shapes: sns.publish(...).promise() / sqs.sendMessage(...).
const jstsAWSv2Fixture = `
const AWS = require("aws-sdk");
const sns = new AWS.SNS();
const sqs = new AWS.SQS();

async function publishLegacy(topicArn, message) {
  await sns.publish({ TopicArn: topicArn, Message: message }).promise();
}

async function enqueueLegacy(queueUrl, body) {
  await sqs.sendMessage({ QueueUrl: queueUrl, MessageBody: body }).promise();
}
`

func TestSniffEffectsJSTS_AWS_V2SNSPublish(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSv2Fixture)
	found := false
	for _, m := range matches {
		if m.Function == "publishLegacy" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sns.publish" {
				t.Errorf("publishLegacy sink = %q, want aws.sns.publish", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on publishLegacy (v2 sns.publish), matches=%+v", matches)
	}
}

func TestSniffEffectsJSTS_AWS_V2SQSSendMessage(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSv2Fixture)
	found := false
	for _, m := range matches {
		if m.Function == "enqueueLegacy" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("enqueueLegacy sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on enqueueLegacy (v2 sqs.sendMessage), matches=%+v", matches)
	}
}

// PRECISION: a file with NO @aws-sdk/aws-sdk import must short-circuit — a
// non-AWS type named SendMessageCommand/PublishInput must not be flagged.
const jstsNonAWSFixture = `
class SendMessageCommand {
  constructor(userId, text) {
    this.userId = userId;
    this.text = text;
  }
}

function relay(userId, text) {
  const cmd = new SendMessageCommand(userId, text);
  bus.dispatch(cmd);
}
`

func TestSniffEffectsJSTS_AWS_NonAWSNotFlagged(t *testing.T) {
	matches := sniffEffectsJSTS(jstsNonAWSFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("file has no @aws-sdk/aws-sdk import — AWS gate must short-circuit; must NOT be flagged; matches=%+v", matches)
		}
	}
}

// SINGLE-EMISSION: the idiomatic `sqs.send(new SendMessageCommand({...}))`
// call-site must yield exactly ONE aws.sqs.send EffectMatch (command arm +
// send-wrapped arm dedup).
func TestSniffEffectsJSTS_AWS_SQSSingleEmission(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSSQSKinesisFixture)
	count := 0
	for _, m := range matches {
		if m.Function == "enqueue" && m.Effect == EffectMessagePublish && m.Sink == "aws.sqs.send" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly ONE aws.sqs.send effect on enqueue, got %d; matches=%+v", count, matches)
	}
}

// REGRESSION: the existing JS/TS HTTP sniffer must be unaffected by the AWS
// additions (append-only proof).
const jstsAWSRegressionFixture = `
import { SQSClient, SendMessageCommand } from "@aws-sdk/client-sqs";

async function fetchThing(url) {
  return fetch(url);
}

const sqs = new SQSClient({});
async function enqueue(queueUrl, body) {
  await sqs.send(new SendMessageCommand({ QueueUrl: queueUrl, MessageBody: body }));
}
`

func TestSniffEffectsJSTS_AWS_HTTPRegressionUnaffected(t *testing.T) {
	matches := sniffEffectsJSTS(jstsAWSRegressionFixture)
	found := false
	for _, m := range matches {
		if m.Function == "fetchThing" && m.Effect == EffectHTTPOut {
			found = true
		}
	}
	if !found {
		t.Fatalf("regression: fetchThing must still be flagged http_out, matches=%+v", matches)
	}
}
