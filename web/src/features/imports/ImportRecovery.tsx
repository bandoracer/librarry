import React, { useState } from "react";
import CalibreRecovery from "./CalibreRecovery";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { RefreshCw } from "lucide-react";
import { Badge, Button, Card, InlineNotice, LoadingRow } from "../../components/ui";
import { fetchImportRecovery, retryImportOperation, type RecoveryPage } from "../../lib/api";
import { keys, useInvalidatingMutation } from "../../lib/queries";
import { formatBytes } from "../../lib/format";

type Collection = "operations" | "calibre" | "issues";
const firstPages = (): Record<Collection, string[]> => ({ operations: [""], calibre: [""], issues: [""] });
function RecoveryPaging({ label, page, history, busy, change }: { label: string; page?: RecoveryPage; history: string[]; busy: boolean; change: (next?: string) => void }) {
  if (!page) return null;
  return <nav aria-label={`${label} pages`} className="imports-recovery-paging">
    <span className="field-hint">{page.total} matching · Page {history.length}</span>
    <Button size="sm" disabled={busy || history.length === 1} onClick={() => change()}>Previous {label}</Button>
    <Button size="sm" disabled={busy || !page.nextCursor} onClick={() => change(page.nextCursor)}>Next {label}</Button>
  </nav>;
}
export default function ImportRecovery() {
  const [pages, setPages] = useState(firstPages);
  const [searchParams] = useSearchParams();
  const [unfinishedOnly, setUnfinishedOnly] = useState(() => searchParams.get("unfinishedOnly") === "true");
  const options = { unfinishedOnly, operationsCursor: pages.operations[pages.operations.length - 1], calibreCursor: pages.calibre[pages.calibre.length - 1], issuesCursor: pages.issues[pages.issues.length - 1] };
  const query = useQuery({ queryKey: [...keys.importRecovery, options], queryFn: () => fetchImportRecovery(options), refetchInterval: 15_000, placeholderData: keepPreviousData });
  const paging = (collection: Collection, label: string, page?: RecoveryPage) => <RecoveryPaging label={label} page={page} history={pages[collection]} busy={query.isFetching} change={next => setPages(previous => ({ ...previous, [collection]: next ? [...previous[collection], next] : previous[collection].slice(0, -1) }))} />;
  const retry = useInvalidatingMutation(retryImportOperation, [keys.importRecovery, keys.libraryFiles("any"), keys.libraryFiles("ebook"), keys.libraryFiles("audiobook"), keys.wanted, keys.downloads()]);
  const report = query.data;
  return <Card title="Import recovery" subtitle="Resume interrupted imports and renames using their saved file plan. Originals stay in place until verified cleanup.">
    {query.isLoading ? <LoadingRow label="Loading import recovery…" /> : query.isFetching ? <LoadingRow label="Updating import recovery…" /> : null}
    {query.isError ? <InlineNotice tone="danger">{query.error.message} <Button size="sm" onClick={() => void query.refetch()}>Try again</Button></InlineNotice> : null}
    {retry.isError ? <InlineNotice tone="danger">{retry.error.message}</InlineNotice> : null}
    {retry.isSuccess ? <InlineNotice tone="success">Recovery completed.</InlineNotice> : null}
    {report ? <>
      <p className="field-hint">{report.unfinished} unfinished operations · {report.unresolved} unresolved legacy links. Newest created records first; up to {report.limit} per collection.</p>
      <label><input type="checkbox" checked={unfinishedOnly} onChange={event => { setUnfinishedOnly(event.target.checked); setPages(firstPages()); }} /> Show only unfinished imports and Calibre handoffs</label>
      <p className="field-hint">These lists update as work changes. Return to the first page to see newly created records.</p>
      {Object.values(pages).some(history => history.length > 1) ? <Button size="sm" onClick={() => setPages(firstPages())}>Return to first pages</Button> : null}
      {!report.operations.length ? <p className="field-hint">No operations on this page. New imports and file renames appear here.</p> : null}
      <div className="imports-recovery-list">
        {report.operations.map(operation => { const bookId = operation.wantedId || operation.metadata?.renameWantedId; const cleanupPending = operation.state === "committed" && (operation.replacementCleanupState === "pending" || (operation.sourceKind === "manual" && operation.cleanupState !== "cleaned")); return <details key={operation.id} className="imports-recovery-item">
          <summary>
            <span>{operation.metadata?.title || operation.downloadId || "Untitled import"}</span>
            <Badge tone={cleanupPending ? "warn" : operation.state === "committed" ? "success" : operation.state === "failed" ? "danger" : "warn"}>{cleanupPending ? "cleanup pending" : operation.state}</Badge>
          </summary>
          <p className="field-hint">{operation.metadata?.renameWantedId ? "Saved book folder rename" : operation.metadata?.renameFileId ? "Saved file rename" : operation.sourceKind === "manual" ? "Manual file import" : operation.client} · {operation.attempts} {operation.attempts === 1 ? "attempt" : "attempts"} · Cleanup: {operation.sourceKind === "manual" ? (operation.cleanupState === "cleaned" ? (operation.files.some(file => file.sourceRemoved) ? "move completed" : "complete; source retained") : "pending") : operation.cleanupState === "cleaned" ? "source removed" : operation.cleanupState === "eligible" ? "verified" : "source retained"}</p>
          {operation.recovery ? <div className="field-hint">
            <p>{operation.recovery.verifiedFiles} of {operation.recovery.totalFiles} manifest files verified · Last recorded activity: {new Date(operation.recovery.recordedAt).toLocaleString()}</p>
            {operation.recovery.leaseState === "held" ? <p>{operation.recovery.leasePurpose === "cleanup" ? "Cleanup" : "Transfer"} lease held until {new Date(operation.recovery.leaseExpiresAt!).toLocaleString()}. A lease does not measure file progress.</p> : operation.recovery.leaseState === "expired" ? <p>{operation.recovery.leasePurpose === "cleanup" ? "Cleanup" : "Transfer"} lease expired. This does not prove the previous process has stopped. Retry checks ownership and the saved file plan.</p> : operation.recovery.leaseState === "none" ? <p>No {operation.recovery.leasePurpose === "cleanup" ? "cleanup" : "transfer"} lease recorded. Retry checks ownership and the saved file plan.</p> : <p>Import and any required local cleanup are committed.</p>}
            <p>Observed {new Date(operation.recovery.observedAt).toLocaleString()}. Large files may take time between recorded updates.</p>
          </div> : null}
          {bookId ? <Link to={`/library/book/${encodeURIComponent(bookId)}`}>View book</Link> : <span className="field-hint">{operation.metadata?.renameFileId ? "Existing book links are preserved" : "No book assigned"}</span>}
          {operation.replacementCleanupState && operation.replacementCleanupState !== "none" ? <p className="field-hint">Replacement backups: {operation.replacementCleanupState === "cleaned" ? "cleanup complete" : "cleanup pending"}</p> : null}
          {operation.replacementCleanupError ? <InlineNotice tone="warn">{operation.replacementCleanupError}</InlineNotice> : null}
          {operation.lastError || operation.cleanupError ? <InlineNotice tone="warn">{operation.lastError || operation.cleanupError}</InlineNotice> : null}
          <ul>{operation.files.map(file => <li key={file.id}>
            <div className="imports-recovery-path">{file.sourcePath}</div>
            <div className="imports-recovery-path">→ {file.destinationPath}</div>
            <span className="field-hint">{formatBytes(file.sizeBytes)} · {file.state}</span>
            {file.previousPath && (operation.replacementCleanupState === "pending" || (operation.sourceKind === "manual" && operation.cleanupState !== "cleaned")) ? <div className="field-hint imports-recovery-path">Previous file recovery path: {file.previousPath}</div> : null}
            {file.stagePath ? <div className="field-hint imports-recovery-path">Temporary copy recorded for recovery: {file.stagePath}</div> : null}
          </li>)}</ul>
          {operation.state !== "committed" || operation.replacementCleanupState === "pending" || (operation.sourceKind === "manual" && operation.cleanupState !== "cleaned") ? <Button size="sm" icon={RefreshCw} busy={retry.isPending && retry.variables === operation.id} disabled={query.isPlaceholderData || retry.isPending || operation.recovery?.leaseState === "held"} onClick={() => retry.mutate(operation.id)}>{operation.state === "committed" ? "Retry cleanup" : operation.metadata?.renameFileId ? "Retry rename" : "Retry import"}</Button> : null}
        </details>; })}
      </div>
      {paging("operations", "imports", report.operationsPage)}
      <CalibreRecovery handoffs={report.calibreHandoffs} unfinished={report.calibreUnfinished} />
      {report.calibrePage?.total === 0 ? <p className="field-hint">No Calibre handoffs match this filter.</p> : null}
      {paging("calibre", "Calibre handoffs", report.calibrePage)}
      {report.unresolved || pages.issues.length > 1 ? <details className="imports-recovery-item">
        <summary>Legacy links needing review ({report.unresolved})</summary>
        <p className="field-hint">These files are retained. Ambiguous links do not authorize download cleanup.</p>
        {!report.issues.length ? <p>No unresolved links on this page.</p> : null}
        <ul>{report.issues.map(issue => <li key={`${issue.fileId}:${issue.kind}`}><div className="imports-recovery-path">{issue.path}</div><span>{issue.reason}</span></li>)}</ul>
        {paging("issues", "legacy links", report.issuesPage)}
      </details> : null}
    </> : null}
  </Card>;
}
