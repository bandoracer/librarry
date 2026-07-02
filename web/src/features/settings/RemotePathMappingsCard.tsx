import React, { useState } from "react";
import { Plus, Trash2, Waypoints } from "lucide-react";
import { Button, Card, DataTable, EmptyState, IconButton, LoadingRow } from "../../components/ui";
import { useToast } from "../../components/toast";
import { createRemotePathMapping, deleteRemotePathMapping, type RemotePathMapping } from "../../lib/api";
import { keys, useInvalidatingMutation, useRemotePathMappings } from "../../lib/queries";
import { QueryErrorNotice } from "./controls";
import { errorMessage } from "./helpers";

const emptyDraft = { host: "", remotePrefix: "", localPrefix: "" };

/**
 * Settings → Connections: remote path mappings translate download-client
 * paths (as the client host reports them) into paths Librarry can read, for
 * split-host and cross-container setups.
 */
export function RemotePathMappingsCard() {
  const toast = useToast();
  const query = useRemotePathMappings();
  const mappings = query.data ?? [];
  const [draft, setDraft] = useState(emptyDraft);

  const create = useInvalidatingMutation(createRemotePathMapping, [keys.remotePathMappings]);
  const remove = useInvalidatingMutation(deleteRemotePathMapping, [keys.remotePathMappings]);

  const draftValid = Boolean(draft.host.trim() && draft.remotePrefix.trim() && draft.localPrefix.trim());

  async function add() {
    if (!draftValid) return;
    try {
      const mapping = await create.mutateAsync({
        host: draft.host.trim(),
        remotePrefix: draft.remotePrefix.trim(),
        localPrefix: draft.localPrefix.trim()
      });
      toast.success(`Remote path mapping for ${mapping.host} added.`);
      setDraft(emptyDraft);
    } catch (error) {
      toast.error(errorMessage(error, "Remote path mapping create failed"));
    }
  }

  async function removeMapping(mapping: RemotePathMapping) {
    try {
      await remove.mutateAsync(mapping.id);
      toast.success(`Remote path mapping for ${mapping.host} removed.`);
    } catch (error) {
      toast.error(errorMessage(error, "Remote path mapping delete failed"));
    }
  }

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Remote path mappings refresh failed" /> : null}
      <Card
        title="Remote Path Mappings"
        subtitle="Rewrite download-client paths when the client runs on another host or container"
      >
        {query.isLoading ? (
          <LoadingRow label="Loading remote path mappings…" />
        ) : mappings.length ? (
          <DataTable className="settings-mappings-table">
            <thead>
              <tr>
                <th>Host</th>
                <th>Remote path</th>
                <th>Local path</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {mappings.map((mapping) => (
                <tr key={mapping.id}>
                  <td className="cell-primary">{mapping.host}</td>
                  <td>
                    <code>{mapping.remotePrefix}</code>
                  </td>
                  <td>
                    <code>{mapping.localPrefix}</code>
                  </td>
                  <td>
                    <div className="cell-actions">
                      <IconButton
                        icon={Trash2}
                        size="sm"
                        tone="danger"
                        label={`Delete mapping for ${mapping.host}`}
                        busy={remove.isPending && remove.variables === mapping.id}
                        disabled={remove.isPending}
                        onClick={() => void removeMapping(mapping)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <EmptyState icon={Waypoints} title="No remote path mappings">
            Only needed when a download client reports paths Librarry cannot see — map the client host's remote prefix
            to the local path Librarry mounts.
          </EmptyState>
        )}
        <div className="settings-mapping-add">
          <input
            value={draft.host}
            onChange={(event) => setDraft((current) => ({ ...current, host: event.target.value }))}
            placeholder="Host (e.g. qbittorrent)"
            aria-label="Download client host"
          />
          <input
            value={draft.remotePrefix}
            onChange={(event) => setDraft((current) => ({ ...current, remotePrefix: event.target.value }))}
            placeholder="Remote path (e.g. /downloads/)"
            aria-label="Remote path prefix"
          />
          <input
            value={draft.localPrefix}
            onChange={(event) => setDraft((current) => ({ ...current, localPrefix: event.target.value }))}
            placeholder="Local path (e.g. /data/torrents/)"
            aria-label="Local path prefix"
          />
          <Button icon={Plus} busy={create.isPending} disabled={!draftValid || create.isPending} onClick={() => void add()}>
            Add
          </Button>
        </div>
      </Card>
    </>
  );
}
