import React, { useEffect, useState } from "react";
import {
  ChevronsDown,
  ChevronsUp,
  FileCheck2,
  Pause,
  Pencil,
  Play,
  RefreshCw,
  SlidersHorizontal,
  Tags,
  Trash2
} from "lucide-react";
import { Badge, Button, Field, LoadingRow, ProgressBar } from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  fetchDownloadDetails,
  runDownloadAction,
  runDownloadFileAction,
  runDownloadTrackerAction,
  type DownloadAction,
  type DownloadDetails,
  type DownloadFileAction,
  type DownloadTrackerAction
} from "../../lib/api";
import { formatBytes, formatPercent, formatSpeed } from "../../lib/format";
import {
  bandwidthInputToBytes,
  downloadSupportsFileActions,
  downloadSupportsTrackerActions,
  filePriorityLabel,
  formatDuration,
  limitBytesToKiBInput,
  sameFormText,
  splitTagInput,
  supportsDownloadAction
} from "./lib";

/**
 * Inline expansion of a queue row: properties, per-download client actions,
 * management forms (rename / tags / category / location / bandwidth limits),
 * trackers, peers, and per-file priority actions. Ported from the legacy
 * download-detail panel.
 */
export default function DownloadDetailsPanel(props: {
  details: DownloadDetails | null;
  loading: boolean;
  busyID: string;
  setBusyID: (id: string) => void;
  onDetails: (details: DownloadDetails) => void;
  onReload: () => void;
  onQueueRefresh: () => Promise<unknown>;
  onAction: (action: DownloadAction, options?: { forceStart?: boolean }) => Promise<void>;
  onRequestDelete: () => void;
}) {
  const toast = useToast();
  const { details, busyID } = props;
  const status = details?.status;

  const [nameInput, setNameInput] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [categoryInput, setCategoryInput] = useState("");
  const [savePathInput, setSavePathInput] = useState("");
  const [downloadLimitKiB, setDownloadLimitKiB] = useState("");
  const [uploadLimitKiB, setUploadLimitKiB] = useState("");
  const [trackerURL, setTrackerURL] = useState("");

  useEffect(() => {
    if (!details) return;
    setNameInput(details.status.name || "");
    setTagsInput((details.status.tags ?? []).join(", "));
    setCategoryInput(details.status.category || "");
    setSavePathInput(details.status.savePath || details.properties?.savePath || "");
    setDownloadLimitKiB(limitBytesToKiBInput(details.properties?.downloadLimit));
    setUploadLimitKiB(limitBytesToKiBInput(details.properties?.uploadLimit));
  }, [details]);

  if (!details || !status) {
    return <LoadingRow label="Loading download details…" />;
  }

  const anyBusy = Boolean(busyID);
  const nameHasChanges = !sameFormText(nameInput, status.name);
  const currentTagSet = new Set((status.tags ?? []).map((tag) => tag.trim().toLowerCase()));
  const inputTags = splitTagInput(tagsInput);
  const tagsCanAdd = inputTags.some((tag) => !currentTagSet.has(tag.toLowerCase()));
  const tagsCanRemove = inputTags.some((tag) => currentTagSet.has(tag.toLowerCase()));
  const categoryHasChanges = !sameFormText(categoryInput, status.category);
  const savePathHasChanges = !sameFormText(savePathInput, status.savePath || details.properties?.savePath);
  const downloadLimitBytes = bandwidthInputToBytes(downloadLimitKiB);
  const uploadLimitBytes = bandwidthInputToBytes(uploadLimitKiB);
  const downloadLimitHasChanges = downloadLimitBytes >= 0 && downloadLimitBytes !== (details.properties?.downloadLimit ?? 0);
  const uploadLimitHasChanges = uploadLimitBytes >= 0 && uploadLimitBytes !== (details.properties?.uploadLimit ?? 0);
  const trackerURLIsNew = Boolean(
    trackerURL.trim() && !(details.trackers ?? []).some((tracker) => sameFormText(tracker.url, trackerURL))
  );
  const trackerURLHasChanges = (url: string) => Boolean(trackerURL.trim() && !sameFormText(trackerURL, url));

  async function refreshDetailsAfter(action: string, run: () => Promise<DownloadDetails | null>) {
    if (!status) return;
    props.setBusyID(`${status.id}:${action}`);
    try {
      const refreshed = await run();
      if (refreshed) {
        props.onDetails(refreshed);
      }
      await props.onQueueRefresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download action failed");
    } finally {
      props.setBusyID("");
    }
  }

  async function runManagement(action: DownloadAction, options: { name?: string; tags?: string[]; category?: string; savePath?: string; downloadLimit?: number; uploadLimit?: number }, successMessage: string) {
    if (!status) return;
    await refreshDetailsAfter(action, async () => {
      await runDownloadAction(action, [status.id], { client: status.client, ...options });
      const refreshed = await fetchDownloadDetails(status.id, status.client);
      toast.success(successMessage);
      return refreshed;
    });
  }

  async function runTracker(action: DownloadTrackerAction, currentURL = "") {
    if (!status) return;
    const nextURL = trackerURL.trim();
    if (action === "add" && !nextURL) return;
    if (action === "edit" && (!currentURL || !nextURL)) return;
    if (action === "remove" && !currentURL) return;
    props.setBusyID(`${status.id}:tracker:${action}:${currentURL || nextURL}`);
    try {
      const result = await runDownloadTrackerAction(status.id, action, {
        client: status.client,
        urls: action === "add" ? [nextURL] : undefined,
        url: action === "remove" ? currentURL : undefined,
        originalUrl: action === "edit" ? currentURL : undefined,
        newUrl: action === "edit" ? nextURL : undefined
      });
      if (result.download) {
        props.onDetails(result.download);
      } else {
        props.onDetails(await fetchDownloadDetails(status.id, status.client));
      }
      if (action !== "remove") {
        setTrackerURL("");
      }
      toast.success(action === "add" ? "Tracker added" : action === "edit" ? "Tracker replaced" : "Tracker removed");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download tracker action failed");
    } finally {
      props.setBusyID("");
    }
  }

  async function runFile(action: DownloadFileAction, ids: number[]) {
    if (!status || ids.length === 0) return;
    props.setBusyID(`${status.id}:file:${action}`);
    try {
      const result = await runDownloadFileAction(status.id, action, ids, { client: status.client });
      if (result.download) {
        props.onDetails(result.download);
      } else {
        props.onDetails(await fetchDownloadDetails(status.id, status.client));
      }
      await props.onQueueRefresh();
      toast.success(`File priority set to ${action}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download file action failed");
    } finally {
      props.setBusyID("");
    }
  }

  const trackerBusy = busyID.startsWith(`${status.id}:tracker:`);
  const fileBusy = busyID.startsWith(`${status.id}:file:`);

  const metrics: [string, string][] = [
    ["Progress", `${Math.round((status.progress ?? 0) * 100)}%`],
    ["ETA", formatDuration(details.properties?.etaSeconds ?? status.etaSeconds)],
    ["Speed", `${formatSpeed(details.properties?.downloadSpeed ?? status.downloadRate ?? 0)} down`],
    ["Peers", `${status.seeders ?? 0} seeders · ${status.peers ?? 0} peers`],
    ["Pieces", `${details.properties?.piecesHave ?? 0}/${details.properties?.piecesTotal ?? 0}`],
    ["Connections", `${details.properties?.connections ?? 0}/${details.properties?.connectionsLimit ?? 0}`]
  ];

  return (
    <div className="activity-details">
      <div className="activity-details-head">
        <div>
          <strong>{status.name || status.id}</strong>
          <div className="cell-muted">
            {status.client || "qBittorrent"} · {status.category || "uncategorized"} ·{" "}
            {formatBytes(status.sizeBytes ?? details.properties?.totalSizeBytes ?? 0)}
          </div>
        </div>
        <Button size="sm" variant="ghost" icon={RefreshCw} busy={props.loading} onClick={props.onReload}>
          Refresh details
        </Button>
      </div>

      <div className="activity-detail-metrics">
        {metrics.map(([label, value]) => (
          <div className="activity-detail-metric" key={label}>
            <span className="cell-muted">{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>

      <div className="activity-detail-actions">
        <Button size="sm" icon={Play} disabled={anyBusy || !supportsDownloadAction(status, "start")} onClick={() => void props.onAction("start")}>
          Start
        </Button>
        <Button size="sm" icon={Pause} disabled={anyBusy || !supportsDownloadAction(status, "stop")} onClick={() => void props.onAction("stop")}>
          Stop
        </Button>
        <Button size="sm" icon={RefreshCw} disabled={anyBusy || !supportsDownloadAction(status, "recheck")} onClick={() => void props.onAction("recheck")}>
          Recheck
        </Button>
        <Button
          size="sm"
          icon={ChevronsUp}
          disabled={anyBusy || !supportsDownloadAction(status, "increasePriority")}
          onClick={() => void props.onAction("increasePriority")}
        >
          Raise
        </Button>
        <Button
          size="sm"
          icon={ChevronsDown}
          disabled={anyBusy || !supportsDownloadAction(status, "decreasePriority")}
          onClick={() => void props.onAction("decreasePriority")}
        >
          Lower
        </Button>
        <Button
          size="sm"
          icon={Play}
          disabled={anyBusy || !supportsDownloadAction(status, "forceStart")}
          onClick={() => void props.onAction("forceStart", { forceStart: true })}
        >
          Force
        </Button>
        <Button
          size="sm"
          icon={SlidersHorizontal}
          disabled={anyBusy || !supportsDownloadAction(status, "toggleSequential")}
          onClick={() => void props.onAction("toggleSequential")}
        >
          Sequential
        </Button>
        <Button
          size="sm"
          icon={FileCheck2}
          disabled={anyBusy || !supportsDownloadAction(status, "toggleFirstLastPiece")}
          onClick={() => void props.onAction("toggleFirstLastPiece")}
        >
          First/last
        </Button>
        <Button size="sm" variant="danger" icon={Trash2} disabled={anyBusy} onClick={props.onRequestDelete}>
          Remove…
        </Button>
      </div>

      <div className="activity-detail-form" aria-label="Download management">
        <Field label="Name">
          <input value={nameInput} onChange={(event) => setNameInput(event.target.value)} placeholder="Torrent display name" />
        </Field>
        <Button
          size="sm"
          icon={Pencil}
          disabled={anyBusy || !nameInput.trim() || !nameHasChanges || !supportsDownloadAction(status, "rename")}
          onClick={() => void runManagement("rename", { name: nameInput.trim() }, "Download renamed")}
        >
          Rename
        </Button>
        <Field label="Tags" hint="Comma separated">
          <input value={tagsInput} onChange={(event) => setTagsInput(event.target.value)} placeholder="librarry, audiobook" />
        </Field>
        <Button
          size="sm"
          icon={Tags}
          disabled={anyBusy || inputTags.length === 0 || !tagsCanAdd || !supportsDownloadAction(status, "addTags")}
          onClick={() => void runManagement("addTags", { tags: inputTags }, "Tags added")}
        >
          Add tags
        </Button>
        <Button
          size="sm"
          variant="danger"
          icon={Tags}
          disabled={anyBusy || inputTags.length === 0 || !tagsCanRemove || !supportsDownloadAction(status, "removeTags")}
          onClick={() => void runManagement("removeTags", { tags: inputTags }, "Tags removed")}
        >
          Remove tags
        </Button>
      </div>

      <div className="activity-detail-form" aria-label="Download placement">
        <Field label="Category">
          <input value={categoryInput} onChange={(event) => setCategoryInput(event.target.value)} placeholder="books-ebook" />
        </Field>
        <Button
          size="sm"
          disabled={anyBusy || !categoryInput.trim() || !categoryHasChanges || !supportsDownloadAction(status, "setCategory")}
          onClick={() => void runManagement("setCategory", { category: categoryInput.trim() }, "Category updated")}
        >
          Set category
        </Button>
        <Field label="Save path">
          <input value={savePathInput} onChange={(event) => setSavePathInput(event.target.value)} placeholder="/data/torrents/books" />
        </Field>
        <Button
          size="sm"
          disabled={anyBusy || !savePathInput.trim() || !savePathHasChanges || !supportsDownloadAction(status, "setLocation")}
          onClick={() => void runManagement("setLocation", { savePath: savePathInput.trim() }, "Location updated")}
        >
          Move
        </Button>
      </div>

      <div className="activity-detail-form" aria-label="Bandwidth limits">
        <Field label="Download KiB/s">
          <input
            className="activity-detail-form-narrow"
            inputMode="numeric"
            min="0"
            type="number"
            value={downloadLimitKiB}
            onChange={(event) => setDownloadLimitKiB(event.target.value)}
            placeholder="unlimited"
          />
        </Field>
        <Button
          size="sm"
          disabled={anyBusy || downloadLimitBytes < 0 || !downloadLimitHasChanges || !supportsDownloadAction(status, "setDownloadLimit")}
          onClick={() => void runManagement("setDownloadLimit", { downloadLimit: downloadLimitBytes }, "Download limit updated")}
        >
          Set down
        </Button>
        <Field label="Upload KiB/s">
          <input
            className="activity-detail-form-narrow"
            inputMode="numeric"
            min="0"
            type="number"
            value={uploadLimitKiB}
            onChange={(event) => setUploadLimitKiB(event.target.value)}
            placeholder="unlimited"
          />
        </Field>
        <Button
          size="sm"
          disabled={anyBusy || uploadLimitBytes < 0 || !uploadLimitHasChanges || !supportsDownloadAction(status, "setUploadLimit")}
          onClick={() => void runManagement("setUploadLimit", { uploadLimit: uploadLimitBytes }, "Upload limit updated")}
        >
          Set up
        </Button>
      </div>

      <div className="activity-detail-form" aria-label="Trackers">
        <Field label="Tracker URL">
          <input value={trackerURL} onChange={(event) => setTrackerURL(event.target.value)} placeholder="https://tracker.example/announce" />
        </Field>
        <Button
          size="sm"
          disabled={!downloadSupportsTrackerActions(status) || !trackerURL.trim() || !trackerURLIsNew || anyBusy}
          onClick={() => void runTracker("add")}
        >
          Add tracker
        </Button>
      </div>
      {details.trackers?.length ? (
        <div className="activity-detail-list" aria-label="Tracker list">
          {details.trackers.map((tracker) => (
            <div className="activity-detail-row" key={`${tracker.url}-${tracker.tier ?? 0}`}>
              <div className="activity-detail-row-main">
                <strong>{tracker.url || "tracker"}</strong>
                <span className="cell-muted">{tracker.message || tracker.status}</span>
              </div>
              <Badge tone={tracker.status === "working" ? "success" : tracker.status === "not_working" ? "danger" : "neutral"}>
                {tracker.status}
              </Badge>
              <span className="cell-muted">
                {tracker.seeds ?? 0} seeds · {tracker.leeches ?? 0} leeches
              </span>
              <div className="activity-detail-row-actions">
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={!downloadSupportsTrackerActions(status) || trackerBusy || !trackerURLHasChanges(tracker.url)}
                  onClick={() => void runTracker("edit", tracker.url)}
                >
                  Replace
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  disabled={!downloadSupportsTrackerActions(status) || trackerBusy}
                  onClick={() => void runTracker("remove", tracker.url)}
                >
                  Remove
                </Button>
              </div>
            </div>
          ))}
        </div>
      ) : null}

      {details.peers?.length ? (
        <div className="activity-detail-list" aria-label="Peers">
          {details.peers.slice(0, 20).map((peer) => (
            <div className="activity-detail-row" key={peer.id || `${peer.ip}:${peer.port ?? 0}`}>
              <div className="activity-detail-row-main">
                <strong>
                  {peer.ip}
                  {peer.port ? `:${peer.port}` : ""}
                </strong>
                <span className="cell-muted">{peer.client || peer.connection || peer.country || "peer"}</span>
              </div>
              <Badge tone="neutral">{Math.round((peer.progress ?? 0) * 100)}%</Badge>
              <span className="cell-muted">
                {formatSpeed(peer.downloadRate ?? 0)} down · {formatSpeed(peer.uploadRate ?? 0)} up
              </span>
            </div>
          ))}
        </div>
      ) : null}

      {details.files?.length ? (
        <div className="activity-detail-list" aria-label="Files">
          {details.files.map((file) => (
            <div className="activity-detail-row" key={`${status.id}:${file.id}`}>
              <div className="activity-detail-row-main">
                <strong>{file.name}</strong>
                <span className="cell-muted">
                  {formatBytes(file.sizeBytes ?? 0)} · {formatPercent(file.progress ?? 0)} · {filePriorityLabel(file.priority)}
                  {file.availability ? ` · ${file.availability.toFixed(1)} available` : ""}
                </span>
                <div className="activity-file-progress">
                  <ProgressBar value={file.progress ?? 0} tone={(file.progress ?? 0) >= 1 ? "success" : "neutral"} />
                </div>
              </div>
              <div className="activity-detail-row-actions">
                <Button size="sm" variant="ghost" disabled={fileBusy || !downloadSupportsFileActions(status)} onClick={() => void runFile("skip", [file.id])}>
                  Skip
                </Button>
                <Button size="sm" variant="ghost" disabled={fileBusy || !downloadSupportsFileActions(status)} onClick={() => void runFile("normal", [file.id])}>
                  Normal
                </Button>
                <Button size="sm" variant="ghost" disabled={fileBusy || !downloadSupportsFileActions(status)} onClick={() => void runFile("high", [file.id])}>
                  High
                </Button>
                <Button size="sm" variant="ghost" disabled={fileBusy || !downloadSupportsFileActions(status)} onClick={() => void runFile("max", [file.id])}>
                  Max
                </Button>
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
