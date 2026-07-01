import React, { useEffect, useMemo, useRef, useState } from "react";
import { CheckCircle2, Plus, RefreshCw, SlidersHorizontal, Trash2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { Badge, Button, Card, EmptyState, Field, FormGrid, IconButton, LoadingRow } from "../../components/ui";
import { useToast } from "../../components/toast";
import { saveQualityProfile, type QualityProfile } from "../../lib/api";
import { keys, useQualityProfiles } from "../../lib/queries";
import { QueryErrorNotice } from "./controls";
import {
  bytesToGiB,
  cloneQualityProfile,
  errorMessage,
  giBToBytes,
  profileKey,
  qualityProfileChanged,
  splitTerms
} from "./helpers";

type ProfileRow = {
  /** Stable render/edit key — profileKey for persisted rows, a counter for new ones. */
  localKey: string;
  profile: QualityProfile;
  isNew?: boolean;
  /** Raw comma-separated text so users can type commas without them being normalized away. */
  termsText: { preferred: string; required: string; rejected: string };
};

function rowFromProfile(profile: QualityProfile): ProfileRow {
  return {
    localKey: profileKey(profile),
    profile: cloneQualityProfile(profile),
    termsText: {
      preferred: (profile.preferredTerms ?? []).join(", "),
      required: (profile.requiredTerms ?? []).join(", "),
      rejected: (profile.rejectedTerms ?? []).join(", ")
    }
  };
}

const mediaFormatOptions: QualityProfile["mediaFormat"][] = ["any", "ebook", "audiobook"];

/**
 * Profiles → release policy editor. Scores, seeders, size cap, term lists,
 * and upgrade policy per profile; profiles are keyed by id (or name:format)
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

  function updateRow(localKey: string, changes: Partial<QualityProfile>, termsText?: Partial<ProfileRow["termsText"]>) {
    setRows((current) =>
      current.map((row) =>
        row.localKey === localKey
          ? { ...row, profile: { ...row.profile, ...changes }, termsText: { ...row.termsText, ...termsText } }
          : row
      )
    );
  }

  function addProfile() {
    const profile: QualityProfile = {
      name: "",
      mediaFormat: "any",
      minScore: 0,
      cutoffScore: 80,
      minSeeders: 1,
      maxSizeBytes: giBToBytes(1),
      preferredTerms: [],
      requiredTerms: [],
      rejectedTerms: [],
      preferredScore: 10,
      upgradeAllowed: true
    };
    setRows((current) => [
      ...current,
      { localKey: `new-${newProfileCounter.current++}`, profile, isNew: true, termsText: { preferred: "", required: "", rejected: "" } }
    ]);
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
        subtitle={`${savedCount} release policy profile${savedCount === 1 ? "" : "s"} used by search, feeds, recovery, and upgrades.`}
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
          <EmptyState icon={SlidersHorizontal} title="No release policy profiles yet">
            They appear once Postgres persistence and the API are configured.
          </EmptyState>
        ) : (
          <div className="settings-profile-list">
            {rows.map((row) => {
              const changed = qualityProfileChanged(row.profile, savedByKey.get(row.localKey));
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
                            onChange={(event) =>
                              updateRow(row.localKey, { mediaFormat: event.target.value as QualityProfile["mediaFormat"] })
                            }
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
                        disabled={Boolean(savingKey) || !changed || (row.isNew && !row.profile.name.trim())}
                        onClick={() => void persistRow(row)}
                      >
                        Save
                      </Button>
                      {row.isNew ? (
                        <IconButton icon={Trash2} label="Discard new profile" tone="danger" size="sm" onClick={() => discardRow(row.localKey)} />
                      ) : null}
                    </div>
                  </div>
                  <FormGrid columns={3}>
                    <Field label="Min score" hint="Releases scoring below this are rejected.">
                      <input
                        inputMode="decimal"
                        value={row.profile.minScore}
                        onChange={(event) => updateRow(row.localKey, { minScore: Number(event.target.value) || 0 })}
                      />
                    </Field>
                    <Field label="Cutoff score" hint="Upgrades stop once a file reaches this score.">
                      <input
                        inputMode="decimal"
                        value={row.profile.cutoffScore}
                        onChange={(event) => updateRow(row.localKey, { cutoffScore: Number(event.target.value) || 0 })}
                      />
                    </Field>
                    <Field label="Preferred score" hint="Bonus applied per preferred-term match.">
                      <input
                        inputMode="decimal"
                        value={row.profile.preferredScore}
                        onChange={(event) => updateRow(row.localKey, { preferredScore: Number(event.target.value) || 0 })}
                      />
                    </Field>
                    <Field label="Min seeders" hint="Minimum torrent seeders to grab.">
                      <input
                        inputMode="numeric"
                        value={row.profile.minSeeders}
                        onChange={(event) => updateRow(row.localKey, { minSeeders: Number(event.target.value) || 0 })}
                      />
                    </Field>
                    <Field label="Max size (GB)" hint="Releases above this size are rejected.">
                      <input
                        inputMode="decimal"
                        value={bytesToGiB(row.profile.maxSizeBytes)}
                        onChange={(event) => updateRow(row.localKey, { maxSizeBytes: giBToBytes(Number(event.target.value) || 0) })}
                      />
                    </Field>
                    <Field label="Upgrade allowed" hint="Replace existing files when a better release appears.">
                      <div className="settings-check">
                        <input
                          type="checkbox"
                          checked={row.profile.upgradeAllowed}
                          onChange={(event) => updateRow(row.localKey, { upgradeAllowed: event.target.checked })}
                        />
                        <span>{row.profile.upgradeAllowed ? "Enabled" : "Disabled"}</span>
                      </div>
                    </Field>
                    <Field label="Preferred terms" hint="Comma separated — boost matching releases.">
                      <input
                        value={row.termsText.preferred}
                        onChange={(event) =>
                          updateRow(row.localKey, { preferredTerms: splitTerms(event.target.value) }, { preferred: event.target.value })
                        }
                        placeholder="epub, retail"
                      />
                    </Field>
                    <Field label="Required terms" hint="Comma separated — every term must match.">
                      <input
                        value={row.termsText.required}
                        onChange={(event) =>
                          updateRow(row.localKey, { requiredTerms: splitTerms(event.target.value) }, { required: event.target.value })
                        }
                        placeholder="Optional"
                      />
                    </Field>
                    <Field label="Rejected terms" hint="Comma separated — any match rejects the release.">
                      <input
                        value={row.termsText.rejected}
                        onChange={(event) =>
                          updateRow(row.localKey, { rejectedTerms: splitTerms(event.target.value) }, { rejected: event.target.value })
                        }
                        placeholder="sample, abridged"
                      />
                    </Field>
                  </FormGrid>
                </article>
              );
            })}
          </div>
        )}
      </Card>
    </>
  );
}
