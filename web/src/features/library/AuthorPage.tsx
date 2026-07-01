import React, { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, BookOpen, FileSearch, Pencil, RefreshCw, Trash2, Users } from "lucide-react";
import {
  deleteAuthorSubscription,
  runAuthorMonitor,
  searchWantedReleases,
  updateAuthorSubscription,
  updateWanted
} from "../../lib/api";
import type { AuthorMissingBookPolicy, AuthorSubscription, WantedItem } from "../../lib/api";
import {
  keys,
  useAuthorSubscriptions,
  useLibraryFiles,
  useLibrarySettings,
  useWanted
} from "../../lib/queries";
import { formatDate, formatDateTime } from "../../lib/format";
import { useToast } from "../../components/toast";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  InlineNotice,
  LoadingRow,
  Modal,
  PageHeader,
  StatBar,
  ToolbarButton
} from "../../components/ui";
import {
  authorMissingPolicyLabel,
  authorMissingPolicyOptions,
  authorMonitorRunSummary,
  normalizedAuthorMissingPolicy
} from "../wanted/lib";
import {
  buildLibraryAuthorRows,
  libraryAuthorKey,
  libraryBookPath,
  libraryErrorMessage,
  presenceLabel,
  presenceTone,
  summarizeWantedItems,
  wantedPresenceMap
} from "./lib";
import "./library.css";

/**
 * Author detail page (route: /library/author/:authorId). The param is the
 * normalized library author key (see libraryAuthorKey); subscription IDs are
 * also accepted so deep links from subscription records resolve. Authors that
 * exist only through wanted items render without subscription controls.
 */
