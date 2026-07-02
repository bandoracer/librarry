import React, { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { RefreshCw, ShieldX, Trash2 } from "lucide-react";
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
import { keys, useBlocklist } from "../../lib/queries";
import { demoModeEnabled } from "../../lib/demo";
import { formatDateTime, formatRelativeTime } from "../../lib/format";
import { clearBlocklist, removeBlocklistItem, type BlocklistItem } from "../../lib/api";
import type { BadgeTone } from "./lib";

function blocklistSourceTone(source: string): BadgeTone {
  switch (source) {
    case "auto-failed":
      return "warn";
    case "history-mark-failed":
      return "info";
    case "queue-remove":
      return "neutral";
    default:
      return "neutral";
  }
}

function blocklistSourceLabel(source: string): string {
  return (source || "unknown").replace(/-/g, " ");
}

function protocolTone(protocol: string): BadgeTone {
  return protocol === "torrent" ? "info" : "accent";
}

/** Blocklist tab: releases excluded from future release evaluation. */
export default function BlocklistTab() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const blocklistQuery = useBlocklist(100);
  const items = useMemo(() => blocklistQuery.data ?? [], [blocklistQuery.data]);

  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [removingID, setRemovingID] = useState("");
  const [isClearing, setIsClearing] = useState(false);
  const [confirmingClearAll, setConfirmingClearAll] = useState(false);

  const selectedSet = useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const visibleSelectedIDs = useMemo(() => items.map((item) => item.id).filter((id) => selectedSet.has(id)), [items, selectedSet]);
  const allSelected = items.length > 0 && items.every((item) => selectedSet.has(item.id));

  function toggleSelection(item: BlocklistItem) {
    setSelectedIDs((current) => (current.includes(item.id) ? current.filter((id) => id !== item.id) : [...current, item.id]));
  }

  function toggleAll() {
    setSelectedIDs(allSelected ? [] : items.map((item) => item.id));
  }

  async function refreshBlocklist() {
    await queryClient.invalidateQueries({ queryKey: keys.blocklist() });
    await blocklistQuery.refetch();
  }

  async function removeItem(item: BlocklistItem) {
    setRemovingID(item.id);
    try {
      await removeBlocklistItem(item.id);
      setSelectedIDs((current) => current.filter((id) => id !== item.id));
      toast.success(`Removed “${item.title}” from the blocklist`);
      await refreshBlocklist();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Blocklist remove failed");
    } finally {
      setRemovingID("");
    }
  }

  async function removeEntries(ids?: string[]) {
    setIsClearing(true);
    try {
      const outcome = await clearBlocklist(ids);
      setConfirmingClearAll(false);
      if (ids?.length) {
        setSelectedIDs((current) => current.filter((id) => !ids.includes(id)));
      } else {
        setSelectedIDs([]);
      }
      toast.success(`Removed ${outcome.removed} blocklist entr${outcome.removed === 1 ? "y" : "ies"}`);
      await refreshBlocklist();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Blocklist clear failed");
    } finally {
      setIsClearing(false);
    }
  }

  const blocklistError =
    blocklistQuery.isError && !demoModeEnabled
      ? blocklistQuery.error instanceof Error
        ? blocklistQuery.error.message
        : "Blocklist refresh failed"
      : "";

  return (
    <>
      <Toolbar align="start">
        <ToolbarButton
          icon={RefreshCw}
          label="Refresh"
          busy={blocklistQuery.isFetching}
          onClick={() => void refreshBlocklist()}
        />
        <ToolbarButton
          icon={Trash2}
          label="Clear All"
          tone="danger"
          disabled={items.length === 0 || isClearing}
          onClick={() => setConfirmingClearAll(true)}
        />
      </Toolbar>

      {blocklistError ? <InlineNotice tone="danger">{blocklistError}</InlineNotice> : null}

      {visibleSelectedIDs.length > 0 ? (
        <div className="activity-bulkbar" aria-label="Bulk blocklist actions">
          <span className="cell-muted activity-bulkbar-count">{visibleSelectedIDs.length} selected</span>
          <Button
            size="sm"
            variant="danger"
            icon={Trash2}
            busy={isClearing}
            onClick={() => void removeEntries(visibleSelectedIDs)}
          >
            Remove Selected
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setSelectedIDs([])}>
            Clear
          </Button>
        </div>
      ) : null}

      {blocklistQuery.isLoading ? (
        <LoadingRow label="Loading blocklist…" />
      ) : items.length === 0 ? (
        <EmptyState icon={ShieldX} title="No blocklisted releases">
          Releases land here when a failed download is blocklisted automatically, when a queue item is removed with
          “Blocklist release” checked, or when a history grab is marked as failed. Blocklisted releases are rejected
          during future release searches with the reason “blocklisted”.
        </EmptyState>
      ) : (
        <DataTable>
          <thead>
            <tr>
              <th>
                <input
                  type="checkbox"
                  aria-label={allSelected ? "Clear all blocklist entries" : "Select all blocklist entries"}
                  checked={allSelected}
                  onChange={toggleAll}
                />
              </th>
              <th>Title</th>
              <th>Book</th>
              <th>Indexer</th>
              <th>Protocol</th>
              <th>Reason</th>
              <th>Source</th>
              <th>Added</th>
              <th aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            {items.map((item) => {
              const isSelected = selectedSet.has(item.id);
              return (
                <tr key={item.id} className={isSelected ? "selected" : undefined}>
                  <td>
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => toggleSelection(item)}
                      aria-label={`Select ${item.title}`}
                    />
                  </td>
                  <td className="cell-primary">
                    <div className="activity-title-cell">
                      <strong title={item.title}>{item.title}</strong>
                      {item.infohash ? (
                        <span className="cell-muted" title={item.infohash}>
                          {item.infohash}
                        </span>
                      ) : null}
                    </div>
                  </td>
                  <td>
                    {item.wantedTitle && item.wantedId ? (
                      <div className="activity-title-cell">
                        <Link to={`/library/book/${encodeURIComponent(item.wantedId)}`} title={`Open ${item.wantedTitle}`}>
                          {item.wantedTitle}
                        </Link>
                        {item.wantedAuthor ? <span className="cell-muted">{item.wantedAuthor}</span> : null}
                      </div>
                    ) : (
                      <span className="cell-muted">—</span>
                    )}
                  </td>
                  <td className="cell-muted">{item.indexer || "—"}</td>
                  <td>
                    <Badge tone={protocolTone(item.protocol)}>{item.protocol || "unknown"}</Badge>
                  </td>
                  <td className="cell-muted">{item.reason || "—"}</td>
                  <td>
                    <Badge tone={blocklistSourceTone(item.source)}>{blocklistSourceLabel(item.source)}</Badge>
                  </td>
                  <td className="activity-history-time" title={formatDateTime(item.createdAt)}>
                    {formatRelativeTime(item.createdAt)}
                  </td>
                  <td className="cell-actions">
                    <IconButton
                      icon={Trash2}
                      label={`Remove ${item.title} from blocklist`}
                      size="sm"
                      tone="danger"
                      busy={removingID === item.id}
                      disabled={Boolean(removingID) || isClearing}
                      onClick={() => void removeItem(item)}
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </DataTable>
      )}

      <Modal
        title="Clear blocklist"
        open={confirmingClearAll}
        onClose={() => setConfirmingClearAll(false)}
        footer={
          <>
            <Button variant="ghost" disabled={isClearing} onClick={() => setConfirmingClearAll(false)}>
              Cancel
            </Button>
            <Button variant="danger" icon={Trash2} busy={isClearing} onClick={() => void removeEntries()}>
              {isClearing ? "Clearing" : "Clear All"}
            </Button>
          </>
        }
      >
        <p>
          Remove all <strong>{items.length}</strong> blocklist entr{items.length === 1 ? "y" : "ies"}? Previously failed
          releases become eligible again in future release searches.
        </p>
      </Modal>
    </>
  );
}
