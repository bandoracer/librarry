import React, { useEffect, useMemo, useRef, useState } from "react";
import { CheckCircle2, ChevronDown, ChevronUp, Filter, Pencil, Plus, RefreshCw, SlidersHorizontal, Trash2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { Badge, Button, Card, DataTable, EmptyState, Field, FormGrid, IconButton, LoadingRow, Modal } from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  createMetadataProfile,
  deleteMetadataProfile,
  qualityTitle,
  saveQualityProfile,
  updateMetadataProfile,
  type MetadataProfile,
  type MetadataProfileRequest,
  type QualityProfile
} from "../../lib/api";
import { keys, metadataProfileKeys, useInvalidatingMutation, useMetadataProfiles, useQualityProfiles } from "../../lib/queries";
import { QueryErrorNotice } from "./controls";
import { cloneQualityProfile, defaultProfileQualities, errorMessage, profileKey, qualityProfileChanged } from "./helpers";

type ProfileRow = {
  /** Stable render/edit key — profileKey for persisted rows, a counter for new ones. */
  localKey: string;
  profile: QualityProfile;
  isNew?: boolean;
};

function rowFromProfile(profile: QualityProfile): ProfileRow {
  return {
    localKey: profileKey(profile),
    profile: cloneQualityProfile(profile)
  };
}

const mediaFormatOptions: QualityProfile["mediaFormat"][] = ["any", "ebook", "audiobook"];

/** First allowed quality id, used when the cutoff quality gets disallowed. */
function firstAllowedQuality(profile: QualityProfile): string {
  return profile.qualities.find((quality) => quality.allowed)?.id ?? "";
}

function cutoffIsAllowed(profile: QualityProfile): boolean {
  return profile.qualities.some((quality) => quality.allowed && quality.id === profile.cutoff);
}

/**
 * Profiles → arr-style quality profile editor. Each profile orders its
 * qualities most-preferred first, toggles which are allowed, and picks the
 * cutoff quality upgrades stop at; profiles are keyed by id (or name:format)
 * and saved individually, exactly like the legacy per-row save.
 */
