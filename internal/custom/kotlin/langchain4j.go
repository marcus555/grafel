package kotlin

import (
	"context"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_kotlin_langchain4j", &langchain4jKotlinExtractor{})
}

type langchain4jKotlinExtractor struct{}

func (e *langchain4jKotlinExtractor) Language() string { return "custom_kotlin_langchain4j" }

var (
	reLc4jKotlinAiService = regexp.MustCompile(
		`(?s)@AiService\b[^{]*?(?:public\s+|private\s+|internal\s+|protected\s+)?interface\s+(\w+)`,
	)
	reLc4jKotlinTool = regexp.MustCompile(
		`(?s)@Tool\b\s*(?:\(\s*"[^"]*"\s*\)\s*|\(\s*\)\s*|\s*)` +
			`(?:(?:public|private|internal|protected|override|suspend)\s+)*fun\s+(\w+)\s*\(`,
	)
	reLc4jKotlinSystemMsg = regexp.MustCompile(
		`(?s)@SystemMessage\b[^(]*(?:\([^)]*\))?[^f]*fun\s+(\w+)\s*\(`,
	)
	reLc4jKotlinUserMsg = regexp.MustCompile(
		`(?s)@UserMessage\b[^(]*(?:\([^)]*\))?[^f]*fun\s+(\w+)\s*\(`,
	)

	// #5013: prompt_template_extraction template-variable resolution.
	//
	// Capture the FULL annotated declaration so we can read both the template
	// string literal carried by the annotation and the parameter list of the
	// fun it decorates. Group 1 = template string body (between the quotes),
	// group 2 = fun name, group 3 = raw parameter list. Supports both the
	// single-arg form `@UserMessage("...{{x}}...")` and Kotlin triple-quoted
	// strings. The two annotations share one shape so a small table drives both.
	// The param list `((?:[^()]|\([^()]*\))*)` tolerates one level of nested
	// parens so `@V("name")` annotations inside the signature don't truncate it.
	reLc4jKotlinSystemMsgTpl = regexp.MustCompile(
		`(?s)@SystemMessage\s*\(\s*(?:value\s*=\s*)?(?:"""(.*?)"""|"((?:[^"\\]|\\.)*)")\s*\)` +
			`[^f]*?fun\s+(\w+)\s*\(((?:[^()]|\([^()]*\))*)\)`,
	)
	reLc4jKotlinUserMsgTpl = regexp.MustCompile(
		`(?s)@UserMessage\s*\(\s*(?:value\s*=\s*)?(?:"""(.*?)"""|"((?:[^"\\]|\\.)*)")\s*\)` +
			`[^f]*?fun\s+(\w+)\s*\(((?:[^()]|\([^()]*\))*)\)`,
	)

	// `PromptTemplate.from("...{{var}}...")` (optionally assigned to a
	// `val/var <name> = ...`). Group 1 = optional binding name, group 2/3 =
	// triple-quoted / double-quoted template body.
	reLc4jKotlinPromptTemplate = regexp.MustCompile(
		`(?s)(?:(?:private|protected|internal|public)\s+)?(?:(?:val|var)\s+(\w+)\s*(?::[^=]+)?=\s*)?` +
			`PromptTemplate\s*\.\s*from\s*\(\s*(?:"""(.*?)"""|"((?:[^"\\]|\\.)*)")`,
	)

	// Template placeholders. langchain4j uses `{{var}}` (mustache-style) by
	// default; the older `{var}` single-brace form is also accepted by
	// PromptTemplate. We match both and normalize to the inner identifier.
	reLc4jKotlinTplVarDouble = regexp.MustCompile(`\{\{\s*([A-Za-z_]\w*)\s*\}\}`)
	reLc4jKotlinTplVarSingle = regexp.MustCompile(`\{\s*([A-Za-z_]\w*)\s*\}`)

	// A single Kotlin parameter, optionally carrying an `@V("name")` /
	// `@V(value = "name")` binding annotation that renames it for the template.
	// Group 1 = @V bound name (optional), group 2 = kotlin param identifier.
	reLc4jKotlinParam = regexp.MustCompile(
		`(?s)(?:@V\s*\(\s*(?:value\s*=\s*)?"((?:[^"\\]|\\.)*)"\s*\)\s*)?` +
			`(?:vararg\s+)?(\w+)\s*:`,
	)
	reLc4jKotlinChatModel = regexp.MustCompile(
		`(?m)(?:private\s+|protected\s+|internal\s+|public\s+)?(?:val|var)\s+(\w+)\s*:\s*(ChatLanguageModel|StreamingChatLanguageModel)`,
	)
	reLc4jKotlinMemory = regexp.MustCompile(
		`(?m)(?:private\s+|protected\s+|internal\s+|public\s+)?(?:val|var)\s+(\w+)\s*:\s*(ChatMemory|ChatMemoryProvider|MessageWindowChatMemory|TokenWindowChatMemory)`,
	)
	reLc4jKotlinRAG = regexp.MustCompile(
		`(?m)(?:private\s+|protected\s+|internal\s+|public\s+)?(?:val|var)\s+(\w+)\s*:\s*(EmbeddingStoreContentRetriever|EmbeddingStoreIngestor|EmbeddingStore|ContentRetriever)`,
	)

	// #5012: runtime chain_composition wiring. Capture an
	// `AiServices.builder(IFace::class.java)` ... `.build()` (or
	// `.create(...)`) assembly assigned to a `val/var <svc> = ...`, so the
	// whole call chain — including newline-separated fluent calls — is one
	// match group we can scan for individual wiring methods.
	reLc4jKotlinServiceWiring = regexp.MustCompile(
		`(?s)(?:(?:private|protected|internal|public)\s+)?(?:val|var)\s+(\w+)\s*(?::[^=]+)?=\s*` +
			`AiServices\s*\.\s*(?:builder|create)\s*\((?:[^)]*)\)((?:\s*\.\s*\w+\s*\([^)]*\))*)`,
	)
	// Individual fluent builder steps inside the captured chain. The arg may be
	// an identifier reference (field/var we wired earlier), a class literal, or
	// an inline expression; we capture the leading identifier when present.
	reLc4jKotlinWireStep = regexp.MustCompile(
		`\.\s*(chatLanguageModel|streamingChatLanguageModel|tools|chatMemory|chatMemoryProvider|contentRetriever|retriever|retrievalAugmentor|systemMessageProvider|moderationModel|toolProvider)\s*\(\s*([A-Za-z_]\w*)?`,
	)
)

