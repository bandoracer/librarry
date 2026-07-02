/** Shared formatting helpers used across feature pages. */

export function formatBytes(bytes: number | undefined | null): string {
  if (!bytes || bytes <= 0) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 100 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

export function formatSpeed(bytesPerSecond: number | undefined | null): string {
  if (!bytesPerSecond || bytesPerSecond <= 0) return "—";
  return `${formatBytes(bytesPerSecond)}/s`;
}

export function formatPercent(ratio: number | undefined | null): string {
  if (ratio === undefined || ratio === null || Number.isNaN(ratio)) return "—";
  return `${Math.round(Math.max(0, Math.min(1, ratio)) * 100)}%`;
}

export function formatDate(iso: string | undefined | null): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function formatDateTime(iso: string | undefined | null): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit"
  });
}

export function formatRelativeTime(iso: string | undefined | null): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  const deltaSeconds = Math.round((date.getTime() - Date.now()) / 1000);
  const table: [Intl.RelativeTimeFormatUnit, number][] = [
    ["year", 31536000],
    ["month", 2592000],
    ["week", 604800],
    ["day", 86400],
    ["hour", 3600],
    ["minute", 60]
  ];
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
  for (const [unit, seconds] of table) {
    if (Math.abs(deltaSeconds) >= seconds) {
      return formatter.format(Math.trunc(deltaSeconds / seconds), unit);
    }
  }
  return "just now";
}

/** Book quality parsed from a release title, mirroring backend detection. */
export function releaseQualityLabel(title: string): string | null {
  const haystack = title.toLowerCase();
  for (const quality of ["azw3", "epub", "mobi", "pdf", "flac", "m4b", "mp3"]) {
    if (new RegExp(`(^|[^a-z0-9])${quality}([^a-z0-9]|$)`).test(haystack)) {
      return quality.toUpperCase();
    }
  }
  return null;
}

/**
 * Release scores are ladder composites (quality rank × 1000 + preferred-word
 * score); legacy sub-1000 scores render as plain numbers.
 */
export function formatReleaseScore(score: number): string {
  if (score >= 1000) {
    const preferred = Math.round(score % 1000);
    return preferred > 0 ? `rank ${Math.floor(score / 1000)} +${preferred}` : `rank ${Math.floor(score / 1000)}`;
  }
  return score.toFixed(1);
}

export function truncateMiddle(value: string, max = 60): string {
  if (value.length <= max) return value;
  const half = Math.floor((max - 1) / 2);
  return `${value.slice(0, half)}…${value.slice(-half)}`;
}
