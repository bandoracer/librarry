import React, { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Badge, Button, Card, Field, InlineNotice, Modal } from "../../components/ui";
import { fetchAcquisitionRecovery, resolveAcquisition, type AcquisitionIntent } from "../../lib/api";
import { keys } from "../../lib/queries";

const recoveryKey = ["acquisition-recovery"];
export default function AcquisitionRecovery() {
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: recoveryKey, queryFn: fetchAcquisitionRecovery, refetchInterval: 15_000 });
  const [decision, setDecision] = useState<{ intent: AcquisitionIntent; action: "attach" | "release" } | null>(null);
  const [downloadId, setDownloadId] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const mutation = useMutation({
    mutationFn: resolveAcquisition,
    onSuccess: () => { setDecision(null); },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: recoveryKey });
      void queryClient.invalidateQueries({ queryKey: keys.downloads() });
      void queryClient.invalidateQueries({ queryKey: keys.wanted });
    }
  });
  function openDecision(intent: AcquisitionIntent, action: "attach" | "release") {
    mutation.reset(); setDownloadId(""); setConfirmed(false); setDecision({ intent, action });
  }
  if (!query.isError && !query.data?.intents.length) return null;
  return <Card title="Acquisitions needing review" subtitle="The client may have accepted these downloads. Resolve them before another request is sent.">
    {query.isError ? <InlineNotice tone="danger">{query.error.message} <Button size="sm" onClick={() => void query.refetch()}>Try again</Button></InlineNotice> : null}
    {!decision && mutation.isError ? <InlineNotice tone="danger">{mutation.error.message}</InlineNotice> : null}
    {query.data?.intents.map(intent => {
      const leased = !!intent.leaseExpiresAt && Date.parse(intent.leaseExpiresAt) > Date.now();
      const backoff = !!intent.nextCheckAt && Date.parse(intent.nextCheckAt) > Date.now();
      return <div key={intent.id} className="acquisition-recovery-item">
        <div><strong>{intent.title || "Download request"}</strong> <Badge tone="warn">{leased ? "In progress" : "Uncertain"}</Badge></div>
        <p className="field-hint">{intent.client}{intent.format ? ` · ${intent.format}` : ""}</p>
        {intent.wantedId ? <Link to={`/library/book/${encodeURIComponent(intent.wantedId)}`}>View book</Link> : null}
        <p>{intent.lastError || "A submission was interrupted before its result was saved."}</p>
        {backoff ? <p className="field-hint">Next client check after {new Date(intent.nextCheckAt!).toLocaleTimeString()}.</p> : null}
        <div className="acquisition-recovery-actions">
          <Button size="sm" disabled={leased || backoff || mutation.isPending} busy={mutation.isPending && mutation.variables.id === intent.id} onClick={() => mutation.mutate({ id: intent.id, action: "check" })}>Check client</Button>
          <Button size="sm" disabled={leased || backoff || mutation.isPending} onClick={() => openDecision(intent, "attach")}>Attach existing download</Button>
          <Button size="sm" variant="ghost" disabled={leased || mutation.isPending} onClick={() => openDecision(intent, "release")}>Allow new attempt…</Button>
        </div>
      </div>;
    })}
    {!!query.data && query.data.intents.length >= query.data.limit ? <p className="field-hint">Showing the oldest {query.data.limit} unresolved acquisitions.</p> : null}
    <Modal title={decision?.action === "attach" ? "Attach an existing download" : "Allow a new acquisition attempt"} open={!!decision} onClose={() => { if (!mutation.isPending) setDecision(null); }} footer={<>
      <Button variant="ghost" disabled={mutation.isPending} onClick={() => setDecision(null)}>Cancel</Button>
      <Button variant={decision?.action === "release" ? "danger" : "primary"} disabled={!confirmed || (decision?.action === "attach" && !downloadId.trim())} busy={mutation.isPending} onClick={() => { if (decision) mutation.mutate({ id: decision.intent.id, action: decision.action, downloadId: downloadId.trim(), confirmed: true }); }}>{decision?.action === "attach" ? "Attach download" : "Allow new attempt"}</Button>
    </>}>
      <p>{decision?.intent.title || "Download request"} · {decision?.intent.client}</p>
      {decision?.action === "attach" ? <>
        <p>Inspect the client and enter the exact ID of the download for this book. This will restore its Librarry association.</p>
        <Field label="Client download ID"><input aria-label="Client download ID" value={downloadId} onChange={event => setDownloadId(event.target.value)} /></Field>
      </> : <p>Check the original client’s queue and history first. A previous request may still finish, so allowing another attempt can create a duplicate. This releases the saved reservation; it does not submit a download immediately.</p>}
      <label className="acquisition-recovery-confirm"><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} />{decision?.action === "attach" ? "I verified this download belongs to this book." : "I inspected the client and understand another attempt may create a duplicate."}</label>
      {mutation.isError ? <InlineNotice tone="danger">{mutation.error.message}</InlineNotice> : null}
    </Modal>
  </Card>;
}
