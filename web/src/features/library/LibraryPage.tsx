import React, { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  ArrowUpRight,
  BookOpen,
  CheckSquare,
  Eye,
  EyeOff,
  FileSearch,
  FolderPen,
  Pencil,
  RadioTower,
  RefreshCw,
  Square,
  Trash2,
  Users
} from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  IconButton,
  InlineNotice,
  LoadingRow,
  Modal,
  PageHeader,
  Segmented,
  StatBar,
  ToolbarButton
} from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  keys,
  useAuthorSubscriptions,
  useInvalidatingMutation,
  useLibraryFiles,
  useLibrarySettings,
  useQualityProfiles,
  useWanted
} from "../../lib/queries";
import {
  bulkUpdateWanted,
  runAuthorMonitor,
  searchWantedReleases,
  type WantedBulkItemResult,
  type WantedBulkUpdateRequest,
  type WantedItem
} from "../../lib/api";
import { formatDateTime } from "../../lib/format";
import { navItems } from "../../app/nav";
// Shared with Settings → Media Management; the rename tooling lives in the
// settings feature and this cross-feature import is intentional.
import { RenameFilesModal } from "../settings/RenameFilesModal";
import {
  authorMonitorBadge,
  buildLibraryAuthorRows,
  compareLibraryBooks,
  isPersistenceRequiredError,
  libraryAuthorKey,
  libraryAuthorPath,
  libraryAuthorVisibleForFilter,
  libraryBookMatchesMonitorFilter,
  libraryBookOverviewLine,
  libraryBookPath,
  libraryBookVisibleForFilter,
  libraryErrorMessage,
  librarySortLabels,
  loadLibraryViewMode,
  presenceLabel,
  presenceTone,
  storeLibraryViewMode,
  summarizeWantedItems,
  wantedPresenceMap,
  type LibraryFormatFilter,
  type LibraryMonitorFilter,
  type LibrarySortMode,
  type LibraryViewMode
} from "./lib";
import "./library.css";

const BOOK_ROW_CAP = 80;

const subtitle = navItems.find((item) => item.id === "library")?.subtitle ?? "Monitored authors and books";

const formatOptions: { value: LibraryFormatFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "ebook", label: "Ebook" },
  { value: "audiobook", label: "Audiobook" }
];

const monitorOptions: { value: LibraryMonitorFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "monitored", label: "Monitored" },
  { value: "unmonitored", label: "Unmonitored" }
];

const viewOptions: { value: LibraryViewMode; label: string }[] = [
  { value: "table", label: "Table" },
  { value: "posters", label: "Posters" },
  { value: "overview", label: "Overview" }
];

const sortModes: LibrarySortMode[] = ["status", "title", "author", "added"];

type BulkAction = "monitor" | "unmonitor" | "profile" | "format" | "delete";

