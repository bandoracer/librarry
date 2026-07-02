import React, { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { grabWanted, importCompletedDownloads, recoverFailedDownloads } from "../../lib/api";
import type { AcquisitionQueueItem } from "../../lib/api";
import { keys } from "../../lib/queries";
import { useToast } from "../../components/toast";
import { Badge, Button } from "../../components/ui";
// The next-action strip moved here from the Wanted page in the wave-B Readarr
// alignment; the presentation helpers (and the .wanted-acquisition-* layout
// classes) intentionally stay with the wanted feature.
import { useWantedReleaseSearch } from "../wanted/components/ReleasesPanel";
import {
  acquisitionBadgeTone,
  acquisitionQueueActionDisabled,
  acquisitionQueueActionID,
  acquisitionQueueActionIcon,
  acquisitionQueueActionLabel,
  acquisitionQueueStateLabel,
  acquisitionQueueStateTone,
  appErrorMessage,
  errorMessage,
  queueDownloadsForImport,
  queueDownloadsForRecovery
} from "../wanted/lib";
import { libraryBookPath } from "../library/lib";
import "../wanted/wanted.css";

/**
 * Actionable per-book next steps for the Dashboard "Acquisition pipeline"
 * card: the top 6 non-imported queue items with the same Readarr-style action
 * buttons the Wanted strip used to have (search / grab / import / recover /
 * view queue). Row titles open the book page.
 */
export function AcquisitionNextActions(props: { items: AcquisitionQueueItem[] }) {
  const navigate = useNavigate();
  const toast = useToast();
  const client = useQueryClient();
  const releaseSearch = useWantedReleaseSearch();

  const [actionID, setActionID] = useState("");

  const rows = useMemo(() => props.items.filter((item) => item.state !== "imported").slice(0, 6), [props.items]);

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  async function runQueueAction(item: AcquisitionQueueItem) {
    const currentActionID = acquisitionQueueActionID(item);
    setActionID(currentActionID);
    try {
      switch (item.state) {
        case "needs_search":
          await releaseSearch.run(item.wantedItem);
          break;
        case "ready_to_grab": {
          const status = await grabWanted(item.wantedItem.id, item.bestRelease?.id, { paused: true });
          toast.success(`Grab queued (paused): ${status.name || item.wantedItem.title}`);
          await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
          break;
        }
        case "import_ready": {
          const downloadsToImport = queueDownloadsForImport(item);
          if (!downloadsToImport.length) {
            throw new Error("No completed download is ready to import for this wanted item");
          }
          const outcome = await importCompletedDownloads({
            downloadIds: downloadsToImport.map((download) => download.id).filter(Boolean),
            move: false,
            importMode: "copy",
            conflictAction: "rename",
            limit: 50
          });
          toast.success(`Import: ${outcome.imported} imported · ${outcome.reviewQueued} review · ${outcome.errored} errors`);
          await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.importReviews(), keys.history());
          break;
        }
        case "blocked": {
          const failedDownloads = queueDownloadsForRecovery(item);
          if (failedDownloads.length) {
            const run = await recoverFailedDownloads({
              downloadIds: failedDownloads.map((download) => download.id).filter(Boolean),
              autoGrab: true,
              force: true
            });
            toast.success(`Recovery: ${run.replacementsFound} replacements · ${run.grabbedCount} grabbed · ${run.errorCount} errors`);
            await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
          } else {
            await invalidate(keys.wantedReleases(item.wantedItem.id), keys.acquisitionQueue);
          }
          break;
        }
        case "queued":
        case "downloading":
          navigate("/downloads");
          break;
        default:
          navigate(libraryBookPath(item.wantedItem.id));
      }
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Queue action failed")));
    } finally {
      setActionID("");
    }
  }

  if (!rows.length) return null;

  return (
    <div className="wanted-acquisition-rows" aria-label="Book acquisition actions">
      {rows.map((item) => {
        const ActionIcon = acquisitionQueueActionIcon(item.state);
        const rowActionID = acquisitionQueueActionID(item);
        return (
          <article className="wanted-acquisition-row" key={item.wantedItem.id}>
            <span className={`wanted-state-dot ${acquisitionQueueStateTone(item.state)}`} aria-hidden />
            <button
              className="wanted-acquisition-row-main"
              onClick={() => navigate(libraryBookPath(item.wantedItem.id))}
              title={`Open ${item.wantedItem.title}`}
              type="button"
            >
              <strong>{item.wantedItem.title}</strong>
              <small>{item.wantedItem.authorName || item.wantedItem.format}</small>
            </button>
            <Badge tone={acquisitionBadgeTone(item.state)}>{acquisitionQueueStateLabel(item.state)}</Badge>
            <Button
              size="sm"
              icon={ActionIcon}
              disabled={acquisitionQueueActionDisabled(item)}
              busy={actionID === rowActionID}
              onClick={() => void runQueueAction(item)}
            >
              {actionID === rowActionID ? "Working" : acquisitionQueueActionLabel(item)}
            </Button>
          </article>
        );
      })}
    </div>
  );
}
