import { useQuery, useQueryClient } from "@tanstack/react-query";
import React, { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  BookOpen,
  Download,
  FileSearch,
  FilterX,
  HardDriveDownload,
  Search,
  SlidersHorizontal,
  UserPlus
} from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  Field,
  InlineNotice,
  LoadingRow,
  Modal,
  PageHeader,
  Segmented
} from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  createWanted,
  fetchBookMatches,
  type BookMatchCandidate,
  type BookMatch,
  grabRelease,
  grabWanted,
  searchMetadataDetailed,
  searchReleases,
  searchWantedReleases,
  subscribeAuthor,
  runAuthorMonitor,
  type AuthorMissingBookPolicy,
  type AuthorSubscription,
  type DownloadStatus,
  type Release,
  type SearchResult,
  type WantedItem
} from "../../lib/api";
import {
  keys,
  useAuthorSubscriptions,
  useInvalidatingMutation,
  useLibrarySettings,
  useMetadataProfiles,
  useQualityProfiles,
  useRootFolders,
  useTags
} from "../../lib/queries";
import { demoModeEnabled, demoSeeds, withDemoFallback } from "../../lib/demo";
import { formatBytes } from "../../lib/format";
import { navItems } from "../../app/nav";
import {
  authorMissingPolicyLabel,
  authorMissingPolicyOptions,
  chipTone,
  compactStringList,
  firstAuthorName,
  groupSearchEditions,
  searchEditionOptionLabel,
  languageLabel,
  searchFormatOptions,
  searchModeOptions,
  searchResultCanBeWanted,
  searchResultCover,
  searchResultEvidenceSummary,
  searchResultExistingWanted,
  searchResultIdentifierLabel,
  searchResultKey,
  searchResultMatchChips,
  searchResultNeedsWantedReview,
  searchResultProviderKey,
  searchResultSeriesLabel,
  searchResultSourceLabel,
  searchResultSourceNames,
  searchResultSubtitle,
  searchResultTitle,
  searchResultVisibleForFilters,
  searchResultWantedFormat,
  searchResultWantedSourceKey,
  searchResultWantedReviewReasons,
  uniqueSearchProviders,
  wantedFormat,
  type SearchEvidenceFilter,
  type SearchMode
} from "./lib";
import "./search.css";

const searchNav = navItems.find((item) => item.id === "search");

