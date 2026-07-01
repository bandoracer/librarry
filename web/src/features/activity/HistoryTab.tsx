import React from "react";
import { History as HistoryIcon, RefreshCw } from "lucide-react";
import { Badge, DataTable, EmptyState, InlineNotice, LoadingRow, Toolbar, ToolbarButton } from "../../components/ui";
import { useHistory } from "../../lib/queries";
import { demoModeEnabled } from "../../lib/demo";
import { formatDateTime, formatRelativeTime } from "../../lib/format";
import type { HistoryEvent } from "../../lib/api";
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

export default function HistoryTab() {
  const history = useHistory(100);
  const events = history.data ?? [];

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
            </tr>
          </thead>
          <tbody>
            {events.map((event) => (
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
              </tr>
            ))}
          </tbody>
        </DataTable>
      )}
    </>
  );
}
