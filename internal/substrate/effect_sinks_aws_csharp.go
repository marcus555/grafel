// AWS SDK publish-effect sniffer for C#/.NET (#5798 Tier 1, mirrors the
// Java pilot #5797 PR1).
//
// AWS SDK for .NET publish call-sites are async wrapper methods (*Async
// suffix), with the channel usually config-injected rather than a literal:
//
//   - SNS   (aws.sns.publish)        — <recv>.PublishAsync(...) / .PublishBatchAsync(...)
//   - SQS   (aws.sqs.send)           — <recv>.SendMessageAsync(...) / .SendMessageBatchAsync(...)
//   - Kinesis (aws.kinesis.put)      — <recv>.PutRecordAsync(...) / .PutRecordsAsync(...)
//   - EventBridge (aws.eventbridge.putevents) — <recv>.PutEventsAsync(...)
//     (PutEventsRequest.Entries is an array — still ONE emission per call-site)
//
// Precision gate: the AWS matchers only run when the file contains an
// `using Amazon.*` directive for one of the four services (SNS is
// Amazon.SimpleNotificationService; EventBridge is Amazon.EventBridge or
// the legacy Amazon.CloudWatchEvents namespace), or a concrete client
// interface/type (IAmazonSNS/AmazonSNSClient/IAmazonSQS/AmazonSQSClient/
// IAmazonKinesis/AmazonKinesisClient/IAmazonEventBridge/
// AmazonEventBridgeClient). This is INDEPENDENT of the request/method names
// matched below — a non-AWS type named PublishRequest/SendMessageRequest in
// a file with no Amazon.* using/client type must not pass the gate
// (mirrors review FIX 1, #5797 PR1).
//
// Method attribution reuses the shared scanCSharpFuncHeaders / nearestHeader
// / appendCSharpMatches scaffolding from effect_sinks_csharp.go.
package substrate

import "regexp"

// csharpAWSGateRe detects INDEPENDENT AWS-SDK evidence in a file: an
// Amazon.* using directive for one of the four services, or a concrete
// client interface/type.
var csharpAWSGateRe = regexp.MustCompile(
	`using\s+Amazon\s*\.\s*(?:SimpleNotificationService|SQS|Kinesis|EventBridge|CloudWatchEvents)\b` +
		`|\b(?:IAmazonSNS|AmazonSNSClient|IAmazonSQS|AmazonSQSClient|IAmazonKinesis|AmazonKinesisClient|IAmazonEventBridge|AmazonEventBridgeClient)\b`,
)

// Each per-sink matcher has TWO arms — a service-hinted receiver arm and an
// Amazon-namespace-qualified request-type arm — so the file-level using-gate
// is NOT the only precision guard (coordinator review FIX): a file that
// legitimately imports one AWS service and ALSO uses an unrelated in-process
// abstraction with a colliding method name (e.g. an event aggregator
// `_events.PublishAsync(evt)`) must not be misflagged. This mirrors the Java
// pilot's receiver-hint + receiver-free two-arm shape.
//
//  1. Receiver-hint arm: `\w*[Ss]ns\w*\s*\.\s*Publish...Async` — a receiver
//     name containing the service token (snsClient, _sns, _snsSvc).
//  2. Qualified-type arm: `Amazon.SimpleNotificationService.Model.PublishRequest`
//     — a fully-qualified AWS SDK request type, strong independent AWS
//     evidence for the case where the receiver is a generically-named DI
//     field (`_client.PublishAsync(new Amazon...Model.PublishRequest{...})`).
//     Idiomatic C# usually `using`-imports the request type (bare
//     `new PublishRequest`), which is UNSAFE to match (it collides with a
//     non-AWS DTO — the self-gating trap), so bare request names are NOT an
//     arm; the receiver-hint carries that common case.

// csharpAWSSNSRe matches SNS PublishAsync/PublishBatchAsync call-sites.
var csharpAWSSNSRe = regexp.MustCompile(
	`\b\w*[Ss]ns\w*\s*\.\s*Publish(?:Batch)?Async\s*\(` +
		`|\bAmazon\s*\.\s*SimpleNotificationService\s*\.\s*Model\s*\.\s*Publish(?:Batch)?Request\b`,
)

// csharpAWSSQSRe matches SQS SendMessageAsync/SendMessageBatchAsync
// call-sites.
var csharpAWSSQSRe = regexp.MustCompile(
	`\b\w*[Ss]qs\w*\s*\.\s*SendMessage(?:Batch)?Async\s*\(` +
		`|\bAmazon\s*\.\s*SQS\s*\.\s*Model\s*\.\s*SendMessage(?:Batch)?Request\b`,
)

// csharpAWSKinesisRe matches Kinesis PutRecordAsync/PutRecordsAsync
// call-sites.
var csharpAWSKinesisRe = regexp.MustCompile(
	`\b\w*[Kk]inesis\w*\s*\.\s*PutRecords?Async\s*\(` +
		`|\bAmazon\s*\.\s*Kinesis\s*\.\s*Model\s*\.\s*PutRecords?Request\b`,
)

// csharpAWSEventBridgeRe matches EventBridge PutEventsAsync call-sites.
var csharpAWSEventBridgeRe = regexp.MustCompile(
	`\b\w*(?:events?|eb|bridge)\w*\s*\.\s*PutEventsAsync\s*\(` +
		`|\bAmazon\s*\.\s*EventBridge\s*\.\s*Model\s*\.\s*PutEventsRequest\b`,
)

// appendAWSCSharpMatches runs the AWS publish matchers over content and
// appends any matches to out. Gated on csharpAWSGateRe — returns out
// unmodified when the file has no AWS SDK token at all.
func appendAWSCSharpMatches(out []EffectMatch, content string, headers []funcHeader) []EffectMatch {
	if !csharpAWSGateRe.MatchString(content) {
		return out
	}
	var aws []EffectMatch
	aws = appendCSharpMatches(aws, content, headers, csharpAWSSNSRe, EffectMessagePublish, "aws.sns.publish", 0.9)
	aws = appendCSharpMatches(aws, content, headers, csharpAWSSQSRe, EffectMessagePublish, "aws.sqs.send", 0.9)
	aws = appendCSharpMatches(aws, content, headers, csharpAWSKinesisRe, EffectMessagePublish, "aws.kinesis.put", 0.9)
	aws = appendCSharpMatches(aws, content, headers, csharpAWSEventBridgeRe, EffectMessagePublish, "aws.eventbridge.putevents", 0.9)

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
