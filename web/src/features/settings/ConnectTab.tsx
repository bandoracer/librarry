import React, { useState } from "react";
import { BellRing, Pencil, Plus, Send, Trash2 } from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  Field,
  FormGrid,
  IconButton,
  LoadingRow,
  Modal
} from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  createNotificationTarget,
  deleteNotificationTarget,
  testNotificationTarget,
  updateNotificationTarget,
  type NotificationTarget,
  type NotificationTargetRequest,
  type NotificationTargetType,
  type NotificationTriggers
} from "../../lib/api";
import { operabilityKeys, useInvalidatingMutation, useNotificationTargets } from "../../lib/queries";
import { QueryErrorNotice, SecretInput } from "./controls";
import { errorMessage } from "./helpers";

/* ------------------------------- Type schema ------------------------------- */

type SettingsField = {
  key: string;
  label: string;
  placeholder?: string;
  hint?: string;
  /** Rendered with SecretInput; blank on edit means "keep saved value". */
  secret?: boolean;
  required?: boolean;
};

/** Per-type settings schema — adding a provider means adding a row here. */
const typeSchemas: Record<NotificationTargetType, { label: string; tone: "neutral" | "accent" | "success" | "info"; fields: SettingsField[] }> = {
  webhook: {
    label: "Webhook",
    tone: "neutral",
    fields: [{ key: "url", label: "Webhook URL", placeholder: "https://example.com/hooks/librarry", required: true }]
  },
  ntfy: {
    label: "ntfy",
    tone: "info",
    fields: [
      { key: "url", label: "Server URL", placeholder: "https://ntfy.sh", required: true },
      { key: "topic", label: "Topic", placeholder: "librarry", required: true },
      { key: "token", label: "Access token", secret: true, hint: "Optional for open topics." }
    ]
  },
  discord: {
    label: "Discord",
    tone: "accent",
    fields: [
      {
        key: "webhookUrl",
        label: "Discord webhook URL",
        placeholder: "https://discord.com/api/webhooks/…",
        required: true
      }
    ]
  },
  telegram: {
    label: "Telegram",
    tone: "success",
    fields: [
      { key: "botToken", label: "Bot token", secret: true, required: true },
      { key: "chatId", label: "Chat ID", placeholder: "-1001234567890", required: true }
    ]
  }
};

const triggerOptions: { key: keyof NotificationTriggers; label: string }[] = [
  { key: "onGrab", label: "On Grab" },
  { key: "onImport", label: "On Import" },
  { key: "onUpgrade", label: "On Upgrade" },
  { key: "onDownloadFailure", label: "On Download Failure" },
  { key: "onHealthIssue", label: "On Health Issue" }
];

const triggerChipLabel: Record<keyof NotificationTriggers, string> = {
  onGrab: "Grab",
  onImport: "Import",
  onUpgrade: "Upgrade",
  onDownloadFailure: "Failure",
  onHealthIssue: "Health"
};

/* --------------------------------- Form state ------------------------------ */

type TargetForm = NotificationTargetRequest & { id?: string };

function emptyTargetForm(): TargetForm {
  return {
    name: "",
    type: "webhook",
    settings: {},
    triggers: { onGrab: true, onImport: true, onUpgrade: true, onDownloadFailure: true, onHealthIssue: true },
    enabled: true
  };
}

function targetToForm(target: NotificationTarget): TargetForm {
  // Secrets are never echoed for editing: blank inputs mean "keep" on PUT.
  const settings: Record<string, string> = {};
  for (const field of typeSchemas[target.type].fields) {
    settings[field.key] = field.secret ? "" : (target.settings[field.key] ?? "");
  }
  return {
    id: target.id,
    name: target.name,
    type: target.type,
    settings,
    triggers: { ...target.triggers },
    enabled: target.enabled
  };
}

/** Settings payload for the selected type only, trimmed (blank secret = keep). */
function formSettingsPayload(form: TargetForm): Record<string, string> {
  const payload: Record<string, string> = {};
  for (const field of typeSchemas[form.type].fields) {
    payload[field.key] = (form.settings[field.key] ?? "").trim();
  }
  return payload;
}

function formValid(form: TargetForm, saved?: NotificationTarget): boolean {
  if (!form.name.trim()) return false;
  const editingSameType = Boolean(saved && saved.type === form.type);
  return typeSchemas[form.type].fields.every((field) => {
    if (!field.required) return true;
    if ((form.settings[field.key] ?? "").trim()) return true;
    // Required secrets may stay blank while editing — blank keeps the saved value.
    return Boolean(field.secret && editingSameType);
  });
}

