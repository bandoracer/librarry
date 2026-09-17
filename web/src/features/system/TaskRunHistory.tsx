import React, { useState } from "react";
import { createPortal } from "react-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Badge, Button, InlineNotice, LoadingRow, Modal } from "../../components/ui";
import { fetchTaskRuns, reviewTaskRun, type TaskRun } from "../../lib/api";
import { operabilityKeys } from "../../lib/queries";
import { formatRelativeTime } from "../../lib/format";

export default function TaskRunHistory({ id, name }: { id: string; name: string }) {
  const [open, setOpen] = useState(false);
  const [view, setView] = useState("all");
  const [offset, setOffset] = useState(0);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const client = useQueryClient();
  const runs = useQuery({ queryKey: ["system-task-runs", id, view, offset], queryFn: () => fetchTaskRuns(id, view, offset), enabled: open, refetchInterval: open ? 10_000 : false });
  async function review(run: TaskRun) {
    setBusy(run.id); setError("");
    try {
      await reviewTaskRun(id, run, !run.reviewedAt);
      await client.invalidateQueries({ queryKey: ["system-task-runs", id] });
      await client.invalidateQueries({ queryKey: operabilityKeys.systemTasks });
    } catch (e) { setError(e instanceof Error ? e.message : "Could not save task review"); }
    finally { setBusy(""); }
  }
  return <>
    <Button size="sm" onClick={() => setOpen(true)} aria-label={`View ${name} run history`}>History</Button>
    {open ? createPortal(<Modal open={open} onClose={() => setOpen(false)} title={`${name} run history`}>
      <p className="field-hint">The latest 100 successful runs are retained. Unreviewed failures stay available. Reviewed failures become eligible for cleanup after 90 days. Reviewing does not fix or rerun a task.</p>
      <label>Show <select aria-label="Task history view" value={view} onChange={event => { setView(event.target.value); setOffset(0); }}><option value="all">All retained runs</option><option value="unreviewed">Unreviewed failures</option></select></label>
      {runs.isPending ? <LoadingRow label="Loading task history…" /> : null}
      {runs.isError ? <InlineNotice tone="danger">{runs.error.message} <Button size="sm" onClick={() => void runs.refetch()}>Try again</Button></InlineNotice> : null}
      {error ? <InlineNotice tone="danger">{error}</InlineNotice> : null}
      {runs.data?.runs.length === 0 ? <p>No matching runs on this page.</p> : null}
      {runs.data?.runs.map(run => <article key={run.id} className="task-history-entry">
        <p><Badge tone={run.state === "completed" ? "success" : run.state === "running" ? "info" : "warn"}>{run.state}</Badge> · {run.trigger} · {formatRelativeTime(run.startedAt)}{run.durationMs != null ? ` · ${(run.durationMs / 1000).toFixed(1)}s` : ""}</p>
        <p className="field-hint">Run {run.id}</p>
        {run.outcome ? <p>{run.outcome}</p> : null}
        {run.error ? <InlineNotice tone="warn">{run.error}</InlineNotice> : null}
        {Object.entries(run.details?.counts ?? {}).length ? <p>{Object.entries(run.details?.counts ?? {}).map(([key, value]) => `${key}: ${value}`).join(" · ")}</p> : null}
        {run.details?.operationIds?.map(operation => <p className="field-hint" key={operation}>Operation {operation}</p>)}
        {run.state !== "completed" && run.details?.nextAction ? <p>{run.details.nextAction}</p> : null}
        {["failed", "interrupted", "degraded"].includes(run.state) ? <Button size="sm" busy={busy === run.id} disabled={!!busy} onClick={() => void review(run)}>{run.reviewedAt ? "Mark unreviewed" : "Mark reviewed"}</Button> : null}
        {run.reviewedAt ? <p className="field-hint">Reviewed {formatRelativeTime(run.reviewedAt)}</p> : null}
      </article>)}
      {runs.data ? <div className="task-history-pagination">
        <Button size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - 100))}>Previous runs</Button>
        <span>{runs.data.total} retained {runs.data.total === 1 ? "run" : "runs"}</span>
        <Button size="sm" disabled={offset + runs.data.runs.length >= runs.data.total} onClick={() => setOffset(offset + 100)}>Next runs</Button>
      </div> : null}
    </Modal>, document.body) : null}
  </>;
}
