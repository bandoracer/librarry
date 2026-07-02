import React, { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, BookOpen, ExternalLink } from "lucide-react";
import { updateWanted } from "../../lib/api";
import { keys, useLibraryFiles, useWanted } from "../../lib/queries";
import { useToast } from "../../components/toast";
import { Badge, Button, Card, EmptyState, InlineNotice, LoadingRow, PageHeader, ToolbarButton } from "../../components/ui";
import { WantedEditForm } from "../wanted/components/WantedEditForm";
import { ProvenancePanel } from "../wanted/components/ProvenancePanel";
import { ReleasesPanel } from "../wanted/components/ReleasesPanel";
import {
  libraryAuthorPath,
  libraryBookOverviewLine,
  libraryErrorMessage,
  presenceLabel,
  presenceTone,
  wantedPresenceMap
} from "./lib";
import "./library.css";

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

  const wanted = useWanted();
  const files = useLibraryFiles("any");

  const wantedItems = useMemo(() => wanted.data ?? [], [wanted.data]);
  const libraryFiles = useMemo(() => files.data ?? [], [files.data]);
  const item = useMemo(() => wantedItems.find((entry) => entry.id === wantedId), [wantedItems, wantedId]);
  const presence = useMemo(() => wantedPresenceMap(wantedItems, libraryFiles), [wantedItems, libraryFiles]);

  const [isTogglingMonitored, setIsTogglingMonitored] = useState(false);

  const authorPath = item ? libraryAuthorPath(item.authorName) : "/library";

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
            It may have been removed, imported, or the link is stale.
          </EmptyState>
        </Card>
      </>
    );
  }

  const state = presence.get(item.id) ?? "missing";

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
        {wanted.error ? <InlineNotice tone="danger">{libraryErrorMessage(wanted.error)}</InlineNotice> : null}

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
              <p className="library-book-header-line">
                By{" "}
                <Link to={authorPath} className="library-book-header-author">
                  {item.authorName || "Unknown author"}
                </Link>
              </p>
              <label className="library-monitor-toggle" title={item.monitored ? "Unmonitor this book" : "Monitor this book"}>
                <input
                  type="checkbox"
                  checked={item.monitored}
                  disabled={isTogglingMonitored}
                  onChange={() => void toggleMonitored()}
                  aria-label={`${item.title} monitored`}
                />
                <span>Monitored</span>
              </label>
            </div>
          </div>
        </Card>

        <WantedEditForm item={item} onDeleted={() => navigate(authorPath)} />
        <ProvenancePanel key={`provenance-${item.id}`} item={item} />
        <ReleasesPanel key={`releases-${item.id}`} item={item} />
      </div>
    </>
  );
}
