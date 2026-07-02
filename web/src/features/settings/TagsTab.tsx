import React, { useState } from "react";
import { Pencil, Plus, Tags, Trash2 } from "lucide-react";
import { Button, Card, DataTable, EmptyState, Field, FormGrid, IconButton, LoadingRow, Modal } from "../../components/ui";
import { useToast } from "../../components/toast";
import { createTag, deleteTag, updateTag, type Tag } from "../../lib/api";
import { keys, m6Keys, useInvalidatingMutation, useTags } from "../../lib/queries";
import { QueryErrorNotice } from "./controls";
import { errorMessage } from "./helpers";

/** Everything a tag delete touches — invalidated so chips vanish everywhere. */
const tagLinkedKeys = [m6Keys.tags, keys.wanted, keys.authorSubscriptions] as const;

/**
 * Settings → Tags: native tags linking wanted books, author subscriptions,
 * notification targets, and import lists. Renames propagate everywhere;
 * deletes strip the tag from every linked record.
 */
export function TagsTab() {
  const toast = useToast();
  const query = useTags();
  const tags = query.data ?? [];

  const [newLabel, setNewLabel] = useState("");
  const [renaming, setRenaming] = useState<Tag | null>(null);
  const [renameLabel, setRenameLabel] = useState("");
  const [deleting, setDeleting] = useState<Tag | null>(null);

  const add = useInvalidatingMutation(createTag, [m6Keys.tags]);
  const rename = useInvalidatingMutation((input: { id: number; label: string }) => updateTag(input.id, input.label), tagLinkedKeys);
  const remove = useInvalidatingMutation(deleteTag, tagLinkedKeys);

  const canAdd = Boolean(newLabel.trim());
  const canRename = Boolean(renaming && renameLabel.trim() && renameLabel.trim() !== renaming.label);

  async function addTag() {
    if (!canAdd || add.isPending) return;
    const label = newLabel.trim();
    try {
      await add.mutateAsync(label);
      toast.success(`Tag “${label}” created.`);
      setNewLabel("");
    } catch (error) {
      toast.error(errorMessage(error, "Tag create failed"));
    }
  }

  async function confirmRename() {
    if (!renaming || !canRename) return;
    const label = renameLabel.trim();
    try {
      await rename.mutateAsync({ id: renaming.id, label });
      toast.success(`Tag renamed to “${label}”.`);
      setRenaming(null);
    } catch (error) {
      toast.error(errorMessage(error, "Tag rename failed"));
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await remove.mutateAsync(deleting.id);
      toast.success(`Tag “${deleting.label}” deleted and removed everywhere.`);
      setDeleting(null);
    } catch (error) {
      toast.error(errorMessage(error, "Tag delete failed"));
    }
  }

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Tags refresh failed" /> : null}
      <Card title="Tags" subtitle="Label wanted books and author subscriptions; filter and target by tag.">
        {query.isLoading ? (
          <LoadingRow label="Loading tags…" />
        ) : tags.length ? (
          <DataTable>
            <thead>
              <tr>
                <th>Label</th>
                <th>Wanted Books</th>
                <th>Authors</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {tags.map((tag) => (
                <tr key={tag.id}>
                  <td className="cell-primary">{tag.label}</td>
                  <td className="cell-muted">{tag.wantedCount}</td>
                  <td className="cell-muted">{tag.authorCount}</td>
                  <td>
                    <div className="cell-actions">
                      <IconButton
                        icon={Pencil}
                        size="sm"
                        label={`Rename tag ${tag.label}`}
                        onClick={() => {
                          setRenaming(tag);
                          setRenameLabel(tag.label);
                        }}
                      />
                      <IconButton
                        icon={Trash2}
                        size="sm"
                        tone="danger"
                        label={`Delete tag ${tag.label}`}
                        onClick={() => setDeleting(tag)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <EmptyState icon={Tags} title="No tags yet">
            Tags link records across Librarry: assign them to wanted books and author subscriptions, then use them to
            filter lists and scope notifications and import lists. Create your first tag below.
          </EmptyState>
        )}
        <div className="form-actions">
          <input
            style={{ flex: "1 1 220px", maxWidth: 320 }}
            value={newLabel}
            onChange={(event) => setNewLabel(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") void addTag();
            }}
            placeholder="New tag label (e.g. sci-fi)"
            aria-label="New tag label"
          />
          <Button icon={Plus} disabled={!canAdd || add.isPending} busy={add.isPending} onClick={() => void addTag()}>
            Add Tag
          </Button>
        </div>
      </Card>

      <Modal
        title="Rename tag"
        open={Boolean(renaming)}
        onClose={() => setRenaming(null)}
        footer={
          <>
            <Button onClick={() => setRenaming(null)}>Cancel</Button>
            <Button variant="primary" busy={rename.isPending} disabled={!canRename || rename.isPending} onClick={() => void confirmRename()}>
              Rename
            </Button>
          </>
        }
      >
        <FormGrid columns={1}>
          <Field label="Label" hint="The new label appears everywhere this tag is used.">
            <input
              value={renameLabel}
              onChange={(event) => setRenameLabel(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void confirmRename();
              }}
              aria-label="Tag label"
              autoFocus
            />
          </Field>
        </FormGrid>
      </Modal>

      <Modal
        title="Delete tag"
        open={Boolean(deleting)}
        onClose={() => setDeleting(null)}
        footer={
          <>
            <Button onClick={() => setDeleting(null)}>Cancel</Button>
            <Button variant="danger" icon={Trash2} busy={remove.isPending} onClick={() => void confirmDelete()}>
              Delete
            </Button>
          </>
        }
      >
        <p>
          Delete tag <strong>{deleting?.label}</strong>? It is stripped from everything it is linked to —{" "}
          {deleting ? `${deleting.wantedCount} wanted book${deleting.wantedCount === 1 ? "" : "s"} and ${deleting.authorCount} author subscription${deleting.authorCount === 1 ? "" : "s"}` : "all linked records"}{" "}
          — plus any notification targets, restrictions, or import lists scoped to it. The records themselves are kept.
        </p>
      </Modal>
    </>
  );
}
