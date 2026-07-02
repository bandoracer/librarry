import React, { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { FolderPen } from "lucide-react";
import { Badge, Button, DataTable, EmptyState, LoadingRow, Modal, Segmented } from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  fetchLibraryFiles,
  previewLibraryRename,
  renameLibraryFiles,
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
 * filter), so the preview resolves ids through the files list first.
 */
async function fetchRenamePreview(format: RenameFormat): Promise<LibraryRenameOutcome> {
  const files = await fetchLibraryFiles(format, 500);
  const ids = files.map((file) => file.id).filter(Boolean);
  if (!ids.length) return emptyOutcome;
  return previewLibraryRename({ ids });
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
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  const preview = useQuery({
    queryKey: ["rename-preview", format],
    queryFn: withDemoFallback(() => fetchRenamePreview(format), () => emptyOutcome),
    enabled: props.open
  });

  const previews = useMemo(() => preview.data?.previews ?? [], [preview.data]);
  const changed = useMemo(() => previews.filter((item) => !item.noop), [previews]);

  // Default selection: every changed row, refreshed whenever the preview does.
  useEffect(() => {
    setSelected(new Set(changed.map((item) => item.file.id)));
  }, [changed]);

  // Prefix key: invalidates every per-format library-files query (same
  // pattern ConnectionsTab uses for ["downloads"]).
  const apply = useInvalidatingMutation((ids: string[]) => renameLibraryFiles({ ids }), [["library-files"]]);

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
      const outcome = await apply.mutateAsync(selectedIDs);
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
            disabled={!selectedIDs.length || apply.isPending || preview.isFetching}
            onClick={() => void execute()}
          >
            Apply {selectedIDs.length} Rename{selectedIDs.length === 1 ? "" : "s"}
          </Button>
        </>
      }
    >
      <div className="settings-rename-head">
        <Segmented options={formatOptions} value={format} onChange={setFormat} ariaLabel="Rename format" />
        <span className="cell-muted">
          {preview.isFetching
            ? "Building preview…"
            : `${changed.length} of ${previews.length} file${previews.length === 1 ? "" : "s"} would be renamed to match the naming templates.`}
        </span>
      </div>
      {preview.isError ? <QueryErrorNotice error={preview.error} fallback="Rename preview failed" /> : null}
      {preview.isLoading ? (
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
                  aria-label="Select all changed files"
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
                    <Badge>Unchanged</Badge>
                  ) : (
                    <div className="settings-rename-dest">
                      <code className="settings-rename-path" title={item.destinationPath}>
                        {item.destinationPath}
                      </code>
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