// lc4jWireKindToTarget maps a builder method to the wiring edge property that
// classifies the composed component. Kept here so the edge Properties carry a
// stable, queryable wire role parallel to the Java structural classification.
var lc4jWireRole = map[string]string{
	"chatLanguageModel":          "chat_model",
	"streamingChatLanguageModel": "streaming_chat_model",
	"tools":                      "tools",
	"toolProvider":               "tool_provider",
	"chatMemory":                 "chat_memory",
	"chatMemoryProvider":         "chat_memory_provider",
	"contentRetriever":           "content_retriever",
	"retriever":                  "content_retriever",
	"retrievalAugmentor":         "retrieval_augmentor",
	"systemMessageProvider":      "system_message_provider",
	"moderationModel":            "moderation_model",
}

func (e *langchain4jKotlinExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/kotlin")
	_, span := tracer.Start(ctx, "indexer.langchain4j_kotlin_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "langchain4j"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "kotlin" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)

	// confidence_overlay (#4974, parity with Java #3093): every langchain4j
	// entity below is produced by regex pattern match over Kotlin source, so the
	// extractor stamps a top-level EntityRecord.Confidence directly rather than
	// relying solely on the framework-blind per-binding substrate overlay.
	regexConf := types.BaseConfidence(types.SourceRegexPattern)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		if ent.Confidence == 0 {
			ent.Confidence = regexConf
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// 1. @AiService interfaces -> SCOPE.Service
	for _, m := range reLc4jKotlinAiService.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Service", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_AI_SERVICE")
		add(ent)
	}

	// 2. @Tool methods -> SCOPE.Operation/function
	for _, m := range reLc4jKotlinTool.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Operation", "function", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_TOOL",
			"tool_method", name)
		add(ent)
	}

	// 3. @SystemMessage -> SCOPE.Pattern (template-variable resolution, #5013).
	// Pass 3a captures annotations that carry an inline template string so we
	// can resolve `{{var}}` placeholders against the fun's @V-bound params.
	tplMatched := make(map[string]bool) // fun names already handled with a template
	for _, m := range reLc4jKotlinSystemMsgTpl.FindAllStringSubmatchIndex(src, -1) {
		tpl := groupOr(src, m, 1, 2) // triple-quoted (1) or double-quoted (2)
		funName := src[m[6]:m[7]]
		params := src[m[8]:m[9]]
		ent := makeEntity(funName+".system_message", "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_PROMPT",
			"prompt_type", "system_message")
		addTemplateVarsAndEdges(&ent, tpl, params, regexConf)
		tplMatched["system:"+funName] = true
		add(ent)
	}
	// 3b. @SystemMessage without a resolvable inline template (e.g. resource ref).
	for _, m := range reLc4jKotlinSystemMsg.FindAllStringSubmatchIndex(src, -1) {
		funName := src[m[2]:m[3]]
		if tplMatched["system:"+funName] {
			continue
		}
		ent := makeEntity(funName+".system_message", "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_PROMPT",
			"prompt_type", "system_message")
		add(ent)
	}

	// 4. @UserMessage -> SCOPE.Pattern (template-variable resolution, #5013).
	for _, m := range reLc4jKotlinUserMsgTpl.FindAllStringSubmatchIndex(src, -1) {
		tpl := groupOr(src, m, 2, 4)
		funName := src[m[6]:m[7]]
		params := src[m[8]:m[9]]
		ent := makeEntity(funName+".user_message", "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_PROMPT",
			"prompt_type", "user_message")
		addTemplateVarsAndEdges(&ent, tpl, params, regexConf)
		tplMatched["user:"+funName] = true
		add(ent)
	}
	for _, m := range reLc4jKotlinUserMsg.FindAllStringSubmatchIndex(src, -1) {
		funName := src[m[2]:m[3]]
		if tplMatched["user:"+funName] {
			continue
		}
		ent := makeEntity(funName+".user_message", "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_PROMPT",
			"prompt_type", "user_message")
		add(ent)
	}

	// 4c. PromptTemplate.from("...{{var}}...") -> SCOPE.Pattern (#5013).
	// Programmatic templates have no surrounding fun param list, so placeholders
	// are recorded as template variables but bind to no resolvable param.
	ptIdx := 0
	for _, m := range reLc4jKotlinPromptTemplate.FindAllStringSubmatchIndex(src, -1) {
		tpl := groupOr(src, m, 2, 3) // triple-quoted (2) or double-quoted (3)
		name := src[m[2]:m[3]]
		if name == "" {
			ptIdx++
			name = "promptTemplate" + itoa(ptIdx)
		}
		ent := makeEntity(name+".prompt_template", "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_PROMPT_TEMPLATE",
			"prompt_type", "prompt_template")
		addTemplateVarsAndEdges(&ent, tpl, "", regexConf)
		add(ent)
	}

	// 5. ChatLanguageModel fields -> SCOPE.Component
	for _, m := range reLc4jKotlinChatModel.FindAllStringSubmatchIndex(src, -1) {
		fieldName := src[m[2]:m[3]]
		modelType := src[m[4]:m[5]]
		ent := makeEntity(fieldName, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_MODEL",
			"model_type", modelType)
		add(ent)
	}

	// 6. ChatMemory fields -> SCOPE.Component
	for _, m := range reLc4jKotlinMemory.FindAllStringSubmatchIndex(src, -1) {
		fieldName := src[m[2]:m[3]]
		memType := src[m[4]:m[5]]
		ent := makeEntity(fieldName, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_MEMORY",
			"memory_type", memType)
		add(ent)
	}

	// 7. RAG components -> SCOPE.Pattern
	for _, m := range reLc4jKotlinRAG.FindAllStringSubmatchIndex(src, -1) {
		fieldName := src[m[2]:m[3]]
		ragType := src[m[4]:m[5]]
		ent := makeEntity(fieldName, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "langchain4j", "provenance", "INFERRED_FROM_LANGCHAIN4J_RAG",
			"rag_type", ragType)
		add(ent)
	}

	// 8. #5012: runtime AiServices builder chain_composition wiring.
	// `val assistant = AiServices.builder(Assistant::class.java)
	//      .chatLanguageModel(model).tools(tools).chatMemory(memory).build()`
	// emits a SCOPE.Service entity for the assembled service carrying USES
	// edges to each wired component (by referenced identifier name), giving the
	// runtime composition graph parity with Java langchain4j chain_composition.
	for _, m := range reLc4jKotlinServiceWiring.FindAllStringSubmatchIndex(src, -1) {
		svcName := src[m[2]:m[3]]
		chain := src[m[4]:m[5]]

		svc := makeEntity(svcName, "SCOPE.Service", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&svc, "framework", "langchain4j",
			"provenance", "INFERRED_FROM_LANGCHAIN4J_AI_SERVICES_BUILDER",
			"assembly", "AiServices.builder")

		var rels []types.RelationshipRecord
		wired := make(map[string]bool)
		for _, sm := range reLc4jKotlinWireStep.FindAllStringSubmatchIndex(chain, -1) {
			method := chain[sm[2]:sm[3]]
			role := lc4jWireRole[method]
			if role == "" {
				continue
			}
			setProps(&svc, "wire."+role, "true")

			// Capture the referenced component identifier when the argument is a
			// plain reference (model, tools, memory). Inline expressions /
			// class-literal-only args still record the wire role above but emit
			// no resolvable USES edge target.
			if sm[4] < 0 {
				continue
			}
			target := chain[sm[4]:sm[5]]
			edgeKey := role + "|" + target
			if wired[edgeKey] {
				continue
			}
			wired[edgeKey] = true
			rels = append(rels, types.RelationshipRecord{
				ToID: target,
				Kind: "USES",
				Properties: map[string]string{
					"framework": "langchain4j",
					"wire_role": role,
					"method":    method,
					"service":   svcName,
				},
				Confidence: regexConf,
			})
		}
		if len(rels) > 0 {
			svc.Relationships = append(svc.Relationships, rels...)
		}
		// Service entities keyed by Kind+Name; if a same-named @AiService
		// interface was already added, the wiring assembly is a distinct var so
		// dedupe on the (kind,name) key only collides when identical — acceptable.
		add(svc)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// groupOr returns the first non-empty capture among the given 1-based submatch
// group indices, reading from the FindAllStringSubmatchIndex pair slice m.
func groupOr(src string, m []int, groups ...int) string {
	for _, g := range groups {
		lo, hi := m[2*g], m[2*g+1]
		if lo >= 0 && hi > lo {
			return src[lo:hi]
		}
	}
	return ""
}

// itoa is a tiny local int->string for unnamed-template fallback names, avoiding
// a strconv import for a single use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// addTemplateVarsAndEdges records the langchain4j prompt template body, its
// resolved `{{var}}` / `{var}` placeholders, and DEPENDS_ON edges from the
// prompt-template pattern entity to the method parameters that bind each
// placeholder (#5013). params is the raw Kotlin parameter list (may be empty for
// PromptTemplate.from(...) which has no surrounding fun). Resolution rule,
// parity with Java langchain4j:
//   - a param annotated `@V("x")` binds template variable `x`;
//   - an un-annotated param `x: T` binds template variable `x` by name.
//
// Each distinct placeholder becomes a `template_var.<name>` property whose value
// is the bound parameter identifier ("" when no param resolves it). The full
// list is also stored as `template_vars` (comma-joined) plus the template body
// (truncated) for queryability.
func addTemplateVarsAndEdges(ent *types.EntityRecord, tpl, params string, conf float64) {
	// Build param binding map: template-var-name -> kotlin param identifier.
	bind := make(map[string]string)
	if params != "" {
		for _, pm := range reLc4jKotlinParam.FindAllStringSubmatchIndex(params, -1) {
			paramName := params[pm[4]:pm[5]]
			varName := paramName
			if pm[2] >= 0 && pm[3] > pm[2] {
				varName = params[pm[2]:pm[3]] // @V("...") override
			}
			if _, ok := bind[varName]; !ok {
				bind[varName] = paramName
			}
		}
	}

	// Collect placeholders in source order, dedup, preferring the {{var}} form.
	var vars []string
	seenVar := make(map[string]bool)
	collect := func(re *regexp.Regexp) {
		for _, vm := range re.FindAllStringSubmatch(tpl, -1) {
			name := vm[1]
			if seenVar[name] {
				continue
			}
			seenVar[name] = true
			vars = append(vars, name)
		}
	}
	collect(reLc4jKotlinTplVarDouble)
	// Single-brace fallback only for vars not already captured as {{ }}.
	collect(reLc4jKotlinTplVarSingle)

	if len(vars) == 0 {
		// Still record the template body so the entity is queryable.
		setProps(ent, "template", truncTemplate(tpl), "template_var_count", "0")
		return
	}

	joined := ""
	for i, v := range vars {
		if i > 0 {
			joined += ","
		}
		joined += v
		boundParam := bind[v]
		setProps(ent, "template_var."+v, boundParam)
		if boundParam != "" {
			ent.Relationships = append(ent.Relationships, types.RelationshipRecord{
				ToID: boundParam,
				Kind: "DEPENDS_ON",
				Properties: map[string]string{
					"framework":     "langchain4j",
					"binding":       "prompt_template_variable",
					"template_var":  v,
					"resolved_from": "method_param",
				},
				Confidence: conf,
			})
		}
	}
	setProps(ent, "template", truncTemplate(tpl),
		"template_vars", joined,
		"template_var_count", itoa(len(vars)))
}

// truncTemplate caps the stored template body so a large prompt does not bloat
// entity Properties; 240 bytes is enough to identify the template at a glance.
func truncTemplate(s string) string {
	const max = 240
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
