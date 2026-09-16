import React from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { RefreshCw } from "lucide-react";
import { Badge, Button, Card, InlineNotice, LoadingRow } from "../../components/ui";
import { fetchImportRecovery, retryImportOperation } from "../../lib/api";
import { keys, useInvalidatingMutation } from "../../lib/queries";
import { formatBytes } from "../../lib/format";

export default function ImportRecovery() {
  const query = useQuery({ queryKey: keys.importRecovery, queryFn: fetchImportRecovery, refetchInterval: 15_000 });
  const retry = useInvalidatingMutation(retryImportOperation, [keys.importRecovery, keys.libraryFiles("any"), keys.libraryFiles("ebook"), keys.libraryFiles("audiobook"), keys.wanted, keys.downloads()]);
  const report = query.data;
  return <Card title="Import recovery" subtitle="Resume interrupted imports using their saved file plan. Originals stay in place until verified cleanup.">
    {query.isLoading ? <LoadingRow label="Loading import recovery…" /> : null}
    {query.isError ? <InlineNotice tone="danger">{query.error.message} <Button size="sm" onClick={() => void query.refetch()}>Try again</Button></InlineNotice> : null}
    {retry.isError ? <InlineNotice tone="danger">{retry.error.message}</InlineNotice> : null}
    {retry.isSuccess ? <InlineNotice tone="success">Import verified and committed.</InlineNotice> : null}
    {report ? <>
      <p className="field-hint">{report.unfinished} unfinished imports · {report.unresolved} unresolved legacy links. Showing up to {report.limit} operations and link issues.</p>
      {!report.operations.length ? <p className="field-hint">New manual and completed-download imports will appear here.</p> : null}
      <div className="imports-recovery-list">
        {report.operations.map(operation => <details key={operation.id} className="imports-recovery-item">
          <summary>
            <span>{operation.metadata?.title || operation.downloadId}</span>
            <Badge tone={operation.state === "committed" ? "success" : operation.state === "failed" ? "danger" : "warn"}>{operation.state}</Badge>
          </summary>
          <p className="field-hint">{operation.sourceKind === "manual" ? "Manual file import" : operation.client} · {operation.attempts} {operation.attempts === 1 ? "attempt" : "attempts"} · Cleanup: {operation.sourceKind === "manual" ? (operation.cleanupState === "cleaned" ? (operation.files.some(file => file.sourceRemoved) ? "move completed" : "complete; source retained") : "pending") : operation.cleanupState === "cleaned" ? "source removed" : operation.cleanupState === "eligible" ? "verified" : "source retained"}</p>
          {operation.wantedId ? <Link to={`/library/book/${encodeURIComponent(operation.wantedId)}`}>View book</Link> : <span className="field-hint">No book assigned</span>}
          {operation.lastError || operation.cleanupError ? <InlineNotice tone="warn">{operation.lastError || operation.cleanupError}</InlineNotice> : null}
          <ul>{operation.files.map(file => <li key={file.id}>
            <div className="imports-recovery-path">{file.sourcePath}</div>
            <div className="imports-recovery-path">→ {file.destinationPath}</div>
            <span className="field-hint">{formatBytes(file.sizeBytes)} · {file.state}</span>
            {file.previousPath && operation.cleanupState !== "cleaned" ? <div className="field-hint imports-recovery-path">Previous file recovery path: {file.previousPath}</div> : null}
            {file.stagePath ? <div className="field-hint imports-recovery-path">Temporary copy recorded for recovery: {file.stagePath}</div> : null}
          </li>)}</ul>
          {operation.state !== "committed" || (operation.sourceKind === "manual" && operation.cleanupState !== "cleaned") ? <Button size="sm" icon={RefreshCw} busy={retry.isPending && retry.variables === operation.id} disabled={retry.isPending} onClick={() => retry.mutate(operation.id)}>{operation.state === "committed" ? "Retry cleanup" : "Retry import"}</Button> : null}
        </details>)}
      </div>
      {report.issues.length ? <details className="imports-recovery-item">
        <summary>Legacy links needing review ({report.unresolved})</summary>
        <p className="field-hint">These files are retained. Ambiguous links do not authorize download cleanup.</p>
        <ul>{report.issues.map(issue => <li key={`${issue.fileId}:${issue.kind}`}><div className="imports-recovery-path">{issue.path}</div><span>{issue.reason}</span></li>)}</ul>
      </details> : null}
    </> : null}
  </Card>;
}