export function ProfilesTab() {
  const toast = useToast();
  const client = useQueryClient();
  const query = useQualityProfiles();
  const data = query.data;

  const [rows, setRows] = useState<ProfileRow[]>([]);
  const [savingKey, setSavingKey] = useState("");
  const newProfileCounter = useRef(1);

  const savedByKey = useMemo(() => {
    const map = new Map<string, QualityProfile>();
    (data ?? []).forEach((profile) => map.set(profileKey(profile), profile));
    return map;
  }, [data]);

  // Sync rows from the query cache, keeping unsaved edits and new rows intact.
  useEffect(() => {
    const fresh = data ?? [];
    setRows((current) => {
      const currentByKey = new Map(current.map((row) => [row.localKey, row]));
      const freshKeys = new Set(fresh.map((profile) => profileKey(profile)));
      const next = fresh.map((profile) => {
        const existing = currentByKey.get(profileKey(profile));
        if (existing && qualityProfileChanged(existing.profile, profile)) return existing;
        return rowFromProfile(profile);
      });
      for (const row of current) {
        if (row.isNew && !freshKeys.has(profileKey(row.profile))) next.push(row);
      }
      return next;
    });
  }, [data]);

  function updateRow(localKey: string, changes: Partial<QualityProfile>) {
    setRows((current) =>
      current.map((row) => (row.localKey === localKey ? { ...row, profile: { ...row.profile, ...changes } } : row))
    );
  }

  function changeMediaFormat(row: ProfileRow, mediaFormat: QualityProfile["mediaFormat"]) {
    const qualities = defaultProfileQualities(mediaFormat);
    updateRow(row.localKey, { mediaFormat, qualities, cutoff: qualities[0]?.id ?? "" });
  }

  /** Move a quality one step toward most-preferred (delta -1) or least (delta +1). */
  function moveQuality(row: ProfileRow, index: number, delta: -1 | 1) {
    const target = index + delta;
    if (target < 0 || target >= row.profile.qualities.length) return;
    const qualities = row.profile.qualities.map((quality) => ({ ...quality }));
    [qualities[index], qualities[target]] = [qualities[target], qualities[index]];
    updateRow(row.localKey, { qualities });
  }

  function toggleQuality(row: ProfileRow, id: string, allowed: boolean) {
    const qualities = row.profile.qualities.map((quality) => (quality.id === id ? { ...quality, allowed } : quality));
    const next: QualityProfile = { ...row.profile, qualities };
    if (!cutoffIsAllowed(next)) next.cutoff = firstAllowedQuality(next);
    updateRow(row.localKey, { qualities: next.qualities, cutoff: next.cutoff });
  }

  function addProfile() {
    const qualities = defaultProfileQualities("any");
    const profile: QualityProfile = {
      name: "",
      mediaFormat: "any",
      qualities,
      cutoff: qualities[0]?.id ?? "",
      upgradeAllowed: true,
      minSeeders: 1
    };
    setRows((current) => [...current, { localKey: `new-${newProfileCounter.current++}`, profile, isNew: true }]);
  }

  function discardRow(localKey: string) {
    setRows((current) => current.filter((row) => row.localKey !== localKey));
  }

  async function persistRow(row: ProfileRow) {
    if (!qualityProfileChanged(row.profile, savedByKey.get(row.localKey))) return;
    setSavingKey(row.localKey);
    try {
      const result = await saveQualityProfile(row.profile);
      setRows((current) => current.map((item) => (item.localKey === row.localKey ? rowFromProfile(result) : item)));
      client.setQueryData<QualityProfile[]>(keys.qualityProfiles, (current) => {
        const list = current ? [...current] : [];
        const key = profileKey(result);
        const index = list.findIndex((item) => profileKey(item) === key);
        if (index >= 0) list[index] = result;
        else list.push(result);
        return list;
      });
      await client.invalidateQueries({ queryKey: keys.readiness });
      toast.success(`Quality profile "${result.name}" saved.`);
    } catch (error) {
      toast.error(errorMessage(error, "Quality profile save failed"));
    } finally {
      setSavingKey("");
    }
  }

  async function refresh() {
    const result = await query.refetch();
    if (result.data) {
      setRows(result.data.map(rowFromProfile));
    } else if (result.error) {
      toast.error(errorMessage(result.error, "Quality profiles refresh failed"));
    }
  }

  const savedCount = rows.filter((row) => !row.isNew).length;

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Quality profiles refresh failed" /> : null}
      <Card
        title="Quality profiles"
        subtitle={`${savedCount} profile${savedCount === 1 ? "" : "s"} deciding which qualities are grabbed and when upgrades stop.`}
        actions={
          <>
            <Button size="sm" icon={Plus} onClick={addProfile}>
              Add profile
            </Button>
            <Button size="sm" icon={RefreshCw} disabled={Boolean(savingKey)} onClick={() => void refresh()}>
              Refresh
            </Button>
          </>
        }
      >
        {query.isLoading ? (
          <LoadingRow label="Loading quality profiles…" />
        ) : rows.length === 0 ? (
          <EmptyState icon={SlidersHorizontal} title="No quality profiles yet">
            They appear once Postgres persistence and the API are configured.
          </EmptyState>
        ) : (
          <div className="settings-profile-list">
            {rows.map((row) => {
              const changed = qualityProfileChanged(row.profile, savedByKey.get(row.localKey));
              const allowedQualities = row.profile.qualities.filter((quality) => quality.allowed);
              const cutoffValid = cutoffIsAllowed(row.profile);
              const saveable = changed && allowedQualities.length > 0 && cutoffValid && (!row.isNew || Boolean(row.profile.name.trim()));
              return (
                <article className="settings-profile" key={row.localKey}>
                  <div className="settings-profile-head">
                    <div className="settings-profile-title">
                      {row.isNew ? (
                        <>
                          <input
                            value={row.profile.name}
                            onChange={(event) => updateRow(row.localKey, { name: event.target.value })}
                            placeholder="Profile name"
                            aria-label="Profile name"
                          />
                          <select
                            value={row.profile.mediaFormat}
                            onChange={(event) => changeMediaFormat(row, event.target.value as QualityProfile["mediaFormat"])}
                            aria-label="Media format"
                          >
                            {mediaFormatOptions.map((format) => (
                              <option key={format} value={format}>
                                {format}
                              </option>
                            ))}
                          </select>
                          <Badge tone="accent">New</Badge>
                        </>
                      ) : (
                        <>
                          <strong>{row.profile.name}</strong>
                          <Badge>{row.profile.mediaFormat}</Badge>
                          {changed ? <Badge tone="warn">Unsaved</Badge> : null}
                        </>
                      )}
                    </div>
                    <div className="settings-profile-actions">
                      <Button
                        size="sm"
                        variant="primary"
                        icon={CheckCircle2}
                        busy={savingKey === row.localKey}
                        disabled={Boolean(savingKey) || !saveable}
                        onClick={() => void persistRow(row)}
                      >
                        Save
                      </Button>
                      {row.isNew ? (
                        <IconButton icon={Trash2} label="Discard new profile" tone="danger" size="sm" onClick={() => discardRow(row.localKey)} />
                      ) : null}
                    </div>
                  </div>
                  <div className="settings-quality-list" aria-label={`Qualities for ${row.profile.name || "new profile"}`}>
                    <span className="field-hint">Most preferred quality on top; unchecked qualities are never grabbed.</span>
                    {row.profile.qualities.map((quality, index) => (
                      <div className="settings-quality-row" key={quality.id}>
                        <IconButton
                          icon={ChevronUp}
                          label={`Move ${qualityTitle(quality.id)} up`}
                          size="sm"
                          disabled={index === 0}
                          onClick={() => moveQuality(row, index, -1)}
                        />
                        <IconButton
                          icon={ChevronDown}
                          label={`Move ${qualityTitle(quality.id)} down`}
                          size="sm"
                          disabled={index === row.profile.qualities.length - 1}
                          onClick={() => moveQuality(row, index, 1)}
                        />
                        <label className="settings-check">
                          <input
                            type="checkbox"
                            checked={quality.allowed}
                            onChange={(event) => toggleQuality(row, quality.id, event.target.checked)}
                          />
                          <span>{qualityTitle(quality.id)}</span>
                        </label>
                      </div>
                    ))}
                  </div>
                  <FormGrid columns={3}>
                    <Field label="Cutoff" hint="Upgrades stop once a file reaches this quality.">
                      <select
                        value={row.profile.cutoff}
                        onChange={(event) => updateRow(row.localKey, { cutoff: event.target.value })}
                        aria-label="Cutoff quality"
                      >
                        {allowedQualities.length === 0 ? <option value="">No qualities allowed</option> : null}
                        {!cutoffValid && row.profile.cutoff ? (
                          <option value={row.profile.cutoff}>{qualityTitle(row.profile.cutoff)} (not allowed)</option>
                        ) : null}
                        {allowedQualities.map((quality) => (
                          <option key={quality.id} value={quality.id}>
                            {qualityTitle(quality.id)}
                          </option>
                        ))}
                      </select>
                    </Field>
                    <Field label="Min seeders" hint="Minimum torrent seeders to grab.">
                      <input
                        inputMode="numeric"
                        value={row.profile.minSeeders}
                        onChange={(event) => updateRow(row.localKey, { minSeeders: Number(event.target.value) || 0 })}
                      />
                    </Field>
                    <Field label="Upgrade allowed" hint="Replace existing files when a better quality appears.">
                      <div className="settings-check">
                        <input
                          type="checkbox"
                          checked={row.profile.upgradeAllowed}
                          onChange={(event) => updateRow(row.localKey, { upgradeAllowed: event.target.checked })}
                        />
                        <span>{row.profile.upgradeAllowed ? "Enabled" : "Disabled"}</span>
                      </div>
                    </Field>
                  </FormGrid>
                </article>
              );
            })}
          </div>
        )}
      </Card>

      <MetadataProfilesCard />
    </>
  );
}

