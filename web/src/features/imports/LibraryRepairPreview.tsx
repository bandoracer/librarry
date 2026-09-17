import React, { useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Badge, Button, Card, InlineNotice } from "../../components/ui";
import { fetchLibraryRepairPreview, type LibraryRepairFinding } from "../../lib/api";

const kinds: Record<string, string> = {
  wanted_link: "Book association",
  legacy_audio_file: "Audiobook file completeness unverified",
  download_link: "Download association",
  duplicate_content: "Duplicate content records",
  possible_move: "Possible moved file",
  imported_download_unlinked: "Imported download without files",
  audiobook_completeness: "Audiobook completeness unverified",
  import_manifest_gap: "Import evidence discrepancy"
};
const evidenceLabels: Record<string, string> = {
  fileId: "File ID", recordedPresence: "Recorded presence",
  savedWantedId: "Saved book ID", currentWantedIds: "Current book IDs",
  savedDownloadId: "Saved download ID", savedClient: "Saved client",
  candidateCount: "Matching records", recordCount: "Matching content records",
  sha256: "Recorded SHA-256", sizeBytes: "Recorded size in bytes",
  client: "Download client", downloadId: "Download ID", projectedFileId: "Previously selected file ID",
  linkedAudioFiles: "Linked audio files", sourceKind: "Import source", gapCount: "Discrepancies"
};
function Evidence({ finding }: { finding: LibraryRepairFinding }) {
  const e = finding.evidence;
  const samples = [e.records, e.candidates, e.gaps].filter(Array.isArray).flat() as Record<string, unknown>[];
  return <>
    <dl className="imports-repair-evidence">{Object.entries(e).filter(([key]) => evidenceLabels[key]).map(([key, value]) => <React.Fragment key={key}><dt>{evidenceLabels[key]}</dt><dd>{Array.isArray(value) ? (value.join(", ") || "None") : String(value ?? "Unknown")}</dd></React.Fragment>)}</dl>
    {samples.length ? <><p className="field-hint">Showing {samples.length} related records{typeof e.sampleLimit === "number" ? ` (up to ${e.sampleLimit})` : ""}.</p><ul>{samples.map((sample, i) => <li key={String(sample.id ?? i)}><div className="imports-recovery-path">{String(sample.path ?? "")}</div>{sample.reason ? <p>{String(sample.reason)}</p> : null}{sample.presence ? <span className="field-hint">Recorded presence: {String(sample.presence)}</span> : null}<div className="field-hint imports-recovery-path">Record {String(sample.id ?? "unknown")}</div></li>)}</ul></> : null}
  </>;
}
export default function LibraryRepairPreview() {
  const [run, setRun] = useState(0);
  const query = useInfiniteQuery({
    queryKey: ["library-repair-preview", run], enabled: run > 0,
    initialPageParam: "", queryFn: ({ pageParam }) => fetchLibraryRepairPreview(pageParam),
    getNextPageParam: page => page.nextCursor || undefined,
    refetchOnWindowFocus: false, retry: false
  });
  const pages = query.data?.pages ?? [];
  const findings = [...new Map(pages.flatMap(page => page.findings).map(f => [f.id, f])).values()];
  const checked = pages.reduce((total, page) => total + page.checked, 0);
  const finished = pages.length > 0 && !query.hasNextPage;
  return <Card title="Library repair preview" subtitle="Review associations, duplicate records, possible moves and unverified audiobook imports. This report makes no changes.">
    <Button size="sm" disabled={query.isFetching} onClick={() => setRun(n => n + 1)}>{run ? "Start a fresh report" : "Preview library repairs"}</Button>
    {run > 0 && query.isFetching ? <p role="status">Checking library records…</p> : null}
    {query.isError ? <InlineNotice tone="danger">{query.error.message} <Button size="sm" onClick={() => void (query.isFetchNextPageError ? query.fetchNextPage() : query.refetch())}>Retry report</Button></InlineNotice> : null}
    {pages.length ? <>
      <p className="field-hint">{checked.toLocaleString()} records checked · {findings.length.toLocaleString()} {findings.length === 1 ? "finding" : "findings"} · {finished ? "Report complete" : "More records to check"}</p>
      <p className="field-hint">Based on saved records as each page was checked. This does not verify current disk contents or download-client inventories.</p>
      {finished && findings.length === 0 ? <p>No repair findings in the checked records.</p> : null}
      {findings.map(finding => <details className="imports-recovery-item" key={finding.id}>
        <summary><span>{kinds[finding.kind] ?? "Review needed"}</span><Badge tone="warn">Review</Badge></summary>
        <div className="imports-recovery-path">{finding.path}</div>
        <p>{finding.reason}</p>
        <p><strong>Recommended action:</strong> {finding.proposedAction}</p>
        <Evidence finding={finding} />
      </details>)}
      {query.hasNextPage ? <Button size="sm" busy={query.isFetchingNextPage} disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>Continue report</Button> : null}
    </> : null}
  </Card>;
}
