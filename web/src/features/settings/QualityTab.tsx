import React, { useEffect, useState } from "react";
import { CheckCircle2, ListFilter, Pencil, Plus, RefreshCw, Ruler, Trash2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { Badge, Button, Card, DataTable, EmptyState, Field, FormGrid, IconButton, LoadingRow, Modal } from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  createReleaseProfile,
  deleteReleaseProfile,
  saveQualityDefinitions,
  updateReleaseProfile,
  type QualityDefinition,
  type ReleaseProfile,
  type ReleaseProfilePreferred,
  type ReleaseProfileRequest
} from "../../lib/api";
import { qualityKeys, useQualityDefinitions, useReleaseProfiles } from "../../lib/queries";
import { QueryErrorNotice } from "./controls";
import { errorMessage, splitTerms } from "./helpers";

/* ----------------------------- Quality definitions ------------------------- */

function definitionsChanged(form: QualityDefinition[], saved: QualityDefinition[]) {
  if (form.length !== saved.length) return true;
  return form.some((definition, index) => {
    const base = saved[index];
    return (
      definition.quality !== base.quality ||
      definition.title.trim() !== base.title.trim() ||
      definition.minSizeMB !== base.minSizeMB ||
      definition.maxSizeMB !== base.maxSizeMB
    );
  });
}

/**
 * Size limits per quality: releases outside [min, max] MB are rejected during
 * scoring. Edits are applied together through the bulk PUT.
 */
