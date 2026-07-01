import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { BookOpen, FileSearch, Pencil, RadioTower, RefreshCw, Users } from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  InlineNotice,
  LoadingRow,
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
  useWanted
} from "../../lib/queries";
import { runAuthorMonitor, searchWantedReleases, type WantedItem } from "../../lib/api";
import { formatDateTime } from "../../lib/format";
import { navItems } from "../../app/nav";
import {
  authorMonitorBadge,
  buildLibraryAuthorRows,
  isPersistenceRequiredError,
  libraryAuthorKey,
  libraryAuthorVisibleForFilter,
  libraryBookVisibleForFilter,
  libraryErrorMessage,
  libraryPresenceRank,
  presenceLabel,
  presenceTone,
  summarizeWantedItems,
  wantedPresenceMap,
  type LibraryFormatFilter
} from "./lib";
import "./library.css";

const BOOK_ROW_CAP = 80;

const subtitle = navItems.find((item) => item.id === "library")?.subtitle ?? "Monitored authors and books";

const formatOptions: { value: LibraryFormatFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "ebook", label: "Ebook" },
  { value: "audiobook", label: "Audiobook" }
];

export default function LibraryPage() {
  const navigate = useNavigate();
  const toast = useToast();

  const wanted = useWanted();
  const files = useLibraryFiles("any");
  const subscriptions = useAuthorSubscriptions();
  const librarySettings = useLibrarySettings();

  const [textFilter, setTextFilter] = useState("");
  const [formatFilter, setFormatFilter] = useState<LibraryFormatFilter>("all");
  const [selectedAuthorKey, setSelectedAuthorKey] = useState("");

  const wantedItems = useMemo(() => wanted.data ?? [], [wanted.data]);
  const libraryFiles = useMemo(() => files.data ?? [], [files.data]);
  const authorSubscriptions = useMemo(() => subscriptions.data ?? [], [subscriptions.data]);

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
      grabbed: wantedSummary.grabbed,
      present: wantedSummary.present,
      files: libraryFiles.length,
      manualOverrides: wantedItems.reduce((count, item) => count + (item.manualOverrides?.length ?? 0), 0)
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
        .filter((item) => libraryBookVisibleForFilter(item, presence.get(item.id), textFilter, formatFilter))
        .sort((a, b) => {
          const stateDelta = libraryPresenceRank(presence.get(a.id)) - libraryPresenceRank(presence.get(b.id));
          if (stateDelta !== 0) return stateDelta;
          return `${a.authorName ?? ""} ${a.title}`.localeCompare(`${b.authorName ?? ""} ${b.title}`);
        }),
    [wantedItems, presence, textFilter, formatFilter]
  );
  const authorScopedBooks = useMemo(
    () =>
      selectedAuthorKey
        ? visibleBooks.filter((item) => libraryAuthorKey(item.authorName) === selectedAuthorKey)
        : visibleBooks,
    [visibleBooks, selectedAuthorKey]
  );
  const shownBooks = authorScopedBooks.slice(0, BOOK_ROW_CAP);
  const selectedAuthorRow = selectedAuthorKey ? authorRows.find((row) => row.key === selectedAuthorKey) : undefined;

  const filtersActive = Boolean(textFilter.trim()) || formatFilter !== "all";

  const monitorMutation = useInvalidatingMutation(
    (force: boolean) => runAuthorMonitor({ force }),
    [keys.wanted, keys.authorSubscriptions, keys.authorMetadataReviews, keys.acquisitionQueue]
  );
  const searchMutation = useInvalidatingMutation(
    (item: WantedItem) => searchWantedReleases(item.id, librarySettings.data?.settings.standardSearchLanguage || "English"),
    [keys.wanted, keys.acquisitionQueue]
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

  function openWantedItem(item: WantedItem) {
    navigate(`/wanted?item=${encodeURIComponent(item.id)}`);
  }

  function clearFilters() {
    setTextFilter("");
    setFormatFilter("all");
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
            { label: "Grabbed", value: summary.grabbed },
            { label: "Present", value: summary.present, tone: summary.present > 0 ? "success" : "neutral" },
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
        </div>

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
                              {row.missing} missing · {row.present} present · {row.grabbed} grabbed
                            </span>
                            {row.subscriptionCount > 0 ? (
                              <span className="cell-muted">
                                {row.lastSyncAt ? `Synced ${formatDateTime(row.lastSyncAt)}` : "Never synced"}
                              </span>
                            ) : null}
                          </button>
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

          <Card title="Monitored books" subtitle={booksSubtitle} padded={false}>
            {wanted.isLoading ? (
              <LoadingRow label="Loading monitored books…" />
            ) : shownBooks.length ? (
              <DataTable className="library-book-table">
                <thead>
                  <tr>
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
                    const searchingThis = searchMutation.isPending && searchMutation.variables?.id === item.id;
                    const overrideCount = item.manualOverrides?.length ?? 0;
                    return (
                      <tr key={item.id}>
                        <td className="library-cover-cell">
                          {item.coverUrl ? (
                            <img className="library-cover" src={item.coverUrl} alt="" loading="lazy" />
                          ) : (
                            <span className="library-cover cell-muted">
                              <BookOpen size={14} aria-hidden />
                            </span>
                          )}
                        </td>
                        <td>
                          <div className="library-title-cell">
                            <span className="cell-primary">{item.title}</span>
                            <span className="cell-muted">
                              {item.sourceProvider || "manual"} · {item.monitored ? "monitored" : "unmonitored"} ·{" "}
                              {item.qualityProfile}
                              {overrideCount ? ` · ${overrideCount} override${overrideCount === 1 ? "" : "s"}` : ""}
                            </span>
                          </div>
                        </td>
                        <td>{item.authorName || "Unknown author"}</td>
                        <td>
                          <Badge>{item.format}</Badge>
                        </td>
                        <td>
                          <Badge tone={presenceTone(state)}>{presenceLabel(state)}</Badge>
                        </td>
                        <td>
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
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </DataTable>
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
    </>
  );
}
