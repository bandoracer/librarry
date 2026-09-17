import React from "react";
import { Download } from "lucide-react";
import { Button, Card, InlineNotice } from "../../components/ui";
import { fetchSupportReport } from "../../lib/api";

export default function SupportDiagnostics() {
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string>();
  const [saved, setSaved] = React.useState(false);

  async function download() {
    setBusy(true);
    setError(undefined);
    setSaved(false);
    try {
      const report = await fetchSupportReport();
      const blob = new Blob([JSON.stringify(report, null, 2) + "\n"], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = "librarry-support.json";
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
      // Allow the browser to begin consuming the blob before releasing it.
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setSaved(true);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Support report could not be downloaded.");
    } finally {
      setBusy(false);
    }
  }

  return <Card title="Support diagnostics" subtitle="Build, selected settings, roots and worker status.">
    <p>Credentials, private paths, book metadata and free-text logs are omitted. Provider status uses recorded observations; downloading does not check external services.</p>
    <Button icon={Download} busy={busy} onClick={() => void download()}>Download support report</Button>
    {error ? <InlineNotice tone="danger">{error}</InlineNotice> : null}
    {saved ? <InlineNotice tone="info">Report prepared. Unknown or unavailable evidence is marked in the file. Review it before sharing.</InlineNotice> : null}
  </Card>;
}
