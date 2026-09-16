import React, { useMemo, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Badge, Button, Card, InlineNotice } from "../../components/ui";
import { keys, useInvalidatingMutation, useWanted } from "../../lib/queries";
import { previewPayloadReview, resolveLibraryImportReview, type ImportReview, type PayloadFile, type PayloadReviewRequest } from "../../lib/api";
import { formatBytes } from "../../lib/format";

const retain = "__retain";
export default function PayloadReview({ review }: { review: ImportReview }) {
  const books = useWanted();
  const payload = review.metadata?.payload as { files?: PayloadFile[] } | undefined;
  const files = useMemo(() => payload?.files ?? [], [payload]);
  const [assignments, setAssignments] = useState<Record<string, string>>({});
  const [allBook, setAllBook] = useState(review.wantedId ?? "");
  const [confirmed, setConfirmed] = useState(false);
  const [mode, setMode] = useState<PayloadReviewRequest["importMode"]>("hardlinkOrCopy");
  const [conflict, setConflict] = useState<PayloadReviewRequest["conflictAction"]>("rename");
  const valueFor = (file: PayloadFile) => assignments[file.relativePath] ?? (file.format === "sidecar" ? "" : review.wantedId ?? "");
  const request: PayloadReviewRequest = {
    action: "import", wantedId: review.wantedId, importMode: mode, conflictAction: conflict, confirmIdentity: confirmed,
    mapping: files.filter(file => file.format !== "excluded").map(file => ({
      relativePath: file.relativePath, wantedId: valueFor(file) === retain ? undefined : valueFor(file), exclude: valueFor(file) === retain
    }))
  };
  const preview = useMutation({ mutationFn: (options: PayloadReviewRequest) => previewPayloadReview(review.id, options) });
  const resolve = useInvalidatingMutation((options: Parameters<typeof resolveLibraryImportReview>[1]) => resolveLibraryImportReview(review.id, options), [
    keys.importReviews("pending"), keys.importReviews("all"), keys.importReviews("resolved"), keys.importRecovery,
    keys.libraryFiles("any"), keys.libraryFiles("ebook"), keys.libraryFiles("audiobook"), keys.wanted, keys.downloads()
  ]);
  const currentPreview = preview.isSuccess && JSON.stringify(preview.variables) === JSON.stringify(request);
  const busy = preview.isPending || resolve.isPending;
  const media = files.filter(file => file.format === "ebook" || file.format === "audiobook");
  function change(path: string, value: string) { setAssignments(previous => ({ ...previous, [path]: value })); setConfirmed(false); }
  const options = (format?: string) => (books.data ?? []).filter(book => !format || book.format === format).map(book =>
    <option key={book.id} value={book.id}>{book.title} — {book.authorName || "Unknown author"} ({book.format})</option>);
  return <Card title={`Review files: ${review.title || "Completed download"}`} subtitle={`${media.length} book files · ${formatBytes(review.sizeBytes ?? 0)} · Originals are retained during import.`}>
    <InlineNotice tone="warn">{review.reason}</InlineNotice>
    {books.isError ? <InlineNotice tone="danger">Book choices could not be loaded. <Button size="sm" onClick={() => void books.refetch()}>Try again</Button></InlineNotice> : null}
    <div className="imports-payload-tools">
      <label className="field">Book for all media files
        <select value={allBook} onChange={event => setAllBook(event.target.value)} aria-label="Book for all media files" disabled={busy}>
          <option value="">Choose a book</option>{options()}
        </select>
      </label>
      <Button size="sm" disabled={!allBook || busy} onClick={() => { setAssignments(previous => ({ ...previous, ...Object.fromEntries(media.map(file => [file.relativePath, allBook])) })); setConfirmed(false); }}>Apply book to all</Button>
      <label className="field">Transfer mode<select value={mode} onChange={event => setMode(event.target.value as PayloadReviewRequest["importMode"])} disabled={busy}>
        <option value="hardlinkOrCopy">Hardlink or copy</option><option value="copy">Copy</option><option value="hardlink">Hardlink</option>
      </select></label>
      <label className="field">Existing destinations<select aria-label="Existing destinations" value={conflict} onChange={event => { setConflict(event.target.value as PayloadReviewRequest["conflictAction"]); setConfirmed(false); }} disabled={busy}>
        <option value="rename">Keep both</option><option value="replace">Replace reviewed files</option>
      </select></label>
    </div>
    <ol className="imports-payload-files">
      {files.map(file => <li key={file.relativePath}>
        <div className="imports-recovery-path"><strong>{file.relativePath}</strong></div>
        <span className="field-hint">{formatBytes(file.sizeBytes)} · {file.format} · {Math.round(file.progress * 100)}% downloaded</span>
        {file.album || file.title || file.author ? <p className="field-hint">Embedded metadata: {file.album || file.title} {file.author ? `— ${file.author}` : ""}</p> : null}
        {file.reason ? <div><Badge tone="warn">{file.reason}</Badge></div> : null}
        {file.format === "excluded" ? <p className="field-hint">Excluded from import: unsupported ancillary file.</p> : <label className="field">Assign file
          <select aria-label={`Book for ${file.relativePath}`} value={valueFor(file)} onChange={event => change(file.relativePath, event.target.value)} disabled={busy}>
            <option value="">{file.format === "sidecar" ? "Match sidecar to its book folder" : "Choose a book"}</option>
            {options(file.format === "sidecar" ? undefined : file.format)}
            <option value={retain}>Keep in downloads; do not import</option>
          </select>
        </label>}
      </li>)}
    </ol>
    <label className="imports-payload-confirm"><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} disabled={busy} />I checked these book assignments and any excluded files. These choices override conflicting metadata.</label>
    <p className="field-hint">{conflict === "replace" ? "Matching destinations will be replaced after their new copies verify. Existing bytes remain recoverable until the complete import commits. A different existing chapter set needs separate review." : "Existing library files are kept."} Retaining a book file or sidecar in downloads blocks automatic source deletion.</p>
    {preview.isError ? <InlineNotice tone="danger">{preview.error.message}</InlineNotice> : null}
    {resolve.isError ? <InlineNotice tone="danger">{resolve.error.message}</InlineNotice> : null}
    <div className="cell-actions">
      <Button variant="ghost" disabled={busy} onClick={() => resolve.mutate({ action: "skip" })}>Skip this download</Button>
      <Button variant="danger" disabled={busy} onClick={() => resolve.mutate({ action: "reject" })}>Reject this download</Button>
    </div>
    <p className="field-hint">Skip and Reject retain the original files and stop automatic import until you reopen this review.</p>
    <Button disabled={!confirmed || busy} busy={preview.isPending} onClick={() => preview.mutate(request)}>Preview destinations</Button>
    {currentPreview ? <div className="imports-payload-preview">
      <h3>Import preview</h3>
      <ol>{preview.data.operation.files.map((file, index) => <li key={`${file.sourcePath}:${index}`}><div className="imports-recovery-path">{file.sourcePath}</div><div className="imports-recovery-path">→ {file.destinationPath}</div>{file.previousPath ? <p className="field-hint">Replaces an existing {formatBytes(file.previousSizeBytes ?? 0)} file. The preview is bound to its current content.</p> : null}</li>)}</ol>
      <Button variant="primary" busy={resolve.isPending} disabled={busy} onClick={() => resolve.mutate({ ...request, previewToken: preview.data.fingerprint }, { onError: () => preview.reset() })}>Import this file set</Button>
    </div> : null}
  </Card>;
}
