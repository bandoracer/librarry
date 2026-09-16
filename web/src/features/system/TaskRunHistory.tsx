import React, { useState } from "react";
import { createPortal } from "react-dom";
import { useQuery } from "@tanstack/react-query";
import { Badge, Button, InlineNotice, LoadingRow, Modal } from "../../components/ui";
import { fetchTaskRuns } from "../../lib/api";
import { formatRelativeTime } from "../../lib/format";

export default function TaskRunHistory({ id, name }: { id: string; name: string }) {
  const [open, setOpen] = useState(false);
  const runs = useQuery({ queryKey: ["system-task-runs", id], queryFn: () => fetchTaskRuns(id), enabled: open, refetchInterval: open ? 10_000 : false });
  return <>
    <Button size="sm" onClick={() => setOpen(true)} aria-label={`View ${name} run history`}>History</Button>
    {open ? createPortal(<Modal open={open} onClose={() => setOpen(false)} title={`${name} run history`}>
      <p className="field-hint">The latest 100 runs are retained across restarts. Interrupted means completion could not be verified.</p>
      {runs.isPending ? <LoadingRow label="Loading task history…" /> : null}
      {runs.isError ? <InlineNotice tone="danger">{runs.error.message} <Button size="sm" onClick={() => void runs.refetch()}>Try again</Button></InlineNotice> : null}
      {runs.data?.length === 0 ? <p>No recorded runs yet.</p> : null}
      {runs.data?.map(run => <article key={run.id} className="task-history-entry">
        <p><Badge tone={run.state === "completed" ? "success" : run.state === "running" ? "info" : "warn"}>{run.state}</Badge> · {run.trigger} · {formatRelativeTime(run.startedAt)}</p>
        {run.outcome ? <p>{run.outcome}</p> : null}
        {run.error ? <InlineNotice tone="warn">{run.error}</InlineNotice> : null}
      </article>)}
    </Modal>, document.body) : null}
  </>;
}
