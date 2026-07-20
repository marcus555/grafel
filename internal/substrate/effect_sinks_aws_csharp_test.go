// Tests for the C#/.NET AWS SDK publish-effect sniffer (#5798 Tier 1).
// Mirrors effect_sinks_aws_test.go (Java pilot, #5797 PR1) but targets AWS
// SDK for .NET async producer call-sites (*Async suffix, wrapper classes,
// config-injected channel), keyed off an Amazon.* using directive or client
// type. Synthetic fixtures only — no live corpus (mirrors
// gap7_node_v3_and_dotnet_producers.md's .NET section).
package substrate

import "testing"

const csharpAWSKinesisEventBridgeFixture = `
using Amazon.Kinesis;
using Amazon.Kinesis.Model;
using Amazon.EventBridge;
using Amazon.EventBridge.Model;

public class StreamPublisher
{
    private readonly IAmazonKinesis _kinesis;
    private readonly IAmazonEventBridge _eventBridge;

    public async Task PublishAsync(string streamName, MemoryStream data, string partitionKey)
    {
        await _kinesis.PutRecordAsync(new PutRecordRequest {
            StreamName = streamName, Data = data, PartitionKey = partitionKey });
    }

    public async Task PutEventsAsync(string busName, string source, string detail)
    {
        await _eventBridge.PutEventsAsync(new PutEventsRequest {
            Entries = { new PutEventsRequestEntry {
                EventBusName = busName, Source = source, Detail = detail } } });
    }
}
`

func TestSniffEffectsCSharp_AWS_KinesisPutRecordAsync(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSKinesisEventBridgeFixture)
	found := false
	for _, m := range matches {
		if m.Function == "PublishAsync" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.kinesis.put" {
				t.Errorf("PublishAsync sink = %q, want aws.kinesis.put", m.Sink)
			}
			if m.Confidence != 0.9 {
				t.Errorf("PublishAsync confidence = %v, want 0.9", m.Confidence)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on PublishAsync (Kinesis PutRecordAsync), matches=%+v", matches)
	}
}

func TestSniffEffectsCSharp_AWS_EventBridgePutEventsAsync(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSKinesisEventBridgeFixture)
	found := false
	for _, m := range matches {
		if m.Function == "PutEventsAsync" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.eventbridge.putevents" {
				t.Errorf("PutEventsAsync sink = %q, want aws.eventbridge.putevents", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on PutEventsAsync (EventBridge PutEventsAsync, array Entries → single emission), matches=%+v", matches)
	}
}

const csharpAWSSNSSQSFixture = `
using Amazon.SimpleNotificationService;
using Amazon.SimpleNotificationService.Model;
using Amazon.SQS;
using Amazon.SQS.Model;

public class OrderNotifier
{
    private readonly IAmazonSNS _sns;
    private readonly IAmazonSQS _sqs;

    public async Task PublishOrderAsync(string topicArn, string message)
    {
        await _sns.PublishAsync(new PublishRequest { TopicArn = topicArn, Message = message });
    }

    public async Task PublishOrdersAsync(string topicArn, List<PublishBatchRequestEntry> entries)
    {
        await _sns.PublishBatchAsync(new PublishBatchRequest { TopicArn = topicArn, PublishBatchRequestEntries = entries });
    }

    public async Task EnqueueAsync(string queueUrl, string body)
    {
        await _sqs.SendMessageAsync(new SendMessageRequest { QueueUrl = queueUrl, MessageBody = body });
    }

    public async Task EnqueueBatchAsync(string queueUrl, List<SendMessageBatchRequestEntry> entries)
    {
        await _sqs.SendMessageBatchAsync(new SendMessageBatchRequest { QueueUrl = queueUrl, Entries = entries });
    }
}
`

func TestSniffEffectsCSharp_AWS_SNSPublishAsync(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSSNSSQSFixture)
	found := false
	for _, m := range matches {
		if m.Function == "PublishOrderAsync" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sns.publish" {
				t.Errorf("PublishOrderAsync sink = %q, want aws.sns.publish", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on PublishOrderAsync (SNS PublishAsync), matches=%+v", matches)
	}
}

func TestSniffEffectsCSharp_AWS_SNSPublishBatchAsync(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSSNSSQSFixture)
	found := false
	for _, m := range matches {
		if m.Function == "PublishOrdersAsync" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sns.publish" {
				t.Errorf("PublishOrdersAsync sink = %q, want aws.sns.publish", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on PublishOrdersAsync (SNS PublishBatchAsync), matches=%+v", matches)
	}
}

func TestSniffEffectsCSharp_AWS_SQSSendMessageAsync(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSSNSSQSFixture)
	found := false
	for _, m := range matches {
		if m.Function == "EnqueueAsync" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("EnqueueAsync sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on EnqueueAsync (SQS SendMessageAsync), matches=%+v", matches)
	}
}

