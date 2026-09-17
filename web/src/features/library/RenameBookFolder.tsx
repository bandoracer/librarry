import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Badge, Button, DataTable, InlineNotice, LoadingRow, Modal } from "../../components/ui";
import { useToast } from "../../components/toast";
import { previewBookRename, renameBookFolder } from "../../lib/api";
import { keys } from "../../lib/queries";
import { formatBytes } from "../../lib/format";

export function RenameBookFolder({ wantedId, open, onClose }: { wantedId: string; open: boolean; onClose: () => void }) {
 const client = useQueryClient();
 const toast = useToast();
 const [page, setPage] = useState(0);
 const preview = useQuery({ queryKey: ["book-rename", wantedId], queryFn: () => previewBookRename(wantedId), enabled: open, staleTime: 0, retry: false });
 const apply = useMutation({ mutationFn: (revision: string) => renameBookFolder(wantedId, revision) });
 const plan = preview.data;
 useEffect(() => { setPage(0); }, [plan?.revision]);
 useEffect(() => { if (!open) apply.reset(); }, [open, apply.reset]);
 const rows = plan?.files.slice(page * 100, (page + 1) * 100) ?? [];
 async function execute() {
  if (!plan || preview.isFetching || preview.isError) return;
  try {
   const result = await apply.mutateAsync(plan.revision);
   await Promise.all([keys.wanted, keys.importRecovery, ["library-files"], ["book-rename", wantedId]].map(queryKey => client.invalidateQueries({ queryKey })));
   toast.success(result.skipped ? "Book folder already matches its saved layout" : "Book folder renamed; file names and book links preserved");
   onClose();
  } catch { await Promise.all([client.invalidateQueries({ queryKey: keys.importRecovery }), preview.refetch()]); }
 }
 return <Modal title="Rename book folder" open={open} onClose={() => { if (!apply.isPending) onClose(); }} wide footer={<>
  <Button disabled={apply.isPending} onClick={onClose}>Close</Button>
  <Button variant="primary" busy={apply.isPending} disabled={!plan || plan.noop || preview.isFetching || preview.isError || apply.isPending} onClick={() => void execute()}>{plan?.operationId ? "Resume complete book rename" : `Rename complete book (${plan?.files.length ?? 0} files)`}</Button>
 </>}>
  {preview.isFetching ? <LoadingRow label="Verifying book files and companion references…" /> : null}
  {preview.isError ? <InlineNotice tone="warn">{preview.error.message}</InlineNotice> : null}
  {apply.isError ? <InlineNotice tone="danger">{apply.error.message} <Link to="/imports">Open recovery</Link></InlineNotice> : null}
  <Button disabled={preview.isFetching || apply.isPending} onClick={() => { apply.reset(); void preview.refetch(); }}>Refresh folder preview</Button>
  {plan && !preview.isError ? <>
   <p>{plan.mediaFiles} book file{plan.mediaFiles === 1 ? "" : "s"} · {plan.companionFiles} companion file{plan.companionFiles === 1 ? "" : "s"}. This action covers the complete recorded set, including files on other preview pages.</p>
   <p className="field-hint">File names and disc directories stay unchanged. Relative playlists stay with their files. Original files remain until the complete set is verified and committed.</p>
   {plan.operationId ? <InlineNotice tone="warn">Resume the previously saved folder plan. Later naming changes do not change this destination.</InlineNotice> : null}
   {plan.noop ? <InlineNotice tone="info">This book folder already matches the naming settings.</InlineNotice> : null}
   <p>Current folder<br /><code className="book-rename-path">{plan.sourceFolder}</code></p>
   <p>New folder<br /><code className="book-rename-path">{plan.destinationFolder}</code></p>
   <div className="library-pagination">
    <span>{rows.length} shown · {plan.files.length} files in this plan · Page {page + 1}</span>
    <Button disabled={page === 0 || apply.isPending} onClick={() => setPage(value => value - 1)}>Previous files</Button>
    <Button disabled={(page + 1) * 100 >= plan.files.length || apply.isPending} onClick={() => setPage(value => value + 1)}>Next files</Button>
   </div>
   <DataTable><thead><tr><th>Path within the book folder</th><th>Kind</th><th>Size</th></tr></thead><tbody>{rows.map(file => <tr key={file.sourcePath}><td><code className="book-rename-path">{file.relativePath}</code></td><td><Badge>{file.format === "sidecar" ? "Companion" : "Book file"}</Badge></td><td>{formatBytes(file.sizeBytes)}</td></tr>)}</tbody></DataTable>
  </> : null}
 </Modal>;
}
