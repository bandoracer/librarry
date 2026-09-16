import React, { useState } from "react";
import { Link } from "react-router-dom";
import { Button, InlineNotice, Modal } from "../../components/ui";
import { fetchWantedItem, restoreBook, type WantedItem } from "../../lib/api";
import { keys, useInvalidatingMutation, useRootFolders } from "../../lib/queries";
import { formatDateTime } from "../../lib/format";

export default function RestoreBookDialog({ book, onClose, onRestored }: { book: WantedItem; onClose: () => void; onRestored: (book: WantedItem) => void }) {
  const [reviewed, setReviewed] = useState(book);
  const roots = useRootFolders();
  const [monitored, setMonitored] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const mutation = useInvalidatingMutation((options: { updatedAt: string; monitored: boolean }) => restoreBook(reviewed.id, options), [keys.wanted, keys.acquisitionQueue, keys.authorSubscriptions, keys.wantedMetadataReview, keys.history()]);
  const busy = loading || mutation.isPending;
  const inactive = ["removed", "ignored"].includes(reviewed.status);
  async function reload() {
    setLoading(true); setError("");
    try { const current = await fetchWantedItem(reviewed.id); if (!current) throw new Error("Book no longer exists."); setReviewed(current); setMonitored(false); }
    catch (e) { setError(e instanceof Error ? e.message : "Book could not be loaded"); }
    finally { setLoading(false); }
  }
  async function restore() {
    setError("");
    try { onRestored(await mutation.mutateAsync({ updatedAt: reviewed.updatedAt, monitored })); }
    catch (e) { setError(e instanceof Error ? e.message : "Book could not be restored"); }
  }
  return <Modal title="Restore book" open onClose={() => { if (!busy) onClose(); }} footer={<>
    <Button variant="ghost" disabled={busy} onClick={onClose}>Cancel</Button>
    <Button variant="primary" disabled={busy || !inactive} busy={mutation.isPending} onClick={() => void restore()}>Restore to Library</Button>
  </>}>
    <p><strong>{reviewed.title}</strong> — {reviewed.authorName || "Unknown author"}</p>
    <p>{reviewed.format} · {reviewed.qualityProfile} · {reviewed.status}</p>
    <p className="field-hint">Saved destination: {reviewed.rootFolderId ? roots.data?.find(root => root.id === reviewed.rootFolderId)?.name || "saved destination (details unavailable)" : "format default"}. Tags: {reviewed.tags?.join(", ") || "none"}. Last updated: {formatDateTime(reviewed.updatedAt)}.</p>
    <p>Restore this tracking record with its existing file links, history, metadata overrides and settings. Restoring does not move or download files.</p>
    <label className="library-restore-monitor"><input type="checkbox" checked={monitored} disabled={busy || !inactive} onChange={event => setMonitored(event.target.checked)} />Enable monitoring after restore</label>
    <p className="field-hint">With monitoring enabled, scheduled automation can search for and grab missing books or upgrades. Author monitoring rules stay unchanged.</p>
    {!inactive ? <InlineNotice tone="info">This book is already active. <Link to={`/library/book/${encodeURIComponent(reviewed.id)}`} onClick={onClose}>Open book</Link></InlineNotice> : null}
    {error ? <InlineNotice tone="danger">{error}</InlineNotice> : null}
    <Button size="sm" disabled={busy} onClick={() => void reload()}>Reload book</Button>
  </Modal>;
}
