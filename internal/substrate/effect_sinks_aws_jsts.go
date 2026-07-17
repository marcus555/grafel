// AWS SDK publish-effect sniffer for Node/TS (#5798 Tier 1, mirrors the
// Java pilot #5797 PR1).
//
// @aws-sdk v3 publish call-sites hand a Command object to a generic
// `client.send(...)` — the producing semantics live in the Command TYPE,
// not the `send` method name — plus the legacy v2 `sns.publish(...)`/
// `sqs.sendMessage(...)` method-call shape:
//
//   - SNS   (aws.sns.publish)        — .send(new PublishCommand(...)) / v2 <sns>.publish(...)
//   - SQS   (aws.sqs.send)           — .send(new SendMessage[Batch]Command(...)) / v2 <sqs>.sendMessage[Batch](...)
//   - Kinesis (aws.kinesis.put)      — .send(new PutRecord[s]Command(...)) / v2 <kinesis>.putRecord[s](...)
//   - EventBridge (aws.eventbridge.putevents) — .send(new PutEventsCommand(...))
//
// Precision gate: the AWS matchers only run when the file imports one of
// the @aws-sdk/client-{sns,sqs,kinesis,eventbridge} v3 packages, or the
// legacy `aws-sdk` v2 package (or constructs `new AWS.<Service>(`). This is
// INDEPENDENT of the Command class names matched below — a non-AWS class
// named SendMessageCommand in a file with no @aws-sdk/aws-sdk import must
// not pass the gate (mirrors review FIX 1, #5797 PR1).
//
// Each sink has TWO arms mirroring the Java pilot's receiver+constructor
// shape: a `new XCommand(` constructor arm (v3, catches the command
// regardless of `send()` wrapping) and, for the idiomatic
// `<client>.send(new XCommand(...))` call, the same construct also matches
// the send-wrapped variant of the arm — these dedup to a single emission
// (review FIX 2, #5797 PR1). The v2 method-call arm requires a
// service-hinted receiver name (mirrors the Java SNS/SQS/Kinesis/
// EventBridge receiver-name heuristic) since bare `.publish(`/
// `.sendMessage(` collide with non-AWS publishers.
//
// Method attribution reuses the shared scanJSTSFuncHeaders / nearestHeader
// / appendJSTSMatches scaffolding from effect_sinks_jsts.go.
package substrate

import "regexp"

// jstsAWSGateRe detects INDEPENDENT AWS-SDK evidence in a file: an
// @aws-sdk/client-* v3 import, the legacy aws-sdk v2 package import/require,
// or a `new AWS.<Service>(` construction.
var jstsAWSGateRe = regexp.MustCompile(
	`@aws-sdk/client-(?:sns|sqs|kinesis|eventbridge)\b` +
		`|['"]aws-sdk['"]` +
		`|\bnew\s+AWS\s*\.\s*(?:SNS|SQS|Kinesis|EventBridge)\s*\(`,
)

// jstsAWSSNSRe matches SNS publish call-sites: v3 `new PublishCommand(`
// (typically inside `.send(...)`) and v2 `<sns-hinted receiver>.publish(`.
var jstsAWSSNSRe = regexp.MustCompile(
	`\bnew\s+PublishCommand\s*\(` +
		`|\b\w*[Ss]ns\w*\s*\.\s*publish\s*\(`,
)

// jstsAWSSQSRe matches SQS sendMessage/sendMessageBatch call-sites: v3
// `new SendMessage[Batch]Command(` and v2
// `<sqs-hinted receiver>.sendMessage[Batch](`.
var jstsAWSSQSRe = regexp.MustCompile(
	`\bnew\s+SendMessage(?:Batch)?Command\s*\(` +
		`|\b\w*[Ss]qs\w*\s*\.\s*sendMessage(?:Batch)?\s*\(`,
)

// jstsAWSKinesisRe matches Kinesis putRecord/putRecords call-sites: v3
// `new PutRecord[s]Command(` and v2
// `<kinesis-hinted receiver>.putRecord[s](`.
var jstsAWSKinesisRe = regexp.MustCompile(
	`\bnew\s+PutRecords?Command\s*\(` +
		`|\b\w*[Kk]inesis\w*\s*\.\s*putRecords?\s*\(`,
)

// jstsAWSEventBridgeRe matches EventBridge putEvents call-sites: v3
// `new PutEventsCommand(` and v2
// `<eventbridge-hinted receiver>.putEvents(`.
var jstsAWSEventBridgeRe = regexp.MustCompile(
	`\bnew\s+PutEventsCommand\s*\(` +
		`|\b\w*(?:events?|eb|bridge)\w*\s*\.\s*putEvents\s*\(`,
)

// appendAWSJSTSMatches runs the AWS publish matchers over content and
// appends any matches to out. Gated on jstsAWSGateRe — returns out
// unmodified when the file has no AWS SDK token at all.
func appendAWSJSTSMatches(out []EffectMatch, content string, headers []funcHeader) []EffectMatch {
	if !jstsAWSGateRe.MatchString(content) {
		return out
	}
	var aws []EffectMatch
	aws = appendJSTSMatches(aws, content, headers, jstsAWSSNSRe, EffectMessagePublish, "aws.sns.publish", 0.9)
	aws = appendJSTSMatches(aws, content, headers, jstsAWSSQSRe, EffectMessagePublish, "aws.sqs.send", 0.9)
	aws = appendJSTSMatches(aws, content, headers, jstsAWSKinesisRe, EffectMessagePublish, "aws.kinesis.put", 0.9)
	aws = appendJSTSMatches(aws, content, headers, jstsAWSEventBridgeRe, EffectMessagePublish, "aws.eventbridge.putevents", 0.9)

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
