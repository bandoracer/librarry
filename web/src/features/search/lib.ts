import type {
  AuthorMissingBookPolicy,
  MetadataSearchType,
  SearchResult,
  WantedItem
} from "../../lib/api";

/*
 * Helpers ported from the legacy single-file App.tsx (searchResult* family).
 * Pure functions over SearchResult/WantedItem — no React, no fetch.
 */

export type SearchMode = MetadataSearchType;
export type SearchConfidenceFilter = "all" | SearchResult["confidence"];
export type SearchEvidenceFilter = "all" | "identifiers" | "published" | "series";
export type SearchEvidenceTone = "high" | "medium" | "review" | "neutral";
export type SearchEvidenceChip = { label: string; tone?: SearchEvidenceTone };
export type SearchEvidenceItem = { label: string; value: string; detail: string };

export const searchModeOptions: SearchMode[] = ["book", "author", "series"];
export const searchConfidenceOptions: SearchResult["confidence"][] = ["high", "medium", "review"];
export const authorMissingPolicyOptions: AuthorMissingBookPolicy[] = [
  "all",
  "future",
  "missing",
  "existing",
  "first",
  "latest",
  "none"
];
export const searchFormatOptions = ["any", "ebook", "audiobook"] as const;

export function firstAuthorName(result: SearchResult) {
  return result.work.authors?.[0]?.name || "Unknown author";
}

export function searchResultCanBeWanted(result: SearchResult) {
  return result.kind !== "author";
}

export function searchResultKey(result: SearchResult) {
  return `${result.provider}:${result.kind}:${result.work.id}:${result.edition?.id || result.rawSourceKey || ""}${result.work.seriesId ? `:${result.work.seriesId}` : ""}`;
}

export function searchResultWantedSourceKey(result: SearchResult) {
  return result.edition?.id || result.work.id || result.rawSourceKey || "";
}

export function wantedFormat(format: string): "ebook" | "audiobook" {
  return format === "audiobook" ? "audiobook" : "ebook";
}

export function searchResultWantedFormat(result: SearchResult, currentFormat: string) {
  if (result.edition?.format === "audiobook") return "audiobook";
  if (result.edition?.format === "ebook") return "ebook";
  return wantedFormat(currentFormat);
}

export function normalizedWantedText(value?: string) {
  return (value || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/^(a|an|the)\s+/, "")
    .replace(/\s+/g, " ");
}

export function searchResultExistingWanted(result: SearchResult, items: WantedItem[], currentFormat: string) {
  if (!searchResultCanBeWanted(result)) return undefined;
  const wantedFormatValue = searchResultWantedFormat(result, currentFormat);
  const provider = result.provider.trim().toLowerCase();
  const sourceKey = searchResultWantedSourceKey(result).trim().toLowerCase();
  return items.find((item) => item.format === wantedFormatValue && provider && sourceKey &&
    (item.sourceProvider || "").trim().toLowerCase() === provider &&
    (item.sourceKey || "").trim().toLowerCase() === sourceKey);

}

export function searchResultMatchLabel(result: SearchResult) {
  if (result.evidence?.length) return result.evidence[0];
  if (result.matchedOn.includes("isbn")) return "Exact ISBN";
  if (result.matchedOn.includes("title") && result.matchedOn.includes("author")) return "Title and author match";
  if (result.kind === "author" && result.matchedOn.includes("author")) return "Author name match";
  return "Provider relevance";
}

export function searchResultCover(result: SearchResult) {
  return result.edition?.coverUrl || result.work.coverUrl || "";
}

export function searchResultMatchChips(result: SearchResult) {
  const chips: SearchEvidenceChip[] = [];
  if (result.conflicts?.length) chips.push({ label: "Edition conflict", tone: "review" });
  if (result.matchedOn.includes("isbn") || result.evidence?.includes("Exact ISBN")) {
    chips.push({ label: "Exact ISBN", tone: "neutral" });
  }
  if (result.edition?.format && result.edition.format !== "any") {
    chips.push({ label: result.edition.format, tone: "neutral" });
  }
  const language = languageLabel(result.edition?.language);
  if (language) chips.push({ label: language, tone: "neutral" });
  return chips;
}