export default function LibraryPage() {
  const navigate = useNavigate();
  const toast = useToast();

  const wanted = useWanted();
  const files = useLibraryFiles("any");
  const subscriptions = useAuthorSubscriptions();
  const librarySettings = useLibrarySettings();
  const profiles = useQualityProfiles();

  const [textFilter, setTextFilter] = useState("");
  const [formatFilter, setFormatFilter] = useState<LibraryFormatFilter>("all");
  const [monitorFilter, setMonitorFilter] = useState<LibraryMonitorFilter>("all");
  const [sortMode, setSortMode] = useState<LibrarySortMode>("status");
  const [viewMode, setViewMode] = useState<LibraryViewMode>(() => loadLibraryViewMode());
  const [selectedAuthorKey, setSelectedAuthorKey] = useState("");
  const [showAllBooks, setShowAllBooks] = useState(false);

  const [editMode, setEditMode] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [selectedBookIDs, setSelectedBookIDs] = useState<string[]>([]);
  const [bulkAction, setBulkAction] = useState<"" | BulkAction>("");
  const [bulkProfile, setBulkProfile] = useState("");
  const [bulkFormat, setBulkFormat] = useState("");
  const [confirmingBulkDelete, setConfirmingBulkDelete] = useState(false);
  const [bulkFailures, setBulkFailures] = useState<WantedBulkItemResult[]>([]);

  const wantedItems = useMemo(() => wanted.data ?? [], [wanted.data]);
  const libraryFiles = useMemo(() => files.data ?? [], [files.data]);
  const authorSubscriptions = useMemo(() => subscriptions.data ?? [], [subscriptions.data]);
  const qualityProfiles = useMemo(() => profiles.data ?? [], [profiles.data]);

  const presence = useMemo(() => wantedPresenceMap(wantedItems, libraryFiles), [wantedItems, libraryFiles]);
  const wantedSummary = useMemo(() => summarizeWantedItems(wantedItems, presence), [wantedItems, presence]);
  const authorRows = useMemo(
    () => buildLibraryAuthorRows(authorSubscriptions, wantedItems, presence),
    [authorSubscriptions, wantedItems, presence]
  );
  const summary = useMemo(
    () => ({
      authors: authorRows.length,
      monitoredAuthors: authorSubscriptions.length,
      monitoredBooks: wantedItems.filter((item) => item.monitored).length,
      missing: wantedSummary.missing,
      downloading: wantedSummary.downloading,
      downloaded: wantedSummary.downloaded,
      files: libraryFiles.length
    }),
    [authorRows.length, authorSubscriptions.length, libraryFiles.length, wantedItems, wantedSummary]
  );

  const visibleAuthorRows = useMemo(
    () => authorRows.filter((row) => libraryAuthorVisibleForFilter(row, textFilter, formatFilter)),
    [authorRows, textFilter, formatFilter]
  );
  const visibleBooks = useMemo(
    () =>
      wantedItems
        .filter(
          (item) =>
            libraryBookVisibleForFilter(item, presence.get(item.id), textFilter, formatFilter) &&
            libraryBookMatchesMonitorFilter(item, monitorFilter)
        )
        .sort((a, b) => compareLibraryBooks(a, b, presence, sortMode)),
    [wantedItems, presence, textFilter, formatFilter, monitorFilter, sortMode]
  );
  const authorScopedBooks = useMemo(
    () =>
      selectedAuthorKey
        ? visibleBooks.filter((item) => libraryAuthorKey(item.authorName) === selectedAuthorKey)
        : visibleBooks,
    [visibleBooks, selectedAuthorKey]
  );
  const shownBooks = showAllBooks ? authorScopedBooks : authorScopedBooks.slice(0, BOOK_ROW_CAP);
  const selectedAuthorRow = selectedAuthorKey ? authorRows.find((row) => row.key === selectedAuthorKey) : undefined;

  const filtersActive = Boolean(textFilter.trim()) || formatFilter !== "all" || monitorFilter !== "all";

  const selectedBookSet = useMemo(() => new Set(selectedBookIDs), [selectedBookIDs]);
  const shownSelectedIDs = useMemo(
    () => shownBooks.map((item) => item.id).filter((id) => selectedBookSet.has(id)),
    [shownBooks, selectedBookSet]
  );
  const allShownSelected = shownBooks.length > 0 && shownBooks.every((item) => selectedBookSet.has(item.id));
  const titleByID = useMemo(() => new Map(wantedItems.map((item) => [item.id, item.title])), [wantedItems]);

  // Drop selections for books that no longer exist (deleted or refreshed away).
  useEffect(() => {
    const available = new Set(wantedItems.map((item) => item.id));
    setSelectedBookIDs((current) => current.filter((id) => available.has(id)));
  }, [wantedItems]);

  function changeViewMode(view: LibraryViewMode) {
    setViewMode(view);
    storeLibraryViewMode(view);
  }

  function toggleBookSelection(item: WantedItem) {
    setSelectedBookIDs((current) =>
      current.includes(item.id) ? current.filter((id) => id !== item.id) : [...current, item.id]
    );
  }

  function toggleAllShown() {
    setSelectedBookIDs((current) => {
      const shownIDs = shownBooks.map((item) => item.id);
      if (shownIDs.every((id) => current.includes(id)) && shownIDs.length > 0) {
        const shown = new Set(shownIDs);
        return current.filter((id) => !shown.has(id));
      }
      const next = new Set(current);
      shownIDs.forEach((id) => next.add(id));
      return Array.from(next);
    });
  }

  const monitorMutation = useInvalidatingMutation(
    (force: boolean) => runAuthorMonitor({ force }),
    [keys.wanted, keys.authorSubscriptions, keys.authorMetadataReviews, keys.acquisitionQueue]
  );
  const searchMutation = useInvalidatingMutation(
    (item: WantedItem) => searchWantedReleases(item.id, librarySettings.data?.settings.standardSearchLanguage || "English"),
    [keys.wanted, keys.acquisitionQueue]
  );
  const bulkMutation = useInvalidatingMutation(
    (request: WantedBulkUpdateRequest) => bulkUpdateWanted(request),
    [keys.wanted, keys.acquisitionQueue, keys.wantedMetadataReview]
  );

  async function handleMonitor(force: boolean) {
    try {
      const run = await monitorMutation.mutateAsync(force);
      if (run.authorsChecked === 0) {
        toast.notify(
          force
            ? "No author subscriptions to monitor yet — add authors from Add New."
            : "No authors due for a refresh — use Force All to re-check every subscription.",
          "info"
        );
        return;
      }
      const message = `Monitored ${run.authorsChecked} author${run.authorsChecked === 1 ? "" : "s"}, ${run.wantedCreated} new book${run.wantedCreated === 1 ? "" : "s"} wanted`;
      if (run.errorCount > 0) {
        toast.notify(`${message} · ${run.errorCount} error${run.errorCount === 1 ? "" : "s"}`, "warn");
      } else {
        toast.success(message);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Author monitor failed");
    }
  }

  async function handleSearch(item: WantedItem) {
    try {
      const outcome = await searchMutation.mutateAsync(item);
      const found = outcome.releases.length;
      const approved = outcome.releases.filter((release) => release.approved).length;
      if (found > 0) {
        toast.success(`Found ${found} release${found === 1 ? "" : "s"} (${approved} approved) for “${item.title}”`);
      } else {
        toast.notify(`No releases found for “${item.title}”`, "info");
      }
      navigate(`/wanted?item=${encodeURIComponent(item.id)}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Wanted release search failed");
    }
  }

  async function runBulk(action: BulkAction) {
    const ids = shownSelectedIDs;
    if (!ids.length) return;
    const request: WantedBulkUpdateRequest = { ids };
    let label = "";
    switch (action) {
      case "monitor":
        request.set = { monitored: true };
        label = "Monitor";
        break;
      case "unmonitor":
        request.set = { monitored: false };
        label = "Unmonitor";
        break;
      case "profile":
        if (!bulkProfile) return;
        request.set = { qualityProfile: bulkProfile };
        label = `Set profile ${bulkProfile}`;
        break;
      case "format":
        if (!bulkFormat) return;
        request.set = { format: bulkFormat };
        label = `Set format ${bulkFormat}`;
        break;
      case "delete":
        request.delete = true;
        label = "Delete";
        break;
    }
    setBulkAction(action);
    try {
      const outcome = await bulkMutation.mutateAsync(request);
      const results = outcome.results ?? [];
      const failures = results.filter((result) => result.status === "error" || Boolean(result.error));
      setBulkFailures(failures);
      const okCount = results.length - failures.length;
      const verb = action === "delete" ? "deleted" : "updated";
      if (failures.length) {
        toast.notify(`${label}: ${okCount} ${verb} · ${failures.length} failed`, "warn");
      } else {
        toast.success(`${label}: ${okCount} book${okCount === 1 ? "" : "s"} ${verb}`);
      }
      if (action === "delete") {
        setConfirmingBulkDelete(false);
        setSelectedBookIDs([]);
      }
    } catch (error) {
      toast.error(libraryErrorMessage(error));
    } finally {
      setBulkAction("");
    }
  }

  function openWantedItem(item: WantedItem) {
    navigate(`/wanted?item=${encodeURIComponent(item.id)}`);
  }

  function clearFilters() {
    setTextFilter("");
    setFormatFilter("all");
    setMonitorFilter("all");
    setSelectedAuthorKey("");
  }

  const noticeMessages = useMemo(() => {
    const seen = new Set<string>();
    return [wanted.error, subscriptions.error, files.error]
      .filter((error): error is Error => error instanceof Error)
      .map((error) => libraryErrorMessage(error))
      .filter((message) => {
        if (seen.has(message)) return false;
        seen.add(message);
        return true;
      });
  }, [wanted.error, subscriptions.error, files.error]);

  const dueBusy = monitorMutation.isPending && monitorMutation.variables === false;
  const forceBusy = monitorMutation.isPending && monitorMutation.variables === true;
  const authorsLoading = subscriptions.isLoading || wanted.isLoading;

  const booksSubtitle = [
    shownBooks.length < authorScopedBooks.length
      ? `Showing ${shownBooks.length} of ${authorScopedBooks.length}`
      : `${authorScopedBooks.length} shown`,
    selectedAuthorRow ? `by ${selectedAuthorRow.authorName}` : null
  ]
    .filter(Boolean)
    .join(" · ");

  /* ------------------------------ Books views ------------------------------ */

  function bookCheckbox(item: WantedItem, className: string) {
    return (
      <label className={className} title="Select for mass edit">
        <input
          type="checkbox"
          checked={selectedBookSet.has(item.id)}
          onChange={() => toggleBookSelection(item)}
          aria-label={`Select ${item.title}`}
        />
      </label>
    );
  }

  function bookActions(item: WantedItem) {
    const searchingThis = searchMutation.isPending && searchMutation.variables?.id === item.id;
    return (
      <div className="cell-actions">
        <Button size="sm" icon={Pencil} onClick={() => openWantedItem(item)} title="Review this book on the Wanted page">
          Review
        </Button>
        <Button
          size="sm"
          icon={FileSearch}
          busy={searchingThis}
          disabled={searchMutation.isPending}
          onClick={() => void handleSearch(item)}
          title="Search indexers for releases"
        >
          {searchingThis ? "Searching" : "Search"}
        </Button>
      </div>
    );
  }

  function renderTableView() {
    return (
      <DataTable className="library-book-table">
        <thead>
          <tr>
            {editMode ? <th className="library-check-cell" aria-label="Select" /> : null}
            <th className="library-cover-cell" aria-label="Cover" />
            <th>Title</th>
            <th>Author</th>
            <th>Format</th>
            <th>Status</th>
            <th aria-label="Actions" />
          </tr>
        </thead>
        <tbody>
          {shownBooks.map((item) => {
            const state = presence.get(item.id) ?? "missing";
            const overrideCount = item.manualOverrides?.length ?? 0;
            return (
              <tr key={item.id} className={editMode && selectedBookSet.has(item.id) ? "selected" : undefined}>
                {editMode ? <td className="library-check-cell">{bookCheckbox(item, "library-monitor-toggle")}</td> : null}
                <td className="library-cover-cell">
                  <Link to={libraryBookPath(item.id)} title={`Open ${item.title}`}>
                    {item.coverUrl ? (
                      <img className="library-cover" src={item.coverUrl} alt="" loading="lazy" />
                    ) : (
                      <span className="library-cover cell-muted">
                        <BookOpen size={14} aria-hidden />
                      </span>
                    )}
                  </Link>
                </td>
                <td>
                  <div className="library-title-cell">
                    <Link className="cell-primary" to={libraryBookPath(item.id)}>
                      {item.title}
                    </Link>
                    <span className="cell-muted">
                      {item.sourceProvider || "manual"} · {item.monitored ? "monitored" : "unmonitored"} ·{" "}
                      {item.qualityProfile}
                      {overrideCount ? ` · ${overrideCount} override${overrideCount === 1 ? "" : "s"}` : ""}
                    </span>
                  </div>
                </td>
                <td>
                  <Link className="library-author-link" to={libraryAuthorPath(item.authorName)}>
                    {item.authorName || "Unknown author"}
                  </Link>
                </td>
                <td>
                  <Badge>{item.format}</Badge>
                </td>
                <td>
                  <Badge tone={presenceTone(state)}>{presenceLabel(state)}</Badge>
                </td>
                <td>{bookActions(item)}</td>
              </tr>
            );
          })}
        </tbody>
      </DataTable>
    );
  }

  function renderPostersView() {
    return (
      <div className="library-poster-grid" aria-label="Book posters">
        {shownBooks.map((item) => {
          const state = presence.get(item.id) ?? "missing";
          return (
            <div
              className={editMode && selectedBookSet.has(item.id) ? "library-poster-card selected" : "library-poster-card"}
              key={item.id}
            >
              <Link className="library-poster-cover" to={libraryBookPath(item.id)} title={`Open ${item.title}`}>
                {item.coverUrl ? (
                  <img
                    src={item.coverUrl}
                    alt=""
                    loading="lazy"
                    onError={(event) => {
                      event.currentTarget.style.visibility = "hidden";
                    }}
                  />
                ) : (
                  <span className="library-poster-placeholder" aria-hidden>
                    <BookOpen size={26} />
                  </span>
                )}
                <span className="library-poster-badge">
                  <Badge tone={presenceTone(state)}>{presenceLabel(state)}</Badge>
                </span>
              </Link>
              {editMode ? <span className="library-poster-check">{bookCheckbox(item, "library-monitor-toggle")}</span> : null}
              <div className="library-poster-caption">
                <Link className="library-poster-title" to={libraryBookPath(item.id)}>
                  {item.title}
                </Link>
                <Link className="library-poster-author" to={libraryAuthorPath(item.authorName)}>
                  {item.authorName || "Unknown author"}
                </Link>
              </div>
            </div>
          );
        })}
      </div>
    );
  }

  function renderOverviewView() {
    return (
      <div className="library-overview-list" aria-label="Book overview">
        {shownBooks.map((item) => {
          const state = presence.get(item.id) ?? "missing";
          return (
            <article
              className={editMode && selectedBookSet.has(item.id) ? "library-overview-row selected" : "library-overview-row"}
              key={item.id}
            >
              {editMode ? bookCheckbox(item, "library-overview-check") : null}
              <Link className="library-overview-cover" to={libraryBookPath(item.id)} title={`Open ${item.title}`}>
                {item.coverUrl ? (
                  <img
                    src={item.coverUrl}
                    alt=""
                    loading="lazy"
                    onError={(event) => {
                      event.currentTarget.style.visibility = "hidden";
                    }}
                  />
                ) : (
                  <span className="library-overview-placeholder" aria-hidden>
                    <BookOpen size={20} />
                  </span>
                )}
              </Link>
              <div className="library-overview-main">
                <div className="library-overview-title">
                  <Link to={libraryBookPath(item.id)}>
                    <strong>{item.title}</strong>
                  </Link>
                  <Badge tone={presenceTone(state)}>{presenceLabel(state)}</Badge>
                  <Badge>{item.format}</Badge>
                </div>
                <Link className="library-overview-author" to={libraryAuthorPath(item.authorName)}>
                  {item.authorName || "Unknown author"}
                </Link>
                <p className="library-overview-line">{libraryBookOverviewLine(item)}</p>
              </div>
              {bookActions(item)}
            </article>
          );
        })}
      </div>
    );
  }

  return (
    <>
      <PageHeader
        title="Library"
        subtitle={subtitle}
        actions={
          <>
            <ToolbarButton
              icon={RadioTower}
              label="Due Authors"
              title="Monitor author subscriptions that are due a refresh"
              busy={dueBusy}
              disabled={monitorMutation.isPending}
              onClick={() => void handleMonitor(false)}
            />
            <ToolbarButton
              icon={RefreshCw}
              label="Force All"
              title="Force-monitor every author subscription"
              busy={forceBusy}
              disabled={monitorMutation.isPending}
              onClick={() => void handleMonitor(true)}
            />
            <ToolbarButton
              icon={FolderPen}
              label="Rename Files"
              title="Preview and apply file renames against the naming templates"
              onClick={() => setRenameOpen(true)}
            />
            <ToolbarButton
              icon={Pencil}
              label={editMode ? "Done" : "Edit Mode"}
              tone={editMode ? "accent" : undefined}
              title={editMode ? "Leave the mass editor" : "Select books for bulk monitor/profile/format/delete edits"}
              onClick={() => {
                setEditMode((current) => {
                  if (current) setSelectedBookIDs([]);
                  return !current;
                });
              }}
            />
          </>
        }
      />
      <div className="library-page">
        {noticeMessages.map((message) => (
          <InlineNotice key={message} tone={isPersistenceRequiredError(message) ? "warn" : "danger"}>
            {message}
          </InlineNotice>
        ))}

        <StatBar
          stats={[
            { label: "Authors", value: summary.authors },
            { label: "Monitored", value: summary.monitoredAuthors },
            { label: "Books", value: summary.monitoredBooks },
            { label: "Missing", value: summary.missing, tone: summary.missing > 0 ? "danger" : "neutral" },
            { label: "Downloading", value: summary.downloading, tone: summary.downloading > 0 ? "info" : "neutral" },
            { label: "Downloaded", value: summary.downloaded, tone: summary.downloaded > 0 ? "success" : "neutral" },
            { label: "Files", value: summary.files }
          ]}
        />

        <div className="library-filter-row">
          <input
            value={textFilter}
            onChange={(event) => setTextFilter(event.target.value)}
            placeholder="Filter authors or books"
            aria-label="Filter authors or books"
          />
          <Segmented options={formatOptions} value={formatFilter} onChange={setFormatFilter} ariaLabel="Library format" />
          <Segmented
            options={monitorOptions}
            value={monitorFilter}
            onChange={setMonitorFilter}
            ariaLabel="Monitored filter"
          />
          <select
            className="library-sort-select"
            value={sortMode}
            onChange={(event) => setSortMode(event.target.value as LibrarySortMode)}
            aria-label="Sort books"
          >
            {sortModes.map((mode) => (
              <option key={mode} value={mode}>
                Sort: {librarySortLabels[mode]}
              </option>
            ))}
          </select>
          <Segmented options={viewOptions} value={viewMode} onChange={changeViewMode} ariaLabel="Books view mode" />
        </div>

        {editMode ? (
          <Card className="library-bulk-card">
            <div className="library-bulkbar" aria-label="Library mass editor">
              <div className="library-bulkbar-text">
                <strong>Mass editor</strong>
                <span>
                  {shownSelectedIDs.length} selected · {shownBooks.length} shown
                </span>
              </div>
              <Button size="sm" icon={allShownSelected ? CheckSquare : Square} onClick={toggleAllShown}>
                {allShownSelected ? "Clear shown" : "Select shown"}
              </Button>
              <Button
                size="sm"
                icon={Eye}
                disabled={!shownSelectedIDs.length || Boolean(bulkAction)}
                busy={bulkAction === "monitor"}
                onClick={() => void runBulk("monitor")}
              >
                Monitor
              </Button>
              <Button
                size="sm"
                icon={EyeOff}
                disabled={!shownSelectedIDs.length || Boolean(bulkAction)}
                busy={bulkAction === "unmonitor"}
                onClick={() => void runBulk("unmonitor")}
              >
                Unmonitor
              </Button>
              <div className="library-bulk-select">
                <select value={bulkProfile} onChange={(event) => setBulkProfile(event.target.value)} aria-label="Bulk quality profile">
                  <option value="">Quality profile…</option>
                  {qualityProfiles.map((profile) => (
                    <option key={`${profile.name}:${profile.mediaFormat}`} value={profile.name}>
                      {profile.name} · {profile.mediaFormat}
                    </option>
                  ))}
                </select>
                <Button
                  size="sm"
                  disabled={!shownSelectedIDs.length || !bulkProfile || Boolean(bulkAction)}
                  busy={bulkAction === "profile"}
                  onClick={() => void runBulk("profile")}
                >
                  Apply
                </Button>
              </div>
              <div className="library-bulk-select">
                <select value={bulkFormat} onChange={(event) => setBulkFormat(event.target.value)} aria-label="Bulk format">
                  <option value="">Format…</option>
                  <option value="ebook">ebook</option>
                  <option value="audiobook">audiobook</option>
                </select>
                <Button
                  size="sm"
                  disabled={!shownSelectedIDs.length || !bulkFormat || Boolean(bulkAction)}
                  busy={bulkAction === "format"}
                  onClick={() => void runBulk("format")}
                >
                  Apply
                </Button>
              </div>
              <Button
                size="sm"
                variant="danger"
                icon={Trash2}
                disabled={!shownSelectedIDs.length || Boolean(bulkAction)}
                onClick={() => setConfirmingBulkDelete(true)}
              >
                Delete
              </Button>
            </div>
          </Card>
        ) : null}

        {bulkFailures.length ? (
          <InlineNotice tone="warn" onDismiss={() => setBulkFailures([])}>
            {bulkFailures.length} bulk edit{bulkFailures.length === 1 ? "" : "s"} failed:{" "}
            {bulkFailures
              .slice(0, 5)
              .map((failure) => `${titleByID.get(failure.id) ?? failure.id}: ${failure.error || failure.status}`)
              .join(" · ")}
            {bulkFailures.length > 5 ? ` · ${bulkFailures.length - 5} more` : ""}
          </InlineNotice>
        ) : null}

        <div className="library-layout">
          <Card
            title="Authors"
            subtitle={`${visibleAuthorRows.length} shown`}
            padded={false}
            actions={
              selectedAuthorKey ? (
                <Button size="sm" variant="ghost" onClick={() => setSelectedAuthorKey("")}>
                  Show all
                </Button>
              ) : undefined
            }
          >
            {authorsLoading ? (
              <LoadingRow label="Loading authors…" />
            ) : visibleAuthorRows.length ? (
              <DataTable className="library-author-table">
                <tbody>
                  {visibleAuthorRows.map((row) => {
                    const badge = authorMonitorBadge(row);
                    const active = row.key === selectedAuthorKey;
                    return (
                      <tr key={row.key} className={active ? "selected" : undefined}>
                        <td>
                          <div className="library-author-shell">
                            <button
                              type="button"
                              className="library-author-button"
                              aria-pressed={active}
                              title={active ? "Show books by all authors" : `Show books by ${row.authorName}`}
                              onClick={() => setSelectedAuthorKey(active ? "" : row.key)}
                            >
                              <span className="library-author-top">
                                <span className="cell-primary">{row.authorName}</span>
                                <Badge tone={badge.tone}>{badge.label}</Badge>
                              </span>
                              <span className="cell-muted">
                                {(row.formats.join(", ") || "any")} · {row.qualityProfiles.join(", ") || "standard"}
                              </span>
                              <span className="cell-muted">
                                {row.missing} missing · {row.downloaded} downloaded · {row.downloading} downloading
                                {row.cutoffUnmet > 0 ? ` · ${row.cutoffUnmet} cutoff unmet` : ""}
                              </span>
                              {row.subscriptionCount > 0 ? (
                                <span className="cell-muted">
                                  {row.lastSyncAt ? `Synced ${formatDateTime(row.lastSyncAt)}` : "Never synced"}
                                </span>
                              ) : null}
                            </button>
                            <IconButton
                              icon={ArrowUpRight}
                              size="sm"
                              label={`Open ${row.authorName} author page`}
                              onClick={() => navigate(`/library/author/${encodeURIComponent(row.key)}`)}
                            />
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </DataTable>
            ) : (
              <EmptyState
                icon={Users}
                title={filtersActive ? "No authors match this filter" : "No authors yet"}
                actions={
                  filtersActive ? (
                    <Button size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  ) : (
                    <Button size="sm" variant="primary" onClick={() => navigate("/search")}>
                      Add an author
                    </Button>
                  )
                }
              >
                {filtersActive
                  ? "Adjust the text or format filter to see monitored authors."
                  : wanted.isError || subscriptions.isError
                    ? "Authors are unavailable until the error above is resolved."
                    : "Search metadata providers and subscribe to authors to start monitoring their books."}
              </EmptyState>
            )}
          </Card>

          <Card
            title="Monitored books"
            subtitle={booksSubtitle}
            padded={false}
            actions={
              authorScopedBooks.length > BOOK_ROW_CAP ? (
                <Button size="sm" variant="ghost" onClick={() => setShowAllBooks((current) => !current)}>
                  {showAllBooks ? `Show first ${BOOK_ROW_CAP}` : `Show all ${authorScopedBooks.length}`}
                </Button>
              ) : undefined
            }
          >
            {wanted.isLoading ? (
              <LoadingRow label="Loading monitored books…" />
            ) : shownBooks.length ? (
              viewMode === "posters" ? (
                renderPostersView()
              ) : viewMode === "overview" ? (
                renderOverviewView()
              ) : (
                renderTableView()
              )
            ) : (
              <EmptyState
                icon={BookOpen}
                title={
                  selectedAuthorRow
                    ? `No books for ${selectedAuthorRow.authorName}`
                    : filtersActive
                      ? "No monitored books match this filter"
                      : "No monitored books yet"
                }
                actions={
                  selectedAuthorKey || filtersActive ? (
                    <Button size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  ) : (
                    <Button size="sm" variant="primary" onClick={() => navigate("/search")}>
                      Add new books
                    </Button>
                  )
                }
              >
                {selectedAuthorKey || filtersActive
                  ? "Adjust the filters or show all authors to see the rest of the library plan."
                  : wanted.isError
                    ? "Books are unavailable until the error above is resolved."
                    : "Search metadata, monitor authors, and mark books wanted to build the library plan."}
              </EmptyState>
            )}
          </Card>
        </div>
      </div>

      <Modal
        title="Delete wanted books"
        open={confirmingBulkDelete}
        onClose={() => setConfirmingBulkDelete(false)}
        footer={
          <>
            <Button onClick={() => setConfirmingBulkDelete(false)}>Cancel</Button>
            <Button variant="danger" icon={Trash2} busy={bulkAction === "delete"} onClick={() => void runBulk("delete")}>
              {bulkAction === "delete" ? "Deleting" : `Delete ${shownSelectedIDs.length}`}
            </Button>
          </>
        }
      >
        <p>
          Remove <strong>{shownSelectedIDs.length}</strong> book{shownSelectedIDs.length === 1 ? "" : "s"} from the
          wanted queue? Stored release decisions and metadata provenance for them are discarded. Imported files stay on
          disk.
        </p>
      </Modal>

      <RenameFilesModal open={renameOpen} onClose={() => setRenameOpen(false)} />
    </>
  );
}
