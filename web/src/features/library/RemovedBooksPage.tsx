import React, { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { BookOpen } from "lucide-react";
import { Badge, Button, Card, EmptyState, InlineNotice, LoadingRow, PageHeader, TabNav } from "../../components/ui";
import { useToast } from "../../components/toast";
import { fetchRemovedBooks, type RemovedBooksOptions, type WantedItem } from "../../lib/api";
import { keys } from "../../lib/queries";
import { formatDateTime } from "../../lib/format";
import RestoreBookDialog from "./RestoreBookDialog";
import "./library.css";

export default function RemovedBooksPage() {
  const toast = useToast();
  const [search, setSearch] = useState("");
  const [format, setFormat] = useState<RemovedBooksOptions["format"]>("all");
  const [status, setStatus] = useState<RemovedBooksOptions["status"]>("removed");
  const [cursors, setCursors] = useState([""]);
  const [restoring, setRestoring] = useState<WantedItem | null>(null);
  const options = { q: search, format, status, cursor: cursors[cursors.length - 1], limit: 50 };
  const query = useQuery({ queryKey: [...keys.wanted, "removed", options], queryFn: ({ signal }) => fetchRemovedBooks(options, signal), refetchInterval: 30_000 });
  const data = query.isError ? undefined : query.data;
  const reset = () => setCursors([""]);
  function paging(position: string) { return <nav className="library-removed-paging" aria-label={`${position} removed book pages`}>
    <span className="field-hint">{data?.filtered ?? "—"} matching · {data?.total ?? "—"} inactive books · Page {cursors.length}</span>
    <Button size="sm" disabled={query.isFetching || cursors.length === 1} onClick={() => setCursors(previous => previous.slice(0, -1))}>Previous books</Button>
    <Button size="sm" disabled={query.isFetching || !data?.nextCursor} onClick={() => { if (data?.nextCursor) setCursors(previous => [...previous, data.nextCursor!]); }}>Next books</Button>
    {cursors.length > 1 ? <Button size="sm" onClick={reset}>First book page</Button> : null}
  </nav>; }
  return <>
    <PageHeader title="Removed books" subtitle="Inspect and restore saved tracking records.">
      <TabNav tabs={[{ label: "Books", to: "/library" }, { label: "Authors", to: "/library/authors" }, { label: "Removed", to: "/library/removed" }]} render={tab => <Link key={tab.to} to={tab.to} className={tab.label === "Removed" ? "active" : undefined}>{tab.label}</Link>} />
    </PageHeader>
    <Card>
      <p>Removed and ignored books stay outside the active Library and automated acquisition. Their file links, history and settings remain saved.</p>
      <div className="library-removed-filters">
        <label>Search<input aria-label="Search removed books" value={search} maxLength={256} placeholder="Title, author or book ID" onChange={event => { setSearch(event.target.value); reset(); }} /></label>
        <label>Format<select aria-label="Removed book format" value={format} onChange={event => { setFormat(event.target.value as RemovedBooksOptions["format"]); reset(); }}><option value="all">All formats</option><option value="ebook">Ebooks</option><option value="audiobook">Audiobooks</option></select></label>
        <label>Status<select aria-label="Removed book status" value={status} onChange={event => { setStatus(event.target.value as RemovedBooksOptions["status"]); reset(); }}><option value="removed">Removed</option><option value="ignored">Ignored</option><option value="all">Both</option></select></label>
        <Button size="sm" disabled={query.isFetching} onClick={() => void query.refetch()}>Refresh removed books</Button>
      </div>
      {paging("Top")}
      <p className="field-hint">{data?.counts?.removed ?? "—"} removed · {data?.counts?.ignored ?? "—"} ignored. Newest added first; edits keep their place. Last updated is not a removal date.</p>
      {query.isError ? <InlineNotice tone="danger">Removed books could not be loaded. <Button size="sm" onClick={() => void query.refetch()}>Retry removed books</Button></InlineNotice> : query.isLoading ? <LoadingRow label="Loading removed books…" /> : !data?.books.length ? <EmptyState icon={BookOpen} title="No removed books on this page">Change filters or return to the first page to see other records.</EmptyState> : <ul className="library-removed-list">
        {data.books.map(book => <li key={book.id}>
          <div><Link to={`/library/book/${encodeURIComponent(book.id)}`}><strong>{book.title || "Untitled book"}</strong></Link><p>{book.authorName || "Unknown author"}</p><div className="library-removed-badges"><Badge>{book.status}</Badge><Badge>{book.format}</Badge><Badge>{book.qualityProfile}</Badge></div><p className="field-hint">Last updated: {formatDateTime(book.updatedAt)}{book.tags?.length ? ` · ${book.tags.join(", ")}` : ""}</p></div>
          <Button size="sm" onClick={() => setRestoring(book)}>Restore…</Button>
        </li>)}
      </ul>}
      {paging("Bottom")}
    </Card>
    {restoring ? <RestoreBookDialog key={restoring.id} book={restoring} onClose={() => setRestoring(null)} onRestored={book => { setRestoring(null); toast.success(`${book.title} restored${book.monitored ? " with monitoring enabled" : " without monitoring"}.`); }} /> : null}
  </>;
}
