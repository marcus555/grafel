// AWS SDK publish-effect sniffer (#5797 PR1).
//
// grafel currently only emits the message_publish effect for SmallRye
// reactive-messaging publish sites (#5782). This file adds a parallel,
// append-only matcher set for AWS SDK publish call-sites so every AWS
// publish site becomes first-class in the effect lattice regardless of
// target resolution:
//
//   - SNS   (aws.sns.publish)        — <receiver>.publish(...) / new
//     PublishRequest(...) / PublishRequest.builder(), v1+v2, +Async.
//   - SQS   (aws.sqs.send)           — <receiver>.sendMessage[Batch]
//     [Async](...) / new SendMessage[Batch]Request.
//   - Kinesis (aws.kinesis.put)      — <receiver>.putRecord[s][Async](...)
//     / new PutRecord[s]Request.
//   - EventBridge (aws.eventbridge.putevents) — <receiver>.putEvents
//     [Async](...) / new PutEventsRequest / new PutEventsCommand.
//
// Precision gate: the AWS matchers only run when the file contains at
// least one AWS SDK token (client type name or request/command type name).
// This is required because bare `.publish(`/`.send(`/`.sendMessage(` shapes
// otherwise collide with SmallRye reactive-messaging, JMS, Android
// Handler, and chat-SDK publishers (see effect_sinks_java.go's
// javaMsgPublishRe doc comment for the same concern in that sniffer).
//
// Method attribution reuses the shared scanJavaFuncHeaders / nearestHeader
// / appendJavaMatches scaffolding from effect_sinks_java.go — identical
// enclosing-method binding, control-keyword rejection, and deterministic
// source-order output as every other Java effect sniffer.
package substrate

import "regexp"

// javaAWSGateRe detects INDEPENDENT AWS-SDK evidence in a file: an AWS SDK
// import path (v2 `software.amazon.awssdk`, v1 `com.amazonaws`) or one of
// the concrete client types. It deliberately does NOT include the
// request/command class names (PublishRequest, SendMessageRequest,
// PutRecord[s]Request, PutEventsRequest) — those are the matchers' own
// constructor-arm targets, so gating on them would be self-gating: a
// non-AWS DTO that happens to be named `SendMessageRequest` would pass the
// gate AND match, producing a false aws.sqs.send (review FIX 1, #5797 PR1).
// A genuine AWS file always carries a client type or an SDK import, so the
// constructor arms still fire there.
var javaAWSGateRe = regexp.MustCompile(
	`software\.amazon\.awssdk` +
		`|\bcom\.amazonaws\b` +
		`|\b(?:SnsClient|AmazonSNS|SqsClient|AmazonSQS|KinesisClient|AmazonKinesis|EventBridgeClient|AmazonEventBridge)\b`,
)

// javaAWSSNSRe matches SNS publish call-sites: v1 (AmazonSNS.publish(new
// PublishRequest(...))) and v2 (snsClient.publish(...) /
// snsClient.publishAsync(...) with a PublishRequest.builder() argument).
// The receiver-shape half requires an identifier containing "sns" (any
// case), mirroring the SmallRye sniffer's receiver-name heuristic since
// there is no type table; the constructor/builder half is receiver-free
// and catches the request object being built regardless of client
// variable naming.
var javaAWSSNSRe = regexp.MustCompile(
	`\b\w*[Ss]ns\w*\s*\.\s*publish(?:Async)?\s*\(` +
		`|\bnew\s+PublishRequest\s*\(` +
		`|\bPublishRequest\s*\.\s*builder\s*\(\s*\)`,
)

// javaAWSSQSRe matches SQS sendMessage/sendMessageBatch call-sites (v1+v2,
// +Async) and the SendMessage[Batch]Request constructor.
var javaAWSSQSRe = regexp.MustCompile(
	`\b\w*[Ss]qs\w*\s*\.\s*sendMessage(?:Batch)?(?:Async)?\s*\(` +
		`|\bnew\s+SendMessage(?:Batch)?Request\b`,
)

// javaAWSKinesisRe matches Kinesis putRecord/putRecords call-sites
// (v1+v2, +Async) and the PutRecord[s]Request constructor.
var javaAWSKinesisRe = regexp.MustCompile(
	`\b\w*[Kk]inesis\w*\s*\.\s*putRecords?(?:Async)?\s*\(` +
		`|\bnew\s+PutRecords?Request\b`,
)

// javaAWSEventBridgeRe matches EventBridge putEvents call-sites (+Async)
// and the PutEventsRequest/PutEventsCommand constructor. The
// receiver-shape half accepts identifiers containing "event"/"events",
// "eb", or "bridge" (any case) since EventBridge client variables are
// commonly named eventBridgeClient, ebClient, or busClient.
var javaAWSEventBridgeRe = regexp.MustCompile(
	`\b\w*(?:events?|eb|bridge)\w*\s*\.\s*putEvents(?:Async)?\s*\(` +
		`|\bnew\s+PutEventsRequest\b` +
		`|\bnew\s+PutEventsCommand\b`,
)

// appendAWSJavaMatches runs the AWS publish matchers over content and
// appends any matches to out. Gated on javaAWSGateRe — returns out
// unmodified when the file has no AWS SDK token at all (precision:
// #5797 PR1).
func appendAWSJavaMatches(out []EffectMatch, content string, headers []funcHeader) []EffectMatch {
	if !javaAWSGateRe.MatchString(content) {
		return out
	}
	// Collect the AWS matches into a scratch slice first, then dedup by
	// (Function, Line, Sink) before appending. This is needed because the
	// idiomatic SNS v2 shape `snsClient.publish(PublishRequest.builder()...)`
	// matches BOTH the receiver `...sns....publish(` arm and the standalone
	// `PublishRequest.builder()` arm of javaAWSSNSRe on the same line, which
	// would otherwise emit two identical aws.sns.publish EffectMatch entries
	// (review FIX 2, #5797 PR1). Keeping both arms (rather than dropping the
	// builder arm) preserves recall for a builder call on a receiver whose
	// name does not contain "sns"; the dedup makes it single-emission.
	var aws []EffectMatch
	aws = appendJavaMatches(aws, content, headers, javaAWSSNSRe, EffectMessagePublish, "aws.sns.publish", 0.9)
	aws = appendJavaMatches(aws, content, headers, javaAWSSQSRe, EffectMessagePublish, "aws.sqs.send", 0.9)
	aws = appendJavaMatches(aws, content, headers, javaAWSKinesisRe, EffectMessagePublish, "aws.kinesis.put", 0.9)
	aws = appendJavaMatches(aws, content, headers, javaAWSEventBridgeRe, EffectMessagePublish, "aws.eventbridge.putevents", 0.9)

	type key struct {
		fn   string
		line int
		sink string
	}
	seen := make(map[key]bool, len(aws))
	for _, m := range aws {
		k := key{m.Function, m.Line, m.Sink}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, m)
	}
	return out
}