func TestSniffEffectsCSharp_AWS_SQSSendMessageBatchAsync(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSSNSSQSFixture)
	found := false
	for _, m := range matches {
		if m.Function == "EnqueueBatchAsync" && m.Effect == EffectMessagePublish {
			found = true
			if m.Sink != "aws.sqs.send" {
				t.Errorf("EnqueueBatchAsync sink = %q, want aws.sqs.send", m.Sink)
			}
		}
	}
	if !found {
		t.Fatalf("expected message_publish effect on EnqueueBatchAsync (SQS SendMessageBatchAsync), matches=%+v", matches)
	}
}

// PRECISION: a file with NO Amazon.* using directive or client type must
// short-circuit — a non-AWS type named PublishRequest/SendMessageRequest
// must not be flagged.
const csharpNonAWSFixture = `
public class ChatService
{
    public async Task RelayAsync(string userId, string text)
    {
        var req = new SendMessageRequest { UserId = userId, Text = text };
        await _bus.PublishAsync(req);
    }
}
`

func TestSniffEffectsCSharp_AWS_NonAWSNotFlagged(t *testing.T) {
	matches := sniffEffectsCSharp(csharpNonAWSFixture)
	for _, m := range matches {
		if m.Effect == EffectMessagePublish {
			t.Fatalf("file has no Amazon.* using directive/client type — AWS gate must short-circuit; must NOT be flagged; matches=%+v", matches)
		}
	}
}

// SINGLE-EMISSION: PutEventsAsync with an array Entries payload must still
// yield exactly ONE aws.eventbridge.putevents EffectMatch per call-site.
func TestSniffEffectsCSharp_AWS_EventBridgeSingleEmission(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSKinesisEventBridgeFixture)
	count := 0
	for _, m := range matches {
		if m.Function == "PutEventsAsync" && m.Effect == EffectMessagePublish && m.Sink == "aws.eventbridge.putevents" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly ONE aws.eventbridge.putevents effect on PutEventsAsync, got %d; matches=%+v", count, matches)
	}
}

// REGRESSION: the existing C# HTTP sniffer must be unaffected by the AWS
// additions (append-only proof).
const csharpAWSRegressionFixture = `
using Amazon.SQS;
using Amazon.SQS.Model;

public class Service
{
    private readonly HttpClient _httpClient;
    private readonly IAmazonSQS _sqs;

    public async Task<string> FetchThingAsync(string url)
    {
        var resp = await _httpClient.GetAsync(url);
        return await resp.Content.ReadAsStringAsync();
    }

    public async Task EnqueueAsync(string queueUrl, string body)
    {
        await _sqs.SendMessageAsync(new SendMessageRequest { QueueUrl = queueUrl, MessageBody = body });
    }
}
`

func TestSniffEffectsCSharp_AWS_HTTPRegressionUnaffected(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSRegressionFixture)
	found := false
	for _, m := range matches {
		if m.Function == "FetchThingAsync" && m.Effect == EffectHTTPOut {
			found = true
		}
	}
	if !found {
		t.Fatalf("regression: FetchThingAsync must still be flagged http_out, matches=%+v", matches)
	}
}

// PRECISION (cross-service FP, coordinator review): a file that legitimately
// imports ONE AWS service (SQS) AND uses an unrelated in-process event
// aggregator whose method happens to be named .PublishAsync(...) on a non-AWS
// receiver (_events) must NOT be misflagged aws.sns.publish. The genuine SQS
// call in the same file MUST still fire (file-gate + service-hint together).
const csharpAWSCrossServiceFPFixture = `
using Amazon.SQS;
using Amazon.SQS.Model;

public class EventBus
{
    private readonly IAmazonSQS _sqs;
    private readonly IEventAggregator _events;

    public async Task EnqueueAsync(string queueUrl, string body)
    {
        await _sqs.SendMessageAsync(new SendMessageRequest { QueueUrl = queueUrl, MessageBody = body });
    }

    public async Task RaiseAsync(DomainEvent e)
    {
        await _events.PublishAsync(e);
    }
}
`

func TestSniffEffectsCSharp_AWS_CrossServiceFalsePositiveNotFlagged(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSCrossServiceFPFixture)
	for _, m := range matches {
		if m.Function == "RaiseAsync" && m.Effect == EffectMessagePublish {
			t.Fatalf("RaiseAsync is an in-process event-aggregator publish on a non-AWS receiver (_events.PublishAsync) — must NOT be flagged aws.sns.publish just because the file imports SQS; matches=%+v", matches)
		}
	}
}

func TestSniffEffectsCSharp_AWS_CrossServiceGenuineStillFires(t *testing.T) {
	matches := sniffEffectsCSharp(csharpAWSCrossServiceFPFixture)
	found := false
	for _, m := range matches {
		if m.Function == "EnqueueAsync" && m.Effect == EffectMessagePublish && m.Sink == "aws.sqs.send" {
			found = true
		}
	}
	if !found {
		t.Fatalf("genuine SQS SendMessageAsync on a service-hinted receiver (_sqs) must still fire; matches=%+v", matches)
	}
}
