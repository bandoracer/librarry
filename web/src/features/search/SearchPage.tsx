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
  grabRelease,
  searchMetadata,
  searchReleases,
  searchWantedReleases,
  subscribeAuthor,
  type AuthorMissingBookPolicy,
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
  useQualityProfiles,
  useWanted
} from "../../lib/queries";
import { demoModeEnabled, demoSeeds, withDemoFallback } from "../../lib/demo";
import { formatBytes } from "../../lib/format";
import { navItems } from "../../app/nav";
import {
  authorMissingPolicyLabel,
  authorMissingPolicyOptions,
  chipTone,
  compactStringList,
  confidenceTone,
  firstAuthorName,
  languageLabel,
  searchConfidenceOptions,
  searchFormatOptions,
  searchModeOptions,
  searchResultCanBeWanted,
  searchResultEvidenceSummary,
  searchResultExistingWanted,
  searchResultIdentifierLabel,
  searchResultKey,
  searchResultMatchChips,
  searchResultNeedsWantedReview,
  searchResultProviderKey,
  searchResultScoreLabel,
  searchResultSeriesLabel,
  searchResultSourceLabel,
  searchResultSourceNames,
  searchResultSubtitle,
  searchResultTitle,
  searchResultVisibleForFilters,
  searchResultWantedFormat,
  searchResultWantedReviewReasons,
  uniqueSearchProviders,
  wantedFormat,
  type SearchConfidenceFilter,
  type SearchEvidenceFilter,
  type SearchMode
} from "./lib";
import "./search.css";

const searchNav = navItems.find((item) => item.id === "search");

/** Persisted "Start search for missing book" choice for the add flow. */
const searchOnAddStorageKey = "librarry.searchOnAdd";

