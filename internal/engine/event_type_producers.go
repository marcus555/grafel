// Producer-side + consumer-side detectors for the generic event-identity
// pass (event_type_edges.go, GAP-005).
package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Shared: allowlisted event-type key extraction
// ---------------------------------------------------------------------------

// eventTypeAllowlistKeyRe matches an allowlisted event-type key bound to a
// STRING LITERAL, in either bare-identifier (Go struct field / JS object
// literal, e.g. `EventType: "OrderPlaced"`) or quoted-key (JSON-style, e.g.
// `"eventType":"OrderPlaced"`) form. The key alternation is matched
// case-insensitively (Go struct fields are conventionally capitalized,
// JSON/JS keys are conventionally camelCase) but anchored on \b so it never
// matches mid-identifier (e.g. "ContentType", "ResourceType").
//
// Allowlist: eventType, eventName, detailType, detail-type — all
// UNAMBIGUOUS event-envelope keys. Bare `type` was DELIBERATELY dropped
// (GAP-005 review FIX 2): because the publish gate is generic
// (`.send`/`.emit`/`.publish`/`.produce`), a bare `type` key over-minted on
// non-event payloads — `logger.emit({type:"error"})`,
// `httpClient.send({type:"json"})`, `styleSheet.emit({type:"css"})` all
// carry a `type` string that is NOT an event contract. The four remaining
// keys have no such collision.
//
// Group 1 = the matched key text, group 2 = the string value.
var eventTypeAllowlistKeyRe = regexp.MustCompile(
	`["']?\b((?i:eventType|detailType|detail-type|eventName))\b["']?\s*:\s*["'` + "`" + `]([^"'` + "`" + `\n\r]+)["'` + "`" + `]`,
)

