import React, { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { InlineNotice } from "../../components/ui";
import { demoModeEnabled } from "../../lib/demo";
import { appErrorMessage, errorMessage, isPersistenceRequiredError } from "./helpers";

/**
 * Password / API-key input with a show-hide toggle. Values are never echoed
 * anywhere else (toasts, logs, subtitles) — masking here is the only render.
 */
export function SecretInput(props: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  ariaLabel?: string;
  disabled?: boolean;
  onEnter?: () => void;
}) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="settings-secret">
      <input
        autoComplete="off"
        type={visible ? "text" : "password"}
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && props.onEnter) props.onEnter();
        }}
        placeholder={props.placeholder}
        aria-label={props.ariaLabel}
        disabled={props.disabled}
      />
      <button
        type="button"
        className="settings-secret-toggle"
        onClick={() => setVisible((current) => !current)}
        aria-label={visible ? "Hide value" : "Show value"}
        title={visible ? "Hide value" : "Show value"}
        tabIndex={-1}
      >
        {visible ? <EyeOff size={14} aria-hidden /> : <Eye size={14} aria-hidden />}
      </button>
    </div>
  );
}

/**
 * Fetch-failure surface: outside demo mode failures show inline. Persistence-
 * required errors render as an informational note with setup guidance (legacy
 * `inline-note` treatment); everything else is a danger notice.
 */
export function QueryErrorNotice(props: { error: unknown; fallback: string }) {
  if (demoModeEnabled) return null;
  const message = errorMessage(props.error, props.fallback);
  return <InlineNotice tone={isPersistenceRequiredError(message) ? "info" : "danger"}>{appErrorMessage(message)}</InlineNotice>;
}
