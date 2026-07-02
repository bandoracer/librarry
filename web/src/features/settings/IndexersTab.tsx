import React, { useEffect, useMemo, useState } from "react";
import { CheckCircle2, RefreshCw } from "lucide-react";
import { Button, Card, Field, FormGrid, InlineNotice, LoadingRow } from "../../components/ui";
import { useToast } from "../../components/toast";
import { saveIntegrationSettings, type IntegrationSettings } from "../../lib/api";
import { keys, useIntegrationSettings, useInvalidatingMutation } from "../../lib/queries";
import { QueryErrorNotice, SecretInput } from "./controls";
import { emptyIntegrationSettings, errorMessage, indexerSettingsChanged, integrationSettingsForm } from "./helpers";

/**
 * Indexers → the Prowlarr aggregator used for release searches. Saves through
 * the shared integration-settings endpoint but only sends the indexer fields;
 * the saved API key is never echoed back ("leave blank to keep" preserves it).
 */
export function IndexersTab() {
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
    keys.readiness
  ]);

  const hasChanges = indexerSettingsChanged(form, saved);

  function update(changes: Partial<IntegrationSettings>) {
    setForm((current) => ({ ...current, ...changes }));
  }

  async function persist() {
    if (!hasChanges) return;
    try {
      const response = await save.mutateAsync({
        prowlarrUrl: form.prowlarrUrl,
        prowlarrApiKey: form.prowlarrApiKey
      });
      setForm(integrationSettingsForm(response.settings));
      if (response.persisted) {
        toast.success("Indexer settings saved and applied.");
      } else {
        toast.notify("Indexer settings applied for this process. Add Postgres to persist them.", "warn");
      }
    } catch (error) {
      toast.error(errorMessage(error, "Indexer settings save failed"));
    }
  }

  async function refresh() {
    const result = await query.refetch();
    if (result.data) {
      setForm(integrationSettingsForm(result.data.settings));
    } else if (result.error) {
      toast.error(errorMessage(result.error, "Indexer settings refresh failed"));
    }
  }

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Indexer settings refresh failed" /> : null}
      {query.isSuccess && !persisted ? (
        <InlineNotice tone="warn">
          No database persistence — indexer settings apply to the running process only and reset on restart. Set
          LIBRARRY_DATABASE_URL to keep them.
        </InlineNotice>
      ) : null}
      <Card
        title="Prowlarr"
        subtitle={`${persisted ? "Postgres" : "runtime"} · ${saved.prowlarrUrl ? "indexer set" : "no indexer"}`}
      >
        {query.isLoading ? (
          <LoadingRow label="Loading indexer settings…" />
        ) : (
          <>
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
              <Field label={saved.prowlarrApiKeyConfigured ? "Prowlarr API key (saved)" : "Prowlarr API key"}>
                <SecretInput
                  value={form.prowlarrApiKey ?? ""}
                  onChange={(value) => update({ prowlarrApiKey: value })}
                  placeholder={saved.prowlarrApiKeyConfigured ? "Leave blank to keep" : "API key"}
                  ariaLabel="Prowlarr API key"
                />
              </Field>
            </FormGrid>
            <div className="form-actions">
              <Button
                variant="primary"
                icon={CheckCircle2}
                busy={save.isPending}
                disabled={save.isPending || !hasChanges}
                onClick={() => void persist()}
              >
                Save indexer
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
