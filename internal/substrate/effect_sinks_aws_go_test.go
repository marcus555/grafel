// Tests for the Go AWS SDK publish-effect sniffer (#5798 Tier 1). Mirrors
// effect_sinks_aws_test.go (Java pilot, #5797 PR1) but targets raw AWS SDK
// for Go v1 (*WithContext) call-sites, keyed off github.com/aws/aws-sdk-go
// (or aws-sdk-go-v2) import evidence. Synthetic fixtures only — no live
// corpus (mirrors the gap6_go_sdkv1_producers.go repro shapes).
package substrate

import "testing"

const goAWSSNSFixture = `
package producers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns"
)

type AwsTopic struct {
	client   *sns.SNS
	topicArn string
}

func (t *AwsTopic) SendMessage(ctx context.Context, body string) error {
	_, err := t.client.PublishWithContext(ctx, &sns.PublishInput{
		TopicArn: aws.String(t.topicArn),
		Message:  aws.String(body),
	})
	return err
}
`

func TestSniffEffectsGo_AWS_SNSPublishWithContext(t *testing.T) {
	matches := sniffEffectsGo(goAWSSNSFixture)
	found := false
	for _, m := range matches {
		if m.Function == "SendMessage" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sns.publish" {
				t.Errorf("SendMessage sink = %q, want aws.sns.publish", m.Sink)
			}
			if m.Confidence != 0.9 {
				t.Errorf("SendMessage confidence = %v, want 0.9", m.Confidence)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on SendMessage (SNS PublishWithContext), matches=%+v", matches)
	}
}

const goAWSSQSFixture = `
package producers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type AwsQueue struct {
	client   *sqs.SQS
	queueURL string
}

func (q *AwsQueue) SendMessageBatch(ctx context.Context, entries []*sqs.SendMessageBatchRequestEntry) error {
	_, err := q.client.SendMessageBatchWithContext(ctx, &sqs.SendMessageBatchInput{
		QueueUrl: aws.String(q.queueURL),
		Entries:  entries,
	})
	return err
}

func (q *AwsQueue) Enqueue(ctx context.Context, body string) error {
	_, err := q.client.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(body),
	})
	return err
}
`

func TestSniffEffectsGo_AWS_SQSSendMessageBatchWithContext(t *testing.T) {
	matches := sniffEffectsGo(goAWSSQSFixture)
	found := false
	for _, m := range matches {
		if m.Function == "SendMessageBatch" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("SendMessageBatch sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on SendMessageBatch (SQS SendMessageBatchWithContext), matches=%+v", matches)
	}
}

func TestSniffEffectsGo_AWS_SQSSendMessageBare(t *testing.T) {
	matches := sniffEffectsGo(goAWSSQSFixture)
	found := false
	for _, m := range matches {
		if m.Function == "Enqueue" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("Enqueue sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on Enqueue (SQS SendMessage), matches=%+v", matches)
	}
}

const goAWSKinesisFixture = `
package producers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

func WriteRecords(ctx context.Context, client *kinesis.Kinesis, stream string, recs []*kinesis.PutRecordsRequestEntry) error {
	_, err := client.PutRecordsWithContext(ctx, &kinesis.PutRecordsInput{
		StreamName: aws.String(stream),
		Records:    recs,
	})
	return err
}

func WriteOneRecord(ctx context.Context, client *kinesis.Kinesis, stream string, data []byte) error {
	_, err := client.PutRecord(&kinesis.PutRecordInput{
		StreamName: aws.String(stream),
		Data:       data,
	})
	return err
}
`