/* ----------------------------------- Tab ----------------------------------- */

/**
 * Settings → Connect: notification targets (webhook / ntfy / Discord /
 * Telegram) with per-event triggers, enable toggles, and a test button.
 * Mirrors the arr Connect surface; providers are schema-driven.
 */
export function ConnectTab() {
  const toast = useToast();
  const query = useNotificationTargets();
  const targets = query.data ?? [];

  const [form, setForm] = useState<TargetForm | null>(null);
  const [deleting, setDeleting] = useState<NotificationTarget | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);

  const save = useInvalidatingMutation(
    (request: TargetForm) => {
      const payload: NotificationTargetRequest = {
        name: request.name.trim(),
        type: request.type,
        settings: formSettingsPayload(request),
        triggers: request.triggers,
        enabled: request.enabled
      };
      return request.id ? updateNotificationTarget(request.id, payload) : createNotificationTarget(payload);
    },
    [operabilityKeys.notificationTargets]
  );
  const remove = useInvalidatingMutation(deleteNotificationTarget, [operabilityKeys.notificationTargets]);
  const toggle = useInvalidatingMutation(
    (target: NotificationTarget) =>
      updateNotificationTarget(target.id, {
        name: target.name,
        type: target.type,
        // Echo stored settings untouched; blank secrets keep their saved values.
        settings: target.settings ?? {},
        triggers: target.triggers,
        enabled: !target.enabled
      }),
    [operabilityKeys.notificationTargets]
  );

  const editingSaved = form?.id ? targets.find((target) => target.id === form.id) : undefined;

  function update(changes: Partial<TargetForm>) {
    setForm((current) => (current ? { ...current, ...changes } : current));
  }

  function updateSetting(key: string, value: string) {
    setForm((current) => (current ? { ...current, settings: { ...current.settings, [key]: value } } : current));
  }

  function updateTrigger(key: keyof NotificationTriggers, value: boolean) {
    setForm((current) => (current ? { ...current, triggers: { ...current.triggers, [key]: value } } : current));
  }

  async function persist() {
    if (!form) return;
    const editing = Boolean(form.id);
    try {
      const saved = await save.mutateAsync(form);
      toast.success(`Connection “${saved.name}” ${editing ? "updated" : "added"}.`);
      setForm(null);
    } catch (error) {
      toast.error(errorMessage(error, editing ? "Connection update failed" : "Connection create failed"));
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await remove.mutateAsync(deleting.id);
      toast.success(`Connection “${deleting.name}” deleted.`);
      setDeleting(null);
    } catch (error) {
      toast.error(errorMessage(error, "Connection delete failed"));
    }
  }

  async function toggleEnabled(target: NotificationTarget) {
    try {
      const saved = await toggle.mutateAsync(target);
      toast.success(`Connection “${saved.name}” ${saved.enabled ? "enabled" : "disabled"}.`);
    } catch (error) {
      toast.error(errorMessage(error, "Connection update failed"));
    }
  }

  async function runTest(target: NotificationTarget) {
    setTestingId(target.id);
    try {
      const outcome = await testNotificationTarget(target.id);
      if (outcome.ok) {
        toast.success(`Test notification sent to “${target.name}”.`);
      } else {
        toast.error(outcome.error || `Test notification to “${target.name}” failed.`);
      }
    } catch (error) {
      toast.error(errorMessage(error, `Test notification to “${target.name}” failed`));
    } finally {
      setTestingId(null);
    }
  }

  function triggersSummary(target: NotificationTarget) {
    const active = triggerOptions.filter((option) => target.triggers?.[option.key]);
    if (active.length === 0) return <span className="cell-muted">No triggers</span>;
    return (
      <span className="settings-connect-chips">
        {active.map((option) => (
          <Badge key={option.key}>{triggerChipLabel[option.key]}</Badge>
        ))}
      </span>
    );
  }

  const schema = form ? typeSchemas[form.type] : null;

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Notification targets refresh failed" /> : null}
      <Card
        title="Connections"
        subtitle="Push grabs, imports, failures, and health issues to external services."
        padded={targets.length === 0}
        actions={
          <Button size="sm" icon={Plus} onClick={() => setForm(emptyTargetForm())}>
            Add Connection
          </Button>
        }
      >
        {query.isLoading ? (
          <LoadingRow label="Loading connections…" />
        ) : targets.length ? (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Triggers</th>
                <th>Enabled</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {targets.map((target) => (
                <tr key={target.id}>
                  <td className="cell-primary">{target.name}</td>
                  <td>
                    <Badge tone={typeSchemas[target.type]?.tone ?? "neutral"}>
                      {typeSchemas[target.type]?.label ?? target.type}
                    </Badge>
                  </td>
                  <td>{triggersSummary(target)}</td>
                  <td>
                    <input
                      type="checkbox"
                      checked={target.enabled}
                      disabled={toggle.isPending}
                      onChange={() => void toggleEnabled(target)}
                      aria-label={`${target.enabled ? "Disable" : "Enable"} connection ${target.name}`}
                    />
                  </td>
                  <td>
                    <div className="cell-actions">
                      <Button
                        size="sm"
                        icon={Send}
                        busy={testingId === target.id}
                        disabled={testingId !== null}
                        onClick={() => void runTest(target)}
                      >
                        Test
                      </Button>
                      <IconButton
                        icon={Pencil}
                        size="sm"
                        label={`Edit connection ${target.name}`}
                        onClick={() => setForm(targetToForm(target))}
                      />
                      <IconButton
                        icon={Trash2}
                        size="sm"
                        tone="danger"
                        label={`Delete connection ${target.name}`}
                        onClick={() => setDeleting(target)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <EmptyState
            icon={BellRing}
            title="No connections yet"
            actions={
              <Button size="sm" variant="primary" icon={Plus} onClick={() => setForm(emptyTargetForm())}>
                Add Connection
              </Button>
            }
          >
            Connections notify external services when Librarry grabs a release, imports or upgrades a book, hits a
            download failure, or reports a health issue. Webhook, ntfy, Discord, and Telegram are supported.
          </EmptyState>
        )}
      </Card>

      <Modal
        title={form?.id ? "Edit Connection" : "Add Connection"}
        open={Boolean(form)}
        onClose={() => setForm(null)}
        footer={
          <>
            <Button onClick={() => setForm(null)}>Cancel</Button>
            <Button
              variant="primary"
              busy={save.isPending}
              disabled={!form || !formValid(form, editingSaved) || save.isPending}
              onClick={() => void persist()}
            >
              {form?.id ? "Save Connection" : "Add Connection"}
            </Button>
          </>
        }
      >
        {form && schema ? (
          <FormGrid columns={2}>
            <Field label="Name" hint="Display name for this connection.">
              <input
                value={form.name}
                onChange={(event) => update({ name: event.target.value })}
                placeholder="Home ntfy"
              />
            </Field>
            <Field label="Type">
              <select
                value={form.type}
                onChange={(event) => update({ type: event.target.value as NotificationTargetType })}
              >
                {(Object.keys(typeSchemas) as NotificationTargetType[]).map((type) => (
                  <option key={type} value={type}>
                    {typeSchemas[type].label}
                  </option>
                ))}
              </select>
            </Field>
            {schema.fields.map((field) => {
              const keepable = Boolean(field.secret && form.id && editingSaved?.type === form.type);
              return (
                <div className="settings-field-wide" key={`${form.type}:${field.key}`}>
                  <Field label={field.label} hint={field.hint}>
                    {field.secret ? (
                      <SecretInput
                        value={form.settings[field.key] ?? ""}
                        onChange={(value) => updateSetting(field.key, value)}
                        placeholder={keepable ? "Leave blank to keep" : field.placeholder}
                        ariaLabel={field.label}
                      />
                    ) : (
                      <input
                        value={form.settings[field.key] ?? ""}
                        onChange={(event) => updateSetting(field.key, event.target.value)}
                        placeholder={field.placeholder}
                      />
                    )}
                  </Field>
                </div>
              );
            })}
            <div className="settings-field-wide">
              <span className="field-label">Triggers</span>
              <div className="settings-scope-grid">
                {triggerOptions.map((option) => (
                  <label className="settings-scope-option" key={option.key}>
                    <input
                      type="checkbox"
                      checked={form.triggers[option.key]}
                      onChange={(event) => updateTrigger(option.key, event.target.checked)}
                    />
                    <span>{option.label}</span>
                  </label>
                ))}
              </div>
            </div>
            <div className="settings-field-wide">
              <label className="settings-check">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(event) => update({ enabled: event.target.checked })}
                />
                <span>Enabled</span>
              </label>
            </div>
          </FormGrid>
        ) : null}
      </Modal>

      <Modal
        title="Delete connection"
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
          Delete connection <strong>{deleting?.name}</strong>? No further notifications will be sent to it. This does
          not affect the external service.
        </p>
      </Modal>
    </>
  );
}
