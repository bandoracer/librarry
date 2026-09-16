import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "../../components/ui";
import { fetchBookChoices, type BookChoicesOptions } from "../../lib/api";
import { keys } from "../../lib/queries";
import { demoSeeds, withDemoFallback } from "../../lib/demo";

// The selected identity remains pinned while the user searches or pages other books.
// Each format/selection/query shares its request with identical file pickers.
export default function BookChoiceSelect({ value, onChange, format, label, emptyLabel = "Choose a book", disabled = false, allowRetain = false }: {
  value: string; onChange: (id: string) => void; format?: BookChoicesOptions["format"];
  label: string; emptyLabel?: string; disabled?: boolean; allowRetain?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [cursors, setCursors] = useState([""]);
  const selectedId = value === "__retain" ? "" : value;
  const options = { q: search, format: format ?? "all", selectedId, cursor: cursors[cursors.length - 1], limit: 50 };
  const query = useQuery({ queryKey: [...keys.wanted, "choices", options], queryFn: ({ signal }) => withDemoFallback(() => fetchBookChoices(options, signal), () => {
    const active = demoSeeds.wantedItems.filter(book => !["removed", "ignored"].includes(book.status)).map(book => ({ id: book.id, title: book.title, authorName: book.authorName ?? "", format: book.format }));
    const matching = active.filter(book => (options.format === "all" || book.format === options.format) && `${book.title} ${book.authorName} ${book.id}`.toLowerCase().includes(search.trim().toLowerCase()));
    return { books: matching, selected: active.find(book => book.id === selectedId && (options.format === "all" || book.format === options.format)), total: active.length, filtered: matching.length, observedAt: new Date().toISOString() };
  })(), staleTime: 30_000 });
  const data = query.isError ? undefined : query.data;
  const choices = [...(data?.selected ? [data.selected] : []), ...(data?.books ?? [])].filter((book, index, items) => items.findIndex(item => item.id === book.id) === index);
  const unknownSelection = selectedId && !choices.some(book => book.id === selectedId);
  const busy = disabled || query.isFetching;
  return <div className="imports-book-choice">
    <select aria-label={label} value={value} onChange={event => onChange(event.target.value)} disabled={busy || query.isError}>
      <option value="">{emptyLabel}</option>
      {unknownSelection ? <option value={selectedId} disabled>{query.isPending ? "Loading selected book…" : "Selected book unavailable"} ({selectedId})</option> : null}
      {choices.map(book => <option key={book.id} value={book.id}>{book.title || "Untitled book"} — {book.authorName || "Unknown author"} ({book.format})</option>)}
      {allowRetain ? <option value="__retain">Keep in downloads; do not import</option> : null}
    </select>
    {query.isPending ? <span className="field-hint" role="status">Loading books…</span> : null}
    {query.isError ? <span className="field-hint" role="alert">Book choices could not be loaded. <Button size="sm" disabled={disabled} onClick={() => void query.refetch()}>Retry books</Button></span> : unknownSelection && !query.isPending ? <span className="field-hint">This book is no longer available in this format. Choose another book.</span> : null}
    <details>
      <summary>Search and browse books</summary>
      <input aria-label={`Search books: ${label}`} value={search} maxLength={256} disabled={disabled} onChange={event => { setSearch(event.target.value); setCursors([""]); }} placeholder="Title, author or book ID" />
      <nav aria-label={`Book pages: ${label}`}>
        <span className="field-hint">{data?.filtered ?? "—"} matching · Page {cursors.length}</span>
        <Button size="sm" disabled={busy || cursors.length === 1} onClick={() => setCursors(previous => previous.slice(0, -1))}>Previous books</Button>
        <Button size="sm" disabled={busy || !data?.nextCursor} onClick={() => { if (data?.nextCursor) setCursors(previous => [...previous, data.nextCursor!]); }}>Next books</Button>
        {cursors.length > 1 ? <Button size="sm" disabled={disabled} onClick={() => setCursors([""])}>First book page</Button> : null}
      </nav>
      <span className="field-hint">Newest added first. Your selected book stays available while you browse.</span>
    </details>
  </div>;
}