func TestSniffEffectsGo_AWS_KinesisPutRecordsWithContext(t *testing.T) {
	matches := sniffEffectsGo(goAWSKinesisFixture)
	found := false
	for _, m := range matches {
		if m.Function == "WriteRecords" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.kinesis.put" {
				t.Errorf("WriteRecords sink = %q, want aws.kinesis.put", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on WriteRecords (Kinesis PutRecordsWithContext), matches=%+v", matches)
	}
}

func TestSniffEffectsGo_AWS_KinesisPutRecordBare(t *testing.T) {
	matches := sniffEffectsGo(goAWSKinesisFixture)
	found := false
	for _, m := range matches {
		if m.Function == "WriteOneRecord" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.kinesis.put" {
				t.Errorf("WriteOneRecord sink = %q, want aws.kinesis.put", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on WriteOneRecord (Kinesis PutRecord), matches=%+v", matches)
	}
}

const goAWSEventBridgeFixture = `
package producers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eventbridge"
)

type EventPublisher struct {
	client *eventbridge.EventBridge
	busARN string
}

func (p *EventPublisher) PublishEvent(ctx context.Context, detail string) error {
	_, err := p.client.PutEventsWithContext(ctx, &eventbridge.PutEventsInput{
		Entries: []*eventbridge.PutEventsRequestEntry{
			{EventBusName: aws.String(p.busARN), Detail: aws.String(detail)},
		},
	})
	return err
}
`

func TestSniffEffectsGo_AWS_EventBridgePutEventsWithContext(t *testing.T) {
	matches := sniffEffectsGo(goAWSEventBridgeFixture)
	found := false
	for _, m := range matches {
		if m.Function == "PublishEvent" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.eventbridge.putevents" {
				t.Errorf("PublishEvent sink = %q, want aws.eventbridge.putevents", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on PublishEvent (EventBridge PutEventsWithContext), matches=%+v", matches)
	}
}

// PRECISION: a file with NO AWS SDK import/client type must short-circuit —
// a non-AWS wrapper method named PublishWithContext or SendMessage must not
// be flagged.
const goNonAWSFixture = `
package notify

func (b *Bus) PublishWithContext(ctx context.Context, msg string) error {
	return b.internal.PublishWithContext(ctx, msg)
}

func (b *Bus) SendMessage(msg string) error {
	return b.internal.SendMessage(msg)
}
`

func TestSniffEffectsGo_AWS_NonAWSNotFlagged(t *testing.T) {
	matches := sniffEffectsGo(goNonAWSFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("file has no AWS SDK import/client type — AWS gate must short-circuit; must NOT be flagged; matches=%+v", matches)
		}
	}
}

// PRECISION (review FIX 1 mirror): a non-AWS file whose only "AWS-looking"
// tokens are Input struct names (PublishInput, SendMessageBatchInput) — with
// NO SDK import or client type — must NOT pass the gate.
const goFalseInputNamesFixture = `
package chat

type PublishInput struct {
	UserID string
	Text   string
}

func (b *Bus) Relay(userID, text string) error {
	in := PublishInput{UserID: userID, Text: text}
	return b.dispatch(in)
}
`

func TestSniffEffectsGo_AWS_InputStructNameWithoutSDKNotFlagged(t *testing.T) {
	matches := sniffEffectsGo(goFalseInputNamesFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("file has non-AWS type named like an AWS Input struct but NO SDK import/client — gate must short-circuit; must NOT be flagged; matches=%+v", matches)
		}
	}
}

// SINGLE-EMISSION: one call-site on one line must yield exactly ONE
// aws.sns.publish EffectMatch, not a duplicate.
func TestSniffEffectsGo_AWS_SNSSingleEmission(t *testing.T) {
	matches := sniffEffectsGo(goAWSSNSFixture)
	count := 0
	for _, m := range matches {
		if m.Function == "SendMessage" && m.Effect == EffectMessagePublish && m.Sink == "aws.sns.publish" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly ONE aws.sns.publish effect on SendMessage, got %d; matches=%+v", count, matches)
	}
}

// REGRESSION: the existing Go HTTP/DB/FS sniffers must be unaffected by the
// AWS additions (append-only proof).
const goAWSRegressionFixture = `
package svc

import (
	"net/http"

	"github.com/aws/aws-sdk-go/service/sns"
)

func FetchThing(client *http.Client, url string) (*http.Response, error) {
	return client.Get(url)
}

func (t *AwsTopic) Publish(body string) error {
	return t.client.Publish(&sns.PublishInput{})
}
`

func TestSniffEffectsGo_AWS_HTTPRegressionUnaffected(t *testing.T) {
	matches := sniffEffectsGo(goAWSRegressionFixture)
	found := false
	for _, m := range matches {
		if m.Function == "FetchThing" && m.Effect == EffectHTTPOut {
			found = true
		}
	}
	if !found {
		t.Fatalf("regression: FetchThing must still be flagged http_out, matches=%+v", matches)
	}
}

// PRECISION (cross-service FP, coordinator review): a file that legitimately
// imports ONE AWS service (SQS) AND defines an unrelated abstraction whose
// method happens to be named .Publish(...) — an in-process fan-out on a
// non-AWS receiver — must NOT be misflagged aws.sns.publish. The genuine SQS
// call in the same file MUST still fire (file-gate + service-hint together).
const goAWSCrossServiceFPFixture = `
package bus

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type EventBus struct {
	client      *sqs.SQS
	queueURL    string
	subscribers []Subscriber
}

func (b *EventBus) Enqueue(ctx context.Context, body string) error {
	_, err := b.client.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(b.queueURL),
		MessageBody: aws.String(body),
	})
	return err
}

func (b *EventBus) Notify(metric Metric) {
	b.subscribers[0].Publish(metric)
}
`

func TestSniffEffectsGo_AWS_CrossServiceFalsePositiveNotFlagged(t *testing.T) {
	matches := sniffEffectsGo(goAWSCrossServiceFPFixture)
	for _, m := range matches {
		if m.Function == "Notify" && m.Effect == EffectMessagePublish {
			t.Fatalf("Notify is an in-process fan-out on a non-AWS receiver (subscribers[0].Publish) — must NOT be flagged aws.sns.publish just because the file imports SQS; matches=%+v", matches)
		}
	}
}

func TestSniffEffectsGo_AWS_CrossServiceGenuineStillFires(t *testing.T) {
	matches := sniffEffectsGo(goAWSCrossServiceFPFixture)
	found := false
	for _, m := range matches {
		if m.Function == "Enqueue" && m.Effect == EffectMessagePublish && m.Sink == "aws.sqs.send" {
			found = true
		}
	}
	if !found {
		t.Fatalf("genuine SQS SendMessage(&sqs.SendMessageInput{...}) must still fire even with a generically-named receiver; matches=%+v", matches)
	}
}