// findAllowlistedEventType scans arg (the text of a call's argument list, or
// any bounded literal window) for the first allowlisted key/string-literal
// pair. Returns ("", "", false) when none is found.
func findAllowlistedEventType(arg string) (key, value string, ok bool) {
	m := eventTypeAllowlistKeyRe.FindStringSubmatch(arg)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// ---------------------------------------------------------------------------
// Producer — Go
// ---------------------------------------------------------------------------

// goPublishSiteRe matches common Go publish-call method names across
// AWS SDK (SNS/SQS/Kinesis), Sarama/Kafka (SendMessage), confluent-kafka-go
// (Produce), and generic wrapper conventions (Publish/Send). This is a
// GENERIC gate (unlike the AWS-only effect_sinks_aws_go.go sniffers) because
// GAP-005 targets any channel, not just AWS.
var goPublishSiteRe = regexp.MustCompile(
	`\.(?:Publish(?:WithContext)?|PublishMessage|SendMessage(?:Batch)?(?:WithContext)?|PutRecords?(?:WithContext)?|Produce|Send)\s*\(`,
)

// applyEventTypeProducerGo scans Go source for publish call-sites and, at
// each one, extracts an allowlisted key/string-literal pair from the call's
// argument list. The argument list is the precision boundary — a matching
// key/value pair OUTSIDE any publish call's parens is never considered.
func applyEventTypeProducerGo(
	src string,
	emitEdge func(fromID, verbatim, kind string, props map[string]string),
) {
	for _, m := range goPublishSiteRe.FindAllStringIndex(src, -1) {
		openParen := m[1] - 1 // regex ends in `\(`, so m[1]-1 is the '(' index.
		arg := extractBalancedParensEngine(src, openParen)
		key, value, ok := findAllowlistedEventType(arg)
		if !ok {
			continue
		}
		caller := findEnclosingGoFunctionName(src, m[0])
		emitEdge(
			fmt.Sprintf("SCOPE.Function:%s", caller),
			value,
			"PUBLISHES_TO",
			map[string]string{"lang": "go", "key": key, "detection": "publish-site-literal"},
		)
	}
}

// ---------------------------------------------------------------------------
// Producer — JS/TS
// ---------------------------------------------------------------------------

// jstsPublishSiteRe matches common JS/TS publish call-sites: generic
// `.publish(`/`.send(`/`.sendMessage(`/`.produce(`/`.emit(` method calls
// (covers AWS SDK v2 style, ioredis/kafkajs, EventEmitter) plus the AWS SDK
// v3 `new XCommand(` construction shape (SNS/SQS/Kinesis).
var jstsPublishSiteRe = regexp.MustCompile(
	`\.(?:publish|send|sendMessage|produce|emit)\s*\(` +
		`|new\s+\w*(?:PublishCommand|SendMessageCommand|PutRecordCommand|PutRecordsCommand)\s*\(`,
)

// applyEventTypeProducerJSTS mirrors applyEventTypeProducerGo for JS/TS.
func applyEventTypeProducerJSTS(
	src string,
	emitEdge func(fromID, verbatim, kind string, props map[string]string),
) {
	for _, m := range jstsPublishSiteRe.FindAllStringIndex(src, -1) {
		openParen := m[1] - 1
		arg := extractBalancedParensEngine(src, openParen)
		key, value, ok := findAllowlistedEventType(arg)
		if !ok {
			continue
		}
		caller := findEnclosingNodeFunctionName(src, m[0])
		emitEdge(
			fmt.Sprintf("SCOPE.Function:%s", caller),
			value,
			"PUBLISHES_TO",
			map[string]string{"lang": "javascript", "key": key, "detection": "publish-site-literal"},
		)
	}
}

// ---------------------------------------------------------------------------
// Consumer — event-source-mapping FilterCriteria (GAP-003 fold-in)
// ---------------------------------------------------------------------------

// eventTypeArrayKeyRe matches an (optionally quoted, case-insensitive)
// `eventType`/`detailType`/`detail-type` key immediately followed by a
// flow-style array opener — the shape FilterCriteria.Pattern's `data.
// eventType: [...]` (or bare `eventType`/`detail-type`) takes in both native
// HCL (`eventType = [...]`) and flow-style YAML/JSON (`eventType: [...]`).
var eventTypeArrayKeyRe = regexp.MustCompile(
	`["']?\b(?i:eventType|detailType|detail-type)\b["']?\s*[:=]\s*\[`,
)

// quotedStringRe extracts a single quoted string value (single or double
// quotes) — used to pull the individual event-type values out of the
// bracketed array extractEventTypeArrayValues locates.
var quotedStringRe = regexp.MustCompile(`["']([^"'\n\r]+)["']`)

// extractEventTypeArrayValues finds every `eventType`/`detailType`/
// `detail-type` array in src (there may be more than one FilterCriteria
// filter block) and returns the de-duplicated union of quoted string values,
// in first-seen order.
func extractEventTypeArrayValues(src string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range eventTypeArrayKeyRe.FindAllStringIndex(src, -1) {
		bracketPos := m[1] - 1 // regex ends in `\[`.
		inner := extractBalancedBracket(src, bracketPos)
		for _, sm := range quotedStringRe.FindAllStringSubmatch(inner, -1) {
			v := sm[1]
			if v != "" && !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// hclEventSourceMappingRe matches `resource "aws_lambda_event_source_mapping"
// "name"` blocks.
var hclEventSourceMappingRe = regexp.MustCompile(`resource\s+"aws_lambda_event_source_mapping"\s+"(\w+)"`)

// hclESMFunctionNameArnRe extracts `function_name = aws_lambda_function.<name>.arn`.
var hclESMFunctionNameArnRe = regexp.MustCompile(`function_name\s*=\s*aws_lambda_function\.(\w+)\.arn`)

// hclESMFunctionNameLiteralRe extracts `function_name = "<name>"`.
var hclESMFunctionNameLiteralRe = regexp.MustCompile(`function_name\s*=\s*"([^"]+)"`)

// applyEventTypeConsumerHCL parses Terraform `aws_lambda_event_source_mapping`
// resources for a FilterCriteria pattern enumerating event-type values,
// mirroring/generalizing applyEventBridgeHCL's rule-block extraction
// (event_bus_edges.go:220-264) to the ESM `data.eventType` shape.
func applyEventTypeConsumerHCL(
	src string,
	emitEdge func(fromID, verbatim, kind string, props map[string]string),
) {
	if !strings.Contains(src, "aws_lambda_event_source_mapping") {
		return
	}
	for _, m := range hclEventSourceMappingRe.FindAllStringSubmatchIndex(src, -1) {
		resName := src[m[2]:m[3]]

		blockStart := m[1]
		bracePos := strings.Index(src[blockStart:], "{")
		if bracePos < 0 {
			continue
		}
		bracePos += blockStart
		body := extractBalancedBraces(src, bracePos)

		lambdaName := ""
		if fm := hclESMFunctionNameArnRe.FindStringSubmatch(body); fm != nil {
			lambdaName = fm[1]
		} else if fm := hclESMFunctionNameLiteralRe.FindStringSubmatch(body); fm != nil {
			lambdaName = fm[1]
		}
		if lambdaName == "" {
			continue
		}

		values := extractEventTypeArrayValues(body)
		if len(values) == 0 {
			continue
		}

		lambdaID := lambdaFunctionID(lambdaName)
		fromID := fmt.Sprintf("%s:%s", serverlessFunctionKind, lambdaID)
		for _, v := range values {
			emitEdge(fromID, v, "SUBSCRIBES_TO",
				map[string]string{"iac": "terraform", "resource": resName, "detection": "event-source-mapping-filter-criteria"})
		}
	}
}

// serverlessESMFunctionNameRe finds the nearest preceding `<2-space-indent>
// <name>:` YAML mapping key under a `functions:` stanza — a best-effort
// heuristic to attribute a filterPatterns block (which is nested several
// levels under that function) back to its owning function name without a
// full YAML parse.
var serverlessESMFunctionNameRe = regexp.MustCompile(`(?m)^  (\w+):\s*$`)

// applyEventTypeConsumerServerlessYML parses serverless.yml `stream.
// filterPatterns` stanzas (flow-style `eventType: [...]` arrays) for
// event-type values, attributing them to the nearest enclosing function
// name found by serverlessESMFunctionNameRe. Mirrors
// applyEventBridgeServerlessYML's text-only path.
func applyEventTypeConsumerServerlessYML(
	src string,
	emitEdge func(fromID, verbatim, kind string, props map[string]string),
) {
	if !strings.Contains(src, "filterPatterns") && !strings.Contains(src, "filter_criteria") {
		return
	}
	for _, m := range eventTypeArrayKeyRe.FindAllStringIndex(src, -1) {
		bracketPos := m[1] - 1
		inner := extractBalancedBracket(src, bracketPos)
		var values []string
		seen := map[string]bool{}
		for _, sm := range quotedStringRe.FindAllStringSubmatch(inner, -1) {
			v := sm[1]
			if v != "" && !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
		if len(values) == 0 {
			continue
		}

		// Attribute to the nearest preceding function name.
		fnName := ""
		for _, fm := range serverlessESMFunctionNameRe.FindAllStringSubmatchIndex(src, -1) {
			if fm[0] > m[0] {
				break
			}
			fnName = src[fm[2]:fm[3]]
		}
		if fnName == "" {
			continue
		}

		lambdaID := lambdaFunctionID(fnName)
		fromID := fmt.Sprintf("%s:%s", serverlessFunctionKind, lambdaID)
		for _, v := range values {
			emitEdge(fromID, v, "SUBSCRIBES_TO",
				map[string]string{"iac": "serverless.yml", "function_name": fnName, "detection": "event-source-mapping-filter-criteria"})
		}
	}
}

// ---------------------------------------------------------------------------
// Consumer — SAM / CloudFormation template FilterCriteria (GAP-005 review
// FIX 1 — the dominant IaC form the HCL + serverless.yml paths missed)
// ---------------------------------------------------------------------------

// cfnTemplateGateRe recognizes a CloudFormation / SAM template. Any of:
// the `AWSTemplateFormatVersion` header, the SAM `Transform: AWS::Serverless`
// macro, or a resource `Type:` naming a SAM function / raw Lambda ESM. A
// serverless-framework serverless.yml (handled separately above) carries
// none of these tokens, so it never double-mints here.
var cfnTemplateGateRe = regexp.MustCompile(
	`AWSTemplateFormatVersion` +
		`|Transform\s*:\s*['"]?AWS::Serverless` +
		`|Type\s*:\s*['"]?AWS::Serverless::Function` +
		`|Type\s*:\s*['"]?AWS::Lambda::EventSourceMapping`,
)

// cfnResourcesRe finds the top-level `Resources:` mapping key (column 0).
var cfnResourcesRe = regexp.MustCompile(`(?m)^Resources:\s*$`)

// cfnTopLevelKeyRe finds a column-0 mapping key — the boundary that ends the
// Resources block (the next top-level section, e.g. `Outputs:`).
var cfnTopLevelKeyRe = regexp.MustCompile(`(?m)^\S`)

// cfnLogicalIDLineRe matches an indented CFN logical-id header line
// (`  MyFn:` on its own line). Group 1 = leading whitespace, group 2 = id.
var cfnLogicalIDLineRe = regexp.MustCompile(`(?m)^(\s+)(\w+):\s*$`)

// cfnResourceTypeRe extracts a resource's `Type: AWS::...` value.
var cfnResourceTypeRe = regexp.MustCompile(`Type\s*:\s*['"]?(AWS::[A-Za-z0-9:]+)`)

// cfnFunctionNameRefRe extracts the standalone-ESM target function:
// `FunctionName: !Ref MyFn`, `FunctionName: !GetAtt MyFn.Arn`, or a literal
// `FunctionName: "my-fn"`. Group 1 = the intrinsic-ref logical id (Ref /
// GetAtt), group 2 = a literal name.
var cfnFunctionNameRefRe = regexp.MustCompile(
	`FunctionName\s*:\s*(?:!Ref\s+(\w+)|!GetAtt\s+(\w+)\.[A-Za-z]+|['"]?([\w-]+)['"]?)`,
)

// applyEventTypeConsumerCFN parses SAM / CloudFormation templates for
// event-source-mapping FilterCriteria.Pattern `data.eventType` arrays and
// mints SUBSCRIBES_TO edges. Two shapes:
//
//  1. Inline `Events:` on an `AWS::Serverless::Function` — the consumer is
//     the enclosing function's logical id.
//  2. Standalone `AWS::Lambda::EventSourceMapping` — the consumer is the
//     `FunctionName: !Ref <id>` (or !GetAtt / literal) target.
//
// The `Pattern` value is a JSON string; extractEventTypeArrayValues finds the
// `eventType`/`detailType`/`detail-type` array inside it verbatim (the JSON
// string's `"eventType": [...]` matches eventTypeArrayKeyRe directly). Only a
// real `FilterCriteria` block is scanned — nothing looser.
func applyEventTypeConsumerCFN(
	src string,
	emitEdge func(fromID, verbatim, kind string, props map[string]string),
) {
	if !cfnTemplateGateRe.MatchString(src) {
		return
	}
	// Isolate the Resources: mapping — logical-id blocks live only there.
	loc := cfnResourcesRe.FindStringIndex(src)
	if loc == nil {
		return
	}
	body := src[loc[1]:]
	if end := cfnTopLevelKeyRe.FindStringIndex(body); end != nil {
		body = body[:end[0]]
	}

	// Detect the logical-id indent from the first header line under Resources.
	first := cfnLogicalIDLineRe.FindStringSubmatch(body)
	if first == nil {
		return
	}
	childIndent := first[1]

	// Split body into per-logical-id blocks at that indent.
	headerRe := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(childIndent) + `(\w+):\s*$`)
	heads := headerRe.FindAllStringSubmatchIndex(body, -1)
	for i, h := range heads {
		logicalID := body[h[2]:h[3]]
		blockStart := h[1]
		blockEnd := len(body)
		if i+1 < len(heads) {
			blockEnd = heads[i+1][0]
		}
		block := body[blockStart:blockEnd]

		// Precision gate: only real FilterCriteria blocks.
		if !strings.Contains(block, "FilterCriteria") {
			continue
		}
		tm := cfnResourceTypeRe.FindStringSubmatch(block)
		if tm == nil {
			continue
		}
		resType := tm[1]

		values := extractEventTypeArrayValues(block)
		if len(values) == 0 {
			continue
		}

		// Resolve the consumer function name per shape.
		fnName := ""
		props := map[string]string{"iac": "cloudformation", "detection": "event-source-mapping-filter-criteria"}
		switch resType {
		case "AWS::Serverless::Function":
			// Shape 1 — inline Events on the SAM function; the function IS the
			// enclosing logical id.
			fnName = logicalID
			props["sam_function"] = logicalID
		case "AWS::Lambda::EventSourceMapping":
			// Shape 2 — standalone ESM; target is FunctionName: !Ref <id>.
			if fm := cfnFunctionNameRefRe.FindStringSubmatch(block); fm != nil {
				switch {
				case fm[1] != "":
					fnName = fm[1]
				case fm[2] != "":
					fnName = fm[2]
				default:
					fnName = fm[3]
				}
			}
			props["esm_resource"] = logicalID
		default:
			continue
		}
		if fnName == "" {
			continue
		}

		fromID := fmt.Sprintf("%s:%s", serverlessFunctionKind, lambdaFunctionID(fnName))
		for _, v := range values {
			emitEdge(fromID, v, "SUBSCRIBES_TO", props)
		}
	}
}