function QualityDefinitionsCard() {
  const toast = useToast();
  const client = useQueryClient();
  const query = useQualityDefinitions();
  const saved = query.data ?? [];

  const [rows, setRows] = useState<QualityDefinition[]>([]);
  const [saving, setSaving] = useState(false);
  const fetched = query.data;
  useEffect(() => {
    if (fetched) setRows(fetched.map((definition) => ({ ...definition })));
  }, [fetched]);

  const hasChanges = definitionsChanged(rows, saved);

  function updateRowAt(index: number, changes: Partial<QualityDefinition>) {
    setRows((current) => current.map((row, rowIndex) => (rowIndex === index ? { ...row, ...changes } : row)));
  }

  async function persist() {
    if (!hasChanges) return;
    setSaving(true);
    try {
      const definitions = await saveQualityDefinitions(rows.map((row) => ({ ...row, title: row.title.trim() })));
      setRows(definitions.map((definition) => ({ ...definition })));
      client.setQueryData(qualityKeys.qualityDefinitions, definitions);
      toast.success("Quality definitions saved.");
    } catch (error) {
      toast.error(errorMessage(error, "Quality definitions save failed"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card
      title="Quality definitions"
      subtitle="Size limits per quality — releases outside the range are rejected."
      padded={rows.length === 0}
      actions={
        <Button size="sm" variant="primary" icon={CheckCircle2} busy={saving} disabled={saving || !hasChanges} onClick={() => void persist()}>
          Save definitions
        </Button>
      }
    >
      {query.isLoading ? (
        <LoadingRow label="Loading quality definitions…" />
      ) : query.isError ? (
        <QueryErrorNotice error={query.error} fallback="Quality definitions refresh failed" />
      ) : rows.length === 0 ? (
        <EmptyState icon={Ruler} title="No quality definitions">
          They appear once the API exposes the quality definitions endpoint.
        </EmptyState>
      ) : (
        <DataTable>
          <thead>
            <tr>
              <th>Quality</th>
              <th>Title</th>
              <th>Min (MB)</th>
              <th>Max (MB)</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => (
              <tr key={row.quality}>
                <td className="cell-muted">
                  <code>{row.quality}</code>
                </td>
                <td>
                  <input
                    value={row.title}
                    onChange={(event) => updateRowAt(index, { title: event.target.value })}
                    aria-label={`Title for ${row.quality}`}
                  />
                </td>
                <td>
                  <input
                    className="settings-size-input"
                    inputMode="decimal"
                    value={row.minSizeMB}
                    onChange={(event) => updateRowAt(index, { minSizeMB: Number(event.target.value) || 0 })}
                    aria-label={`Minimum size in MB for ${row.quality}`}
                  />
                </td>
                <td>
                  <input
                    className="settings-size-input"
                    inputMode="decimal"
                    value={row.maxSizeMB}
                    onChange={(event) => updateRowAt(index, { maxSizeMB: Number(event.target.value) || 0 })}
                    aria-label={`Maximum size in MB for ${row.quality}`}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      )}
    </Card>
  );
}

/* ------------------------------ Release profiles --------------------------- */

type ReleaseProfileDraft = {
  id?: string;
  name: string;
  enabled: boolean;
  requiredText: string;
  ignoredText: string;
  preferred: ReleaseProfilePreferred[];
};

function draftFromProfile(profile?: ReleaseProfile): ReleaseProfileDraft {
  return {
    id: profile?.id,
    name: profile?.name ?? "",
    enabled: profile?.enabled ?? true,
    requiredText: (profile?.required ?? []).join(", "),
    ignoredText: (profile?.ignored ?? []).join(", "),
    preferred: (profile?.preferred ?? []).map((term) => ({ ...term }))
  };
}

function requestFromDraft(draft: ReleaseProfileDraft): ReleaseProfileRequest {
  return {
    name: draft.name.trim(),
    enabled: draft.enabled,
    required: splitTerms(draft.requiredText),
    ignored: splitTerms(draft.ignoredText),
    preferred: draft.preferred
      .filter((term) => term.term.trim())
      .map((term) => ({ term: term.term.trim(), score: term.score }))
  };
}

function requestFromProfile(profile: ReleaseProfile): ReleaseProfileRequest {
  return {
    name: profile.name,
    enabled: profile.enabled,
    required: profile.required ?? [],
    ignored: profile.ignored ?? [],
    preferred: profile.preferred ?? []
  };
}

/**
 * Term rules applied to every release decision: required/ignored gate the
 * grab, preferred terms adjust the score. Rules live outside quality
 * profiles so one list applies across formats (arr release-profile model).
 */
function ReleaseProfilesCard() {
  const toast = useToast();
  const client = useQueryClient();
  const query = useReleaseProfiles();
  const profiles = query.data ?? [];

  const [draft, setDraft] = useState<ReleaseProfileDraft | null>(null);
  const [saving, setSaving] = useState(false);
  const [togglingId, setTogglingId] = useState("");
  const [deleting, setDeleting] = useState<ReleaseProfile | null>(null);
  const [removing, setRemoving] = useState(false);

  function invalidate() {
    return client.invalidateQueries({ queryKey: qualityKeys.releaseProfiles });
  }

  function updateDraft(changes: Partial<ReleaseProfileDraft>) {
    setDraft((current) => (current ? { ...current, ...changes } : current));
  }

  function updatePreferredAt(index: number, changes: Partial<ReleaseProfilePreferred>) {
    setDraft((current) => {
      if (!current) return current;
      const preferred = current.preferred.map((term, termIndex) => (termIndex === index ? { ...term, ...changes } : term));
      return { ...current, preferred };
    });
  }

  function removePreferredAt(index: number) {
    setDraft((current) =>
      current ? { ...current, preferred: current.preferred.filter((_, termIndex) => termIndex !== index) } : current
    );
  }

  async function persistDraft() {
    if (!draft || !draft.name.trim()) return;
    setSaving(true);
    try {
      const request = requestFromDraft(draft);
      const profile = draft.id ? await updateReleaseProfile(draft.id, request) : await createReleaseProfile(request);
      toast.success(`Release profile "${profile.name}" saved.`);
      setDraft(null);
      await invalidate();
    } catch (error) {
      toast.error(errorMessage(error, "Release profile save failed"));
    } finally {
      setSaving(false);
    }
  }

  async function toggleEnabled(profile: ReleaseProfile) {
    setTogglingId(profile.id);
    try {
      const updated = await updateReleaseProfile(profile.id, { ...requestFromProfile(profile), enabled: !profile.enabled });
      toast.success(`Release profile "${updated.name}" ${updated.enabled ? "enabled" : "disabled"}.`);
      await invalidate();
    } catch (error) {
      toast.error(errorMessage(error, "Release profile update failed"));
    } finally {
      setTogglingId("");
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    setRemoving(true);
    try {
      await deleteReleaseProfile(deleting.id);
      toast.success(`Release profile "${deleting.name}" deleted.`);
      setDeleting(null);
      await invalidate();
    } catch (error) {
      toast.error(errorMessage(error, "Release profile delete failed"));
    } finally {
      setRemoving(false);
    }
  }

  return (
    <Card
      title="Release Profiles"
      subtitle="Required, ignored, and preferred release terms applied to every grab decision."
      padded={profiles.length === 0}
      actions={
        <Button size="sm" icon={Plus} onClick={() => setDraft(draftFromProfile())}>
          Add release profile
        </Button>
      }
    >
      {query.isLoading ? (
        <LoadingRow label="Loading release profiles…" />
      ) : query.isError ? (
        <QueryErrorNotice error={query.error} fallback="Release profiles refresh failed" />
      ) : profiles.length === 0 ? (
        <EmptyState
          icon={ListFilter}
          title="No release profiles yet"
          actions={
            <Button size="sm" icon={Plus} onClick={() => setDraft(draftFromProfile())}>
              Add release profile
            </Button>
          }
        >
          Add required, ignored, or preferred terms to steer which releases get grabbed.
        </EmptyState>
      ) : (
        <DataTable>
          <thead>
            <tr>
              <th>Name</th>
              <th>Enabled</th>
              <th>Required</th>
              <th>Ignored</th>
              <th>Preferred</th>
              <th aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            {profiles.map((profile) => (
              <tr key={profile.id}>
                <td className="cell-primary">
                  <strong>{profile.name}</strong>
                </td>
                <td>
                  <div className="settings-check">
                    <input
                      type="checkbox"
                      checked={profile.enabled}
                      disabled={Boolean(togglingId)}
                      onChange={() => void toggleEnabled(profile)}
                      aria-label={`${profile.enabled ? "Disable" : "Enable"} ${profile.name}`}
                    />
                    <Badge tone={profile.enabled ? "success" : "neutral"}>{profile.enabled ? "On" : "Off"}</Badge>
                  </div>
                </td>
                <td className="cell-muted">{(profile.required ?? []).length}</td>
                <td className="cell-muted">{(profile.ignored ?? []).length}</td>
                <td className="cell-muted">{(profile.preferred ?? []).length}</td>
                <td>
                  <div className="cell-actions">
                    <IconButton icon={Pencil} size="sm" label={`Edit ${profile.name}`} onClick={() => setDraft(draftFromProfile(profile))} />
                    <IconButton
                      icon={Trash2}
                      size="sm"
                      tone="danger"
                      label={`Delete ${profile.name}`}
                      disabled={removing}
                      onClick={() => setDeleting(profile)}
                    />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      )}

      <Modal
        title={draft?.id ? "Edit release profile" : "Add release profile"}
        open={Boolean(draft)}
        onClose={() => setDraft(null)}
        wide
        footer={
          <>
            <Button onClick={() => setDraft(null)}>Cancel</Button>
            <Button
              variant="primary"
              icon={CheckCircle2}
              busy={saving}
              disabled={saving || !draft?.name.trim()}
              onClick={() => void persistDraft()}
            >
              Save
            </Button>
          </>
        }
      >
        {draft ? (
          <>
            <FormGrid columns={2}>
              <Field label="Name">
                <input value={draft.name} onChange={(event) => updateDraft({ name: event.target.value })} placeholder="Profile name" />
              </Field>
              <Field label="Enabled" hint="Disabled profiles are kept but not applied.">
                <div className="settings-check">
                  <input
                    type="checkbox"
                    checked={draft.enabled}
                    onChange={(event) => updateDraft({ enabled: event.target.checked })}
                  />
                  <span>{draft.enabled ? "Enabled" : "Disabled"}</span>
                </div>
              </Field>
              <Field label="Required terms" hint="Comma separated — every term must match.">
                <input
                  value={draft.requiredText}
                  onChange={(event) => updateDraft({ requiredText: event.target.value })}
                  placeholder="retail, epub"
                />
              </Field>
              <Field label="Ignored terms" hint="Comma separated — any match rejects the release.">
                <input
                  value={draft.ignoredText}
                  onChange={(event) => updateDraft({ ignoredText: event.target.value })}
                  placeholder="sample, abridged"
                />
              </Field>
            </FormGrid>
            <div className="settings-term-list" aria-label="Preferred terms">
              <span className="field-label">Preferred terms</span>
              <span className="field-hint">Each matching term adds its score to the release (negative scores penalize).</span>
              {draft.preferred.map((term, index) => (
                <div className="settings-term-row" key={index}>
                  <input
                    value={term.term}
                    onChange={(event) => updatePreferredAt(index, { term: event.target.value })}
                    placeholder="Term"
                    aria-label={`Preferred term ${index + 1}`}
                  />
                  <input
                    className="settings-size-input"
                    inputMode="decimal"
                    value={term.score}
                    onChange={(event) => updatePreferredAt(index, { score: Number(event.target.value) || 0 })}
                    aria-label={`Score for preferred term ${index + 1}`}
                  />
                  <IconButton icon={Trash2} size="sm" tone="danger" label={`Remove preferred term ${index + 1}`} onClick={() => removePreferredAt(index)} />
                </div>
              ))}
              <div>
                <Button
                  size="sm"
                  icon={Plus}
                  onClick={() => updateDraft({ preferred: [...draft.preferred, { term: "", score: 10 }] })}
                >
                  Add preferred term
                </Button>
              </div>
            </div>
          </>
        ) : null}
      </Modal>

      <Modal
        title="Delete release profile"
        open={Boolean(deleting)}
        onClose={() => setDeleting(null)}
        footer={
          <>
            <Button onClick={() => setDeleting(null)}>Cancel</Button>
            <Button variant="danger" icon={Trash2} busy={removing} onClick={() => void confirmDelete()}>
              Delete
            </Button>
          </>
        }
      >
        <p>
          Delete release profile <strong>{deleting?.name}</strong>? Its required, ignored, and preferred terms stop
          applying to release decisions.
        </p>
      </Modal>
    </Card>
  );
}

/* ---------------------------------- Tab ------------------------------------ */

/** Quality → size definitions per quality plus the release-term profiles. */
export function QualityTab() {
  return (
    <>
      <QualityDefinitionsCard />
      <ReleaseProfilesCard />
    </>
  );
}
