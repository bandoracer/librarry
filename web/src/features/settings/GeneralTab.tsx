import React, { useState } from "react";
import { CheckCircle2, Trash2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { Badge, Button, Card, Field, FormGrid } from "../../components/ui";
import { useToast } from "../../components/toast";
import { fetchSystemStatus, getStoredAPIKey, setStoredAPIKey } from "../../lib/api";
import { keys } from "../../lib/queries";
import { SecretInput } from "./controls";
import { normalizedFormText } from "./helpers";

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
  );
}