/* --------------------------- Metadata profiles (wave B) --------------------- */

/** Editable string form of a metadata profile (comma inputs, number text). */
type MetadataProfileForm = {
  id?: string;
  name: string;
  allowedLanguages: string;
  mustNotContain: string;
  skipMissingIsbn: boolean;
  minPages: string;
};

function emptyMetadataProfileForm(): MetadataProfileForm {
  return { name: "", allowedLanguages: "", mustNotContain: "", skipMissingIsbn: false, minPages: "" };
}

function metadataProfileToForm(profile: MetadataProfile): MetadataProfileForm {
  return {
    id: profile.id,
    name: profile.name,
    allowedLanguages: (profile.allowedLanguages ?? []).join(", "),
    mustNotContain: (profile.mustNotContain ?? []).join(", "),
    skipMissingIsbn: profile.skipMissingIsbn,
    minPages: profile.minPages > 0 ? String(profile.minPages) : ""
  };
}

function splitCommaTerms(value: string): string[] {
  return value
    .split(",")
    .map((term) => term.trim())
    .filter(Boolean);
}

function metadataProfilePayload(form: MetadataProfileForm): MetadataProfileRequest {
  return {
    name: form.name.trim(),
    allowedLanguages: splitCommaTerms(form.allowedLanguages),
    mustNotContain: splitCommaTerms(form.mustNotContain),
    skipMissingIsbn: form.skipMissingIsbn,
    minPages: Math.max(0, Math.round(Number(form.minPages) || 0))
  };
}

