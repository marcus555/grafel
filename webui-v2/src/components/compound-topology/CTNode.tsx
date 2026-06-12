/* CTNode.tsx — one entity in the compound topology, tinted by its tier lane. */

import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { CTNodeData } from "./layout";
import { tierStyle } from "./tierStyle";

function CTNodeImpl({ data }: NodeProps) {
  const d = data as CTNodeData;
  const s = tierStyle(d.tier);

  // Cross-link highlight (Model 2). "primary" = same entity in the other lens,
  // "linked" = edge-connected counterpart. Non-matching nodes dim when a
  // selection is active so the cross-link reads at a glance.
  const hl = d.highlight;
  const isPrimary = hl === "primary";
  const isLinked = hl === "linked";
  const dim = d.dimmed && hl === "none";
  const borderColor = isPrimary ? "var(--accent)" : isLinked ? "var(--info)" : s.color;

  return (
    <div
      className="flex h-full w-full flex-col justify-center rounded-md border px-2.5 py-1 transition-opacity"
      style={{
        borderColor,
        borderWidth: isPrimary || isLinked ? 2 : 1,
        background: s.tint,
        opacity: dim ? 0.28 : 1,
        boxShadow: isPrimary
          ? "0 0 0 2px color-mix(in srgb, var(--accent) 40%, transparent)"
          : isLinked
            ? "0 0 0 2px color-mix(in srgb, var(--info) 32%, transparent)"
            : undefined,
      }}
      title={`${d.label} · ${d.kind} · ${s.label}${d.repo ? ` · ${d.repo}` : ""}${
        isPrimary ? " · cross-linked (same entity)" : isLinked ? " · linked by edge" : ""
      }`}
    >
      <Handle type="target" position={Position.Left} className="!h-1.5 !w-1.5 !border-0 !bg-text-4" />
      <span className="truncate text-[11px] font-medium text-text" style={{ color: s.color }}>
        {d.label || d.kind}
      </span>
      <span className="truncate text-[9px] uppercase tracking-wide text-text-4">{d.kind}</span>
      <Handle type="source" position={Position.Right} className="!h-1.5 !w-1.5 !border-0 !bg-text-4" />
    </div>
  );
}

export const CTNode = memo(CTNodeImpl);
