import React, { useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  BookOpen,
  CheckCircle2,
  CheckSquare,
  EyeOff,
  FileSearch,
  RadioTower,
  Square,
  TrendingUp,
  UserRoundSearch
} from "lucide-react";
import {
  bulkUpdateWanted,
  confirmWantedMetadataReviewCanonical,
  runUpgradeSearch,
  runWantedMonitor,
  searchWantedReleases
} from "../../lib/api";
import type { MetadataReviewConfirmOutcome, MetadataReviewOptions, WantedItem } from "../../lib/api";
import {
  keys,
  useLibrarySettings,
  useBookCollection,
  useWantedMetadataReview
} from "../../lib/queries";
import { formatDate } from "../../lib/format";
import { useToast } from "../../components/toast";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  IconButton,
  InlineNotice,
  LoadingRow,
  PageHeader,
  StatBar,
  TabNav,
  ToolbarButton
} from "../../components/ui";
import { useWantedReleaseSearch } from "./components/ReleasesPanel";
import { libraryBookPath } from "../library/lib";
import {
  appErrorMessage,
  errorMessage,
  metadataReviewBadgeLabel,
  metadataReviewMap,
  monitorRunSummary,
  upgradeRunSummary,
} from "./lib";
import "./wanted.css";

type WantedTab = "missing" | "cutoff" | "review" | "incomplete" | "unknown";

/**
 * Wanted: native gap/evidence views in the Readarr page shape. Tabs include
 * Missing (/wanted), Incomplete/Unknown, Cutoff Unmet (/wanted/cutoff-unmet), and Review
 * (/wanted/review, the metadata-review queue + bulk confirm flow). Rows link
 * to /library/book/:id; author subscriptions live under Library → Authors and
 * the acquisition-queue strip lives on the Dashboard.
 *
 * Legacy URL contract: `?item=<id>` redirects to /library/book/<id>,
 * `?filter=cutoff-unmet|review` redirect to their tabs, `?tab=authors`
 * redirects to /library/authors, and any other `?filter=` lands on Missing.
 */
