import React, { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { FolderPen } from "lucide-react";
import { Badge, Button, DataTable, EmptyState, InlineNotice, LoadingRow, Modal, Segmented } from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  fetchFileCollection,
  previewLibraryRename,
  renameLibraryFiles,
  type LibraryRenameRequest,
  type LibraryRenameOutcome
} from "../../lib/api";
import { useInvalidatingMutation } from "../../lib/queries";
import { withDemoFallback } from "../../lib/demo";
import { QueryErrorNotice } from "./controls";
import { errorMessage } from "./helpers";

type RenameFormat = "any" | "ebook" | "audiobook";

const formatOptions: { value: RenameFormat; label: string }[] = [
  { value: "any", label: "Any" },
  { value: "ebook", label: "Ebook" },
  { value: "audiobook", label: "Audiobook" }
];

const emptyOutcome: LibraryRenameOutcome = {
  requested: 0,
  renamed: 0,
  skipped: 0,
  errored: 0,
  previews: [],
  results: []
};

/*
 * The rename endpoints select files by id/path (no server-side format
 * filter), so each preview resolves the exact IDs on one collection page.
 */
async function fetchRenamePreview(format: RenameFormat, cursor: string, q: string) {
  const page = await fetchFileCollection({ format, cursor, q, sort: "path", limit: 100 });
  const ids = page.files.map(file => file.id);
  const outcome = ids.length ? await previewLibraryRename({ ids }) : emptyOutcome;
  return { ...outcome, total: page.total, filtered: page.filtered, nextCursor: page.nextCursor };
}

/**
 * Old-path → new-path rename preview over the library naming templates, with
 * per-row selection and an execute action. Lives in features/settings but is
 * intentionally shared with the Library toolbar ("Rename Files").
 */
