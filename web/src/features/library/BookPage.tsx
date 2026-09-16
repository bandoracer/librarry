import React, { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, BookOpen, ExternalLink, FolderPen } from "lucide-react";
import { updateWanted } from "../../lib/api";
import { keys, useWantedItem } from "../../lib/queries";
import { useToast } from "../../components/toast";
import { Badge, Button, Card, EmptyState, InlineNotice, LoadingRow, PageHeader, ToolbarButton } from "../../components/ui";
import { WantedEditForm } from "../wanted/components/WantedEditForm";
import { ProvenancePanel } from "../wanted/components/ProvenancePanel";
import { ReleasesPanel } from "../wanted/components/ReleasesPanel";
import {
  libraryWantedAuthorPath,
  libraryBookOverviewLine,
  libraryErrorMessage,
  presenceLabel,
  presenceTone,
} from "./lib";
import "./library.css";
import RestoreBookDialog from "./RestoreBookDialog";
import { BookFiles } from "./FileBrowser";
import { RenameBookFolder } from "./RenameBookFolder";

/**
 * Book detail page (route: /library/book/:wantedId): a routable version of
 * the Wanted detail panel — cover header, status badges, monitored toggle,
 * and the shared edit form / provenance / releases components. Delete (with
 * confirmation) lives in the edit form and returns to the author page.
 */
export default function BookPage() {
  const { wantedId = "" } = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const client = useQueryClient();

  const wanted = useWantedItem(wantedId);
  const item = wanted.data;

  const [isTogglingMonitored, setIsTogglingMonitored] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [restoreOpen, setRestoreOpen] = useState(false);

  const authorPath = item ? libraryWantedAuthorPath(item) : "/library";

  async function toggleMonitored() {
    if (!item) return;
    setIsTogglingMonitored(true);
    try {
      const updated = await updateWanted(item.id, { monitored: !item.monitored });
      toast.success(`${updated.title}: ${updated.monitored ? "monitored" : "unmonitored"}`);
      await Promise.all(
        [keys.wanted, keys.acquisitionQueue].map((key) => client.invalidateQueries({ queryKey: key }))
      );
    } catch (error) {
      toast.error(libraryErrorMessage(error));
    } finally {
      setIsTogglingMonitored(false);
    }
  }

  if (wanted.isLoading) {
    return (
      <>
        <PageHeader title="Book" subtitle="Loading…" />
        <LoadingRow label="Loading book…" />
      </>
    );
  }

  if (wanted.isError) {
    return <><PageHeader title="Book unavailable" /><InlineNotice tone="danger">{libraryErrorMessage(wanted.error)}</InlineNotice><Button onClick={() => void wanted.refetch()}>Try again</Button></>;
  }

  if (!item) {
    return (
      <>
        <PageHeader
          title="Book not found"
          subtitle="This wanted item no longer exists."
          actions={<ToolbarButton icon={ArrowLeft} label="Library" onClick={() => navigate("/library")} />}
        />
        <Card>
          <EmptyState
            icon={BookOpen}
            title="No wanted item with this ID"
            actions={
              <Button size="sm" variant="primary" onClick={() => navigate("/library")}>
                Back to Library
              </Button>
            }
          >
            It may have been removed, or the link is stale.
          </EmptyState>
        </Card>
      </>
    );
  }

  const state = item.derivedState ?? "unknown";
  const inactive = ["removed", "ignored"].includes(item.status);

  return (
    <>
      <PageHeader
        title={item.title}
        subtitle={item.authorName || "Unknown author"}
        actions={
          <>
            <ToolbarButton
              icon={ArrowLeft}
              label={item.authorName || "Author"}
              title="Back to the author page"
              onClick={() => navigate(authorPath)}
            />
            <ToolbarButton icon={FolderPen} label="Rename book folder" title="Preview a complete book folder move while preserving chapter names" onClick={() => setRenameOpen(true)} />
            <ToolbarButton
              icon={ExternalLink}
              label="Wanted Queue"
              title="Open the Wanted gap view"
              onClick={() => navigate("/wanted")}
            />
          </>
        }
      />
      <div className="library-page">
        {inactive ? <InlineNotice tone="info">This book is {item.status} and is excluded from the active Library and automatic acquisition. File records and history remain saved. <Button size="sm" onClick={() => setRestoreOpen(true)}>Restore…</Button> <Link to="/library/removed">Removed books</Link></InlineNotice> : null}
        {restoreOpen ? <RestoreBookDialog book={item} onClose={() => setRestoreOpen(false)} onRestored={() => { setRestoreOpen(false); toast.success("Book restored to Library."); }} /> : null}
        <Card padded>
          <div className="library-book-header">
            {item.coverUrl ? (
              <img className="library-book-header-cover" src={item.coverUrl} alt="" loading="lazy" />
            ) : (
              <span className="library-book-header-cover library-book-header-cover-placeholder" aria-hidden>
                <BookOpen size={28} />
              </span>
            )}
            <div className="library-book-header-main">
              <div className="library-book-header-badges">
                <Badge tone={presenceTone(state)}>{presenceLabel(state)}</Badge>
                <Badge>{item.format}</Badge>
                <Badge tone={item.monitored ? "accent" : "neutral"}>
                  {item.monitored ? "Monitored" : "Unmonitored"}
                </Badge>
                {item.manualOverrides?.length ? (
                  <Badge tone="accent">
                    {item.manualOverrides.length} override{item.manualOverrides.length === 1 ? "" : "s"}
                  </Badge>
                ) : null}
              </div>
              <p className="library-book-header-line">{libraryBookOverviewLine(item)}</p>
              {item.stateEvidence ? <p className="library-book-header-line" role="status">
                {item.stateEvidence.message || item.stateEvidence.files.reason}
                {item.stateEvidence.files.state === "present" ? " Based on recorded import or scan observations; files are not checked during this page load." : ""}
              </p> : null}
              <p className="library-book-header-line">
                By{" "}
                <Link to={authorPath} className="library-book-header-author">
                  {item.authorName || "Unknown author"}
                </Link>
              </p>
              {!inactive ? <label className="library-monitor-toggle" title={item.monitored ? "Unmonitor this book" : "Monitor this book"}>
                <input
                  type="checkbox"
                  checked={item.monitored}
                  disabled={isTogglingMonitored}
                  onChange={() => void toggleMonitored()}
                  aria-label={`${item.title} monitored`}
                />
                <span>Monitored</span>
              </label> : null}
            </div>
          </div>
        </Card>

        <RenameBookFolder wantedId={item.id} open={renameOpen} onClose={() => setRenameOpen(false)} />
        <BookFiles key={item.id} wantedId={item.id} />
        {!inactive ? <WantedEditForm item={item} onDeleted={() => navigate("/library/removed")} /> : null}
        <ProvenancePanel key={`provenance-${item.id}`} item={item} />
        {!inactive ? <ReleasesPanel key={`releases-${item.id}`} item={item} /> : null}
      </div>
    </>
  );
}
