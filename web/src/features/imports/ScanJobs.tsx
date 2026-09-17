import React, { useEffect, useRef, useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { Badge, Button, Card, InlineNotice } from "../../components/ui";
import { controlLibraryScan, fetchLibraryScanMoves, fetchLibraryScans } from "../../lib/api";
import { keys, useInvalidatingMutation } from "../../lib/queries";

export const scanJobsKey = ["library-scans"];
export function useLibraryScanJobs() {
  return useQuery({ queryKey: scanJobsKey, queryFn: fetchLibraryScans, refetchInterval: 2_000 });
}
export default function ScanJobs({ query }: { query: ReturnType<typeof useLibraryScanJobs> }) {
  const client = useQueryClient();
  const action = useInvalidatingMutation(controlLibraryScan, [scanJobsKey]);
  const completed = useRef(new Set<string>());
  useEffect(() => {
    const finished = (query.data?.scans ?? []).filter(scan => scan.state === "completed" && !completed.current.has(scan.id));
    if (!finished.length) return;
    finished.forEach(scan => completed.current.add(scan.id));
    for (const format of ["any", "ebook", "audiobook"]) void client.invalidateQueries({ queryKey: keys.libraryFiles(format) });
    void client.invalidateQueries({ queryKey: keys.wanted });
  }, [query.data, client]);
  if (!query.isError && !query.data?.scans.length) return null;
  return <Card title="Library scans" subtitle="Progress survives restarts. Missing files are confirmed only after every root has been checked successfully.">
    {query.isError ? <InlineNotice tone="danger">{query.error.message} <Button size="sm" onClick={() => void query.refetch()}>Try again</Button></InlineNotice> : null}
    {action.isError ? <InlineNotice tone="danger">{action.error.message}</InlineNotice> : null}
    {(query.data?.scans ?? []).map(scan => <details className="imports-recovery-item" key={scan.id}>
      <summary><span>{scan.format === "any" ? "All books" : scan.format === "ebook" ? "Ebooks" : "Audiobooks"}</span><Badge tone={scan.state === "failed" ? "danger" : scan.state === "completed" ? "success" : "warn"}>{scan.cancelRequested && scan.state !== "cancelled" ? "Cancelling" : scan.state}</Badge><span>{scan.scanned.toLocaleString()} checked</span></summary>
      <ul>{scan.roots.map(root => <li className="imports-recovery-path" key={root}>{root}</li>)}</ul>
      <p className="field-hint">{scan.state === "cancelled" ? "Stopped" : scan.phase === "discover" ? "Reading files" : scan.phase === "reconcile" ? "Checking file presence" : "Complete"} · {scan.skipped} skipped · {scan.missing} confirmed missing · {scan.moved ?? 0} reattached</p>
      {(scan.moved ?? 0) > 0 ? <ScanMoves id={scan.id} /> : null}
      {scan.lastError ? <InlineNotice tone="warn">{scan.lastError}</InlineNotice> : null}
      {scan.state === "failed" ? <Button size="sm" disabled={action.isPending} busy={action.isPending && action.variables.id === scan.id} onClick={() => action.mutate({ id: scan.id, action: "retry" })}>Resume scan</Button> : null}
      {["queued", "running", "failed"].includes(scan.state) ? <Button size="sm" variant="ghost" disabled={scan.cancelRequested || action.isPending} onClick={() => action.mutate({ id: scan.id, action: "cancel" })}>Cancel scan</Button> : null}
    </details>)}
    {!!query.data && query.data.scans.length >= query.data.limit ? <p className="field-hint">Showing up to {query.data.limit} scans.</p> : null}
  </Card>;
}

function ScanMoves({ id }: { id: string }) {
  const [opened, setOpened] = useState(false);
  const query = useInfiniteQuery({
    queryKey: ["library-scan-moves", id], enabled: opened,
    initialPageParam: "", queryFn: ({ pageParam }) => fetchLibraryScanMoves(id, pageParam),
    getNextPageParam: page => page.nextCursor || undefined
  });
  if (!opened) return <Button size="sm" variant="ghost" onClick={() => setOpened(true)}>View reattached files</Button>;
  return <div>
    <p className="field-hint">Original file identities and book associations were retained. No files were moved or deleted by this scan. Import history still records the original destinations.</p>
    {query.isLoading ? <p role="status">Loading reattached files…</p> : null}
    {query.isError ? <InlineNotice tone="danger">{query.error.message} <Button size="sm" onClick={() => void (query.isFetchNextPageError ? query.fetchNextPage() : query.refetch())}>Retry history</Button></InlineNotice> : null}
    <ul>{query.data?.pages.flatMap(page => page.moves).map(move => <li key={move.fileId}>
      <div className="imports-recovery-path">{move.previousPath}</div>
      <div className="imports-recovery-path">→ {move.currentPath}</div>
      <div className="field-hint imports-recovery-path">Retained file ID: {move.fileId}</div>
    </li>)}</ul>
    {query.hasNextPage ? <Button size="sm" busy={query.isFetchingNextPage} disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>Load more reattached files</Button> : null}
  </div>;
}
