import React, { useState } from "react";
import { Download } from "lucide-react";
import { Button, Field, FormGrid, Modal } from "../../components/ui";
import { useToast } from "../../components/toast";
import { grabManualDownload } from "../../lib/api";

/**
 * Replaces the legacy inline manual-grab panel. Adds a magnet/NZB URL or an
 * uploaded .torrent file as a paused download (legacy always added paused —
 * there was no start-paused toggle).
 */
export default function AddDownloadModal(props: { open: boolean; onClose: () => void; onAdded: () => Promise<unknown> | void }) {
  const toast = useToast();
  const [url, setURL] = useState("");
  const [title, setTitle] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [fileInputKey, setFileInputKey] = useState(0);
  const [format, setFormat] = useState("ebook");
  const [client, setClient] = useState("");
  const [busy, setBusy] = useState(false);

  const canSubmit = Boolean(url.trim() || file) && !busy;

  async function submit() {
    const releaseUrl = url.trim();
    if (!releaseUrl && !file) return;
    setBusy(true);
    try {
      const status = await grabManualDownload({
        releaseUrl: releaseUrl || undefined,
        file: file ?? undefined,
        title: title.trim() || undefined,
        format,
        client: client || undefined,
        paused: true
      });
      toast.success(`Added ${status.name || status.id} (${status.state}) — paused`);
      setURL("");
      setFile(null);
      setFileInputKey((current) => current + 1);
      setTitle("");
      await props.onAdded();
      props.onClose();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Manual grab failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title="Add Download"
      open={props.open}
      onClose={props.onClose}
      footer={
        <>
          <Button variant="ghost" onClick={props.onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant="primary" icon={Download} busy={busy} disabled={!canSubmit} onClick={() => void submit()}>
            Add paused
          </Button>
        </>
      }
    >
      <FormGrid columns={1}>
        <Field label="Magnet, torrent, or NZB URL">
          <input
            value={url}
            onChange={(event) => setURL(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") void submit();
            }}
            placeholder="magnet:?xt=... or https://indexer.example/download/..."
          />
        </Field>
        <Field label="Display title" hint="Optional queue name">
          <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Optional queue name" />
        </Field>
        <Field label="Torrent file" hint="Uploading a .torrent file requires a torrent client">
          <input
            key={fileInputKey}
            accept=".torrent,application/x-bittorrent"
            onChange={(event) => {
              const nextFile = event.currentTarget.files?.[0] ?? null;
              setFile(nextFile);
              if (nextFile && client === "SABnzbd") {
                setClient("");
              }
            }}
            type="file"
          />
        </Field>
      </FormGrid>
      <FormGrid columns={2}>
        <Field label="Format">
          <select value={format} onChange={(event) => setFormat(event.target.value)} aria-label="Download format">
            <option value="ebook">Ebook</option>
            <option value="audiobook">Audiobook</option>
          </select>
        </Field>
        <Field label="Client">
          <select value={client} onChange={(event) => setClient(event.target.value)} aria-label="Download client">
            <option value="">Auto client</option>
            <option value="qBittorrent">qBittorrent</option>
            <option value="Transmission">Transmission</option>
            <option value="SABnzbd" disabled={Boolean(file)}>
              SABnzbd
            </option>
          </select>
        </Field>
      </FormGrid>
    </Modal>
  );
}