export function RenameFilesModal(props: { open: boolean; onClose: () => void }) {
  const toast = useToast();
  const client = useQueryClient();
  const [format, setFormat] = useState<RenameFormat>("any");
  const [cursors, setCursors] = useState<string[]>([""]);
  const [search, setSearch] = useState("");
  const [querySearch, setQuerySearch] = useState("");
  useEffect(() => { if (search === querySearch) return; const timer = setTimeout(() => { setQuerySearch(search); setCursors([""]); }, 250); return () => clearTimeout(timer); }, [search, querySearch]);
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  const preview = useQuery({
    queryKey: ["rename-preview", format, cursors[cursors.length - 1], querySearch],
    queryFn: withDemoFallback(() => fetchRenamePreview(format, cursors[cursors.length - 1], querySearch), () => ({ ...emptyOutcome, total: 0, filtered: 0, nextCursor: undefined })),
    enabled: props.open
  });

  const previews = useMemo(() => preview.data?.previews ?? [], [preview.data]);
  const changed = useMemo(() => previews.filter((item) => !item.noop), [previews]);

  // Default selection: every changed row, refreshed whenever the preview does.
  useEffect(() => {
    setSelected(new Set(changed.map((item) => item.file.id)));
  }, [changed]);

  // Prefix key: invalidates every per-format library-files query (same
  // pattern DownloadClientsTab uses for ["downloads"]).
  const apply = useInvalidatingMutation((request: LibraryRenameRequest) => renameLibraryFiles(request), [["library-files"], ["import-recovery"], ["wanted"]]);

  const selectedIDs = useMemo(() => changed.map((item) => item.file.id).filter((id) => selected.has(id)), [changed, selected]);
  const allChangedSelected = changed.length > 0 && selectedIDs.length === changed.length;

  function toggleRow(id: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  function toggleAll() {
    setSelected(allChangedSelected ? new Set() : new Set(changed.map((item) => item.file.id)));
  }

  async function execute() {
    if (!selectedIDs.length) return;
    try {
      const selectedPreviews = changed.filter(item => selected.has(item.file.id));
      if (selectedPreviews.some(item => !item.revision)) throw new Error("Preview is out of date. Refresh before applying.");
      const revisions = Object.fromEntries(selectedPreviews.map(item => [item.file.id, item.revision!]));
      const outcome = await apply.mutateAsync({ ids: selectedIDs, revisions });
      const parts = [`${outcome.renamed} renamed`];
      if (outcome.skipped) parts.push(`${outcome.skipped} skipped`);
      if (outcome.errored) parts.push(`${outcome.errored} failed`);
      const detail = outcome.results.find((result) => result.status === "error")?.message;
      const message = `Rename: ${parts.join(" · ")}${detail ? ` — ${detail}` : ""}`;
      if (outcome.errored) {
        toast.notify(message, "warn");
      } else {
        toast.success(message);
      }
      await client.invalidateQueries({ queryKey: ["rename-preview"] });
    } catch (error) {
      toast.error(errorMessage(error, "Rename failed"));
    }
  }

  return (
    <Modal
      title="Preview Rename"
      open={props.open}
      onClose={props.onClose}
      wide
      footer={
        <>
          <Button onClick={props.onClose}>Close</Button>
          <Button
            variant="primary"
            icon={FolderPen}
            busy={apply.isPending}
            disabled={!selectedIDs.length || apply.isPending || preview.isFetching || preview.isError}
            onClick={() => void execute()}
          >
            Apply {selectedIDs.length} Rename{selectedIDs.length === 1 ? "" : "s"}
          </Button>
        </>
      }
    >
      <div className="settings-rename-head">
        <Segmented options={formatOptions} value={format} onChange={value => { if (!apply.isPending) { setFormat(value); setCursors([""]); setSelected(new Set()); } }} ariaLabel="Rename format" />
        <span className="cell-muted">
          {preview.isFetching
            ? "Building preview…"
            : `${changed.length} of ${previews.length} file${previews.length === 1 ? "" : "s"} would be renamed to match the naming templates.`}
        </span>
      </div>
      <div className="library-pagination">
        <input aria-label="Search rename files" placeholder="Search paths, titles or authors" disabled={apply.isPending} value={search} onChange={event => setSearch(event.target.value)} />
        <span>{previews.length} shown · {preview.data?.filtered ?? 0} matching · {preview.data?.total ?? 0} total files</span>
        <Button disabled={cursors.length === 1 || preview.isFetching || apply.isPending} onClick={() => { setSelected(new Set()); setCursors(value => value.slice(0, -1)); }}>Previous preview</Button>
        <span>Page {cursors.length}</span>
        <Button disabled={!preview.data?.nextCursor || preview.isFetching || apply.isPending} onClick={() => { if (preview.data?.nextCursor) { setSelected(new Set()); setCursors(value => [...value, preview.data!.nextCursor!]); } }}>Next preview</Button>
        <Button disabled={preview.isFetching || apply.isPending} onClick={() => void preview.refetch()}>Refresh preview</Button>
      </div>
      <p className="field-hint">Selection and Apply cover only the files shown on this page.</p>
      {preview.data?.results.filter(result => result.status === "error").map(result => <InlineNotice key={result.preview.file.id || result.preview.sourcePath} tone="warn">{result.preview.sourcePath}: {result.message}</InlineNotice>)}
      {preview.isError ? <QueryErrorNotice error={preview.error} fallback="Rename preview failed" /> : null}
      {preview.isError ? <Button onClick={() => void preview.refetch()}>Retry preview</Button> : preview.isLoading ? (
        <LoadingRow label="Building rename preview…" />
      ) : previews.length ? (
        <DataTable className="settings-rename-table">
          <thead>
            <tr>
              <th className="settings-rename-check">
                <input
                  type="checkbox"
                  checked={allChangedSelected}
                  disabled={!changed.length}
                  onChange={toggleAll}
                  aria-label="Select changed files on this page"
                />
              </th>
              <th>Current path</th>
              <th>New path</th>
            </tr>
          </thead>
          <tbody>
            {previews.map((item) => (
              <tr key={item.file.id || item.sourcePath} className={item.noop ? "settings-rename-noop" : undefined}>
                <td className="settings-rename-check">
                  <input
                    type="checkbox"
                    checked={selected.has(item.file.id)}
                    disabled={item.noop}
                    onChange={() => toggleRow(item.file.id)}
                    aria-label={`Rename ${item.sourcePath}`}
                  />
                </td>
                <td>
                  <code className="settings-rename-path" title={item.sourcePath}>
                    {item.sourcePath}
                  </code>
                </td>
                <td>
                  {item.noop ? (
                    <><Badge>{item.reason ? "Retained" : "Unchanged"}</Badge>{item.reason ? <p className="field-hint">{item.reason}</p> : null}</>
                  ) : (
                    <div className="settings-rename-dest">
                      <code className="settings-rename-path" title={item.destinationPath}>
                        {item.destinationPath}
                      </code>
                      {item.operationId ? <Badge tone="warn">Resume saved rename</Badge> : null}
                      {item.exists ? (
                        <Badge tone="warn" title="A file already exists at the destination; this rename will be skipped.">
                          Exists
                        </Badge>
                      ) : null}
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      ) : (
        <EmptyState icon={FolderPen} title="Nothing to rename">
          {format === "any"
            ? "No imported library files yet — import books first, then preview renames here."
            : `No imported ${format} files yet.`}
        </EmptyState>
      )}
    </Modal>
  );
}
