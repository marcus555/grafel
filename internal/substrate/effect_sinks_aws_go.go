// AWS SDK publish-effect sniffer for Go (#5798 Tier 1, mirrors the Java
// pilot #5797 PR1).
//
// Go's raw AWS SDK usage is typically a thin wrapper method around the SDK
// v1 *WithContext calls (or the bare v1/v2 call without context):
//
//   - SNS   (aws.sns.publish)        — <recv>.Publish(...) / .PublishWithContext(...)
//   - SQS   (aws.sqs.send)           — <recv>.SendMessage[Batch][WithContext](...)
//   - Kinesis (aws.kinesis.put)      — <recv>.PutRecord[s][WithContext](...)
//   - EventBridge (aws.eventbridge.putevents) — <recv>.PutEvents[WithContext](...)
//
// Precision gate: the AWS matchers only run when the file contains an
// AWS SDK import path (v1 github.com/aws/aws-sdk-go/service/{sns,sqs,
// kinesis,eventbridge}, v2 github.com/aws-sdk-go-v2/service/...) or a
// concrete client type (*sns.SNS, *sqs.SQS, *kinesis.Kinesis,
// *eventbridge.EventBridge, or the v2 .Client types). This is
// INDEPENDENT of the method names matched below (Publish, SendMessage,
// PutRecord[s], PutEvents) — a non-AWS wrapper method that happens to be
// named PublishWithContext/SendMessage in a file with no AWS SDK
// evidence must not pass the gate (mirrors review FIX 1, #5797 PR1).
//
// Method attribution reuses the shared scanGoFuncHeaders / nearestHeader /
// appendGoMatches scaffolding from effect_sinks_golang.go.
package substrate

import "regexp"

// goAWSGateRe detects INDEPENDENT AWS-SDK evidence in a file: a v1/v2 SDK
// import path for one of the four services, or a concrete client type.
var goAWSGateRe = regexp.MustCompile(
	`github\.com/aws/aws-sdk-go/service/(?:sns|sqs|kinesis|eventbridge)\b` +
		`|github\.com/aws-sdk-go-v2/service/(?:sns|sqs|kinesis|eventbridge)\b` +
		`|\b(?:sns\.SNS|sqs\.SQS|kinesis\.Kinesis|eventbridge\.EventBridge|sns\.Client|sqs\.Client|kinesis\.Client|eventbridge\.Client)\b`,
)

// Each per-sink matcher has TWO arms — a service-hinted receiver arm and an
// AWS-namespaced Input-type arm — so the file-level import gate is NOT the
// only precision guard (coordinator review FIX): a file that legitimately
// imports one AWS service and ALSO defines an unrelated abstraction with a
// colliding method name (e.g. an in-process `EventBus.Notify` calling
// `subscribers[0].Publish(metric)`) must not be misflagged. This mirrors the
// Java pilot's receiver-hint + receiver-free-constructor two-arm shape.
//
//  1. Receiver-hint arm: `\w*[Ss]ns\w*\s*\.\s*Publish...` — a receiver name
//     containing the service token (snsClient, _sns, snsSvc).
//  2. Input-type arm: `&?sns\.PublishInput\b` — the AWS-namespaced SDK input
//     struct being constructed at the call site. This recovers the common
//     shape where the receiver is a generically-named DI field
//     (`t.client.PublishWithContext(ctx, &sns.PublishInput{...})`) that the
//     receiver-hint alone would miss; the SDK-qualified `sns.PublishInput`
//     type name is strong, independent AWS evidence (a non-AWS DTO is never
//     `sns.PublishInput`).

// goAWSSNSRe matches SNS Publish call-sites (v1+v2, +WithContext).
var goAWSSNSRe = regexp.MustCompile(
	`\b\w*[Ss]ns\w*\s*\.\s*Publish(?:WithContext)?\s*\(` +
		`|&?\bsns\.PublishInput\b`,
)

// goAWSSQSRe matches SQS SendMessage/SendMessageBatch call-sites
// (v1+v2, +WithContext).
var goAWSSQSRe = regexp.MustCompile(
	`\b\w*[Ss]qs\w*\s*\.\s*SendMessage(?:Batch)?(?:WithContext)?\s*\(` +
		`|&?\bsqs\.SendMessage(?:Batch)?Input\b`,
)

// goAWSKinesisRe matches Kinesis PutRecord/PutRecords call-sites
// (v1+v2, +WithContext).
var goAWSKinesisRe = regexp.MustCompile(
	`\b\w*[Kk]inesis\w*\s*\.\s*PutRecords?(?:WithContext)?\s*\(` +
		`|&?\bkinesis\.PutRecords?Input\b`,
)

// goAWSEventBridgeRe matches EventBridge PutEvents call-sites (+WithContext).
var goAWSEventBridgeRe = regexp.MustCompile(
	`\b\w*(?:events?|eb|bridge)\w*\s*\.\s*PutEvents(?:WithContext)?\s*\(` +
		`|&?\beventbridge\.PutEventsInput\b`,
)

// appendAWSGoMatches runs the AWS publish matchers over content and appends
// any matches to out. Gated on goAWSGateRe — returns out unmodified when
// the file has no AWS SDK token at all.
func appendAWSGoMatches(out []EffectMatch, content string, headers []funcHeader) []EffectMatch {
	if !goAWSGateRe.MatchString(content) {
		return out
	}
	var aws []EffectMatch
	aws = appendGoMatches(aws, content, headers, goAWSSNSRe, EffectMessagePublish, "aws.sns.publish", 0.9)
	aws = appendGoMatches(aws, content, headers, goAWSSQSRe, EffectMessagePublish, "aws.sqs.send", 0.9)
	aws = appendGoMatches(aws, content, headers, goAWSKinesisRe, EffectMessagePublish, "aws.kinesis.put", 0.9)
	aws = appendGoMatches(aws, content, headers, goAWSEventBridgeRe, EffectMessagePublish, "aws.eventbridge.putevents", 0.9)

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
