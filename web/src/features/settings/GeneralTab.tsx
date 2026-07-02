import React, { useEffect, useState } from "react";
import { CheckCircle2, ShieldCheck, Trash2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { Badge, Button, Card, Field, FormGrid, InlineNotice } from "../../components/ui";
import { useToast } from "../../components/toast";
import { fetchSystemStatus, getStoredAPIKey, saveAuthConfig, setStoredAPIKey, type AuthMethod } from "../../lib/api";
import { keys, m6Keys, useAuthStatus } from "../../lib/queries";
import { SecretInput } from "./controls";
import { errorMessage, normalizedFormText } from "./helpers";

/**
 * General → API key management. The key lives in this browser's localStorage
 * only and is attached to every request as the X-Api-Key header.
 */
export function GeneralTab() {
  const toast = useToast();
  const client = useQueryClient();
  const [storedKey, setStoredKeyState] = useState(() => getStoredAPIKey());
  const [input, setInput] = useState(storedKey);
  const [checking, setChecking] = useState(false);

  const hasChanges = normalizedFormText(input) !== normalizedFormText(storedKey);

  async function refreshAuthedQueries() {
    await Promise.all(
      [keys.providerHealth, keys.integrationHealth, keys.systemStatus, keys.readiness, keys.readarrCompatibility].map((key) =>
        client.invalidateQueries({ queryKey: key })
      )
    );
  }

  async function saveKey() {
    if (!hasChanges) return;
    setStoredAPIKey(input);
    const next = getStoredAPIKey();
    setStoredKeyState(next);
    toast.success(next ? "API key saved for this browser." : "API key cleared for this browser.");
    setChecking(true);
    try {
      await fetchSystemStatus();
    } catch {
      toast.notify("API key saved, but the API is still unavailable", "warn");
    } finally {
      await refreshAuthedQueries();
      setChecking(false);
    }
  }

  function clearKey() {
    setInput("");
    setStoredAPIKey("");
    setStoredKeyState("");
    toast.success("API key cleared for this browser.");
  }

  return (
    <>
      <Card
        title="API key"
        subtitle="Authenticates this browser against the Librarry API."
        actions={<Badge tone={storedKey ? "success" : "neutral"}>{storedKey ? "Saved in this browser" : "Not saved"}</Badge>}
      >
        <FormGrid columns={1}>
          <Field
            label="Key"
            hint="Stored in this browser's localStorage only and sent as the X-Api-Key header with every request."
          >
            <SecretInput
              value={input}
              onChange={setInput}
              placeholder="Readarr-compatible API key"
              ariaLabel="API key"
              onEnter={() => void saveKey()}
            />
          </Field>
        </FormGrid>
        <div className="form-actions">
          <Button variant="primary" icon={CheckCircle2} disabled={!hasChanges} busy={checking} onClick={() => void saveKey()}>
            Save key
          </Button>
          <Button variant="danger" icon={Trash2} onClick={clearKey}>
            Clear
          </Button>
        </div>
      </Card>
      <AuthenticationCard />
    </>
  );
}

/* ------------------------- Authentication (M6.6) --------------------------- */

const authMethodOptions: { value: AuthMethod; label: string }[] = [
  { value: "none", label: "None (open install)" },
  { value: "basic", label: "Basic (browser popup)" },
  { value: "forms", label: "Forms (login page)" }
];

const authMethodBadge: Record<AuthMethod, string> = {
  none: "Open",
  basic: "Basic",
  forms: "Forms"
};

/**
 * General → Authentication: in-app session auth (arr-style). Method + username
 * + password persist through PUT /api/v1/auth/config; a blank password keeps
 * the stored one. API-key clients bypass sessions entirely.
 */
function AuthenticationCard() {
  const toast = useToast();
  const client = useQueryClient();
  const status = useAuthStatus();

  const [method, setMethod] = useState<AuthMethod>("none");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [hydratedFor, setHydratedFor] = useState("");
  const [saving, setSaving] = useState(false);

  // Hydrate the form once per distinct server state (method + username).
  const serverStamp = status.data ? `${status.data.method}:${status.data.username ?? ""}` : "";
  useEffect(() => {
    if (!status.data || hydratedFor === serverStamp) return;
    setMethod(status.data.method);
    setUsername(status.data.username ?? "");
    setPassword("");
    setHydratedFor(serverStamp);
  }, [status.data, serverStamp, hydratedFor]);

  const savedMethod = status.data?.method ?? "none";
  const needsCredentials = method !== "none";
  // Enabling auth for the first time requires a password; editing may keep it blank.
  const passwordRequired = needsCredentials && savedMethod === "none" && !password;
  const valid = !needsCredentials || (Boolean(username.trim()) && !passwordRequired);
  const hasChanges =
    method !== savedMethod ||
    (needsCredentials && (normalizedFormText(username) !== normalizedFormText(status.data?.username) || Boolean(password)));

  async function save() {
    if (!valid || !hasChanges || saving) return;
    setSaving(true);
    try {
      await saveAuthConfig({
        method,
        username: needsCredentials ? username.trim() : undefined,
        password: needsCredentials && password ? password : undefined
      });
      toast.success(
        method === "none" ? "Authentication disabled — the UI is open again." : `Authentication set to ${authMethodBadge[method]}.`
      );
      setPassword("");
      setHydratedFor("");
      await client.invalidateQueries({ queryKey: m6Keys.authStatus });
    } catch (error) {
      toast.error(errorMessage(error, "Authentication settings update failed"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card
      title="Authentication"
      subtitle="Protect the UI with a username and password (arr-style sessions)."
      actions={<Badge tone={savedMethod === "none" ? "neutral" : "success"}>{authMethodBadge[savedMethod]}</Badge>}
    >
      <FormGrid columns={2}>
        <Field
          label="Method"
          hint="Basic uses the browser's credential popup; Forms shows a Librarry login page."
        >
          <select value={method} onChange={(event) => setMethod(event.target.value as AuthMethod)} aria-label="Authentication method">
            {authMethodOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </Field>
        {needsCredentials ? (
          <>
            <Field label="Username">
              <input
                autoComplete="off"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                aria-label="Authentication username"
              />
            </Field>
            <Field label="Password" hint={savedMethod !== "none" ? "Leave blank to keep the current password." : "Required."}>
              <SecretInput value={password} onChange={setPassword} ariaLabel="Authentication password" />
            </Field>
          </>
        ) : null}
      </FormGrid>
      {needsCredentials ? (
        <div style={{ marginTop: 12 }}>
          <InlineNotice tone="warn">
            Don't lock yourself out: make sure the username and password are correct before saving — every browser
            session will need them immediately. API clients using the X-Api-Key header bypass sessions and keep
            working.
          </InlineNotice>
        </div>
      ) : null}
      <div className="form-actions">
        <Button
          variant="primary"
          icon={ShieldCheck}
          busy={saving}
          disabled={!valid || !hasChanges || status.isPending}
          onClick={() => void save()}
        >
          Save authentication
        </Button>
      </div>
    </Card>
  );
}
