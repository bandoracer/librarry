import React, { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, ExternalLink, FileSearch, RefreshCw } from "lucide-react";
import { grabWanted, searchWantedReleases } from "../../../lib/api";
import type { ReleaseDecision, WantedItem } from "../../../lib/api";
import { keys, useLibrarySettings, useWantedReleases } from "../../../lib/queries";
import { formatBytes } from "../../../lib/format";
import { useToast } from "../../../components/toast";
import { Badge, Button, Card, EmptyState, InlineNotice, LoadingRow, Segmented } from "../../../components/ui";
import {
  appErrorMessage,
  errorMessage,
  releaseActionID,
  releaseDecisionFilters,
  releaseDecisionVisibleForFilter,
  summarizeReleaseDecisions
} from "../lib";
import type { ReleaseDecisionFilter } from "../lib";
import "../wanted.css";

/**
 * Runs an indexer release search for a wanted item, stores the outcome in the
 * shared per-item releases cache, and toasts the summary. Shared between the
 * releases panel and callers that trigger searches for arbitrary items (the
 * acquisition queue strip, the `&search=1` URL contract, library pages).
 */
export function useWantedReleaseSearch() {
  const toast = useToast();
  const client = useQueryClient();
  const librarySettings = useLibrarySettings();
  const [searchingID, setSearchingID] = useState("");

  async function run(item: WantedItem): Promise<boolean> {
    const language = librarySettings.data?.settings.standardSearchLanguage || "English";
    setSearchingID(item.id);
    try {
      const outcome = await searchWantedReleases(item.id, language);
      client.setQueryData(keys.wantedReleases(item.id), outcome.releases);
      const summary = summarizeReleaseDecisions(outcome.releases);
      toast.success(
        `Release search: ${outcome.releases.length} found · ${summary.approved} approved · ${summary.rejected} rejected`
      );
      await Promise.all(
        [keys.wanted, keys.acquisitionQueue].map((key) => client.invalidateQueries({ queryKey: key }))
      );
      return true;
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted release search failed")));
      return false;
    } finally {
      setSearchingID("");
    }
  }

  return { run, searchingID, isSearching: Boolean(searchingID) };
}

/**
 * Release review panel for one wanted item: stored decisions with an
 * approved/rejected filter, indexer search, and grab actions. Extracted from
 * the legacy BooksTab detail panel; owns its own fetching and mutations.
 */
export function ReleasesPanel(props: { item: WantedItem }) {
  const { item } = props;
  const toast = useToast();
  const client = useQueryClient();
  const releasesQuery = useWantedReleases(item.id);
  const releaseSearch = useWantedReleaseSearch();

  const [releaseFilter, setReleaseFilter] = useState<ReleaseDecisionFilter>("all");
  const [grabbingReleaseID, setGrabbingReleaseID] = useState("");

  const releases = releasesQuery.data ?? [];
  const releaseSummary = summarizeReleaseDecisions(releases);
  const visibleReleases = releases.filter((release) => releaseDecisionVisibleForFilter(release, releaseFilter));
  const isLoadingReleases = releasesQuery.isLoading || releasesQuery.isFetching;
  const isSearchingReleases = releaseSearch.searchingID === item.id;
  const releasesError = releasesQuery.error
    ? appErrorMessage(errorMessage(releasesQuery.error, "Wanted release decisions refresh failed"))
    : "";

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  async function runSearch() {
    const ok = await releaseSearch.run(item);
    if (ok) setReleaseFilter("all");
  }

  async function reloadStoredReleases() {
    await releasesQuery.refetch();
    await invalidate(keys.acquisitionQueue);
  }

  async function grabRelease(release?: ReleaseDecision, force = false) {
    const actionID = release ? releaseActionID(release, force) : "auto";
    setGrabbingReleaseID(actionID);
    try {
      const status = await grabWanted(item.id, release?.id, { paused: true, force });
      toast.success(`Grab queued (paused): ${status.name || item.title}`);
      await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted grab failed")));
    } finally {
      setGrabbingReleaseID("");
    }
  }

  return (
    <Card
      title="Release review"
      subtitle={
        releases.length
          ? `${releaseSummary.approved} approved · ${releaseSummary.rejected} rejected · ${releases.length} stored`
          : "No stored decisions for this wanted item."
      }
      actions={
        <div className="wanted-release-toolbar">
          <Segmented<ReleaseDecisionFilter>
            ariaLabel="Release decision filter"
            options={releaseDecisionFilters.map((value) => ({ value, label: value }))}
            value={releaseFilter}
            onChange={setReleaseFilter}
          />
          <Button
            size="sm"
            icon={RefreshCw}
            busy={isLoadingReleases}
            title="Reload stored release decisions"
            onClick={() => void reloadStoredReleases()}
          >
            {isLoadingReleases ? "Loading" : "Stored"}
          </Button>
          <Button size="sm" icon={FileSearch} busy={isSearchingReleases} onClick={() => void runSearch()}>
            {isSearchingReleases ? "Searching" : "Search Releases"}
          </Button>
          <Button
            size="sm"
            variant="primary"
            icon={CheckCircle2}
            busy={grabbingReleaseID === "auto"}
            title="Grab the best approved release for this item"
            onClick={() => void grabRelease()}
          >
            {grabbingReleaseID === "auto" ? "Grabbing" : "Grab Best"}
          </Button>
        </div>
      }
    >
      {releasesError ? <InlineNotice tone="danger">{releasesError}</InlineNotice> : null}
      {visibleReleases.length ? (
        <div className="wanted-release-list">
          {visibleReleases.map((release) => {
            const grabID = releaseActionID(release, !release.approved);
            return (
              <article className="wanted-release-row" key={release.id || release.sourceId || release.title}>
                <div className="wanted-release-main">
                  <div className="wanted-release-title">
                    <strong>{release.title}</strong>
                    <Badge tone={release.approved ? "success" : "danger"}>
                      {release.approved ? "Approved" : "Rejected"}
                    </Badge>
                  </div>
                  <p>
                    {release.indexer} · {release.protocol || "release"} · score {release.score.toFixed(1)} ·{" "}
                    {formatBytes(release.sizeBytes ?? 0)} · {release.seeders ?? 0} seeders · {release.leechers ?? 0}{" "}
                    leechers
                  </p>
                  {release.categories?.length ? <small>{release.categories.join(", ")}</small> : null}
                  {release.rejectedReason ? (
                    <small className="wanted-release-rejection">{release.rejectedReason}</small>
                  ) : null}
                </div>
                <div className="wanted-release-actions">
                  {release.infoUrl ? (
                    <a className="btn btn-secondary btn-sm" href={release.infoUrl} rel="noreferrer" target="_blank">
                      <ExternalLink size={13} aria-hidden />
                      Details
                    </a>
                  ) : null}
                  <Button
                    size="sm"
                    variant={release.approved ? "secondary" : "danger"}
                    busy={grabbingReleaseID === grabID}
                    onClick={() => void grabRelease(release, !release.approved)}
                  >
                    {grabbingReleaseID === grabID ? "Grabbing" : release.approved ? "Grab paused" : "Force grab"}
                  </Button>
                </div>
              </article>
            );
          })}
        </div>
      ) : isLoadingReleases ? (
        <LoadingRow label="Loading stored release decisions…" />
      ) : (
        <EmptyState icon={FileSearch} title="No release decisions">
          Search wanted releases to evaluate candidates for this book.
        </EmptyState>
      )}
    </Card>
  );
}
