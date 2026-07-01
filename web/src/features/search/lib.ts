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

export type SearchMode = Extract<MetadataSearchType, "book" | "author">;
export type SearchConfidenceFilter = "all" | SearchResult["confidence"];
export type SearchEvidenceFilter = "all" | "identifiers" | "published" | "series";
export type SearchEvidenceTone = "high" | "medium" | "review" | "neutral";
export type SearchEvidenceChip = { label: string; tone?: SearchEvidenceTone };
export type SearchEvidenceItem = { label: string; value: string; detail: string };

export const searchModeOptions: SearchMode[] = ["book", "author"];
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
  return `${result.provider}:${result.kind}:${result.work.id}:${result.edition?.id || result.rawSourceKey || ""}`;
}

export function searchResultWantedSourceKey(result: SearchResult) {
  return result.edition?.id || result.work.id || result.rawSourceKey || "";
}

export function wantedFormat(format: string): "ebook" | "audiobook" {
  return format === "audiobook" ? "audiobook" : "ebook";
}

export function searchResultWantedFormat(result: SearchResult, currentFormat: string) {
  if (result.edition?.format === "audiobook") return "audiobook";
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
  const title = normalizedWantedText(result.work.title);
  const author = normalizedWantedText(firstAuthorName(result));
  return items.find((item) => {
    if (item.format !== wantedFormatValue) return false;
    const itemProvider = (item.sourceProvider || "").trim().toLowerCase();
    const itemSourceKey = (item.sourceKey || "").trim().toLowerCase();
    if (provider && sourceKey && itemProvider === provider && itemSourceKey === sourceKey) return true;

    const itemTitle = normalizedWantedText(item.title);
    const itemAuthor = normalizedWantedText(item.authorName || "");
    return Boolean(title && itemTitle === title && (!author || !itemAuthor || itemAuthor === author));
  });
}

export function searchResultScoreLabel(result: SearchResult) {
  if (!Number.isFinite(result.score) || result.score <= 0) return "unscored";
  return result.score <= 1 ? `${Math.round(result.score * 100)}%` : result.score.toFixed(1);
}

export function searchResultMatchChips(result: SearchResult) {
  const chips: SearchEvidenceChip[] = [
    { label: `score ${searchResultScoreLabel(result)}`, tone: result.confidence }
  ];
  const sourceCount = searchResultSourceNames(result).length;
  if (sourceCount > 1) chips.push({ label: `${sourceCount} sources`, tone: "high" });
  result.matchedOn.forEach((field) => chips.push({ label: searchMatchFieldLabel(field), tone: "neutral" }));
  if (result.kind !== "author" && searchResultIdentifierSummary(result, 1)) chips.push({ label: "identifier", tone: "high" });
  if (result.kind !== "author" && searchResultPublishedLabel(result)) chips.push({ label: "published", tone: "neutral" });
  if (result.kind !== "author" && searchResultSeriesLabel(result)) chips.push({ label: "series", tone: "neutral" });
  if (result.kind === "author" && searchResultProviderKey(result)) chips.push({ label: "author id", tone: "neutral" });
  return uniqueEvidenceChips(chips).slice(0, 5);
}

export function searchResultEvidenceSummary(result: SearchResult, currentFormat: string): SearchEvidenceItem[] {
  const sourceKey = searchResultSourceIdentity(result);
  const matchedFields = searchResultMatchedFieldsLabel(result);
  if (result.kind === "author") {
    return [
      {
        label: "Match",
        value: `${result.confidence} · ${searchResultScoreLabel(result)}`,
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
        detail: "Fields that contributed to the normalized match score."
      }
    ];
  }
  return [
    {
      label: "Match",
      value: `${result.confidence} · ${searchResultScoreLabel(result)}`,
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
      detail: "Fields that contributed to the normalized match score."
    },
    {
      label: "Source identity",
      value: sourceKey,
      detail: `${searchResultSourceLabel(result)} stored with provider records for future corrections.`
    }
  ];
}

export function searchResultConfidenceDescription(result: SearchResult) {
  switch (result.confidence) {
    case "high":
      return "Strong enough to create a wanted item without review in normal cases.";
    case "medium":
      return "Likely match; check edition evidence before marking wanted.";
    case "review":
      return "Low-confidence match that should be reviewed before acquisition.";
  }
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
  const reasons: string[] = [];
  const matched = new Set(result.matchedOn.map((field) => field.toLowerCase()));
  const hasIdentifier = Boolean(result.edition?.asin || result.edition?.isbns?.length);
  const matchedIdentifier = matched.has("isbn") || matched.has("asin") || matched.has("identifier");
  const matchedTitleAndAuthor = matched.has("title") && matched.has("author");

  if (result.confidence === "review") {
    reasons.push("Provider match is low confidence.");
  } else if (result.confidence === "medium") {
    reasons.push("Provider match is medium confidence.");
  }
  if (!hasIdentifier && !matchedIdentifier) {
    reasons.push("No ISBN or ASIN evidence is attached to this edition.");
  }
  if (!matchedIdentifier && !matchedTitleAndAuthor) {
    reasons.push("The match did not include both title and author evidence.");
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
    result.edition?.format || "any",
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