/** Desktop breakpoint mirrors the .search-layout media query in search.css. */
function useIsDesktop() {
  const [isDesktop, setIsDesktop] = useState(
    () => typeof window !== "undefined" && window.matchMedia("(min-width: 1100px)").matches
  );
  useEffect(() => {
    const media = window.matchMedia("(min-width: 1100px)");
    const onChange = (event: MediaQueryListEvent) => setIsDesktop(event.matches);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);
  return isDesktop;
}

export default function SearchPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const isDesktop = useIsDesktop();
  const [searchParams, setSearchParams] = useSearchParams();

  // --- URL contract: read ?query= and &mode= once on mount. -----------------
  const initialModeRef = useRef<SearchMode>(searchParams.get("mode") === "author" ? "author" : searchParams.get("mode") === "series" ? "series" : "book");
  const initialQueryRef = useRef(searchParams.get("query") ?? "");

  const [mode, setMode] = useState<SearchMode>(initialModeRef.current);
  const [bookQuery, setBookQuery] = useState(() => {
    if (initialModeRef.current !== "author" && initialQueryRef.current) return initialQueryRef.current;
    return demoModeEnabled ? "Project Hail Mary" : "";
  });
  const [authorQuery, setAuthorQuery] = useState(() => {
    if (initialModeRef.current === "author" && initialQueryRef.current) return initialQueryRef.current;
    return demoModeEnabled ? "Andy Weir" : "";
  });
  const [format, setFormat] = useState<string>("any");

  const [results, setResults] = useState<SearchResult[]>([]);
  const [hasSearched, setHasSearched] = useState(false);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState("");
  const [selectedKey, setSelectedKey] = useState("");
  const [detailOpen, setDetailOpen] = useState(false);

  const [filtersOpen, setFiltersOpen] = useState(false);
  const [providerFilter, setProviderFilter] = useState("");
  const [evidenceFilter, setEvidenceFilter] = useState<SearchEvidenceFilter>("all");

  const [pendingReview, setPendingReview] = useState<SearchResult | null>(null);
  const [authorPolicy, setAuthorPolicy] = useState<AuthorMissingBookPolicy>("all");
  const [authorMetadataProfileID, setAuthorMetadataProfileID] = useState("");
  const [qualityProfile, setQualityProfile] = useState("standard");
  // "" = use the format's default root folder; ids are validated per format.
  const [rootFolderID, setRootFolderID] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [pendingDownload, setPendingDownload] = useState(false);
  const [downloadPhase, setDownloadPhase] = useState("");
  const addingRef = useRef(false);

  const [releases, setReleases] = useState<Release[]>([]);
  const [releasesSearched, setReleasesSearched] = useState(false);
  const [isSearchingReleases, setIsSearchingReleases] = useState(false);
  const [releaseError, setReleaseError] = useState("");
  const [downloadStatus, setDownloadStatus] = useState<DownloadStatus | null>(null);

  // --- Shared data -----------------------------------------------------------
  const librarySettings = useLibrarySettings();
  const language = librarySettings.data?.settings.standardSearchLanguage || "English";
  const authorSubscriptions = useAuthorSubscriptions().data ?? [];
  const qualityProfiles = useQualityProfiles().data ?? [];
  const rootFolders = useRootFolders().data ?? [];
  const knownTags = useTags().data ?? [];
  const metadataProfiles = useMetadataProfiles().data ?? [];

  // --- Derived state ---------------------------------------------------------
  const query = mode === "author" ? authorQuery : bookQuery;
  const providerOptions = useMemo(() => uniqueSearchProviders(results), [results]);
  const visibleResults = useMemo(
    () =>
      results.filter((result) =>
        searchResultVisibleForFilters(result, {
          provider: providerFilter,
          confidence: "all",
          evidence: evidenceFilter
        })
      ),
    [results, providerFilter, evidenceFilter]
  );
  const editionGroups = useMemo(() => groupSearchEditions(visibleResults), [visibleResults]);
  const activeFilterCount = [
    providerFilter,
    evidenceFilter !== "all" ? evidenceFilter : ""
  ].filter(Boolean).length;

  const matchCandidates = useMemo(() => Array.from(new Map(results.filter(searchResultCanBeWanted).map(result => {
    const candidate: BookMatchCandidate = { key: searchResultKey(result), provider: result.provider,
      workIds: [result.work.id, ...(result.work.providerIds ?? [])].filter(Boolean),
      editionIds: [result.edition?.id ?? "", ...(result.edition?.providerIds ?? [])].filter(Boolean),
      sourceKey: searchResultWantedSourceKey(result), format: searchResultWantedFormat(result, format) };
    return [candidate.key, candidate] as const;
  })).values()), [results, format]);
  const bookMatches = useQuery({
    queryKey: [...keys.wanted, "search-identities", matchCandidates],
    enabled: matchCandidates.length > 0,
    queryFn: ({ signal }) => withDemoFallback(() => fetchBookMatches(matchCandidates, signal), () =>
      matchCandidates.map(candidate => {
        const result = results.find(result => searchResultKey(result) === candidate.key)!;
        const book = searchResultExistingWanted(result, demoSeeds.wantedItems, format);
        return { key: candidate.key, total: book ? 1 : 0, books: book ? [book] : [] };
      }))(),
    refetchInterval: 30_000
  });
  const matchesReady = bookMatches.isSuccess && !bookMatches.isError;
  const wantedBySearchKey = useMemo(() => new Map<string, BookMatch>(
    matchesReady ? bookMatches.data.map(match => [match.key, match]) : []
  ), [matchesReady, bookMatches.data]);
  function trackingLabel(match: BookMatch) {
    if (match.total > 1) return `${match.total} saved matches`;
    const status = match.books[0]?.status;
    return status === "removed" ? "Removed" : status === "ignored" ? "Ignored" : "Tracked";
  }

  const selected = useMemo(
    () =>
      visibleResults.find((result) => searchResultKey(result) === selectedKey || result.work.id === selectedKey) ??
      visibleResults[0] ??
      results[0],
    [visibleResults, results, selectedKey]
  );
  const selectedSearchKey = selected ? searchResultKey(selected) : "";
  const selectedIsBookCandidate = Boolean(selected && searchResultCanBeWanted(selected));
  const selectedCanSearchReleases = selectedIsBookCandidate;

  const selectedAuthorFormat = selected ? searchResultWantedFormat(selected, format) : wantedFormat(format);
  const selectedAuthorSubscription = useMemo(() => {
    const author = selected?.work.authors?.[0];
    if (!author) return undefined;
    const authorID = author.id.trim().toLowerCase();
    const authorName = author.name.trim().toLowerCase();
    return authorSubscriptions.find((subscription) => {
      if (subscription.format !== selectedAuthorFormat) return false;
      const providerKey = subscription.providerKey.trim().toLowerCase();
      const subscriptionName = subscription.authorName.trim().toLowerCase();
      return authorID ? providerKey === authorID : Boolean(authorName && subscriptionName === authorName && subscription.provider === selected?.provider);
    });
  }, [authorSubscriptions, selected, selectedAuthorFormat]);

  const selectedWantedFormat = selected ? searchResultWantedFormat(selected, format) : wantedFormat(format);
  const profileOptions = useMemo(
    () =>
      qualityProfiles.filter(
        (profile) => profile.mediaFormat === "any" || profile.mediaFormat === selectedWantedFormat
      ),
    [qualityProfiles, selectedWantedFormat]
  );
  const effectiveProfile = profileOptions.some((profile) => profile.name === qualityProfile)
    ? qualityProfile
    : profileOptions[0]?.name ?? "standard";

  // Root folders matching the target format; default = the format's default root.
  const rootFolderOptions = useMemo(
    () => rootFolders.filter((folder) => folder.mediaFormat === selectedWantedFormat),
    [rootFolders, selectedWantedFormat]
  );
  const defaultRootFolder = rootFolderOptions.find((folder) => folder.isDefault) ?? rootFolderOptions[0];
  const effectiveRootFolderID = rootFolderOptions.some((folder) => folder.id === rootFolderID)
    ? rootFolderID
    : defaultRootFolder?.id ?? "";

  /** Free-text comma tag entry → labels array for the create payload. */
  const tagLabels = useMemo(
    () =>
      tagsInput
        .split(",")
        .map((label) => label.trim())
        .filter(Boolean),
    [tagsInput]
  );

  // Legacy behavior: drop a provider filter that no longer matches any result.
  useEffect(() => {
    if (providerFilter && !providerOptions.includes(providerFilter)) {
      setProviderFilter("");
    }
  }, [providerFilter, providerOptions]);

  // Legacy behavior: discard a pending review that no longer maps to a result.
  useEffect(() => {
    if (pendingReview && !results.some((result) => searchResultKey(result) === searchResultKey(pendingReview))) {
      setPendingReview(null);
    }
  }, [pendingReview, results]);

  // --- Search ---------------------------------------------------------------
  async function runSearch(overrides: { mode?: SearchMode; query?: string } = {}) {
    const activeMode = overrides.mode ?? mode;
    const activeQuery = (overrides.query ?? (activeMode === "author" ? authorQuery : bookQuery)).trim();
    if (!activeQuery) return;
    setIsSearching(true);
    setSearchError("");
    setSearchParams({ query: activeQuery, mode: activeMode }, { replace: true });
    try {
      const fetcher = withDemoFallback(
        () => searchMetadataDetailed(activeQuery, activeMode === "author" ? "any" : format, activeMode, language),
        () => ({ results: demoSeeds.results, providerErrors: [] })
      );
      const outcome = await fetcher();
      const nextResults = outcome.results;
      if (outcome.providerErrors.length) setSearchError(outcome.providerErrors.map(error => `${error.provider}: ${error.message}`).join(" · "));
      setResults(nextResults);
      setSelectedKey(nextResults[0] ? searchResultKey(nextResults[0]) : "");
      setPendingReview(null);
      setReleases([]);
      setReleasesSearched(false);
      setReleaseError("");
      setDownloadStatus(null);
    } catch (error) {
      setResults([]);
      setSelectedKey("");
      setSearchError(error instanceof Error ? error.message : "Metadata search failed");
    } finally {
      setHasSearched(true);
      setIsSearching(false);
    }
  }

  // URL contract: auto-run when a query arrived via ?query= (or the demo prefill).
  const autoRanRef = useRef(false);
  useEffect(() => {
    if (autoRanRef.current) return;
    autoRanRef.current = true;
    const initialQuery = initialModeRef.current === "author" ? authorQuery : bookQuery;
    if (initialQuery.trim() && (initialQueryRef.current || demoModeEnabled)) {
      void runSearch({ mode: initialModeRef.current, query: initialQuery });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function switchMode(nextMode: SearchMode) {
    if (nextMode === mode) return;
    setMode(nextMode);
    setResults([]);
    setHasSearched(false);
    setSearchError("");
    setSelectedKey("");
    setDetailOpen(false);
    setPendingReview(null);
    setReleases([]);
    setReleasesSearched(false);
    setReleaseError("");
    setDownloadStatus(null);
    clearFilters();
  }

  function clearFilters() {
    setProviderFilter("");
    setEvidenceFilter("all");
  }

  function selectResult(result: SearchResult) {
    setSelectedKey(searchResultKey(result));
    if (pendingReview && searchResultKey(pendingReview) !== searchResultKey(result)) {
      setPendingReview(null);
    }
    if (!isDesktop) setDetailOpen(true);
  }

  function openWanted(item: WantedItem) {
    navigate(`/library/book/${encodeURIComponent(item.id)}`);
  }

  // --- Mutations --------------------------------------------------------------
  const addWanted = useInvalidatingMutation(
    (args: { result: SearchResult; format: string; profile: string; tags: string[]; rootFolderId?: string }) =>
      createWanted(args.result, args.format, args.profile, args.tags, args.rootFolderId, true),
    [keys.wanted, keys.acquisitionQueue]
  );

  const monitorAuthor = useInvalidatingMutation(
    async (args: { result: SearchResult; format: string; policy: AuthorMissingBookPolicy; metadataProfileId?: string; profile: string; rootFolderId?: string; tags: string[]; existing?: AuthorSubscription }) => {
      const subscription = args.existing ?? await subscribeAuthor(args.result, args.format, args.profile, args.policy, args.metadataProfileId, args.rootFolderId, args.tags);
      let refreshError = "";
      if (subscription.monitorNewItems && subscription.missingBookPolicy !== "none") {
        try {
          const run = await runAuthorMonitor({ authorIds: [subscription.id], force: true });
          if (run.errorCount) refreshError = run.items?.find(item => item.error)?.error || `Refresh reported ${run.errorCount} error(s).`;
        } catch (error) {
          refreshError = error instanceof Error ? error.message : "Author refresh failed";
        }
      }
      return { subscription, refreshError };
    },
    [keys.authorSubscriptions, keys.wanted, keys.authorMetadataReviews]
  );

  const grab = useInvalidatingMutation(
    (args: { release: Release; format: string }) => grabRelease(args.release, args.format),
    [keys.downloads()]
  );

  async function requestAddBook(result: SearchResult, options: { force?: boolean; download?: boolean } = {}) {
    if (!searchResultCanBeWanted(result) || addingRef.current) return;
    const key = searchResultKey(result);
    const existing = wantedBySearchKey.get(key);
    if (!matchesReady || !existing) return;
    if (existing.total) {
      setPendingReview(null);
      if (existing.total === 1) openWanted(existing.books[0]);
      return;
    }
    if (!options.force && searchResultNeedsWantedReview(result)) {
      setSelectedKey(key);
      setPendingDownload(Boolean(options.download));
      setPendingReview(result);
      return;
    }
    addingRef.current = true;
    setSelectedKey(key);
    setDownloadPhase("Adding");
    let item: WantedItem | undefined;
    try {
      item = await addWanted.mutateAsync({
        result,
        format: searchResultWantedFormat(result, format),
        profile: effectiveProfile,
        tags: tagLabels,
        rootFolderId: effectiveRootFolderID || undefined
      });
      setPendingReview(null);
      if (options.download) {
        setDownloadPhase("Finding download");
        const outcome = await searchWantedReleases(item.id, language);
        // Search returns ranked, persisted decisions. Grab only a fresh approved
        // decision; the server still requires approval on grab.
        const best = outcome.releases.find(release => release.approved && release.id);
        if (best) {
          setDownloadPhase("Starting download");
          await grabWanted(item.id, best.id, { paused: false, force: false });
          toast.success(`Download queued: ${item.title}`);
        } else {
          toast.notify(`Saved "${item.title}". No suitable download found yet.`, "info");
        }
      } else {
        toast.success(`Added "${item.title}" to your library.`);
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "Please try again.";
      toast.error(item ? `Saved "${item.title}", but couldn't start the download: ${message}` : message);
    } finally {
      void bookMatches.refetch();
      for (const queryKey of [keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history()]) {
        void queryClient.invalidateQueries({ queryKey });
      }
      addingRef.current = false;
      setDownloadPhase("");
      // A saved book is the recovery destination too: retry there without
      // creating another record or replaying an uncertain download request.
      if (item) openWanted(item);
    }
  }

  function requestMonitorAuthor(result: SearchResult | undefined) {
    if (!result?.work.authors?.[0]) return;
    setSelectedKey(searchResultKey(result));
    monitorAuthor.mutate(
      {
        result,
        format: searchResultWantedFormat(result, format),
        policy: authorPolicy,
        metadataProfileId: authorMetadataProfileID || undefined,
        profile: effectiveProfile,
        rootFolderId: effectiveRootFolderID || undefined,
        tags: tagLabels,
        existing: selectedAuthorSubscription
      },
      {
        onSuccess: ({ subscription, refreshError }) => {
          if (refreshError) {
            toast.notify(`${subscription.authorName} is saved, but the refresh failed: ${refreshError}`, "warn");
          } else {
            toast.success(`Saved ${subscription.authorName} (${authorMissingPolicyLabel(subscription.missingBookPolicy)}, ${subscription.format}).`);
          }
        },
        onError: (error) => {
          toast.error(error instanceof Error ? error.message : "Author subscription failed");
        }
      }
    );
  }

  async function runReleaseSearch() {
    if (!selected || !searchResultCanBeWanted(selected)) return;
    const releaseQuery = selected.work.title || query;
    if (!releaseQuery.trim()) return;
    setIsSearchingReleases(true);
    setReleaseError("");
    try {
      const nextReleases = await searchReleases(releaseQuery, selected.edition?.format ?? format, language);
      setReleases(nextReleases);
    } catch (error) {
      setReleases([]);
      setReleaseError(error instanceof Error ? error.message : "Release search failed");
    } finally {
      setReleasesSearched(true);
      setIsSearchingReleases(false);
    }
  }

  function requestGrab(release: Release) {
    setReleaseError("");
    grab.mutate(
      { release, format: selected?.edition?.format ?? format },
      {
        onSuccess: (status) => {
          setDownloadStatus(status);
          toast.success(`Queued "${release.title}" paused in ${status.category}.`);
        },
        onError: (error) => {
          const message = error instanceof Error ? error.message : "Grab failed";
          setReleaseError(message);
          toast.error(message);
        }
      }
    );
  }

  // --- Renderers --------------------------------------------------------------

  /** Root Folder + Tags fields shared by the add form and the review modal. */
  function renderAddBookFields() {
    return (
      <>
        <Field label="Root folder" hint="Destination for newly added books.">
          <select
            value={effectiveRootFolderID}
            onChange={(event) => setRootFolderID(event.target.value)}
            aria-label="Root folder"
          >
            {rootFolderOptions.length ? (
              rootFolderOptions.map((folder) => (
                <option key={folder.id} value={folder.id}>
                  {folder.name || folder.path}
                  {folder.isDefault ? " (default)" : ""}
                </option>
              ))
            ) : (
              <option value="">Format default</option>
            )}
          </select>
        </Field>
        <Field label="Tags" hint="Comma-separated labels for newly added books.">
          <input
            list="search-tag-suggestions"
            value={tagsInput}
            onChange={(event) => setTagsInput(event.target.value)}
            placeholder="sci-fi, book-club"
            aria-label="Tags"
          />
        </Field>
      </>
    );
  }

  function renderResultRow(result: SearchResult, editionCount = 1) {
    const key = searchResultKey(result);
    const existing = wantedBySearchKey.get(key);
    const sources = searchResultSourceNames(result);
    return (
      <button
        key={key}
        type="button"
        className={key === selectedSearchKey ? "search-result-row selected" : "search-result-row"}
        onClick={() => selectResult(result)}
      >
        <span className="search-result-thumb" aria-hidden>
          <SearchCover key={searchResultCover(result)} result={result} size={20} />
        </span>
        <span className="search-result-main">
          <span className="search-result-title">
            <strong>{searchResultTitle(result)}</strong>
            {result.kind !== "author" && result.work.firstPublishYear ? (
              <small>{result.work.firstPublishYear}</small>
            ) : null}
          </span>
          <span className="search-result-sub">{searchResultSubtitle(result)}</span>
          {result.work.seriesId ? <span className="search-result-sub">{searchResultSeriesLabel(result)}</span> : null}
          <span className="search-result-chips">
            {searchResultMatchChips(result).map((chip) => (
              <Badge key={chip.label} tone={chipTone(chip.tone)}>
                {chip.label}
              </Badge>
            ))}
          </span>
        </span>
        <span className="search-result-side">
          <Badge tone="neutral" title={sources.join(", ")}>
            {searchResultSourceLabel(result)}
          </Badge>
          {editionCount > 1 ? <Badge tone="neutral">{editionCount} editions</Badge> : null}
          {existing?.total ? <Badge tone="neutral">{trackingLabel(existing)}</Badge> : null}
        </span>
      </button>
    );
  }

  function renderDetail(result: SearchResult) {
    const key = searchResultKey(result);
    const existing = wantedBySearchKey.get(key);
    const canBeWanted = searchResultCanBeWanted(result) && !existing?.total;
    const sources = searchResultSourceNames(result);
    const editions = editionGroups.find(group => group.some(candidate => searchResultKey(candidate) === key)) ?? [result];
    const renderOptions = () => (
      <div className="search-detail-form">
        {canBeWanted || !selectedAuthorSubscription ? (
          <>
            <Field label="Quality profile" hint="Applied to newly added books.">
              <select aria-label="Quality profile" value={effectiveProfile} onChange={event => setQualityProfile(event.target.value)}>
                {profileOptions.length ? profileOptions.map(profile => (
                  <option key={profile.name} value={profile.name}>{profile.name}</option>
                )) : <option value="standard">standard</option>}
              </select>
            </Field>
            {renderAddBookFields()}
          </>
        ) : null}
      </div>
    );
    const renderAuthorFields = () => (
      <>
        <Field label="Missing books" hint="Which existing books to add when monitoring this author.">
          <select
            value={selectedAuthorSubscription?.missingBookPolicy ?? authorPolicy}
            disabled={Boolean(selectedAuthorSubscription)}
            onChange={event => setAuthorPolicy(event.target.value as AuthorMissingBookPolicy)}
          >
            {authorMissingPolicyOptions.map(policy => (
              <option key={policy} value={policy}>{authorMissingPolicyLabel(policy)}</option>
            ))}
          </select>
        </Field>
        <Field label="Metadata profile" hint="Filter set applied when monitoring this author.">
          <select
            value={selectedAuthorSubscription?.metadataProfileId ?? authorMetadataProfileID}
            disabled={Boolean(selectedAuthorSubscription)}
            onChange={event => setAuthorMetadataProfileID(event.target.value)}
            aria-label="Author metadata profile"
          >
            <option value="">None</option>
            {metadataProfiles.map(profile => (
              <option key={profile.id} value={profile.id}>{profile.name}</option>
            ))}
          </select>
        </Field>
      </>
    );
    const renderMonitorAuthor = () => (
      <Button
        icon={UserPlus}
        disabled={!result.work.authors?.length}
        busy={monitorAuthor.isPending}
        onClick={() => requestMonitorAuthor(result)}
      >
        {monitorAuthor.isPending ? "Saving" : selectedAuthorSubscription ? "Refresh Author" : "Monitor Author"}
      </Button>
    );
    return (
      <>
        <div className="search-detail-head">
          <span className="search-detail-cover" aria-hidden>
            <SearchCover key={searchResultCover(result)} result={result} size={34} />
          </span>
          <div className="search-detail-head-text">
            <h3>{searchResultTitle(result)}</h3>
            <p>{searchResultSubtitle(result)}</p>
            <div className="search-detail-badges">
              <Badge tone="neutral" title={sources.join(", ")}>
                {searchResultSourceLabel(result)}
              </Badge>
              {existing?.total ? <Badge tone="neutral">{trackingLabel(existing)}</Badge> : null}
            </div>
          </div>
        </div>

        {editions.length > 1 ? (
          <Field label="Edition">
            <select aria-label="Edition" value={key} onChange={event => {
              const edition = editions.find(candidate => searchResultKey(candidate) === event.target.value);
              if (edition) selectResult(edition);
            }}>
              {editions.map(edition => <option key={searchResultKey(edition)} value={searchResultKey(edition)}>{searchEditionOptionLabel(edition)}</option>)}
            </select>
          </Field>
        ) : null}

        {searchResultCanBeWanted(result) && !matchesReady ? (
          bookMatches.isError ? <InlineNotice tone="danger">
            <strong>Library check unavailable. </strong>
            {bookMatches.error instanceof Error ? bookMatches.error.message : "Retry before adding this book."}
            <Button size="sm" onClick={() => void bookMatches.refetch()}>Retry library check</Button>
          </InlineNotice> : <LoadingRow label="Checking saved book identities…" />
        ) : existing?.total ? (
          <div className="search-tracked-callout" aria-label="Saved book matches">
            <Badge tone="neutral">{trackingLabel(existing)}</Badge>
            <span>Matched by saved provider identity and format. Open a book to inspect its files or restore tracking.</span>
            {existing.books.map(book => <Button key={book.id} size="sm" icon={HardDriveDownload} onClick={() => openWanted(book)}>
              {book.title} · {book.format} · {book.status} · {book.sourceKey || book.id}
            </Button>)}
            {existing.total > existing.books.length ? <span>Showing {existing.books.length} of {existing.total} saved matches. Resolve duplicates in Library before adding.</span> : null}
          </div>
        ) : null}

        {canBeWanted ? (
          <div className="search-download-summary">
            {result.edition?.format && result.edition.format !== "any" ? (
              <p>{selectedWantedFormat === "audiobook" ? "Audiobook" : "Ebook"}{result.edition.language ? ` · ${languageLabel(result.edition.language)}` : ""}</p>
            ) : (
              <Field label="Download format">
                <select aria-label="Download format" value={selectedWantedFormat} onChange={event => setFormat(event.target.value)}>
                  <option value="ebook">Ebook</option>
                  <option value="audiobook">Audiobook</option>
                </select>
              </Field>
            )}
            <small>Finds and starts the best download that meets your quality settings.</small>
          </div>
        ) : null}
        <div className="search-detail-actions">
          {existing?.total === 1 ? (
            <Button icon={HardDriveDownload} onClick={() => openWanted(existing.books[0])}>Open book</Button>
          ) : canBeWanted ? (
            <>
              <Button variant="primary" icon={Download} busy={Boolean(downloadPhase)} disabled={!matchesReady}
                onClick={() => void requestAddBook(result, { download: true })}>
                {downloadPhase || `Download ${selectedWantedFormat}`}
              </Button>
              <Button disabled={!matchesReady || Boolean(downloadPhase)} onClick={() => void requestAddBook(result)}>
                Add Book
              </Button>
            </>
          ) : null}
        </div>

        {result.kind === "author" ? (
          <>
            {renderOptions()}

            <div className="search-detail-form">
              {renderAuthorFields()}
            </div>
            {renderMonitorAuthor()}
          </>
        ) : (
          <details className="search-disclosure">
            <summary>Options</summary>
            {renderOptions()}

            <Button icon={Search} busy={isSearchingReleases} onClick={() => void runReleaseSearch()}>Search Releases</Button>
            <details className="search-disclosure">
              <summary>Monitor this author</summary>
              <div className="search-detail-form">
                {renderAuthorFields()}
              </div>
              {renderMonitorAuthor()}
            </details>
          </details>
        )}
        <details className="search-disclosure">
          <summary>Book details and sources</summary>
          <div className="search-evidence-grid" aria-label="Selected metadata evidence">
            {searchResultEvidenceSummary(result, format).map((item) => (
              <article className="search-evidence-item" key={item.label}>
                <span>{item.label}</span>
                <strong>{item.value}</strong>
                <small>{item.detail}</small>
              </article>
            ))}
          </div>

          <dl className="search-detail-list">
            <div>
              <dt>Sources</dt>
              <dd>{sources.join(", ") || result.provider}</dd>
            </div>
            {result.kind === "author" ? (
              <div>
                <dt>Provider ID</dt>
                <dd>{searchResultProviderKey(result)}</dd>
              </div>
            ) : (
              <div>
                <dt>First published</dt>
                <dd>{result.work.firstPublishYear ?? "Unknown"}</dd>
              </div>
            )}
            <div>
              <dt>{result.kind === "author" ? "Target format" : "Format"}</dt>
              <dd>{result.kind === "author" ? wantedFormat(format) : result.edition?.format ?? "Any"}</dd>
            </div>
            {result.kind === "author" ? null : (
              <div>
                <dt>Edition</dt>
                <dd>{result.edition?.title || result.work.title}</dd>
              </div>
            )}
            {result.kind === "author" ? null : (
              <div>
                <dt>Language</dt>
                <dd>{languageLabel(result.edition?.language) || "Unknown"}</dd>
              </div>
            )}
            {result.kind === "author" ? null : (
              <div>
                <dt>Published</dt>
                <dd>
                  {compactStringList([result.edition?.publishedDate, result.edition?.publisher]).join(" · ") ||
                    result.work.firstPublishYear ||
                    "Unknown"}
                </dd>
              </div>
            )}
            {result.kind === "author" ? (
              <div>
                <dt>Top work</dt>
                <dd>{result.work.description || "Unknown"}</dd>
              </div>
            ) : (
              <div>
                <dt>Identifiers</dt>
                <dd>{searchResultIdentifierLabel(result, 4)}</dd>
              </div>
            )}
            {result.kind === "author" ? null : (
              <div>
                <dt>Series</dt>
                <dd>{searchResultSeriesLabel(result) || "None"}</dd>
              </div>
            )}
            <div>
              <dt>Matched on</dt>
              <dd>{result.matchedOn.join(", ")}</dd>
            </div>
          </dl>

        </details>
      </>
    );
  }

  const resultsTitle = mode === "series" ? "Series books" : mode === "author" ? "Author identities" : "Search results";
  const resultsSubtitle =
    mode === "author"
      ? `${visibleResults.length} of ${results.length} author records shown.`
      : `${editionGroups.length} ${editionGroups.length === 1 ? "book" : "books"}`;

  return (
    <>
      <PageHeader title="Add New" subtitle={searchNav?.subtitle} />
      {mode === "series" ? <InlineNotice tone="info">Hardcover series order. Up to 3 matching series and 25 books each, excluding collections; at most 50 edition records shown. Missing positions come last. Refine the name to narrow results.</InlineNotice> : null}

      <form
        className="search-hero"
        aria-label="Metadata search controls"
        onSubmit={(event) => {
          event.preventDefault();
          void runSearch();
        }}
      >
        <Segmented<SearchMode>
          ariaLabel="Search type"
          options={searchModeOptions.map((option) => ({ value: option, label: option === "author" ? "Author" : option === "series" ? "Series" : "Book" }))}
          value={mode}
          onChange={switchMode}
        />
        <div className="search-hero-input">
          <Search size={16} aria-hidden />
          <input
            value={query}
            onChange={(event) => {
              if (mode === "author") {
                setAuthorQuery(event.target.value);
                return;
              }
              setBookQuery(event.target.value);
            }}
            placeholder={mode === "series" ? "Search series name" : mode === "author" ? "Search author name" : "Search title, author, series, or ISBN"}
            aria-label={mode === "series" ? "Series query" : mode === "author" ? "Author query" : "Book query"}
          />
        </div>
        <select
          value={format}
          onChange={(event) => setFormat(event.target.value)}
          aria-label={mode === "author" ? "Target format" : "Format"}
        >
          {searchFormatOptions.map((option) => (
            <option key={option} value={option}>
              {option === "any" ? "Any format" : option}
            </option>
          ))}
        </select>
        <Button type="submit" variant="primary" icon={FileSearch} busy={isSearching}>
          {isSearching ? "Searching" : mode === "author" ? "Find author" : "Search"}
        </Button>
      </form>

      {searchError ? (
        <InlineNotice tone={results.length ? "warn" : "danger"} onDismiss={() => setSearchError("")}>
          {searchError}
        </InlineNotice>
      ) : null}

      <div className="search-layout">
        <Card
          title={resultsTitle}
          subtitle={resultsSubtitle}
          actions={
            <Button
              size="sm"
              icon={SlidersHorizontal}
              onClick={() => setFiltersOpen((open) => !open)}
              title="Filter results"
            >
              Filters
              {activeFilterCount ? <Badge tone="accent">{activeFilterCount}</Badge> : null}
            </Button>
          }
        >
          {filtersOpen ? (
            <div className="search-filter-panel">
              <Field label="Provider">
                <select value={providerFilter} onChange={(event) => setProviderFilter(event.target.value)}>
                  <option value="">All providers</option>
                  {providerOptions.map((provider) => (
                    <option key={provider} value={provider}>
                      {provider}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Evidence">
                <select
                  value={evidenceFilter}
                  onChange={(event) => setEvidenceFilter(event.target.value as SearchEvidenceFilter)}
                >
                  <option value="all">All evidence</option>
                  <option value="identifiers">ISBN or ASIN</option>
                  <option value="published">Publisher or date</option>
                  <option value="series">Series position</option>
                </select>
              </Field>
              <Button size="sm" variant="ghost" icon={FilterX} disabled={!activeFilterCount} onClick={clearFilters}>
                Clear
              </Button>
            </div>
          ) : null}

          {isSearching ? (
            <LoadingRow label="Searching metadata providers…" />
          ) : visibleResults.length ? (
            <div className="search-result-list" role="list">
              {editionGroups.map(group => renderResultRow(group.find(result => searchResultKey(result) === selectedSearchKey) ?? group[0], group.length))}
            </div>
          ) : results.length ? (
            <EmptyState icon={FilterX} title="No metadata candidates match the current filters.">
              Clear filters or run a broader provider search.
            </EmptyState>
          ) : hasSearched ? (
            <EmptyState icon={Search} title="No metadata candidates found.">
              Try a different title, author, series, or ISBN.
            </EmptyState>
          ) : (
            <EmptyState icon={Search} title="Search metadata providers.">
              Results from Open Library, Google Books, and Hardcover land here.
            </EmptyState>
          )}
        </Card>

        {isDesktop ? (
          <div className="search-detail-aside">
            <Card title="Details" subtitle={selected ? undefined : "Select a result."}>
              {selected ? renderDetail(selected) : <EmptyState icon={BookOpen} title="Select a result." />}
            </Card>
          </div>
        ) : null}
      </div>

      {mode !== "author" && selectedCanSearchReleases && (isSearchingReleases || releasesSearched) ? (
        <Card
          title="Release search"
          subtitle={
            releases.length
              ? `${releases.length} Prowlarr releases ready for paused download-client grab.`
              : "Search releases from the selected metadata match."
          }
          actions={downloadStatus ? <Badge tone="info">Queued: {downloadStatus.category}</Badge> : null}
        >
          {releaseError ? (
            <InlineNotice tone="danger" onDismiss={() => setReleaseError("")}>
              {releaseError}
            </InlineNotice>
          ) : null}
          {isSearchingReleases ? (
            <LoadingRow label="Searching releases…" />
          ) : releases.length ? (
            <DataTable>
              <thead>
                <tr>
                  <th>Release</th>
                  <th>Indexer</th>
                  <th>Protocol</th>
                  <th>Size</th>
                  <th>Seeders</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {releases.map((release) => (
                  <tr key={release.id}>
                    <td className="cell-primary search-release-title">{release.title}</td>
                    <td className="cell-muted">{release.indexer}</td>
                    <td>
                      <Badge tone={release.protocol === "torrent" ? "info" : "accent"}>{release.protocol}</Badge>
                    </td>
                    <td className="cell-muted">{formatBytes(release.sizeBytes ?? 0)}</td>
                    <td className="cell-muted">{release.seeders ?? 0}</td>
                    <td className="cell-actions">
                      <Button
                        size="sm"
                        icon={Download}
                        busy={grab.isPending && grab.variables?.release.id === release.id}
                        onClick={() => requestGrab(release)}
                      >
                        Grab paused
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </DataTable>
          ) : releasesSearched ? (
            <EmptyState icon={Download} title="No releases found.">
              No indexer results for this candidate — adjust the query or check Prowlarr.
            </EmptyState>
          ) : (
            <EmptyState
              icon={Download}
              title="No release search yet."
              actions={
                <Button icon={Download} onClick={() => void runReleaseSearch()}>
                  Search Releases
                </Button>
              }
            >
              Search releases from the selected metadata match.
            </EmptyState>
          )}
        </Card>
      ) : null}

      {!isDesktop ? (
        <Modal title="Result details" open={detailOpen && Boolean(selected)} onClose={() => setDetailOpen(false)}>
          {selected ? renderDetail(selected) : null}
        </Modal>
      ) : null}

      <Modal
        title="Review before adding"
        open={Boolean(pendingReview)}
        onClose={() => setPendingReview(null)}
        footer={
          <>
            <Button variant="ghost" disabled={Boolean(downloadPhase)} onClick={() => setPendingReview(null)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              icon={HardDriveDownload}
              busy={Boolean(downloadPhase)}
              disabled={!matchesReady}
              onClick={() => pendingReview && void requestAddBook(pendingReview, { force: true, download: pendingDownload })}
            >
              {downloadPhase || (pendingDownload ? "Download anyway" : "Add anyway")}
            </Button>
          </>
        }
      >
        {pendingReview ? (
          <>
            <p>
              <strong>{searchResultTitle(pendingReview)}</strong> — {firstAuthorName(pendingReview)}
            </p>
            <p>Check these details before continuing:</p>
            <ul className="search-review-reasons">
              {searchResultWantedReviewReasons(pendingReview).map((reason) => (
                <li key={reason}>{reason}</li>
              ))}
            </ul>
            <div className="search-detail-form">{renderAddBookFields()}</div>
          </>
        ) : null}
      </Modal>

      <datalist id="search-tag-suggestions">
        {knownTags.map((tag) => (
          <option key={tag.id} value={tag.label} />
        ))}
      </datalist>
    </>
  );
}

function SearchCover({ result, size }: { result: SearchResult; size: number }) {
  const [failed, setFailed] = useState(false);
  const url = searchResultCover(result);
  if (url && !failed) return <img src={url} alt="" loading="lazy" onError={() => setFailed(true)} />;
  return result.kind === "author" ? <UserPlus size={size} /> : <BookOpen size={size} />;
}
