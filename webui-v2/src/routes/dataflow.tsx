/* ============================================================
   Data-flow & Taint view (#4265, epic #4249).

   Route: /g/:groupId/taint

   The taint analysis (internal/links/taint_flow.go) and the data-flow
   link pass (internal/links/dataflow_pass.go) had no dashboard surface.
   This screen surfaces both:

     - FINDINGS tab — ranked source→sink security findings: a tainted
       request input reaches a dangerous sink (SQL exec / shell / fs /
       template / SSRF / …) with no sanitizer on the path. Each finding
       carries a vulnerability category, an aggregated confidence, the
       resolved source + sink entity refs, and the inter-procedural path
       (expandable). Ranked by confidence descending.

     - FLOWS tab — the raw request-input → sink DATA_FLOWS_TO edges: the
       tainted request `field`, the `sink_kind`, and the hop_path the
       sniffer followed. A flow is a single observed data movement; a
       finding is a flow the taint pass judged exploitable.

   Data: GET /api/dataflow/{group} (handlers_dataflow.go → handleDataflow),
   raw-JSON DataflowReport.

   Layout mirrors Security / IaC: full-height column, a Tabs strip with
   count pills, a summary stat row, a category / sink-kind filter, and a
   ranked list with source→sink rows, sink-kind / category badges, a
   confidence pill, an expandable hop-path, and a RefLine source ref.
   Reuses Badge / Card / Pill / Tabs / Skeleton + RefLine + RepoChip.

   HONESTY: only genuinely-extracted flows/findings render. The taint pass
   drops anything below the confidence floor and never fabricates a flow it
   could not soundly follow; when neither sidecar exists the screen shows a
   clean empty state. Unresolved endpoints show their raw key tail — never
   an invented name.
   ============================================================ */

import { useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import {
  ShieldAlert,
  Waypoints,
  ArrowRight,
  ChevronRight,
  AlertTriangle,
  Crosshair,
  Target,
  ListFilter,
} from "lucide-react";

import {
  Badge,
  Card,
  CardBody,
  Pill,
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from "@/components/ui";
import { Skeleton } from "@/components/ui/skeleton";
import { RefLine } from "@/components/RefLine";
import { RepoChip } from "@/lib/repo-color";
import { cn } from "@/lib/utils";
import { useDataflow } from "@/hooks/use-dataflow";
import type {
  DataflowEndpoint,
  DataflowReport,
  SecurityFindingView,
  TaintFlow,
} from "@/data/types";

type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";

// ---------------------------------------------------------------------------
// § Styling helpers
// ---------------------------------------------------------------------------

/** Vulnerability category → tone + readable label. */
function categoryMeta(cat: string): { label: string; tone: Tone } {
  switch ((cat || "").toLowerCase()) {
    case "sql_injection":
      return { label: "SQL injection", tone: "danger" };
    case "command_injection":
      return { label: "Command injection", tone: "danger" };
    case "deserialization":
      return { label: "Unsafe deserialization", tone: "danger" };
    case "ssrf":
      return { label: "SSRF", tone: "warning" };
    case "path_traversal":
      return { label: "Path traversal", tone: "warning" };
    case "xss":
      return { label: "XSS", tone: "warning" };
    case "redos":
      return { label: "ReDoS", tone: "info" };
    default:
      return { label: cat || "finding", tone: "neutral" };
  }
}

/** sink_kind → tone. */
function sinkKindTone(kind: string): Tone {
  switch ((kind || "").toLowerCase()) {
    case "sql":
    case "command":
    case "shell":
    case "exec":
      return "danger";
    case "fs":
    case "path":
    case "template":
      return "warning";
    case "http":
    case "url":
      return "info";
    default:
      return "neutral";
  }
}

/** Confidence → tone (high = danger because high-confidence taint is worse). */
function confidenceTone(c: number): Tone {
  if (c >= 0.85) return "danger";
  if (c >= 0.7) return "warning";
  return "info";
}

function ConfidencePill({ value }: { value: number }) {
  return (
    <Badge
      tone={confidenceTone(value)}
      className="tabular-nums shrink-0"
      title={`Aggregated taint confidence ${value.toFixed(2)}`}
    >
      {Math.round(value * 100)}%
    </Badge>
  );
}

// ---------------------------------------------------------------------------
// § Endpoint ref (source / sink)
// ---------------------------------------------------------------------------

function EndpointRef({
  endpoint,
  groupId,
  icon,
}: {
  endpoint: DataflowEndpoint;
  groupId: string;
  icon: "source" | "sink";
}) {
  const Icon = icon === "source" ? Crosshair : Target;
  return (
    <span className="inline-flex items-center gap-1.5 min-w-0">
      <Icon size={12} className="text-text-4 shrink-0" />
      <span className="font-mono text-sm text-text truncate" title={endpoint.entity_id}>
        {endpoint.name}
      </span>
      {endpoint.primitive && (
        <span
          className="font-mono text-[10px] text-text-4 shrink-0"
          title={`taint primitive: ${endpoint.primitive}`}
        >
          {endpoint.primitive}
        </span>
      )}
      {endpoint.repo && (
        <RepoChip slug={endpoint.repo} groupId={groupId} maxLength={14} />
      )}
    </span>
  );
}

function EndpointRefLine({ endpoint }: { endpoint: DataflowEndpoint }) {
  if (!endpoint.source_file || !endpoint.repo) return null;
  return (
    <RefLine
      repo={endpoint.repo}
      file={endpoint.source_file}
      line={endpoint.line ?? 0}
      name={endpoint.name}
      className="text-[11px]"
    />
  );
}

// ---------------------------------------------------------------------------
// § Shared shells
// ---------------------------------------------------------------------------

function SkeletonRows({ n = 6 }: { n?: number }) {
  return (
    <div className="space-y-2">
      {Array.from({ length: n }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-3 h-14 px-4 rounded-lg border border-border"
        >
          <Skeleton w="w-1/4" />
          <Skeleton w="w-6" h="h-2" />
          <Skeleton w="w-1/4" />
          <Skeleton w="w-10" h="h-4" />
        </div>
      ))}
    </div>
  );
}

function EmptyState({ title, hint }: { title: string; hint: string }) {
  return (
    <div className="flex flex-col items-center py-16 text-center gap-3">
      <Waypoints size={32} className="text-text-4" />
      <p className="text-md font-medium text-text">{title}</p>
      <p className="text-sm text-text-3 max-w-md">{hint}</p>
    </div>
  );
}

function ErrorState() {
  return (
    <div className="flex flex-col items-center py-16 text-center gap-3">
      <AlertTriangle size={32} className="text-danger" />
      <p className="text-md font-medium text-text">Could not load data-flow analysis</p>
      <p className="text-sm text-text-3 max-w-sm">
        The daemon returned an error. Confirm the group is indexed and the daemon
        is reachable, then retry.
      </p>
    </div>
  );
}

function SummaryStat({ label, value }: { label: string; value: number }) {
  return (
    <Card className={cn("flex-1 min-w-[120px]")}>
      <CardBody className="py-3">
        <p className="text-2xl font-semibold tabular-nums text-text">{value}</p>
        <p className="text-xs text-text-4 mt-0.5">{label}</p>
      </CardBody>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// § One finding row (source → sink, ranked, expandable path)
// ---------------------------------------------------------------------------

function FindingRow({
  finding,
  groupId,
}: {
  finding: SecurityFindingView;
  groupId: string;
}) {
  const [open, setOpen] = useState(false);
  const { label, tone } = categoryMeta(finding.category);
  const hasPath = finding.path.length > 2; // more than just [source, sink]

  return (
    <div className="flex flex-col gap-1.5 px-3 py-2.5 rounded-lg border border-border bg-surface hover:bg-surface-2 transition-colors">
      <div className="flex items-center gap-2 min-w-0 flex-wrap">
        <Badge tone={tone} className="shrink-0">
          {label}
        </Badge>
        <EndpointRef endpoint={finding.source} groupId={groupId} icon="source" />
        <ArrowRight size={13} className="text-text-4 shrink-0" />
        <EndpointRef endpoint={finding.sink} groupId={groupId} icon="sink" />
        <div className="ml-auto flex items-center gap-1.5 shrink-0">
          {finding.hops > 0 && (
            <span
              className="text-[11px] text-text-4 tabular-nums"
              title={`${finding.hops} call hop(s) from source to sink`}
            >
              {finding.hops} hop{finding.hops === 1 ? "" : "s"}
            </span>
          )}
          <ConfidencePill value={finding.confidence} />
        </div>
      </div>

      {finding.explanation && (
        <p className="text-[12px] text-text-3 leading-snug">{finding.explanation}</p>
      )}

      {/* Source + sink refs */}
      <div className="flex flex-col gap-0.5">
        <EndpointRefLine endpoint={finding.source} />
        <EndpointRefLine endpoint={finding.sink} />
      </div>

      {/* Expandable inter-procedural path */}
      {hasPath && (
        <div>
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="inline-flex items-center gap-1 text-[11px] text-text-3 hover:text-text"
          >
            <ChevronRight
              size={12}
              className={cn("transition-transform", open && "rotate-90")}
            />
            {open ? "Hide" : "Show"} path ({finding.path.length} nodes)
          </button>
          {open && (
            <div className="mt-1 flex flex-wrap items-center gap-1 pl-4">
              {finding.path.map((step, i) => (
                <span key={`${step.entity_id}:${i}`} className="inline-flex items-center gap-1">
                  {i > 0 && <ArrowRight size={11} className="text-text-4" />}
                  <span
                    className="font-mono text-[11px] text-text-3"
                    title={step.entity_id}
                  >
                    {step.name}
                  </span>
                </span>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// § One flow row (request-input → sink edge)
// ---------------------------------------------------------------------------

function FlowRow({ flow, groupId }: { flow: TaintFlow; groupId: string }) {
  return (
    <div className="flex flex-col gap-1.5 px-3 py-2.5 rounded-lg border border-border bg-surface hover:bg-surface-2 transition-colors">
      <div className="flex items-center gap-2 min-w-0 flex-wrap">
        {flow.sink_kind && (
          <Badge tone={sinkKindTone(flow.sink_kind)} className="uppercase shrink-0">
            {flow.sink_kind}
          </Badge>
        )}
        <EndpointRef endpoint={flow.source} groupId={groupId} icon="source" />
        {flow.field && (
          <span
            className="font-mono text-[11px] text-text-4 shrink-0"
            title={`tainted request field: ${flow.field}`}
          >
            .{flow.field}
          </span>
        )}
        <ArrowRight size={13} className="text-text-4 shrink-0" />
        <EndpointRef endpoint={flow.sink} groupId={groupId} icon="sink" />
        <div className="ml-auto flex items-center gap-1.5 shrink-0">
          {(flow.hop_count ?? 0) > 0 && (
            <span className="text-[11px] text-text-4 tabular-nums">
              {flow.hop_count} hop{flow.hop_count === 1 ? "" : "s"}
            </span>
          )}
          {flow.confidence > 0 && <ConfidencePill value={flow.confidence} />}
        </div>
      </div>

      {flow.hop_path && (
        <p
          className="font-mono text-[11px] text-text-4 truncate"
          title={flow.hop_path}
        >
          {flow.hop_path}
        </p>
      )}

      <div className="flex flex-col gap-0.5">
        <EndpointRefLine endpoint={flow.source} />
        <EndpointRefLine endpoint={flow.sink} />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// § Findings tab
// ---------------------------------------------------------------------------

function FindingsTab({
  data,
  groupId,
}: {
  data: DataflowReport;
  groupId: string;
}) {
  const [category, setCategory] = useState<string>("all");

  const categories = useMemo(
    () =>
      Object.entries(data.findings_by_category).sort(
        (a, b) => b[1] - a[1] || a[0].localeCompare(b[0]),
      ),
    [data.findings_by_category],
  );

  const filtered = useMemo(
    () =>
      category === "all"
        ? data.findings
        : data.findings.filter((f) => f.category === category),
    [data.findings, category],
  );

  if (data.total_findings === 0) {
    return (
      <EmptyState
        title="No taint findings"
        hint="The taint pass found no request-input → dangerous-sink path that crosses the confidence floor for this group. Findings appear once an indexed repo has a tainted source reaching an unsanitized sink."
      />
    );
  }

  return (
    <div className="space-y-3">
      {categories.length > 1 && (
        <div className="flex flex-wrap items-center gap-2">
          <ListFilter size={13} className="text-text-4" />
          <Pill active={category === "all"} onClick={() => setCategory("all")}>
            All ({data.total_findings})
          </Pill>
          {categories.map(([cat, n]) => (
            <Pill key={cat} active={category === cat} onClick={() => setCategory(cat)}>
              {categoryMeta(cat).label} ({n})
            </Pill>
          ))}
        </div>
      )}
      <div className="space-y-2">
        {filtered.map((f) => (
          <FindingRow key={f.fingerprint} finding={f} groupId={groupId} />
        ))}
      </div>
      <p className="text-[10px] text-text-4">
        Ranked by aggregated taint confidence. The pass drops findings below the{" "}
        {Math.round(data.confidence_floor * 100)}% floor — anything shown crossed it
        and was not sanitized on the path.
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// § Flows tab
// ---------------------------------------------------------------------------

function FlowsTab({ data, groupId }: { data: DataflowReport; groupId: string }) {
  const [sinkKind, setSinkKind] = useState<string>("all");

  const kinds = useMemo(
    () =>
      Object.entries(data.flows_by_sink_kind).sort(
        (a, b) => b[1] - a[1] || a[0].localeCompare(b[0]),
      ),
    [data.flows_by_sink_kind],
  );

  const filtered = useMemo(
    () =>
      sinkKind === "all"
        ? data.flows
        : data.flows.filter((f) => f.sink_kind === sinkKind),
    [data.flows, sinkKind],
  );

  if (data.total_flows === 0) {
    return (
      <EmptyState
        title="No data flows resolved"
        hint="No request-input → sink DATA_FLOWS_TO edges were recorded for this group. They appear once the data-flow link pass soundly follows a request field into a sink."
      />
    );
  }

  return (
    <div className="space-y-3">
      {kinds.length > 1 && (
        <div className="flex flex-wrap items-center gap-2">
          <ListFilter size={13} className="text-text-4" />
          <Pill active={sinkKind === "all"} onClick={() => setSinkKind("all")}>
            All ({data.total_flows})
          </Pill>
          {kinds.map(([k, n]) => (
            <Pill key={k} active={sinkKind === k} onClick={() => setSinkKind(k)}>
              {k} ({n})
            </Pill>
          ))}
        </div>
      )}
      <div className="space-y-2">
        {filtered.map((f) => (
          <FlowRow key={f.id} flow={f} groupId={groupId} />
        ))}
      </div>
      <p className="text-[10px] text-text-4">
        Each row is one observed request-input → sink movement the sniffer followed
        (intra-function + bounded inter-procedural hops). A flow becomes a finding
        only when the taint pass judges it reaches a dangerous sink unsanitized.
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// § Screen
// ---------------------------------------------------------------------------

export default function DataflowScreen() {
  const { groupId = "" } = useParams<{ groupId: string }>();
  const { data, isLoading, isError } = useDataflow(groupId);
  const [tab, setTab] = useState<"findings" | "flows">("findings");

  const hasAny =
    (data?.total_findings ?? 0) > 0 || (data?.total_flows ?? 0) > 0;

  return (
    <div className="flex flex-col h-full bg-bg">
      {isLoading ? (
        <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4">
          <SkeletonRows />
        </div>
      ) : isError ? (
        <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4">
          <ErrorState />
        </div>
      ) : !hasAny ? (
        <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4">
          <EmptyState
            title="No data-flow or taint analysis available"
            hint="Neither taint findings nor request-input → sink flows were extracted for this group. They appear once an indexed repo has request handlers that reach data sinks (SQL / shell / fs / template / HTTP) — the taint and data-flow link passes then populate this view."
          />
        </div>
      ) : (
        <Tabs
          value={tab}
          onValueChange={(v) => setTab(v as typeof tab)}
          className="flex flex-col flex-1 min-h-0"
        >
          <div className="border-b border-border shrink-0 px-4">
            <TabsList className="border-0">
              <TabsTrigger value="findings">
                <ShieldAlert size={14} className="mr-1.5" />
                Findings
                {data!.total_findings > 0 && (
                  <Pill className="ml-1.5">{data!.total_findings}</Pill>
                )}
              </TabsTrigger>
              <TabsTrigger value="flows">
                <Waypoints size={14} className="mr-1.5" />
                Flows
                {data!.total_flows > 0 && (
                  <Pill className="ml-1.5">{data!.total_flows}</Pill>
                )}
              </TabsTrigger>
            </TabsList>
          </div>

          <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4 space-y-4">
            {/* Summary */}
            <div className="flex flex-wrap gap-3">
              <SummaryStat label="Findings" value={data!.total_findings} />
              <SummaryStat label="Flows" value={data!.total_flows} />
              <SummaryStat
                label="Vuln categories"
                value={Object.keys(data!.findings_by_category).length}
              />
              <SummaryStat
                label="Sink kinds"
                value={Object.keys(data!.flows_by_sink_kind).length}
              />
            </div>

            <TabsContent value="findings">
              <FindingsTab data={data!} groupId={groupId} />
            </TabsContent>
            <TabsContent value="flows">
              <FlowsTab data={data!} groupId={groupId} />
            </TabsContent>
          </div>
        </Tabs>
      )}
    </div>
  );
}
