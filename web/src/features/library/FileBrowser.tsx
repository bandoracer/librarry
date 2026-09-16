import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Badge, Button, Card, DataTable, EmptyState, LoadingRow } from "../../components/ui";
import type { FileCollectionOptions } from "../../lib/api";
import { useFileCollection } from "../../lib/queries";
import { formatBytes } from "../../lib/format";

export function useFileBrowser(wantedId?: string) {
  const [search, setSearch] = useState("");
  const [querySearch, setQuerySearch] = useState("");
  const [format, setFormat] = useState<FileCollectionOptions["format"]>("all");
  const [presence, setPresence] = useState<FileCollectionOptions["presence"]>("all");
  const [sort, setSort] = useState<FileCollectionOptions["sort"]>("path");
  const [cursors, setCursors] = useState<string[]>([""]);
  useEffect(() => { if (search === querySearch) return; const timer = setTimeout(() => { setQuerySearch(search); setCursors([""]); }, 250); return () => clearTimeout(timer); }, [search, querySearch]);
  const query = useFileCollection({ q: querySearch, wantedId, format, presence, sort, cursor: cursors[cursors.length - 1], limit: 100 });
  return { query, search, setSearch, format, setFormat, presence, setPresence, sort, setSort, cursors, setCursors };
}

export function FileBrowser({ model }: { model: ReturnType<typeof useFileBrowser> }) {
  const { query, search, setSearch, format, setFormat, presence, setPresence, sort, setSort, cursors, setCursors } = model;
  const data = query.isError ? undefined : query.data;
  return <Card title="Tracked files" subtitle={data ? `${data.files.length} shown · ${data.filtered} matching · ${data.total} total files` : "Files recorded by library scans and imports"}>
    <div className="library-pagination">
      <input aria-label="Search tracked files" placeholder="Search paths, titles or authors" value={search} onChange={event => setSearch(event.target.value)} />
      <select aria-label="Tracked file format" value={format} onChange={event => { setFormat(event.target.value as FileCollectionOptions["format"]); setCursors([""]); }}>
        <option value="all">All formats</option><option value="ebook">Ebooks</option><option value="audiobook">Audiobooks</option>
      </select>
      <select aria-label="Recorded file presence" value={presence} onChange={event => { setPresence(event.target.value as FileCollectionOptions["presence"]); setCursors([""]); }}>
        <option value="all">All presence states</option><option value="present">Present</option><option value="missing">Missing</option><option value="unknown">Unknown</option>
      </select>
      <select aria-label="File sort" value={sort} onChange={event => { setSort(event.target.value as FileCollectionOptions["sort"]); setCursors([""]); }}>
        <option value="path">Path</option><option value="title">Title</option><option value="updated">Recently updated</option>
      </select>
      <Button disabled={cursors.length === 1 || query.isFetching} onClick={() => setCursors(value => value.slice(0, -1))}>Previous files</Button>
      <span>Page {cursors.length}</span>
      <Button disabled={!data?.nextCursor || query.isFetching} onClick={() => { if (data?.nextCursor) setCursors(value => [...value, data.nextCursor!]); }}>Next files</Button>
      <Button disabled={query.isFetching} onClick={() => void query.refetch()}>Refresh files</Button>
    </div>
    <p className="field-hint">Presence reflects the last recorded scan or import. Browsing does not check the current bytes on disk.</p>
    {query.isError ? <EmptyState title="Files could not be loaded"><p>{query.error.message}</p><Button onClick={() => void query.refetch()}>Retry files</Button></EmptyState>
      : query.isLoading ? <LoadingRow label="Loading tracked files…" />
      : !data?.files.length ? <EmptyState title="No matching files">Try another filter, or scan/import files to add them to the library.</EmptyState>
      : <DataTable><thead><tr><th>File</th><th>Presence</th><th>Format</th><th>Import status</th><th>Size</th><th>Books</th><th>Author</th></tr></thead>
        <tbody>{data.files.map(file => <tr key={file.id}>
          <td><div className="library-file-name"><strong>{file.path.split("/").pop() || file.path}</strong>{file.title && file.title !== file.path.split("/").pop() ? <span className="cell-muted">{file.title}</span> : null}<code title={file.path}>{file.path}</code></div></td>
          <td><Badge tone={file.presenceState === "missing" ? "danger" : file.presenceState === "present" ? "success" : "neutral"}>{file.presenceState === "present" ? "Present" : file.presenceState === "missing" ? "Missing" : "Unknown"}</Badge></td>
          <td>{file.mediaFormat}</td><td>{file.importStatus}</td><td>{formatBytes(file.sizeBytes ?? 0)}</td>
          <td>{file.wantedIds.length ? file.wantedIds.map((id, index) => <div key={id}><Link to={`/library/book/${encodeURIComponent(id)}`}>Open book{file.wantedIds.length > 1 ? ` ${index + 1}` : ""}</Link></div>) : "Unassigned"}</td><td>{file.authorName || "Unknown author"}</td>
        </tr>)}</tbody></DataTable>}
  </Card>;
}

export function BookFiles({ wantedId }: { wantedId: string }) {
  const model = useFileBrowser(wantedId);
  return <FileBrowser model={model} />;
}