export function searchResultEvidenceSummary(result: SearchResult, currentFormat: string): SearchEvidenceItem[] {
  const sourceKey = searchResultSourceIdentity(result);
  const matchedFields = searchResultMatchedFieldsLabel(result);
  if (result.kind === "author") {
    return [
      {
        label: "Match",
        value: searchResultMatchLabel(result),
        detail: searchResultConfidenceDescription(result)
      },
      {
        label: "Author identity",
        value: sourceKey,
        detail: "Provider-backed author ID used for monitored-author refreshes."
      },
      {
        label: "Target",
        value: wantedFormat(currentFormat),
        detail: "New wanted items from this author will use this format policy."
      },
      {
        label: "Matched fields",
        value: matchedFields || "Provider rank",
        detail: "Evidence supplied by the metadata provider."
      }
    ];
  }
  return [
    {
      label: "Match",
      value: searchResultMatchLabel(result),
      detail: searchResultConfidenceDescription(result)
    },
    {
      label: "Edition evidence",
      value: searchResultEditionSummary(result, currentFormat) || "Any format",
      detail: searchResultEditionSubline(result)
    },
    {
      label: "Matched fields",
      value: matchedFields || "Provider rank",
      detail: "Evidence supplied by the metadata provider."
    },
    {
      label: "Source identity",
      value: sourceKey,
      detail: `${searchResultSourceLabel(result)} stored with provider records for future corrections.`
    }
  ];
}

export function searchResultConfidenceDescription(result: SearchResult) {
  if (result.conflicts?.length) return result.conflicts.join(". ");
  return result.evidence?.join(" · ") || "Returned by the metadata provider.";
}

export function searchResultMatchedFieldsLabel(result: SearchResult) {
  return Array.from(new Set(result.matchedOn.map(searchMatchFieldLabel).filter(Boolean))).join(", ");
}

export function searchMatchFieldLabel(field: string) {
  switch (field.toLowerCase()) {
    case "isbn":
      return "ISBN";
    case "asin":
      return "ASIN";
    case "title":
      return "title";
    case "author":
      return "author";
    case "series":
      return "series";
    case "provider corroboration":
      return "provider corroboration";
    default:
      return field;
  }
}

