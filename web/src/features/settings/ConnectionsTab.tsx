import React, { useEffect, useMemo, useState } from "react";
import { CheckCircle2, RefreshCw } from "lucide-react";
import { Button, Card, Field, FormGrid, LoadingRow } from "../../components/ui";
import { useToast } from "../../components/toast";
import { saveIntegrationSettings, type IntegrationSettings } from "../../lib/api";
import { keys, useIntegrationSettings, useInvalidatingMutation } from "../../lib/queries";
import { QueryErrorNotice, SecretInput } from "./controls";
import { emptyIntegrationSettings, errorMessage, integrationSettingsChanged, integrationSettingsForm } from "./helpers";

function secretLabel(base: string, configured: boolean) {
  return configured ? `${base} (saved)` : base;
}

/**
 * Connections → Prowlarr indexer plus qBittorrent / Transmission / SABnzbd
 * download clients, categories, and the torrent root. Saved secrets are never
 * echoed back: inputs start blank and "leave blank to keep" preserves them.
 */
export function ConnectionsTab() {
  const toast = useToast();
  const query = useIntegrationSettings();
  const defaults = useMemo(() => emptyIntegrationSettings(), []);
  const saved = query.data?.settings ?? defaults;
  const persisted = query.data?.persisted ?? false;

  const [form, setForm] = useState<IntegrationSettings>(() => integrationSettingsForm(defaults));
  const fetched = query.data?.settings;
  useEffect(() => {
    if (fetched) setForm(integrationSettingsForm(fetched));
  }, [fetched]);

  const save = useInvalidatingMutation(saveIntegrationSettings, [
    keys.integrationSettings,
    keys.integrationHealth,
    keys.readiness,
    ["downloads"]
  ]);

  const hasChanges = integrationSettingsChanged(form, saved);
  const clientConfigured = Boolean(saved.qbittorrentUrl || saved.transmissionUrl || saved.sabnzbdUrl);

  function update(changes: Partial<IntegrationSettings>) {
    setForm((current) => ({ ...current, ...changes }));
  }

  async function persist() {
    if (!hasChanges) return;
    try {
      const response = await save.mutateAsync(form);
      setForm(integrationSettingsForm(response.settings));
      if (response.persisted) {
        toast.success("Integration settings saved and applied.");
      } else {
        toast.notify("Integration settings applied for this process. Add Postgres to persist them.", "info");
      }
    } catch (error) {
      toast.error(errorMessage(error, "Integration settings save failed"));
    }
  }

  async function refresh() {
    const result = await query.refetch();
    if (result.data) {
      setForm(integrationSettingsForm(result.data.settings));
    } else if (result.error) {
      toast.error(errorMessage(result.error, "Integration settings refresh failed"));
    }
  }

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Integration settings refresh failed" /> : null}
      <Card
        title="Indexer and download clients"
        subtitle={`${persisted ? "Postgres" : "runtime"} · ${saved.prowlarrUrl ? "indexer set" : "no indexer"} · ${
          clientConfigured ? "client set" : "no client"
        }`}
      >
        {query.isLoading ? (
          <LoadingRow label="Loading integration settings…" />
        ) : (
          <>
            <div className="settings-group">
              <h3 className="settings-group-title field-label">Prowlarr</h3>
              <FormGrid columns={2}>
                <div className="settings-field-wide">
                  <Field label="Prowlarr URL" hint="Indexer aggregator used for release searches.">
                    <input
                      value={form.prowlarrUrl}
                      onChange={(event) => update({ prowlarrUrl: event.target.value })}
                      placeholder="http://prowlarr:9696"
                    />
                  </Field>
                </div>
                <Field label={secretLabel("Prowlarr API key", saved.prowlarrApiKeyConfigured)}>
                  <SecretInput
                    value={form.prowlarrApiKey ?? ""}
                    onChange={(value) => update({ prowlarrApiKey: value })}
                    placeholder={saved.prowlarrApiKeyConfigured ? "Leave blank to keep" : "API key"}
                    ariaLabel="Prowlarr API key"
                  />
                </Field>
              </FormGrid>
            </div>
            <div className="settings-group">
              <h3 className="settings-group-title field-label">qBittorrent</h3>
              <FormGrid columns={2}>
                <div className="settings-field-wide">
                  <Field label="qBittorrent URL">
                    <input
                      value={form.qbittorrentUrl}
                      onChange={(event) => update({ qbittorrentUrl: event.target.value })}
                      placeholder="http://qbittorrent:8080"
                    />
                  </Field>
                </div>
                <Field label="qBittorrent user">
                  <input
                    value={form.qbittorrentUsername}
                    onChange={(event) => update({ qbittorrentUsername: event.target.value })}
                    placeholder="admin"
                  />
                </Field>
                <Field label={secretLabel("qBittorrent password", saved.qbittorrentPasswordConfigured)}>
                  <SecretInput
                    value={form.qbittorrentPassword ?? ""}
                    onChange={(value) => update({ qbittorrentPassword: value })}
                    placeholder={saved.qbittorrentPasswordConfigured ? "Leave blank to keep" : "Password"}
                    ariaLabel="qBittorrent password"
                  />
                </Field>
              </FormGrid>
            </div>
            <div className="settings-group">
              <h3 className="settings-group-title field-label">Transmission</h3>
              <FormGrid columns={2}>
                <div className="settings-field-wide">
                  <Field label="Transmission URL">
                    <input
                      value={form.transmissionUrl}
                      onChange={(event) => update({ transmissionUrl: event.target.value })}
                      placeholder="http://transmission:9091"
                    />
                  </Field>
                </div>
                <Field label="Transmission user">
                  <input
                    value={form.transmissionUsername}
                    onChange={(event) => update({ transmissionUsername: event.target.value })}
                    placeholder="Optional"
                  />
                </Field>
                <Field label={secretLabel("Transmission password", saved.transmissionPasswordConfigured)}>
                  <SecretInput
                    value={form.transmissionPassword ?? ""}
                    onChange={(value) => update({ transmissionPassword: value })}
                    placeholder={saved.transmissionPasswordConfigured ? "Leave blank to keep" : "Password"}
                    ariaLabel="Transmission password"
                  />
                </Field>
              </FormGrid>
            </div>
            <div className="settings-group">
              <h3 className="settings-group-title field-label">SABnzbd</h3>
              <FormGrid columns={2}>
                <div className="settings-field-wide">
                  <Field label="SABnzbd URL">
                    <input
                      value={form.sabnzbdUrl}
                      onChange={(event) => update({ sabnzbdUrl: event.target.value })}
                      placeholder="http://sabnzbd:8080"
                    />
                  </Field>
                </div>
                <Field label={secretLabel("SABnzbd API key", saved.sabnzbdApiKeyConfigured)}>
                  <SecretInput
                    value={form.sabnzbdApiKey ?? ""}
                    onChange={(value) => update({ sabnzbdApiKey: value })}
                    placeholder={saved.sabnzbdApiKeyConfigured ? "Leave blank to keep" : "API key"}
                    ariaLabel="SABnzbd API key"
                  />
                </Field>
                <Field label="SABnzbd user">
                  <input
                    value={form.sabnzbdUsername}
                    onChange={(event) => update({ sabnzbdUsername: event.target.value })}
                    placeholder="Optional"
                  />
                </Field>
                <Field label={secretLabel("SABnzbd password", saved.sabnzbdPasswordConfigured)}>
                  <SecretInput
                    value={form.sabnzbdPassword ?? ""}
                    onChange={(value) => update({ sabnzbdPassword: value })}
                    placeholder={saved.sabnzbdPasswordConfigured ? "Leave blank to keep" : "Password"}
                    ariaLabel="SABnzbd password"
                  />
                </Field>
              </FormGrid>
            </div>
            <div className="settings-group">
              <h3 className="settings-group-title field-label">Categories and paths</h3>
              <FormGrid columns={2}>
                <Field label="Ebook category" hint="Download client category applied to ebook grabs.">
                  <input
                    value={form.ebookCategory}
                    onChange={(event) => update({ ebookCategory: event.target.value })}
                    placeholder="books-ebook"
                  />
                </Field>
                <Field label="Audiobook category" hint="Download client category applied to audiobook grabs.">
                  <input
                    value={form.audiobookCategory}
                    onChange={(event) => update({ audiobookCategory: event.target.value })}
                    placeholder="books-audiobook"
                  />
                </Field>
                <div className="settings-field-wide">
                  <Field label="Torrent root" hint="Book torrent save path on the download client.">
                    <input
                      value={form.bookTorrentRoot}
                      onChange={(event) => update({ bookTorrentRoot: event.target.value })}
                      placeholder="/data/torrents/books"
                    />
                  </Field>
                </div>
              </FormGrid>
            </div>
            <div className="form-actions">
              <Button
                variant="primary"
                icon={CheckCircle2}
                busy={save.isPending}
                disabled={save.isPending || !hasChanges}
                onClick={() => void persist()}
              >
                Save integrations
              </Button>
              <Button icon={RefreshCw} disabled={save.isPending} onClick={() => void refresh()}>
                Refresh
              </Button>
            </div>
          </>
        )}
      </Card>
    </>
  );
}