function metadataProfileFilterSummary(profile: MetadataProfile): string {
  const parts: string[] = [];
  if (profile.allowedLanguages?.length) parts.push(`languages: ${profile.allowedLanguages.join(", ")}`);
  if (profile.mustNotContain?.length) parts.push(`must not contain: ${profile.mustNotContain.join(", ")}`);
  if (profile.skipMissingIsbn) parts.push("skip books without ISBN");
  if (profile.minPages > 0) parts.push(`min ${profile.minPages} pages`);
  return parts.length ? parts.join(" · ") : "No filters — allows every candidate";
}

/**
 * Profiles → Metadata Profiles: reusable filter sets applied when author
 * monitoring evaluates candidate books (per-author filter fields remain as
 * overrides). A profile still assigned to authors refuses deletion with a
 * 409 reason, surfaced as a toast.
 */
function MetadataProfilesCard() {
  const toast = useToast();
  const query = useMetadataProfiles();
  const profiles = query.data ?? [];

  const [form, setForm] = useState<MetadataProfileForm | null>(null);
  const [deleting, setDeleting] = useState<MetadataProfile | null>(null);

  const save = useInvalidatingMutation(
    (request: MetadataProfileForm) =>
      request.id ? updateMetadataProfile(request.id, metadataProfilePayload(request)) : createMetadataProfile(metadataProfilePayload(request)),
    [metadataProfileKeys.metadataProfiles]
  );
  const remove = useInvalidatingMutation(deleteMetadataProfile, [metadataProfileKeys.metadataProfiles]);

  function update(changes: Partial<MetadataProfileForm>) {
    setForm((current) => (current ? { ...current, ...changes } : current));
  }

  async function persist() {
    if (!form) return;
    const editing = Boolean(form.id);
    try {
      const saved = await save.mutateAsync(form);
      toast.success(`Metadata profile "${saved.name}" ${editing ? "updated" : "added"}.`);
      setForm(null);
    } catch (error) {
      toast.error(errorMessage(error, editing ? "Metadata profile update failed" : "Metadata profile create failed"));
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await remove.mutateAsync(deleting.id);
      toast.success(`Metadata profile "${deleting.name}" deleted.`);
      setDeleting(null);
    } catch (error) {
      // 409 refusals carry the reason (e.g. authors still assigned to it).
      toast.error(errorMessage(error, "Metadata profile delete failed"));
      setDeleting(null);
    }
  }

  const formValid = Boolean(form && form.name.trim());

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Metadata profiles refresh failed" /> : null}
      <Card
        title="Metadata Profiles"
        subtitle={`${profiles.length} profile${profiles.length === 1 ? "" : "s"} filtering which author-monitor candidates become wanted.`}
        padded={profiles.length === 0}
        actions={
          <Button size="sm" icon={Plus} onClick={() => setForm(emptyMetadataProfileForm())}>
            Add Metadata Profile
          </Button>
        }
      >
        {query.isLoading ? (
          <LoadingRow label="Loading metadata profiles…" />
        ) : profiles.length ? (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Filters</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {profiles.map((profile) => (
                <tr key={profile.id}>
                  <td className="cell-primary">{profile.name}</td>
                  <td className="cell-muted">{metadataProfileFilterSummary(profile)}</td>
                  <td>
                    <div className="cell-actions">
                      <IconButton
                        icon={Pencil}
                        size="sm"
                        label={`Edit metadata profile ${profile.name}`}
                        onClick={() => setForm(metadataProfileToForm(profile))}
                      />
                      <IconButton
                        icon={Trash2}
                        size="sm"
                        tone="danger"
                        label={`Delete metadata profile ${profile.name}`}
                        onClick={() => setDeleting(profile)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <EmptyState
            icon={Filter}
            title="No metadata profiles yet"
            actions={
              <Button size="sm" variant="primary" icon={Plus} onClick={() => setForm(emptyMetadataProfileForm())}>
                Add Metadata Profile
              </Button>
            }
          >
            Metadata profiles bundle language, title, ISBN, and page filters so author subscriptions can share one
            filter set instead of per-author overrides.
          </EmptyState>
        )}
      </Card>

      <Modal
        title={form?.id ? "Edit Metadata Profile" : "Add Metadata Profile"}
        open={Boolean(form)}
        onClose={() => setForm(null)}
        footer={
          <>
            <Button onClick={() => setForm(null)}>Cancel</Button>
            <Button variant="primary" busy={save.isPending} disabled={!formValid || save.isPending} onClick={() => void persist()}>
              {form?.id ? "Save Metadata Profile" : "Add Metadata Profile"}
            </Button>
          </>
        }
      >
        {form ? (
          <FormGrid columns={2}>
            <div className="settings-field-wide">
              <Field label="Name" hint="Display name for this filter set.">
                <input value={form.name} onChange={(event) => update({ name: event.target.value })} placeholder="English ebooks" />
              </Field>
            </div>
            <Field label="Allowed languages" hint="Comma-separated; leave empty to allow every language.">
              <input
                value={form.allowedLanguages}
                onChange={(event) => update({ allowedLanguages: event.target.value })}
                placeholder="English, German"
              />
            </Field>
            <Field label="Must not contain" hint="Comma-separated terms that reject a candidate title.">
              <input
                value={form.mustNotContain}
                onChange={(event) => update({ mustNotContain: event.target.value })}
                placeholder="omnibus, boxed set"
              />
            </Field>
            <Field label="Minimum pages" hint="0 disables; applies when the provider reports pages.">
              <input
                type="number"
                min={0}
                value={form.minPages}
                onChange={(event) => update({ minPages: event.target.value })}
                placeholder="0"
              />
            </Field>
            <label className="settings-check" style={{ alignSelf: "end", paddingBottom: 6 }}>
              <input
                type="checkbox"
                checked={form.skipMissingIsbn}
                onChange={(event) => update({ skipMissingIsbn: event.target.checked })}
              />
              <span>Skip books without ISBN</span>
            </label>
          </FormGrid>
        ) : null}
      </Modal>

      <Modal
        title="Delete metadata profile"
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
          Delete metadata profile <strong>{deleting?.name}</strong>? The server refuses the delete (with a reason) while
          author subscriptions still use it.
        </p>
      </Modal>
    </>
  );
}