export function uniqueEvidenceChips(chips: SearchEvidenceChip[]) {
  const seen = new Set<string>();
  return chips.filter((chip) => {
    const key = chip.label.toLowerCase();
    if (!chip.label || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function searchResultWantedReviewReasons(result: SearchResult) {
  if (!searchResultCanBeWanted(result)) return [];
  const reasons: string[] = [...(result.conflicts ?? [])];
  if (!result.work.authors?.some(author => author.name?.trim() && author.name.trim().toLowerCase() !== "unknown author")) {
    reasons.push("This record has no author. Check that it is the book you want.");
  }
  return Array.from(new Set(reasons));
}

export function searchResultNeedsWantedReview(result: SearchResult) {
  return searchResultWantedReviewReasons(result).length > 0;
}

export function uniqueSearchProviders(results: SearchResult[]) {
  return Array.from(new Set(results.flatMap(searchResultSourceNames))).sort((a, b) => a.localeCompare(b));
}

export function searchResultVisibleForFilters(
  result: SearchResult,
  filters: { provider: string; confidence: SearchConfidenceFilter; evidence: SearchEvidenceFilter }
) {
  if (filters.provider && !searchResultSourceNames(result).includes(filters.provider)) return false;
  if (filters.confidence !== "all" && result.confidence !== filters.confidence) return false;
  return searchResultHasEvidence(result, filters.evidence);
}

export function searchResultHasEvidence(result: SearchResult, evidence: SearchEvidenceFilter) {
  if (evidence === "all") return true;
  if (result.kind === "author") return false;
  switch (evidence) {
    case "identifiers":
      return Boolean(result.edition?.asin || result.edition?.isbns?.length);
    case "published":
      return Boolean(result.edition?.publisher || result.edition?.publishedDate || result.work.firstPublishYear);
    case "series":
      return Boolean(result.work.series || result.work.seriesPosition);
  }
}

export function searchResultTitle(result: SearchResult) {
  return result.kind === "author" ? firstAuthorName(result) : result.work.title;
}

export function searchResultSubtitle(result: SearchResult) {
  if (result.kind === "author") {
    return result.work.description || result.rawSourceKey || result.provider;
  }
  const parts = compactStringList([
    firstAuthorName(result),
    searchResultSeriesLabel(result),
    result.work.firstPublishYear ? String(result.work.firstPublishYear) : ""
  ]);
  return parts.join(" · ");
}

export function searchResultProviderKey(result: SearchResult) {
  return result.work.authors?.[0]?.id || result.rawSourceKey || result.work.providerIds?.[0] || result.work.id || "Unknown";
}

export function searchResultSourceNames(result: SearchResult) {
  const names = compactStringList([
    result.provider,
    ...(result.work.providerIds ?? []).map(providerNameFromIdentity),
    ...(result.edition?.providerIds ?? []).map(providerNameFromIdentity),
    ...(result.work.authors ?? []).flatMap((author) => (author.providerIds ?? []).map(providerNameFromIdentity))
  ]);
  const seen = new Set<string>();
  return names.filter((name) => {
    const key = name.toLowerCase();
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function searchResultSourceLabel(result: SearchResult) {
  const names = searchResultSourceNames(result);
  if (names.length === 0) return result.provider || "Unknown";
  if (names.length === 1) return names[0];
  return `${names[0]} + ${names.length - 1}`;
}

export function providerNameFromIdentity(identity: string) {
  const value = identity.trim().toLowerCase();
  if (!value) return "";
  if (value.startsWith("hardcover:") || value.startsWith("hardcover-author:")) return "Hardcover";
  if (value.startsWith("openlibrary:") || value.includes("/authors/") || value.includes("/works/")) return "Open Library";
  if (value.startsWith("googlebooks:") || value.startsWith("googlebooks-author:")) return "Google Books";
  return "";
}

export function searchResultSourceIdentity(result: SearchResult) {
  if (result.kind === "author") return searchResultProviderKey(result);
  return searchResultWantedSourceKey(result) || result.work.providerIds?.[0] || "Unknown";
}

export function searchResultEditionSummary(result: SearchResult, currentFormat: string) {
  if (result.kind === "author") {
    return wantedFormat(currentFormat);
  }
  return compactStringList([
    result.edition?.format && result.edition.format !== "any" ? result.edition.format : "Format unknown",
    languageLabel(result.edition?.language)
  ]).join(" · ");
}

export function searchResultEditionSubline(result: SearchResult) {
  if (result.kind === "author") {
    return searchResultProviderKey(result);
  }
  return compactStringList([
    searchResultPublishedLabel(result),
    searchResultIdentifierSummary(result, 1),
    searchResultSeriesLabel(result)
  ]).join(" · ") || "No edition evidence";
}

export function searchResultPublishedLabel(result: SearchResult) {
  return compactStringList([
    result.edition?.publishedDate || (result.work.firstPublishYear ? String(result.work.firstPublishYear) : ""),
    result.edition?.publisher
  ]).join(" · ");
}

export function searchResultIdentifierLabel(result: SearchResult, limit = 2) {
  return searchResultIdentifierSummary(result, limit) || "None";
}

export function searchResultIdentifierSummary(result: SearchResult, limit = 2) {
  const identifiers = compactStringList([
    ...(result.edition?.isbns ?? []).slice(0, limit),
    result.edition?.asin ? `ASIN ${result.edition.asin}` : ""
  ]);
  const remaining = Math.max(0, (result.edition?.isbns?.length ?? 0) - limit);
  if (!identifiers.length) return "";
  return remaining > 0 ? `${identifiers.join(", ")} +${remaining} more` : identifiers.join(", ");
}

export function searchResultSeriesLabel(result: SearchResult) {
  if (!result.work.series) return "";
  return compactStringList([
    result.work.series,
    result.work.seriesPosition ? `#${result.work.seriesPosition}` : ""
  ]).join(" ");
}

export function languageLabel(value?: string) {
  const normalized = (value || "").trim().toLowerCase();
  const labels: Record<string, string> = {
    en: "English",
    eng: "English",
    es: "Spanish",
    spa: "Spanish",
    fr: "French",
    fre: "French",
    fra: "French",
    de: "German",
    ger: "German",
    deu: "German",
    it: "Italian",
    ita: "Italian",
    ja: "Japanese",
    jpn: "Japanese",
    pt: "Portuguese",
    por: "Portuguese"
  };
  return labels[normalized] || value || "";
}

export function compactStringList(values: Array<string | number | null | undefined>) {
  return values.map((value) => String(value ?? "").trim()).filter(Boolean);
}

export function authorMissingPolicyLabel(policy: AuthorMissingBookPolicy) {
  switch (policy) {
    case "future":
      return "Future Books";
    case "missing":
      return "Missing Books";
    case "existing":
      return "Existing Books";
    case "first":
      return "First Book";
    case "latest":
      return "Latest Book";
    case "none":
      return "None";
    default:
      return "All Books";
  }
}

/** Badge tone mapping for confidence levels (ui.tsx Badge tones). */
export function confidenceTone(confidence: SearchResult["confidence"]): "success" | "warn" | "danger" {
  switch (confidence) {
    case "high":
      return "success";
    case "medium":
      return "warn";
    case "review":
      return "danger";
  }
}

/** Chip tone (legacy high/medium/review/neutral) → ui.tsx Badge tone. */
export function chipTone(tone: SearchEvidenceTone | undefined): "success" | "warn" | "danger" | "neutral" {
  switch (tone) {
    case "high":
      return "success";
    case "medium":
      return "warn";
    case "review":
      return "danger";
    default:
      return "neutral";
  }
}

/** Group only the same provider-backed work; names and covers are not identity.
 * Each option retains its complete SearchResult for library checks and add. */
export function groupSearchEditions(results: SearchResult[]): SearchResult[][] {
  const groups: Array<{ keys: Set<string>; results: SearchResult[] }> = [];
  for (const result of results) {
    // Aliases here were verified by the backend using provider identities or a
    // shared edition ISBN. Titles, author names and covers never create a link.
    const keys = new Set(result.kind === "book" ? [result.work.id, ...(result.work.providerIds ?? [])]
      .filter(id => /^openlibrary:OL\d+W$/.test(id) || /^hardcover:\d+$/.test(id))
      .map(id => `${id}:${result.work.seriesId ?? ""}`) : []);
    const matches = groups.filter(group => [...keys].some(key => group.keys.has(key)));
    if (!matches.length) groups.push({ keys, results: [result] });
    else {
      const target = matches[0];
      // A verified alias can bridge two groups discovered earlier. Keep their
      // original order and every complete selected-edition payload.
      for (const match of matches.slice(1)) {
        match.keys.forEach(key => target.keys.add(key));
        target.results.push(...match.results);
        groups.splice(groups.indexOf(match), 1);
      }
      keys.forEach(key => target.keys.add(key));
      target.results.push(result);
    }
  }
  return groups.map(group => group.results);
}

export function searchGroupSection(group: SearchResult[]): "primary" | "related" | "incomplete" {
  if (group.some(result => !result.discoverySection)) return "primary";
  return group.some(result => result.discoverySection === "related") ? "related" : "incomplete";
}

export function searchEditionOptionLabel(result: SearchResult) {
  return compactStringList([
    searchResultEditionSummary(result, "any"),
    result.edition?.publisher,
    result.edition?.publishedDate,
    result.edition?.id || "Work metadata only"
  ]).join(" · ");
}
