import React, { useState } from "react";
import "./kindle.css";
import { Link } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, InlineNotice } from "../../components/ui";
import { getKindleHistory, getKindleSettings, sendKindle, type KindleDelivery } from "../../lib/api";
import { useLibraryFiles } from "../../lib/queries";

const labels = { accepted: "Accepted by email server", failed: "Failed", unknown: "Outcome unknown", sending: "Sending" };
export function KindleSendButton({ wantedId, fileId, disabled, label = "Send to Kindle" }: { wantedId?: string; fileId?: string; disabled?: boolean; label?: string }) {
  const client = useQueryClient();
  const storageKey = `kindle-pending:${wantedId ?? "test"}:${fileId ?? "test"}`;
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<KindleDelivery>();
  async function send() {
    if (pending) return;
    setPending(true); setError(""); setResult(undefined);
    try {
      // Preserve the same request ID after lost HTTP responses and page reloads.
      let requestId = sessionStorage.getItem(storageKey);
      if (!requestId) { requestId = Array.from(crypto.getRandomValues(new Uint8Array(16)), b => b.toString(16).padStart(2, "0")).join(""); sessionStorage.setItem(storageKey, requestId); }
      const delivery = await sendKindle(requestId, wantedId, fileId);
      setResult(delivery);
      if (delivery.state !== "sending") sessionStorage.removeItem(storageKey);
    } catch (e) { setError(`${String(e)}. Check history, then use this button to recover the same attempt.`); }
    finally { setPending(false); await client.invalidateQueries({ queryKey: ["kindle-history", wantedId ?? ""] }); }
  }
  return <div>
    <Button variant="primary" disabled={disabled || pending} onClick={() => void send()}>{pending ? "Sending…" : result && result.state !== "sending" ? "Send again" : label}</Button>
    {result ? <InlineNotice tone={result.state === "failed" ? "danger" : "info"}>{result.message || labels[result.state]}</InlineNotice> : null}
    {error ? <InlineNotice tone="danger">{error}</InlineNotice> : null}
  </div>;
}
export function KindleHistory({ wantedId = "" }: { wantedId?: string }) {
  const history = useQuery({ queryKey: ["kindle-history", wantedId], queryFn: () => getKindleHistory(wantedId), refetchInterval: query => query.state.data?.some(d => d.state === "sending") ? 3000 : false });
  return <div className="kindle-history">
    <h3>Recent deliveries</h3>
    {history.isLoading ? <p>Loading deliveries…</p> : null}
    {history.error ? <InlineNotice tone="danger">{String(history.error)}</InlineNotice> : null}
    {!history.isLoading && !history.error && !history.data?.length ? <p>No deliveries yet.</p> : null}
    {history.data?.map(d => <div key={d.id} className="kindle-history-row"><strong>{labels[d.state]}</strong><span>{new Date(d.createdAt).toLocaleString()} · {d.recipient}</span><span>{d.title}</span><span>{d.message}</span></div>)}
    <p>Acceptance confirms submission to the email server, not arrival on your Kindle. Failed and uncertain attempts are never retried automatically. Check your Kindle before sending again.</p>
    <Button size="sm" onClick={() => void history.refetch()}>Refresh history</Button>
  </div>;
}
export function KindleDeliveryPanel({ wantedId }: { wantedId: string }) {
  const settings = useQuery({ queryKey: ["kindle-settings"], queryFn: getKindleSettings });
  const files = useLibraryFiles("ebook", wantedId);
  const [selected, setSelected] = useState("");
  const eligible = (files.data ?? []).filter(f => f.mediaFormat === "ebook" && /\.(epub|pdf)$/i.test(f.path) && f.presenceState === "present" && ["available", "imported"].includes(f.importStatus));
  const fileId = eligible.some(f => f.id === selected) ? selected : eligible[0]?.id;
  return <Card title="Read on Kindle">
    {settings.error || files.error ? <InlineNotice tone="danger">{String(settings.error || files.error)}</InlineNotice> : null}
    {!settings.data?.enabled ? <p><Link to="/settings/kindle">Configure Kindle delivery</Link> to send books wirelessly.</p> : <>
      <p>Send to {settings.data.recipient}. <Link to="/settings/kindle">Edit settings</Link></p>
      <label className="kindle-file-choice">Book file<select value={fileId ?? ""} onChange={e => setSelected(e.target.value)} disabled={!eligible.length}><option value="" disabled>Select a file</option>{eligible.map(f => <option key={f.id} value={f.id}>{f.path.split("/").pop()}</option>)}</select></label>
      {!eligible.length && !files.isLoading ? <p>No present native EPUB or PDF is available. Convert other formats or refresh the library scan first.</p> : null}
      {fileId ? <KindleSendButton key={fileId} wantedId={wantedId} fileId={fileId} disabled={!!settings.error || !!files.error} /> : null}
    </>}
    <KindleHistory wantedId={wantedId} />
  </Card>;
}
