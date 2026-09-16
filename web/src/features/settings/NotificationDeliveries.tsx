import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Badge, Button, Card, InlineNotice, LoadingRow, Modal } from "../../components/ui";
import { fetchNotificationDeliveries, resolveNotificationDelivery, type NotificationDelivery } from "../../lib/api";
import { formatRelativeTime } from "../../lib/format";

type Decision = { delivery: NotificationDelivery; action: "retry" | "accepted" | "cancel" };

export default function NotificationDeliveries() {
  const [offset, setOffset] = useState(0);
  const [decision, setDecision] = useState<Decision | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const query = useQuery({ queryKey: ["notification-deliveries", offset], queryFn: () => fetchNotificationDeliveries(offset), refetchInterval: 10_000 });
  function choose(delivery: NotificationDelivery, action: Decision["action"]) {
    setConfirmed(false); setError(""); setDecision({ delivery, action });
  }
  async function resolve() {
    if (!decision || !confirmed) return;
    setBusy(true); setError("");
    try {
      await resolveNotificationDelivery(decision.delivery, decision.action);
      setDecision(null); await query.refetch();
    } catch (e) { setError(e instanceof Error ? e.message : "Could not save the delivery decision"); }
    finally { setBusy(false); }
  }
  return <>
    <Card title="Notification delivery" subtitle="Native connections only. Accepted means the receiver accepted the request; it does not prove someone read it.">
      {query.isPending ? <LoadingRow label="Loading notification delivery…" /> : null}
      {query.isError ? <InlineNotice tone="danger">{query.error.message} <Button size="sm" onClick={() => void query.refetch()}>Try again</Button></InlineNotice> : null}
      {query.data?.items.length === 0 ? <p>No recorded deliveries on this page. New events use the connections enabled when they occur.</p> : null}
      {query.data?.items.map(delivery => <article key={delivery.id} className="notification-delivery-entry">
        <p><strong>{delivery.event.title}</strong></p>
        <p><Badge tone={delivery.state === "accepted" ? "success" : ["pending", "sending", "retry"].includes(delivery.state) ? "info" : "warn"}>{delivery.state}</Badge> · {delivery.targetName} · {formatRelativeTime(delivery.createdAt)} · {delivery.attempts} attempts</p>
        {delivery.message ? <p>{delivery.message}</p> : null}
        {delivery.state === "retry" ? <p>Next attempt: {new Date(delivery.nextAttemptAt).toLocaleString()}</p> : null}
        {delivery.currentTargetRevision !== delivery.targetRevision ? <p>Connection settings have changed or the connection was deleted.</p> : null}
        <div className="cell-actions">
          {["uncertain", "failed", "cancelled"].includes(delivery.state) && delivery.targetAvailable ? <Button size="sm" aria-label={`Review retry for ${delivery.event.title} to ${delivery.targetName}`} onClick={() => choose(delivery, "retry")}>Review retry</Button> : null}
          {delivery.state === "uncertain" ? <Button size="sm" onClick={() => choose(delivery, "accepted")}>Confirm acceptance</Button> : null}
          {["pending", "retry", "uncertain", "failed"].includes(delivery.state) ? <Button size="sm" onClick={() => choose(delivery, "cancel")}>Cancel delivery</Button> : null}
        </div>
      </article>)}
      {query.data ? <div className="cell-actions">
        <Button size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - 25))}>Previous deliveries</Button>
        <span>{query.data.total} recorded deliveries</span>
        <Button size="sm" disabled={offset + query.data.items.length >= query.data.total} onClick={() => setOffset(offset + 25)}>Next deliveries</Button>
      </div> : null}
    </Card>
    <Modal open={decision !== null} title={decision?.action === "retry" ? "Review notification retry" : decision?.action === "accepted" ? "Confirm receiver acceptance" : "Cancel notification delivery"} onClose={() => { if (!busy) setDecision(null); }} footer={<>
      <Button disabled={busy} onClick={() => setDecision(null)}>Back</Button>
      <Button variant="primary" disabled={!confirmed || busy} busy={busy} onClick={() => void resolve()}>Save decision</Button>
    </>}>
      <p>{decision?.delivery.event.title} → {decision?.delivery.targetName}</p>
      {decision?.action === "retry" ? <>
        <InlineNotice tone="warn">A previous request may already have been accepted. Inspect the receiver first; retrying can create a duplicate message.</InlineNotice>
        <p>This queues another attempt using the connection’s current settings.</p>
      </> : decision?.action === "accepted" ? <p>Only mark this accepted after verifying the message at the receiver. This records your confirmation without sending another request.</p> : <p>This stops future attempts. It cannot retract a request the receiver already accepted.</p>}
      <label><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} /> {decision?.action === "retry" ? "I reviewed the receiver and confirm a retry with current settings." : decision?.action === "accepted" ? "I verified that the receiver accepted this message." : "I confirm cancellation of this delivery."}</label>
      {error ? <InlineNotice tone="danger">{error}</InlineNotice> : null}
    </Modal>
  </>;
}