export default function AuthorPage() {
  const { authorId = "" } = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const client = useQueryClient();

  const wanted = useWanted();
  const files = useLibraryFiles("any");
  const subscriptions = useAuthorSubscriptions();
  const librarySettings = useLibrarySettings();

  const wantedItems = useMemo(() => wanted.data ?? [], [wanted.data]);
  const libraryFiles = useMemo(() => files.data ?? [], [files.data]);
  const allSubscriptions = useMemo(() => subscriptions.data ?? [], [subscriptions.data]);

  // Resolve the author key: prefer a subscription-ID match, else treat the
  // param as the normalized author key.
  const authorKey = useMemo(() => {
    const byID = allSubscriptions.find((subscription) => subscription.id === authorId);
    return byID ? libraryAuthorKey(byID.authorName) : authorId;
  }, [allSubscriptions, authorId]);

  const authorSubscriptions = useMemo(
    () => allSubscriptions.filter((subscription) => libraryAuthorKey(subscription.authorName) === authorKey),
    [allSubscriptions, authorKey]
  );
  const books = useMemo(
    () =>
      wantedItems
        .filter((item) => libraryAuthorKey(item.authorName) === authorKey)
        .sort((a, b) => a.title.localeCompare(b.title)),
    [wantedItems, authorKey]
  );

  const presence = useMemo(() => wantedPresenceMap(wantedItems, libraryFiles), [wantedItems, libraryFiles]);
  const stats = useMemo(() => summarizeWantedItems(books, presence), [books, presence]);
  const authorRow = useMemo(
    () => buildLibraryAuthorRows(allSubscriptions, wantedItems, presence).find((row) => row.key === authorKey),
    [allSubscriptions, wantedItems, presence, authorKey]
  );

  const authorName =
    authorSubscriptions[0]?.authorName || books[0]?.authorName?.trim() || authorRow?.authorName || "Unknown author";
  const monitoredCount = books.filter((item) => item.monitored).length;
  const searchLanguage = librarySettings.data?.settings.standardSearchLanguage || "English";

  const [isSearchingAll, setIsSearchingAll] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [confirmingUnsubscribe, setConfirmingUnsubscribe] = useState(false);
  const [isUnsubscribing, setIsUnsubscribing] = useState(false);
  const [updatingSubscriptionID, setUpdatingSubscriptionID] = useState("");
  const [togglingBookID, setTogglingBookID] = useState("");
  const [searchingBookID, setSearchingBookID] = useState("");

  const isLoading = wanted.isLoading || subscriptions.isLoading;
  const notFound = !isLoading && books.length === 0 && authorSubscriptions.length === 0;

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  async function searchAuthorBooks() {
    const targets = books.filter((item) => item.monitored);
    if (!targets.length) {
      toast.notify(`No monitored books to search for ${authorName}`, "info");
      return;
    }
    setIsSearchingAll(true);
    let found = 0;
    let approved = 0;
    let errors = 0;
    for (const item of targets) {
      try {
        const outcome = await searchWantedReleases(item.id, searchLanguage);
        found += outcome.releases.length;
        approved += outcome.releases.filter((release) => release.approved).length;
      } catch {
        errors += 1;
      }
    }
    const summary = `Searched ${targets.length} book${targets.length === 1 ? "" : "s"}: ${found} release${found === 1 ? "" : "s"} found · ${approved} approved`;
    if (errors > 0) {
      toast.notify(`${summary} · ${errors} error${errors === 1 ? "" : "s"}`, "warn");
    } else {
      toast.success(summary);
    }
    await invalidate(keys.wanted, keys.acquisitionQueue);
    setIsSearchingAll(false);
  }

  async function refreshAuthor() {
    if (!authorSubscriptions.length) return;
    setIsRefreshing(true);
    try {
      const run = await runAuthorMonitor({
        authorIds: authorSubscriptions.map((subscription) => subscription.id).filter(Boolean),
        providerKeys: authorSubscriptions.map((subscription) => subscription.providerKey).filter(Boolean),
        force: true
      });
      toast.success(authorMonitorRunSummary(run));
      await invalidate(keys.authorSubscriptions, keys.authorMetadataReviews, keys.wanted, keys.acquisitionQueue, keys.history());
    } catch (error) {
      toast.error(libraryErrorMessage(error));
    } finally {
      setIsRefreshing(false);
    }
  }

  async function updateMissingPolicy(subscription: AuthorSubscription, missingBookPolicy: AuthorMissingBookPolicy) {
    if (!subscription.id || subscription.missingBookPolicy === missingBookPolicy) return;
    setUpdatingSubscriptionID(subscription.id);
    try {
      const updated = await updateAuthorSubscription(subscription.id, { missingBookPolicy });
      toast.success(`${updated.authorName}: missing-book policy set to ${authorMissingPolicyLabel(missingBookPolicy)}`);
      await invalidate(keys.authorSubscriptions);
    } catch (error) {
      toast.error(libraryErrorMessage(error));
    } finally {
      setUpdatingSubscriptionID("");
    }
  }

  async function unsubscribeAuthor() {
    const targets = authorSubscriptions.filter((subscription) => subscription.id);
    if (!targets.length) return;
    setIsUnsubscribing(true);
    try {
      for (const subscription of targets) {
        await deleteAuthorSubscription(subscription.id);
      }
      setConfirmingUnsubscribe(false);
      toast.success(`Unsubscribed from ${authorName}`);
      await invalidate(keys.authorSubscriptions, keys.authorMetadataReviews);
    } catch (error) {
      toast.error(libraryErrorMessage(error));
    } finally {
      setIsUnsubscribing(false);
    }
  }

  async function toggleMonitored(item: WantedItem) {
    setTogglingBookID(item.id);
    try {
      const updated = await updateWanted(item.id, { monitored: !item.monitored });
      toast.success(`${updated.title}: ${updated.monitored ? "monitored" : "unmonitored"}`);
      await invalidate(keys.wanted, keys.acquisitionQueue);
    } catch (error) {
      toast.error(libraryErrorMessage(error));
    } finally {
      setTogglingBookID("");
    }
  }

  async function searchBook(item: WantedItem) {
    setSearchingBookID(item.id);
    try {
      const outcome = await searchWantedReleases(item.id, searchLanguage);
      const found = outcome.releases.length;
      const approved = outcome.releases.filter((release) => release.approved).length;
      if (found > 0) {
        toast.success(`Found ${found} release${found === 1 ? "" : "s"} (${approved} approved) for “${item.title}”`);
      } else {
        toast.notify(`No releases found for “${item.title}”`, "info");
      }
      await invalidate(keys.wanted, keys.acquisitionQueue);
    } catch (error) {
      toast.error(libraryErrorMessage(error));
    } finally {
      setSearchingBookID("");
    }
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

  const profileLine = authorRow
    ? `${authorRow.formats.join(", ") || "any"} · ${authorRow.qualityProfiles.join(", ") || "standard"}`
    : "";

  return (
    <>
      <PageHeader
        title={authorName}
        subtitle={profileLine || "Author detail"}
        actions={
          <>
            <ToolbarButton icon={ArrowLeft} label="Library" title="Back to Library" onClick={() => navigate("/library")} />
            <ToolbarButton
              icon={FileSearch}
              label="Search Author's Books"
              title="Search indexers for every monitored book by this author"
              busy={isSearchingAll}
              disabled={isSearchingAll || books.length === 0}
              onClick={() => void searchAuthorBooks()}
            />
            {authorSubscriptions.length ? (
              <>
                <ToolbarButton
                  icon={RefreshCw}
                  label="Refresh Author"
                  title="Force-refresh this author's subscriptions for new or missing books"
                  busy={isRefreshing}
                  disabled={isRefreshing}
                  onClick={() => void refreshAuthor()}
                />
                <ToolbarButton
                  icon={Trash2}
                  label="Unsubscribe"
                  tone="danger"
                  title="Stop monitoring this author"
                  disabled={isUnsubscribing}
                  onClick={() => setConfirmingUnsubscribe(true)}
                />
              </>
            ) : null}
          </>
        }
      />
      <div className="library-page">
        {noticeMessages.map((message) => (
          <InlineNotice key={message} tone="danger">
            {message}
          </InlineNotice>
        ))}

        {isLoading ? (
          <LoadingRow label="Loading author…" />
        ) : notFound ? (
          <Card>
            <EmptyState
              icon={Users}
              title="Author not found"
              actions={
                <Button size="sm" variant="primary" onClick={() => navigate("/library")}>
                  Back to Library
                </Button>
              }
            >
              No subscription or monitored books match this author. It may have been removed or renamed.
            </EmptyState>
          </Card>
        ) : (
          <>
            <StatBar
              stats={[
                { label: "Books", value: books.length },
                { label: "Monitored", value: monitoredCount },
                { label: "Missing", value: stats.missing, tone: stats.missing > 0 ? "danger" : "neutral" },
                { label: "Grabbed", value: stats.grabbed, tone: stats.grabbed > 0 ? "info" : "neutral" },
                { label: "Present", value: stats.present, tone: stats.present > 0 ? "success" : "neutral" }
              ]}
            />

            {authorSubscriptions.length ? (
              <Card title="Subscription" subtitle="Monitor policy per subscribed format" padded={false}>
                <div className="library-subscription-list">
                  {authorSubscriptions.map((subscription) => {
                    const policy = normalizedAuthorMissingPolicy(subscription.missingBookPolicy);
                    return (
                      <article className="library-subscription-row" key={subscription.id || subscription.providerKey}>
                        <div className="library-subscription-main">
                          <strong>
                            {subscription.provider} · {subscription.format} · {subscription.qualityProfile}
                          </strong>
                          <span>
                            {subscription.lastSyncAt
                              ? `Synced ${formatDateTime(subscription.lastSyncAt)}`
                              : "Never synced"}
                          </span>
                        </div>
                        <Badge tone={subscription.monitorNewItems ? "success" : "neutral"}>
                          {subscription.monitorNewItems ? "Monitoring new" : "Existing only"}
                        </Badge>
                        <select
                          aria-label={`${subscription.authorName} ${subscription.format} missing-book policy`}
                          disabled={updatingSubscriptionID === subscription.id}
                          onChange={(event) =>
                            void updateMissingPolicy(subscription, event.target.value as AuthorMissingBookPolicy)
                          }
                          value={policy}
                        >
                          {authorMissingPolicyOptions.map((option) => (
                            <option key={option} value={option}>
                              {authorMissingPolicyLabel(option)}
                            </option>
                          ))}
                        </select>
                      </article>
                    );
                  })}
                </div>
              </Card>
            ) : (
              <InlineNotice tone="info">
                No author subscription — these books are tracked individually. Subscribe from Add New to monitor this
                author for new or missing books.
              </InlineNotice>
            )}

            <Card title="Books" subtitle={`${books.length} tracked book${books.length === 1 ? "" : "s"}`} padded={false}>
              {books.length ? (
                <DataTable className="library-book-table">
                  <thead>
                    <tr>
                      <th className="library-cover-cell" aria-label="Cover" />
                      <th>Title</th>
                      <th>Added</th>
                      <th>Format</th>
                      <th>Status</th>
                      <th>Monitored</th>
                      <th aria-label="Actions" />
                    </tr>
                  </thead>
                  <tbody>
                    {books.map((item) => {
                      const state = presence.get(item.id) ?? "missing";
                      return (
                        <tr key={item.id}>
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
                                {item.sourceProvider || "manual"} · {item.qualityProfile}
                              </span>
                            </div>
                          </td>
                          <td>{formatDate(item.createdAt)}</td>
                          <td>
                            <Badge>{item.format}</Badge>
                          </td>
                          <td>
                            <Badge tone={presenceTone(state)}>{presenceLabel(state)}</Badge>
                          </td>
                          <td>
                            <label className="library-monitor-toggle" title={item.monitored ? "Unmonitor" : "Monitor"}>
                              <input
                                type="checkbox"
                                checked={item.monitored}
                                disabled={togglingBookID === item.id}
                                onChange={() => void toggleMonitored(item)}
                                aria-label={`${item.title} monitored`}
                              />
                            </label>
                          </td>
                          <td>
                            <div className="cell-actions">
                              <Button
                                size="sm"
                                icon={FileSearch}
                                busy={searchingBookID === item.id}
                                disabled={Boolean(searchingBookID) || isSearchingAll}
                                onClick={() => void searchBook(item)}
                                title="Search indexers for releases"
                              >
                                {searchingBookID === item.id ? "Searching" : "Search"}
                              </Button>
                              <Button
                                size="sm"
                                icon={Pencil}
                                onClick={() => navigate(libraryBookPath(item.id))}
                                title="Review this book's metadata and releases"
                              >
                                Review
                              </Button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </DataTable>
              ) : (
                <EmptyState icon={BookOpen} title="No tracked books for this author">
                  Refresh the author subscription (or mark books wanted from Add New) to start tracking books.
                </EmptyState>
              )}
            </Card>
          </>
        )}
      </div>

      <Modal
        title="Unsubscribe author"
        open={confirmingUnsubscribe}
        onClose={() => setConfirmingUnsubscribe(false)}
        footer={
          <>
            <Button onClick={() => setConfirmingUnsubscribe(false)}>Cancel</Button>
            <Button variant="danger" icon={Trash2} busy={isUnsubscribing} onClick={() => void unsubscribeAuthor()}>
              {isUnsubscribing ? "Removing" : "Unsubscribe"}
            </Button>
          </>
        }
      >
        <p>
          Stop monitoring <strong>{authorName}</strong>
          {authorSubscriptions.length > 1 ? ` (${authorSubscriptions.length} subscriptions)` : ""}? Pending metadata
          reviews for this author are discarded; existing wanted books are kept.
        </p>
      </Modal>
    </>
  );
}