export default function WantedPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const toast = useToast();
  const client = useQueryClient();

  const tab: WantedTab = location.pathname.endsWith("/cutoff-unmet")
    ? "cutoff"
    : location.pathname.endsWith("/review")
      ? "review"
      : location.pathname.endsWith("/incomplete") ? "incomplete" : location.pathname.endsWith("/unknown") ? "unknown" : "missing";

  /* ------------------------------ URL contract ----------------------------- */

  useEffect(() => {
    const item = searchParams.get("item");
    if (item) {
      navigate(libraryBookPath(item), { replace: true });
      return;
    }
    if (searchParams.get("tab") === "authors") {
      navigate("/library/authors", { replace: true });
      return;
    }
    const filter = searchParams.get("filter");
    if (filter === "cutoff-unmet") {
      navigate("/wanted/cutoff-unmet", { replace: true });
    } else if (filter === "incomplete" || filter === "unknown") {
      navigate(`/wanted/${filter}`, { replace: true });
    } else if (filter === "review") {
      navigate("/wanted/review", { replace: true });
    } else if (filter) {
      navigate("/wanted", { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  /* --------------------------------- Data ---------------------------------- */

  const activeTabRef = useRef<HTMLAnchorElement>(null);
  useEffect(() => {
    const link = activeTabRef.current;
    const nav = link?.parentElement;
    if (!link || !nav) return;
    const item = link.getBoundingClientRect(), container = nav.getBoundingClientRect();
    if (item.left < container.left) nav.scrollLeft -= container.left - item.left;
    else if (item.right > container.right) nav.scrollLeft += item.right - container.right;
  }, [tab]);
  const [pagination, setPagination] = useState<{ tab: WantedTab; cursors: string[] }>({ tab, cursors: [""] });
  const cursors = pagination.tab === tab ? pagination.cursors : [""];
  const cursor = cursors[cursors.length - 1];
  const wantedQuery = useBookCollection({ state: tab === "cutoff" ? "cutoffUnmet" : tab === "review" ? "all" : tab, sort: "title", limit: tab === "review" ? 1 : 100, cursor: tab === "review" ? "" : cursor });
  const [reviewSearch, setReviewSearch] = useState("");
  const [reviewQuerySearch, setReviewQuerySearch] = useState("");
  const [reviewFormat, setReviewFormat] = useState<MetadataReviewOptions["format"]>("all");
  useEffect(() => { if (reviewSearch === reviewQuerySearch) return; const timer = setTimeout(() => { setReviewQuerySearch(reviewSearch); setPagination(value => value.tab === "review" ? { tab: "review", cursors: [""] } : value); }, 250); return () => clearTimeout(timer); }, [reviewSearch, reviewQuerySearch]);
  const reviewQuery = useWantedMetadataReview({ q: reviewQuerySearch, format: reviewFormat, cursor: tab === "review" ? cursor : "", limit: tab === "review" ? 100 : 1 });
  const activeCollection = tab === "review" ? reviewQuery : wantedQuery;
  const librarySettings = useLibrarySettings();
  const releaseSearch = useWantedReleaseSearch();
  const language = librarySettings.data?.settings.standardSearchLanguage || "English";
  const reviewByID = useMemo(() => metadataReviewMap(reviewQuery.data), [reviewQuery.data]);
  const reviewItems = useMemo(() => (reviewQuery.data?.items ?? []).map((entry) => entry.wantedItem), [reviewQuery.data]);
  const rows: WantedItem[] = useMemo(() => tab === "review" ? reviewItems : wantedQuery.data?.books ?? [], [tab, reviewItems, wantedQuery.data]);
  const rowsLoading = tab === "review" ? reviewQuery.isLoading : wantedQuery.isLoading;
  const rowsError = tab === "review" ? reviewQuery.isError : wantedQuery.isError;
  const counts = wantedQuery.data?.counts ?? {};

  /* ------------------------------- Selection -------------------------------- */

  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);

  // Selections don't carry across tabs; also drop rows that left the list.
  useEffect(() => {
    setSelectedIDs([]);
  }, [tab, cursor, reviewSearch, reviewFormat]);
  useEffect(() => {
    const available = new Set(rows.map((item) => item.id));
    setSelectedIDs((current) => current.filter((id) => available.has(id)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wantedQuery.data, reviewQuery.data]);

  const selectedSet = useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const selectedRows = useMemo(() => rows.filter((item) => selectedSet.has(item.id)), [rows, selectedSet]);
  const allSelected = rows.length > 0 && rows.every((item) => selectedSet.has(item.id));

  function toggleSelection(item: WantedItem) {
    setSelectedIDs((current) =>
      current.includes(item.id) ? current.filter((id) => id !== item.id) : [...current, item.id]
    );
  }

  function toggleAll() {
    setSelectedIDs(allSelected ? [] : rows.map((item) => item.id));
  }

  /* -------------------------------- Actions --------------------------------- */

  const [isSearchingSelected, setIsSearchingSelected] = useState(false);
  const [isSearchingAll, setIsSearchingAll] = useState(false);
  const [isUpgradingSelected, setIsUpgradingSelected] = useState(false);
  const [isUpgradingAll, setIsUpgradingAll] = useState(false);
  const [isUnmonitoring, setIsUnmonitoring] = useState(false);
  const [isConfirmingReviews, setIsConfirmingReviews] = useState(false);
  const [reviewConfirmOutcome, setReviewConfirmOutcome] = useState<MetadataReviewConfirmOutcome | null>(null);
  const [interactiveSearchID, setInteractiveSearchID] = useState("");

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  /** Sequential release search for the selected rows with one summary toast. */
  async function runSearchSelected() {
    if (!selectedRows.length) return;
    setIsSearchingSelected(true);
    toast.notify(`Release search started for ${selectedRows.length} book${selectedRows.length === 1 ? "" : "s"}…`, "info");
    let found = 0;
    let approved = 0;
    let errors = 0;
    for (const item of selectedRows) {
      try {
        const outcome = await searchWantedReleases(item.id, language);
        client.setQueryData(keys.wantedReleases(item.id), outcome.releases);
        found += outcome.releases.length;
        approved += outcome.releases.filter((release) => release.approved).length;
      } catch {
        errors += 1;
      }
    }
    const summary = `Release search: ${found} found · ${approved} approved across ${selectedRows.length} book${selectedRows.length === 1 ? "" : "s"}`;
    if (errors) {
      toast.notify(`${summary} · ${errors} error${errors === 1 ? "" : "s"}`, "warn");
    } else {
      toast.success(summary);
    }
    await invalidate(keys.wanted, keys.acquisitionQueue);
    setIsSearchingSelected(false);
  }

  async function runSearchAll() {
    setIsSearchingAll(true);
    try {
      const run = await runWantedMonitor({ force: true, autoGrab: false, paused: true });
      toast.success(monitorRunSummary(run));
      await invalidate(keys.wanted, keys.acquisitionQueue, keys.wantedMetadataReview, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted search failed")));
    } finally {
      setIsSearchingAll(false);
    }
  }

  async function runUpgrade(selectedOnly: boolean) {
    const wantedIds = selectedOnly ? selectedRows.map((item) => item.id) : [];
    if (selectedOnly && !wantedIds.length) return;
    const setBusy = selectedOnly ? setIsUpgradingSelected : setIsUpgradingAll;
    setBusy(true);
    try {
      const run = await runUpgradeSearch({ wantedIds, autoGrab: false, paused: true, force: true });
      toast.success(upgradeRunSummary(run));
      await invalidate(keys.wanted, keys.wantedCutoffUnmet, keys.acquisitionQueue, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Upgrade search failed")));
    } finally {
      setBusy(false);
    }
  }

  async function unmonitorSelected() {
    const ids = selectedRows.map((item) => item.id);
    if (!ids.length) return;
    setIsUnmonitoring(true);
    try {
      const outcome = await bulkUpdateWanted({ ids, set: { monitored: false } });
      const failures = (outcome.results ?? []).filter((result) => result.status === "error" || Boolean(result.error));
      if (failures.length) {
        toast.notify(`Unmonitor: ${ids.length - failures.length} updated · ${failures.length} failed`, "warn");
      } else {
        toast.success(`Unmonitored ${ids.length} book${ids.length === 1 ? "" : "s"}`);
      }
      setSelectedIDs([]);
      await invalidate(keys.wanted, keys.wantedCutoffUnmet, keys.acquisitionQueue, keys.wantedMetadataReview);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted bulk update failed")));
    } finally {
      setIsUnmonitoring(false);
    }
  }

  async function confirmSelectedReviews() {
    const wantedIds = selectedRows.map((item) => item.id).filter((id) => reviewByID.has(id));
    if (!wantedIds.length) return;
    setIsConfirmingReviews(true);
    try {
      const revisions = Object.fromEntries(wantedIds.flatMap(id => { const revision = reviewByID.get(id)?.revision; return revision ? [[id, revision]] : []; }));
      const outcome = await confirmWantedMetadataReviewCanonical({ wantedIds, ...(Object.keys(revisions).length ? { revisions } : {}) });
      setReviewConfirmOutcome(outcome);
      (outcome.items ?? []).forEach((provenance) => {
        client.setQueryData(keys.wantedMetadata(provenance.wantedItem.id), provenance);
      });
      setSelectedIDs((current) => current.filter((id) => !wantedIds.includes(id)));
      toast.success(
        `${outcome.fieldsConfirmed} field${outcome.fieldsConfirmed === 1 ? "" : "s"} confirmed across ${outcome.itemsReviewed} item${outcome.itemsReviewed === 1 ? "" : "s"}${outcome.skippedItems ? `; ${outcome.skippedItems} skipped — open book details to choose unresolved values` : ""}`
      );
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Metadata review confirmation failed")));
    } finally {
      setIsConfirmingReviews(false);
    }
  }

  /** Interactive search affordance: run the release search, then open the book page. */
  async function runInteractiveSearch(item: WantedItem) {
    setInteractiveSearchID(item.id);
    try {
      await releaseSearch.run(item);
      navigate(libraryBookPath(item.id));
    } finally {
      setInteractiveSearchID("");
    }
  }

  /* -------------------------------- Render ---------------------------------- */

  const queryNotices = [
    wantedQuery.error ? appErrorMessage(errorMessage(wantedQuery.error, "Wanted refresh failed")) : "",
    tab === "review" && reviewQuery.error ? appErrorMessage(errorMessage(reviewQuery.error, "Wanted metadata review failed")) : ""
  ].filter(Boolean);

  const tabs = [
    { label: "Missing", to: "/wanted", active: tab === "missing" },
    { label: "Incomplete", to: "/wanted/incomplete", active: tab === "incomplete" },
    { label: "Unknown", to: "/wanted/unknown", active: tab === "unknown" },
    { label: "Cutoff Unmet", to: "/wanted/cutoff-unmet", active: tab === "cutoff" },
    { label: "Review", to: "/wanted/review", active: tab === "review" }
  ];

  const anyBulkBusy = isSearchingSelected || isSearchingAll || isUpgradingSelected || isUpgradingAll || isUnmonitoring || isConfirmingReviews;

  function renderRow(item: WantedItem) {
    const review = reviewByID.get(item.id);
    const searchingThis = releaseSearch.searchingID === item.id;
    return (
      <tr key={item.id} className={selectedSet.has(item.id) ? "selected" : undefined}>
        <td className="wanted-check-cell">
          <input
            type="checkbox"
            checked={selectedSet.has(item.id)}
            onChange={() => toggleSelection(item)}
            aria-label={`Select ${item.title}`}
          />
        </td>
        <td className="wanted-cover-cell">
          <Link to={libraryBookPath(item.id)} title={`Open ${item.title}`}>
            {item.coverUrl ? (
              <img
                className="wanted-item-cover"
                src={item.coverUrl}
                alt=""
                loading="lazy"
                onError={(event) => {
                  event.currentTarget.style.visibility = "hidden";
                }}
              />
            ) : (
              <span className="wanted-item-cover-placeholder" aria-hidden>
                <BookOpen size={16} />
              </span>
            )}
          </Link>
        </td>
        <td>
          <Link className="cell-primary" to={libraryBookPath(item.id)}>
            {item.title}
          </Link>
        </td>
        <td className="cell-muted">{item.authorName || "Unknown author"}</td>
        <td className="cell-muted">{item.createdAt ? formatDate(item.createdAt) : "—"}</td>
        <td>
          <Badge>{item.qualityProfile || "standard"}</Badge>
        </td>
        {tab === "review" ? (
          <td>{review ? <Badge tone="warn">{metadataReviewBadgeLabel(review)}</Badge> : null}</td>
        ) : (
          <td className="cell-actions">
            <Button
              size="sm"
              icon={FileSearch}
              busy={searchingThis && !interactiveSearchID}
              disabled={releaseSearch.isSearching}
              title="Search indexers for releases"
              onClick={() => void releaseSearch.run(item)}
            >
              {searchingThis && !interactiveSearchID ? "Searching" : "Search"}
            </Button>
            <IconButton
              icon={UserRoundSearch}
              size="sm"
              label={`Interactive search for ${item.title}`}
              busy={interactiveSearchID === item.id}
              disabled={releaseSearch.isSearching}
              onClick={() => void runInteractiveSearch(item)}
            />
          </td>
        )}
      </tr>
    );
  }

  return (
    <>
      <PageHeader
        title="Wanted"
        subtitle="Missing books, quality-cutoff gaps, and metadata review."
        actions={
          ["missing", "incomplete", "unknown"].includes(tab) ? (
            <>
              <ToolbarButton
                icon={FileSearch}
                label={isSearchingSelected ? "Searching selected" : "Search Selected"}
                busy={isSearchingSelected}
                disabled={anyBulkBusy || selectedRows.length === 0}
                title="Run a release search for the selected books"
                onClick={() => void runSearchSelected()}
              />
              <ToolbarButton
                icon={RadioTower}
                label={isSearchingAll ? "Checking batch" : "Check Next Batch"}
                busy={isSearchingAll}
                disabled={anyBulkBusy}
                title="Check the next 50 monitored books and search eligible gaps; never grab automatically"
                onClick={() => void runSearchAll()}
              />
              <ToolbarButton
                icon={EyeOff}
                label={isUnmonitoring ? "Unmonitoring" : "Unmonitor Selected"}
                busy={isUnmonitoring}
                disabled={anyBulkBusy || selectedRows.length === 0}
                title="Stop monitoring the selected books"
                onClick={() => void unmonitorSelected()}
              />
            </>
          ) : tab === "cutoff" ? (
            <>
              <ToolbarButton
                icon={TrendingUp}
                label={isUpgradingSelected ? "Searching upgrades" : "Upgrade Search Selected"}
                busy={isUpgradingSelected}
                disabled={anyBulkBusy || selectedRows.length === 0}
                title="Look for better-scored releases for the selected books"
                onClick={() => void runUpgrade(true)}
              />
              <ToolbarButton
                icon={TrendingUp}
                label={isUpgradingAll ? "Checking upgrades" : "Check Upgrade Batch"}
                busy={isUpgradingAll}
                disabled={anyBulkBusy}
                title="Check the next 50 monitored books for eligible upgrades; never grab automatically"
                onClick={() => void runUpgrade(false)}
              />
              <ToolbarButton
                icon={EyeOff}
                label={isUnmonitoring ? "Unmonitoring" : "Unmonitor Selected"}
                busy={isUnmonitoring}
                disabled={anyBulkBusy || selectedRows.length === 0}
                title="Stop monitoring the selected books"
                onClick={() => void unmonitorSelected()}
              />
            </>
          ) : null
        }
      >
        <div className="wanted-tab-row">
          <TabNav
            tabs={tabs.map(({ label, to }) => ({ label, to }))}
            render={(navTab) => {
              const active = tabs.find((entry) => entry.label === navTab.label)?.active;
              return (
                <Link key={navTab.label} ref={active ? activeTabRef : undefined} to={navTab.to} className={active ? "active" : undefined}>
                  {navTab.label}
                </Link>
              );
            }}
          />
        </div>
      </PageHeader>

      {queryNotices.map((notice) => (
        <InlineNotice key={notice} tone="danger">
          {notice}
        </InlineNotice>
      ))}

      {wantedQuery.data ? <StatBar
        stats={[
          { label: "Missing", value: counts.missing ?? 0, tone: counts.missing ? "danger" : "neutral" },
          { label: "Cutoff Unmet", value: counts.cutoffUnmet ?? 0, tone: counts.cutoffUnmet ? "warn" : "neutral" },
          { label: "Review", value: reviewQuery.data?.total ?? 0, tone: reviewQuery.data?.total ? "info" : "neutral" },
          { label: "Incomplete", value: counts.incomplete ?? 0, tone: counts.incomplete ? "warn" : "neutral" },
          { label: "Unknown", value: counts.unknown ?? 0, tone: counts.unknown ? "warn" : "neutral" },
          { label: "Library books", value: wantedQuery.data.total }
        ]}
      /> : null}
      {wantedQuery.data && ["partial", "unavailable"].includes(wantedQuery.data.downloads) ? <InlineNotice tone="warn">Download status is unavailable or incomplete. Books without verified media may show Unknown.</InlineNotice> : null}

      {tab === "review" && <div className="wanted-pagination wanted-review-filters" aria-label="Metadata review filters">
        <input aria-label="Filter metadata reviews" placeholder="Filter metadata reviews" value={reviewSearch} onChange={event => setReviewSearch(event.target.value)} />
        <select aria-label="Metadata review format" value={reviewFormat} onChange={event => { setReviewFormat(event.target.value as MetadataReviewOptions["format"]); setPagination({ tab, cursors: [""] }); }}>
          <option value="all">All formats</option><option value="ebook">Ebooks</option><option value="audiobook">Audiobooks</option>
        </select>
      </div>}
      {tab === "review" && rows.length ? (
        <Card className="wanted-review-bulk-card">
          <div className="wanted-review-bulkbar" aria-label="Metadata review bulk actions">
            <div className="wanted-review-bulkbar-text">
              <strong>{reviewQuery.data?.conflictCount ?? 0} unresolved metadata conflicts in the library</strong>
              <span>
                {selectedRows.length} selected · {rows.length} shown
                {reviewConfirmOutcome
                  ? ` · ${reviewConfirmOutcome.fieldsConfirmed} field${reviewConfirmOutcome.fieldsConfirmed === 1 ? "" : "s"} confirmed`
                  : ""}
              </span>
            </div>
            <Button size="sm" disabled={anyBulkBusy} icon={allSelected ? CheckSquare : Square} onClick={toggleAll}>
              {allSelected ? "Clear shown" : "Select shown"}
            </Button>
            <Button
              size="sm"
              variant="primary"
              icon={CheckCircle2}
              disabled={anyBulkBusy || selectedRows.length === 0}
              busy={isConfirmingReviews}
              onClick={() => void confirmSelectedReviews()}
            >
              {isConfirmingReviews ? "Confirming" : "Keep current"}
            </Button>
          </div>
        </Card>
      ) : null}

      <Card padded={rows.length === 0} subtitle={`${rows.length} shown · ${activeCollection.data?.filtered ?? 0} matching · ${selectedRows.length} selected on this page`}
        actions={<div className="wanted-pagination" aria-label="Wanted pages">
          <Button size="sm" disabled={cursors.length < 2 || activeCollection.isFetching || anyBulkBusy} onClick={() => setPagination({ tab, cursors: cursors.slice(0, -1) })}>Previous</Button>
          <span>Page {cursors.length}</span>
          <Button size="sm" disabled={!activeCollection.data?.nextCursor || activeCollection.isFetching || anyBulkBusy} onClick={() => setPagination({ tab, cursors: [...cursors, activeCollection.data?.nextCursor ?? ""] })}>Next</Button>
          <Button size="sm" disabled={activeCollection.isFetching || anyBulkBusy} onClick={() => void activeCollection.refetch()}>Refresh page</Button>
        </div>}>
        {rowsLoading ? (
          <LoadingRow
            label={
              tab === "cutoff"
                ? "Loading cutoff unmet books…"
                : tab === "review"
                  ? "Loading metadata reviews…"
                  : `Loading ${tab} books…`
            }
          />
        ) : rows.length ? (
          <DataTable className="wanted-gap-table">
            <thead>
              <tr>
                <th className="wanted-check-cell">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    onChange={toggleAll}
                    aria-label={allSelected ? "Clear all rows" : "Select all rows"}
                  />
                </th>
                <th className="wanted-cover-cell" aria-label="Cover" />
                <th>Title</th>
                <th>Author</th>
                <th>Added</th>
                <th>Quality Profile</th>
                {tab === "review" ? <th>Review</th> : <th aria-label="Actions" />}
              </tr>
            </thead>
            <tbody>{rows.map(renderRow)}</tbody>
          </DataTable>
        ) : rowsError ? <EmptyState icon={BookOpen} title="Books could not be loaded" actions={<Button size="sm" onClick={() => void (tab === "review" ? reviewQuery.refetch() : wantedQuery.refetch())}>Retry</Button>}>Retry loading this collection.</EmptyState>
        : cursor ? <EmptyState icon={BookOpen} title="No books on this page" actions={<Button size="sm" onClick={() => setPagination({ tab, cursors: [""] })}>First page</Button>}>The collection may have changed. Return to the first page to refresh your view.</EmptyState>
        : tab === "cutoff" ? (
          <EmptyState icon={TrendingUp} title="No books below their quality cutoff">
            Books with a tracked file scoring under their quality profile’s cutoff appear here so you can run upgrade
            searches for better releases.
          </EmptyState>
        ) : tab === "review" ? (
          <EmptyState icon={CheckCircle2} title={reviewQuery.data?.total ? "No matching metadata reviews" : "No metadata reviews pending"}>
            Tracked books, including imported books, with conflicting provider metadata appear here for a bulk keep-current decision.
          </EmptyState>
        ) : (wantedQuery.data?.total ?? 0) > 0 ? (
          <EmptyState icon={FileSearch} title={`No ${tab} books in this list`}>
            No library books have this status. Incomplete and unknown evidence are listed separately.
          </EmptyState>
        ) : (
          <EmptyState icon={BookOpen} title="No wanted items">
            Mark a metadata search result wanted to start Readarr-style acquisition planning.
          </EmptyState>
        )}
      </Card>
    </>
  );
}
