import React, { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Ban, History as HistoryIcon, RefreshCw } from "lucide-react";
import {
  Badge,
  Button,
  DataTable,
  EmptyState,
  IconButton,
  InlineNotice,
  LoadingRow,
  Modal,
  Toolbar,
  ToolbarButton
} from "../../components/ui";
import { useToast } from "../../components/toast";
import { keys, useHistory } from "../../lib/queries";
import { demoModeEnabled } from "../../lib/demo";
import { formatDateTime, formatRelativeTime } from "../../lib/format";
import { markDownloadFailed, type HistoryEvent } from "../../lib/api";
import type { BadgeTone } from "./lib";

function severityTone(severity: string): BadgeTone {
  const normalized = severity.toLowerCase();
  if (normalized.includes("error") || normalized.includes("fail")) return "danger";
  if (normalized.includes("warn")) return "warn";
  if (normalized.includes("success")) return "success";
  return "info";
}

function stringField(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/** Best-effort related book/author label pulled from the event payload. */
function relatedLabel(event: HistoryEvent): string {
  const data = event.data ?? {};
  const title = stringField(data["title"]) || stringField(data["bookTitle"]) || stringField(data["name"]);
  const author = stringField(data["authorName"]) || stringField(data["author"]);
  if (title && author) return `${title} — ${author}`;
  if (title) return title;
  if (author) return author;
  if (event.entityType && event.entityId) return `${event.entityType} ${event.entityId}`;
  return "—";
}

/**
 * Grab events carry the client-reported download id in `data.downloadId`
 * (see release_grabbed in backend/internal/wanted/service.go). "Mark as
 * failed" is gated on that id being present, which also excludes
 * *_grab_failed events that never produced a download.
 */
function grabDownloadID(event: HistoryEvent): string {
  if (!event.eventType.toLowerCase().includes("grab")) return "";
  return stringField((event.data ?? {})["downloadId"]);
}

type MarkFailedTarget = {
  downloadId: string;
  client?: string;
  label: string;
};

export default function HistoryTab() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const history = useHistory(100);
  const events = history.data ?? [];

  const [markFailedTarget, setMarkFailedTarget] = useState<MarkFailedTarget | null>(null);
  const [markFailedBlocklist, setMarkFailedBlocklist] = useState(true);
  const [markFailedResearch, setMarkFailedResearch] = useState(true);
  const [isMarkingFailed, setIsMarkingFailed] = useState(false);

  function openMarkFailed(event: HistoryEvent, downloadId: string) {
    const data = event.data ?? {};
    setMarkFailedTarget({
      downloadId,
      client: stringField(data["client"]) || undefined,
      label: stringField(data["title"]) || relatedLabel(event)
    });
    setMarkFailedBlocklist(true);
    setMarkFailedResearch(true);
  }

  async function confirmMarkFailed() {
    const target = markFailedTarget;
    if (!target) return;
    setIsMarkingFailed(true);
    try {
      const outcome = await markDownloadFailed({
        id: target.downloadId,
        client: target.client,
        blocklist: markFailedBlocklist,
        research: markFailedResearch
      });
      setMarkFailedTarget(null);
      toast.success(
        `Marked “${target.label}” failed${outcome.blocklisted ? " · release blocklisted" : ""}${
          outcome.searchTriggered ? " · replacement search triggered" : ""
        }`
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: keys.downloads() }),
        queryClient.invalidateQueries({ queryKey: keys.blocklist() }),
        queryClient.invalidateQueries({ queryKey: keys.acquisitionQueue }),
        queryClient.invalidateQueries({ queryKey: keys.wanted }),
        queryClient.invalidateQueries({ queryKey: keys.history(100) })
      ]);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Mark as failed request failed");
    } finally {
      setIsMarkingFailed(false);
    }
  }

  return (
    <>
      <Toolbar align="start">
        <ToolbarButton
          icon={RefreshCw}
          label="Refresh"
          busy={history.isFetching}
          onClick={() => void history.refetch()}
        />
      </Toolbar>
      {history.isError && !demoModeEnabled ? (
        <InlineNotice tone="danger">
          {history.error instanceof Error ? history.error.message : "History refresh failed"}
        </InlineNotice>
      ) : null}
      {history.isLoading ? (
        <LoadingRow label="Loading history…" />
      ) : events.length === 0 ? (
        <EmptyState icon={HistoryIcon} title="No history yet">
          Monitor runs, grabs, imports, and download events will appear here as they happen.
        </EmptyState>
      ) : (
        <DataTable>
          <thead>
            <tr>
              <th>Time</th>
              <th>Event</th>
              <th>Message</th>
              <th>Related</th>
              <th aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            {events.map((event) => {
              const downloadID = grabDownloadID(event);
              return (
                <tr key={event.id}>
                  <td className="activity-history-time" title={formatDateTime(event.createdAt)}>
                    {formatRelativeTime(event.createdAt)}
                  </td>
                  <td>
                    <Badge tone={severityTone(event.severity)} title={event.severity}>
                      {event.eventType.replace(/_/g, " ")}
                    </Badge>
                  </td>
                  <td className="cell-primary">{event.message}</td>
                  <td className="cell-muted">{relatedLabel(event)}</td>
                  <td className="cell-actions">
                    {downloadID ? (
                      <IconButton
                        icon={Ban}
                        label="Mark download as failed"
                        size="sm"
                        tone="danger"
                        disabled={isMarkingFailed}
                        onClick={() => openMarkFailed(event, downloadID)}
                      />
                    ) : null}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </DataTable>
      )}

      <Modal
        title="Mark download as failed"
        open={Boolean(markFailedTarget)}
        onClose={() => setMarkFailedTarget(null)}
        footer={
          <>
            <Button variant="ghost" disabled={isMarkingFailed} onClick={() => setMarkFailedTarget(null)}>
              Cancel
            </Button>
            <Button variant="danger" icon={Ban} busy={isMarkingFailed} onClick={() => void confirmMarkFailed()}>
              {isMarkingFailed ? "Marking" : "Mark Failed"}
            </Button>
          </>
        }
      >
        {markFailedTarget ? (
          <>
            <p>
              Mark <strong>{markFailedTarget.label}</strong> as failed?
            </p>
            <label className="activity-modal-check">
              <input
                type="checkbox"
                checked={markFailedBlocklist}
                onChange={(event) => setMarkFailedBlocklist(event.target.checked)}
              />
              <span>Blocklist this release so it is not grabbed again</span>
            </label>
            <label className="activity-modal-check">
              <input
                type="checkbox"
                checked={markFailedResearch}
                onChange={(event) => setMarkFailedResearch(event.target.checked)}
              />
              <span>Search for a replacement release</span>
            </label>
          </>
        ) : null}
      </Modal>
    </>
  );
}
