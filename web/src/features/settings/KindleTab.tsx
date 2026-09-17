import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, InlineNotice, LoadingRow } from "../../components/ui";
import { getKindleSettings, saveKindleSettings, type KindleSettings } from "../../lib/api";
import { SecretInput } from "./controls";
import { KindleSendButton, KindleHistory } from "../library/KindleDelivery";

export function KindleTab() {
  const query = useQuery({ queryKey: ["kindle-settings"], queryFn: getKindleSettings });
  if (query.isLoading) return <LoadingRow label="Loading Kindle settings…" />;
  if (query.error) return <InlineNotice tone="danger">{String(query.error)}</InlineNotice>;
  if (!query.data) return null;
  return <KindleForm initial={query.data} />;
}
function KindleForm({ initial }: { initial: KindleSettings }) {
  const client = useQueryClient();
  const [form, setForm] = useState(initial);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [message, setMessage] = useState("");
  const [failed, setFailed] = useState(false);
  const [clearPassword, setClearPassword] = useState(false);
  function change<K extends keyof KindleSettings>(key: K, value: KindleSettings[K]) { setForm(f => ({ ...f, [key]: value })); setDirty(true); setMessage(""); }
  async function save(event: React.FormEvent) {
    event.preventDefault(); setSaving(true); setMessage("");
    try {
      const result = await saveKindleSettings({ ...form, clearPassword });
      setForm(result); setClearPassword(false); setDirty(false); setFailed(false);
      client.setQueryData(["kindle-settings"], result); setMessage("Kindle settings saved.");
    } catch (error) { setFailed(true); setMessage(String(error)); }
    finally { setSaving(false); }
  }
  return <>
    <Card title="Send to Kindle" subtitle="Send a selected EPUB or PDF to your Kindle over email.">
      <form onSubmit={save} className="kindle-form">
        <label><input type="checkbox" checked={form.enabled} onChange={e => change("enabled", e.target.checked)} /> Enable Kindle delivery</label>
        <label>Kindle email<input type="email" value={form.recipient} placeholder="you@kindle.com" onChange={e => change("recipient", e.target.value)} /></label>
        <label>Sender email<input type="email" value={form.from} onChange={e => change("from", e.target.value)} /></label>
        <label>Sender name<input value={form.fromName} maxLength={100} onChange={e => change("fromName", e.target.value)} /></label>
        <Button type="button" onClick={() => { setForm(f => ({ ...f, host: "smtp.resend.com", port: 465, tlsMode: "implicit", username: "resend" })); setDirty(true); }}>Use Resend defaults</Button>
        <div className="kindle-form-grid">
          <label>SMTP host<input value={form.host} onChange={e => change("host", e.target.value)} /></label>
          <label>Port<input type="number" min={1} max={65535} value={form.port} onChange={e => change("port", Number(e.target.value))} /></label>
          <label>Encryption<select value={form.tlsMode} onChange={e => change("tlsMode", e.target.value as KindleSettings["tlsMode"])}><option value="implicit">TLS</option><option value="starttls">STARTTLS</option></select></label>
          <label>Username<input autoComplete="off" value={form.username} onChange={e => change("username", e.target.value)} /></label>
        </div>
        <label>Password / API key<SecretInput value={form.password ?? ""} onChange={v => { change("password", v); setClearPassword(false); }} placeholder={form.passwordConfigured ? "Saved — leave blank to keep" : "SMTP password or Resend API key"} /></label>
        <label><input type="checkbox" checked={clearPassword} onChange={e => { setClearPassword(e.target.checked); setDirty(true); if (e.target.checked) change("password", ""); }} /> Clear saved password</label>
        <p>Add <strong>{form.from || "your sender email"}</strong> to Amazon’s Approved Personal Document E-mail List. Files must be EPUB or PDF, no larger than 25 MiB.</p>
        {message ? <InlineNotice tone={failed ? "danger" : "info"}>{message}</InlineNotice> : null}
        <Button type="submit" variant="primary" disabled={saving}>{saving ? "Saving…" : "Save settings"}</Button>
      </form>
    </Card>
    <Card title="Test delivery" subtitle="Send a small text document to the saved Kindle address.">
      {dirty ? <p>Save your changes before testing.</p> : <KindleSendButton disabled={!form.enabled || saving} label="Send test document" />}
      <KindleHistory />
    </Card>
  </>;
}
