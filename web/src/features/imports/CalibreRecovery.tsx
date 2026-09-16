import React, { useState } from "react";
import { Link } from "react-router-dom";
import { Badge, Button, InlineNotice } from "../../components/ui";
import { CalibreHandoff, retryCalibreHandoff, resolveCalibreHandoff } from "../../lib/api";
import { keys, useInvalidatingMutation } from "../../lib/queries";

const invalidations = [keys.importRecovery, keys.libraryFiles("any"), keys.libraryFiles("ebook"), keys.libraryFiles("audiobook"), keys.wanted, keys.downloads()];
function Handoff({ handoff: h }: { handoff: CalibreHandoff }) {
  const [confirm, setConfirm] = useState(false);
  const [bookId, setBookId] = useState("");
  const retry = useInvalidatingMutation(retryCalibreHandoff, invalidations);
  const resolve = useInvalidatingMutation(resolveCalibreHandoff, invalidations);
  const busy = retry.isPending || resolve.isPending;
  const conversions = h.conversions || [];
  const needsReview = h.phase === "uploading" || conversions.some(c => ["starting", "polling", "unknown", "failed"].includes(c.state));
  const act = (action: string, format?: string) => resolve.mutate({ id: h.id, action, confirm, format, ...(action === "attach-book" ? { bookId: Number(bookId) } : {}) }, { onSuccess: () => setConfirm(false) });
  return <details className="imports-recovery-item">
    <summary><span className="imports-recovery-path">{h.sourcePath}</span><Badge tone={h.phase === "committed" ? "success" : "warn"}>{h.phase}</Badge></summary>
    <p className="field-hint">Calibre handoff · {h.bookId ? `Saved book ID ${h.bookId}` : "No acknowledged book ID"} · Source retained</p>
    {h.wantedId ? <Link to={`/library/book/${encodeURIComponent(h.wantedId)}`}>View book</Link> : null}
    {h.lastError ? <InlineNotice tone="warn">{h.lastError}</InlineNotice> : null}
    {retry.isError || resolve.isError ? <InlineNotice tone="danger">{retry.error?.message || resolve.error?.message}</InlineNotice> : null}
    {retry.isSuccess ? <InlineNotice tone={retry.data.imported ? "success" : "warn"}>{retry.data.message}</InlineNotice> : null}
    {resolve.isSuccess ? <InlineNotice tone="success">Decision saved. Retry the handoff to continue.</InlineNotice> : null}
    {needsReview ? <>
      <p>Inspect the original Calibre library before resolving this handoff. Attaching a book accepts your identification of it. Another upload or conversion may create a duplicate if the earlier request is still running.</p>
      <label><input type="checkbox" checked={confirm} onChange={e => setConfirm(e.target.checked)} /> I checked the original library and confirmed the previous request has stopped.</label>
    </> : null}
    {h.phase === "uploading" ? <div>
      <label>Existing Calibre book ID <input type="number" min="1" step="1" value={bookId} onChange={e => setBookId(e.target.value)} /></label>
      <Button size="sm" disabled={busy || !confirm || !Number.isSafeInteger(Number(bookId)) || Number(bookId) <= 0} onClick={() => act("attach-book")}>Attach existing book</Button>
      <Button size="sm" disabled={busy || !confirm} onClick={() => act("retry-upload")}>Confirmed absent: allow another upload</Button>
    </div> : null}
    <ul>{conversions.map(c => <li key={c.format}>
      {c.format}: {c.state}{c.jobId !== undefined ? ` · Job ${c.jobId}` : ""}
      {["starting", "polling", "unknown", "failed"].includes(c.state) ? <div>
        <Button size="sm" disabled={busy || !confirm} onClick={() => act("use-format", c.format)}>Use existing {c.format}</Button>
        <Button size="sm" disabled={busy || !confirm} onClick={() => act("retry-conversion", c.format)}>Allow another {c.format} conversion</Button>
      </div> : null}
    </li>)}</ul>
    {h.phase !== "committed" ? <Button size="sm" disabled={busy || needsReview} busy={retry.isPending} onClick={() => retry.mutate(h.id)}>Retry Calibre handoff</Button> : null}
  </details>;
}
export default function CalibreRecovery({ handoffs = [], unfinished = 0 }: { handoffs: CalibreHandoff[]; unfinished: number }) {
  if (!handoffs.length) return null;
  return <section aria-label="Calibre recovery"><h3>Calibre handoffs</h3><p className="field-hint">{unfinished} unfinished. Saved book IDs survive metadata, conversion, and local bookkeeping failures.</p>{handoffs.map(h => <Handoff key={h.id} handoff={h} />)}</section>;
}