function storedSearchOnAdd(): boolean {
  if (typeof window === "undefined") return false;
  return window.localStorage.getItem(searchOnAddStorageKey) === "true";
}

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
  const navigate = useNavigate();
  const isDesktop = useIsDesktop();
  const [searchParams, setSearchParams] = useSearchParams();

  // --- URL contract: read ?query= and &mode= once on mount. -----------------
  const initialModeRef = useRef<SearchMode>(searchParams.get("mode") === "author" ? "author" : "book");
  const initialQueryRef = useRef(searchParams.get("query") ?? "");

  const [mode, setMode] = useState<SearchMode>(initialModeRef.current);
  const [bookQuery, setBookQuery] = useState(() => {
    if (initialModeRef.current === "book" && initialQueryRef.current) return initialQueryRef.current;
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
  const [confidenceFilter, setConfidenceFilter] = useState<SearchConfidenceFilter>("all");
  const [evidenceFilter, setEvidenceFilter] = useState<SearchEvidenceFilter>("all");

  const [pendingReview, setPendingReview] = useState<SearchResult | null>(null);
  const [authorPolicy, setAuthorPolicy] = useState<AuthorMissingBookPolicy>("all");
  const [qualityProfile, setQualityProfile] = useState("standard");
  const [addedByKey, setAddedByKey] = useState<Record<string, WantedItem>>({});
  const [searchOnAdd, setSearchOnAdd] = useState(storedSearchOnAdd);

  const [releases, setReleases] = useState<Release[]>([]);
  const [releasesSearched, setReleasesSearched] = useState(false);
  const [isSearchingReleases, setIsSearchingReleases] = useState(false);
  const [releaseError, setReleaseError] = useState("");
  const [downloadStatus, setDownloadStatus] = useState<DownloadStatus | null>(null);

  // --- Shared data -----------------------------------------------------------
  const librarySettings = useLibrarySettings();
  const language = librarySettings.data?.settings.standardSearchLanguage || "English";
  const wantedItems = useWanted().data ?? [];
  const authorSubscriptions = useAuthorSubscriptions().data ?? [];
  const qualityProfiles = useQualityProfiles().data ?? [];

  // --- Derived state ---------------------------------------------------------
  const query = mode === "author" ? authorQuery : bookQuery;
  const providerOptions = useMemo(() => uniqueSearchProviders(results), [results]);
  const visibleResults = useMemo(
    () =>
      results.filter((result) =>
        searchResultVisibleForFilters(result, {
          provider: providerFilter,
          confidence: confidenceFilter,
          evidence: evidenceFilter
        })
      ),
    [results, providerFilter, confidenceFilter, evidenceFilter]
  );
  const activeFilterCount = [
    providerFilter,
    confidenceFilter !== "all" ? confidenceFilter : "",
    evidenceFilter !== "all" ? evidenceFilter : ""
  ].filter(Boolean).length;

  const wantedBySearchKey = useMemo(() => {
    const entries = new Map<string, WantedItem>();
    results.forEach((result) => {
      const key = searchResultKey(result);
      const item = searchResultExistingWanted(result, wantedItems, format) ?? addedByKey[key];
      if (item) entries.set(key, item);
    });
    return entries;
  }, [results, wantedItems, format, addedByKey]);

  const selected = useMemo(
    () =>
      visibleResults.find((result) => searchResultKey(result) === selectedKey || result.work.id === selectedKey) ??
      visibleResults[0] ??
      results[0],
    [visibleResults, results, selectedKey]
  );
  const selectedSearchKey = selected ? searchResultKey(selected) : "";
  const selectedExistingWanted = selectedSearchKey ? wantedBySearchKey.get(selectedSearchKey) : undefined;
  const selectedIsBookCandidate = Boolean(selected && searchResultCanBeWanted(selected));
  const selectedCanBeWanted = selectedIsBookCandidate && !selectedExistingWanted;
  const selectedCanSearchReleases = selectedIsBookCandidate;
  const selectedWantedReviewReasons = selected && selectedCanBeWanted ? searchResultWantedReviewReasons(selected) : [];

  const selectedAuthorFormat = selected ? wantedFormat(selected.edition?.format ?? format) : wantedFormat(format);
  const selectedAuthorSubscription = useMemo(() => {
    const author = selected?.work.authors?.[0];
    if (!author) return undefined;
    const authorID = author.id.trim().toLowerCase();
    const authorName = author.name.trim().toLowerCase();
    return authorSubscriptions.find((subscription) => {
      if (subscription.format !== selectedAuthorFormat) return false;
      const providerKey = subscription.providerKey.trim().toLowerCase();
      const subscriptionName = subscription.authorName.trim().toLowerCase();
      return Boolean((authorID && providerKey === authorID) || (authorName && subscriptionName === authorName));
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
        () => searchMetadata(activeQuery, activeMode === "author" ? "any" : format, activeMode, language),
        () => demoSeeds.results
      );
      const nextResults = await fetcher();
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
    setConfidenceFilter("all");
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
    navigate(`/wanted?item=${encodeURIComponent(item.id)}`);
  }

  function updateSearchOnAdd(next: boolean) {
    setSearchOnAdd(next);
    if (typeof window !== "undefined") {
      window.localStorage.setItem(searchOnAddStorageKey, String(next));
    }
  }

  // --- Mutations --------------------------------------------------------------
  const addWanted = useInvalidatingMutation(
    (args: { result: SearchResult; format: string; profile: string }) =>
      createWanted(args.result, args.format, args.profile),
    [keys.wanted, keys.acquisitionQueue]
  );

  const monitorAuthor = useInvalidatingMutation(
    (args: { result: SearchResult; format: string; policy: AuthorMissingBookPolicy }) =>
      subscribeAuthor(args.result, args.format, "standard", args.policy),
    [keys.authorSubscriptions]
  );

  const grab = useInvalidatingMutation(
    (args: { release: Release; format: string }) => grabRelease(args.release, args.format),
    [keys.downloads()]
  );

  function requestAddBook(result: SearchResult, options: { force?: boolean } = {}) {
    if (!searchResultCanBeWanted(result)) return;
    const key = searchResultKey(result);
    const existing = wantedBySearchKey.get(key);
    if (existing) {
      setPendingReview(null);
      openWanted(existing);
      return;
    }
    if (!options.force && searchResultNeedsWantedReview(result)) {
      setSelectedKey(key);
      setPendingReview(result);
      return;
    }
    setSelectedKey(key);
    addWanted.mutate(
      { result, format: result.edition?.format ?? format, profile: effectiveProfile },
      {
        onSuccess: async (item) => {
          setAddedByKey((current) => ({ ...current, [key]: item }));
          setPendingReview(null);
          if (!searchOnAdd) {
            toast.success(`Added "${item.title}" to Wanted — open the Wanted page to search and grab releases.`);
            return;
          }
          // Search-on-add: review-first rule intact — releases are evaluated
          // and stored, never grabbed.
          try {
            const outcome = await searchWantedReleases(item.id, language);
            const approved = outcome.releases.filter((release) => release.approved).length;
            toast.success(
              `Added "${item.title}" to Wanted · release search: ${outcome.releases.length} found, ${approved} approved, ${
                outcome.releases.length - approved
              } rejected.`
            );
          } catch (error) {
            toast.notify(
              `Added "${item.title}" to Wanted, but the release search failed: ${
                error instanceof Error ? error.message : "Wanted release search failed"
              }`,
              "warn"
            );
          }
        },
        onError: (error) => {
          toast.error(error instanceof Error ? error.message : "Mark wanted failed");
        }
      }
    );
  }

  function requestMonitorAuthor(result: SearchResult | undefined) {
    if (!result?.work.authors?.[0]) return;
    setSelectedKey(searchResultKey(result));
    monitorAuthor.mutate(
      { result, format: result.edition?.format ?? format, policy: authorPolicy },
      {
        onSuccess: (subscription) => {
          toast.success(
            `Monitoring ${subscription.authorName} (${authorMissingPolicyLabel(authorPolicy)} missing books, ${subscription.format}).`
          );
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
  function renderResultRow(result: SearchResult) {
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
          {result.work.coverUrl ? (
            <img src={result.work.coverUrl} alt="" loading="lazy" />
          ) : result.kind === "author" ? (
            <UserPlus size={20} />
          ) : (
            <BookOpen size={20} />
          )}
        </span>
        <span className="search-result-main">
          <span className="search-result-title">
            <strong>{searchResultTitle(result)}</strong>
            {result.kind !== "author" && result.work.firstPublishYear ? (
              <small>{result.work.firstPublishYear}</small>
            ) : null}
          </span>
          <span className="search-result-sub">{searchResultSubtitle(result)}</span>
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
          <Badge tone={confidenceTone(result.confidence)}>{result.confidence}</Badge>
          {existing ? <Badge tone="success">Tracked</Badge> : null}
          <span className="search-result-score">{searchResultScoreLabel(result)}</span>
        </span>
      </button>
    );
  }

  function renderDetail(result: SearchResult) {
    const key = searchResultKey(result);
    const existing = wantedBySearchKey.get(key);
    const canBeWanted = searchResultCanBeWanted(result) && !existing;
    const reviewReasons = canBeWanted ? searchResultWantedReviewReasons(result) : [];
    const sources = searchResultSourceNames(result);
    return (
      <>
        <div className="search-detail-head">
          <span className="search-detail-cover" aria-hidden>
            {result.work.coverUrl ? (
              <img src={result.work.coverUrl} alt="" />
            ) : result.kind === "author" ? (
              <UserPlus size={34} />
            ) : (
              <BookOpen size={34} />
            )}
          </span>
          <div className="search-detail-head-text">
            <h3>{searchResultTitle(result)}</h3>
            <p>{searchResultSubtitle(result)}</p>
            <div className="search-detail-badges">
              <Badge tone={confidenceTone(result.confidence)}>
                {result.confidence} · {searchResultScoreLabel(result)}
              </Badge>
              <Badge tone="neutral" title={sources.join(", ")}>
                {searchResultSourceLabel(result)}
              </Badge>
              {existing ? <Badge tone="success">Tracked</Badge> : null}
            </div>
          </div>
        </div>

        <div className="search-evidence-grid" aria-label="Selected metadata evidence">
          {searchResultEvidenceSummary(result, format).map((item) => (
            <article className="search-evidence-item" key={item.label}>
              <span>{item.label}</span>
              <strong>{item.value}</strong>
              <small>{item.detail}</small>
            </article>
          ))}
        </div>

        {existing ? (
          <div className="search-tracked-callout" aria-label="Existing wanted item">
            <Badge tone="success">Already tracked</Badge>
            <span>
              {existing.derivedState === "cutoffUnmet" ? "cutoff unmet" : existing.derivedState || existing.status} ·{" "}
              {existing.format}
            </span>
            <Button size="sm" icon={HardDriveDownload} onClick={() => openWanted(existing)}>
              Open wanted
            </Button>
          </div>
        ) : null}

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

        <div className="search-detail-form">
          {canBeWanted ? (
            <Field label="Quality profile" hint="Applied to the new wanted item.">
              <select value={effectiveProfile} onChange={(event) => setQualityProfile(event.target.value)}>
                {profileOptions.length ? (
                  profileOptions.map((profile) => (
                    <option key={profile.name} value={profile.name}>
                      {profile.name}
                    </option>
                  ))
                ) : (
                  <option value="standard">standard</option>
                )}
              </select>
            </Field>
          ) : null}
          <Field label="Missing books" hint="Which existing books to add when monitoring this author.">
            <select
              value={authorPolicy}
              onChange={(event) => setAuthorPolicy(event.target.value as AuthorMissingBookPolicy)}
            >
              {authorMissingPolicyOptions.map((policy) => (
                <option key={policy} value={policy}>
                  {authorMissingPolicyLabel(policy)}
                </option>
              ))}
            </select>
          </Field>
        </div>

        {canBeWanted ? (
          <label className="search-add-search-toggle" title="After adding, run a release search for the new wanted item (paused review-first — nothing is grabbed).">
            <input type="checkbox" checked={searchOnAdd} onChange={(event) => updateSearchOnAdd(event.target.checked)} />
            <span>Start search for missing book</span>
          </label>
        ) : null}

        <div className="search-detail-actions">
          {existing ? (
            <Button icon={HardDriveDownload} onClick={() => openWanted(existing)}>
              Open wanted
            </Button>
          ) : canBeWanted ? (
            <Button
              variant="primary"
              icon={HardDriveDownload}
              busy={addWanted.isPending}
              onClick={() => requestAddBook(result)}
            >
              {addWanted.isPending ? "Adding" : reviewReasons.length ? "Review & Add Book" : "Add Book"}
            </Button>
          ) : null}
          <Button
            icon={UserPlus}
            disabled={!result.work.authors?.length}
            busy={monitorAuthor.isPending}
            onClick={() => requestMonitorAuthor(result)}
          >
            {monitorAuthor.isPending ? "Saving" : selectedAuthorSubscription ? "Refresh Author" : "Monitor Author"}
          </Button>
          {searchResultCanBeWanted(result) ? (
            <Button icon={Download} busy={isSearchingReleases} onClick={() => void runReleaseSearch()}>
              {isSearchingReleases ? "Searching releases" : "Search Releases"}
            </Button>
          ) : null}
        </div>
      </>
    );
  }

  const resultsTitle = mode === "author" ? "Author identities" : "Candidate matches";
  const resultsSubtitle =
    mode === "author"
      ? `${visibleResults.length} of ${results.length} author records shown.`
      : `${visibleResults.length} of ${results.length} normalized results shown.`;

  return (
    <>
      <PageHeader title="Add New" subtitle={searchNav?.subtitle} />

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
          options={searchModeOptions.map((option) => ({ value: option, label: option === "author" ? "Author" : "Book" }))}
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
            placeholder={mode === "author" ? "Search author name" : "Search title, author, series, or ISBN"}
            aria-label={mode === "author" ? "Author query" : "Book query"}
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
        <InlineNotice tone="danger" onDismiss={() => setSearchError("")}>
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
              <Field label="Confidence">
                <select
                  value={confidenceFilter}
                  onChange={(event) => setConfidenceFilter(event.target.value as SearchConfidenceFilter)}
                >
                  <option value="all">All confidence</option>
                  {searchConfidenceOptions.map((confidence) => (
                    <option key={confidence} value={confidence}>
                      {confidence}
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
              {visibleResults.map(renderResultRow)}
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

      {mode !== "author" && selectedCanSearchReleases ? (
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
            <Button variant="ghost" disabled={addWanted.isPending} onClick={() => setPendingReview(null)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              icon={HardDriveDownload}
              busy={addWanted.isPending}
              onClick={() => pendingReview && requestAddBook(pendingReview, { force: true })}
            >
              {addWanted.isPending ? "Adding" : "Add anyway"}
            </Button>
          </>
        }
      >
        {pendingReview ? (
          <>
            <p>
              <strong>{searchResultTitle(pendingReview)}</strong> — {firstAuthorName(pendingReview)}
            </p>
            <p>This candidate can become wanted, but the match evidence is not strong enough for a blind add.</p>
            <ul className="search-review-reasons">
              {searchResultWantedReviewReasons(pendingReview).map((reason) => (
                <li key={reason}>{reason}</li>
              ))}
            </ul>
            <label className="search-add-search-toggle" title="After adding, run a release search for the new wanted item (paused review-first — nothing is grabbed).">
              <input type="checkbox" checked={searchOnAdd} onChange={(event) => updateSearchOnAdd(event.target.checked)} />
              <span>Start search for missing book</span>
            </label>
          </>
        ) : null}
      </Modal>
    </>
  );
}
