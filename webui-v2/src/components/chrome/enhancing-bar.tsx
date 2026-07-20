/* ============================================================
   EnhancingBar — secondary "enhancing relationships in the background" bar
   (#47 phase 2). Web parity with the TUI's bgProgressBlock
   (internal/cli/wiztui/indexview.go): once the graph is queryable the main
   aggregate bar completes, and THIS indeterminate bar covers the background
   enrichment tail. It is indeterminate (a traveling segment) because the
   background pass reports no progress signal — only an independently-running
   elapsed timer conveys that work is ongoing. A queryable+enhancing group is
   success, never a failure.
   ============================================================ */

import { useEffect, useRef, useState } from "react";
import { Sparkles } from "lucide-react";

import { cn } from "@/lib/utils";

export interface EnhancingBarProps {
  /** Optional epoch-ms the enhancement started; defaults to first-mount time. */
  startedAt?: number;
  className?: string;
}

function elapsedText(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return `${m}m ${rem.toString().padStart(2, "0")}s`;
}

export function EnhancingBar({ startedAt, className }: EnhancingBarProps) {
  const startRef = useRef<number>(startedAt ?? Date.now());
  const [now, setNow] = useState<number>(Date.now());

  useEffect(() => {
    if (startedAt) startRef.current = startedAt;
  }, [startedAt]);

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const elapsed = elapsedText(now - startRef.current);

  return (
    <div className={cn("space-y-1", className)} data-testid="enhancing-bar">
      <div className="flex items-center justify-between gap-2 text-[11px] text-text-4">
        <span className="flex items-center gap-1.5">
          <Sparkles size={12} className="text-accent-strong" />
          Enhancing relationships in the background
        </span>
        <span className="tabular-nums">+{elapsed}</span>
      </div>
      <div className="enhancing-bar-track h-1.5 w-full rounded-full bg-surface-2">
        <div className="enhancing-bar-seg bg-accent/70" />
      </div>
    </div>
  );
}
