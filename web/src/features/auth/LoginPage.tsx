import React, { useState } from "react";
import { LogIn } from "lucide-react";
import { Button, Field, InlineNotice } from "../../components/ui";
import { login } from "../../lib/api";
import "./auth.css";

/**
 * Standalone forms-auth login page. Rendered outside the app shell (the
 * integrator gates routes on useAuthStatus); a successful login reloads the
 * app so every query restarts with the new session cookie.
 */
export default function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [rememberMe, setRememberMe] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (busy || !username.trim() || !password) return;
    setBusy(true);
    setError("");
    try {
      await login({ username: username.trim(), password, rememberMe });
      window.location.reload();
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : "Sign in failed.");
      setBusy(false);
    }
  }

  return (
    <div className="auth-shell">
      <form className="auth-card card" onSubmit={(event) => void submit(event)}>
        <div className="auth-brand">
          <span className="brand-mark" aria-hidden>
            L
          </span>
          <h1>Librarry</h1>
        </div>
        {error ? (
          <InlineNotice tone="danger" onDismiss={() => setError("")}>
            {error}
          </InlineNotice>
        ) : null}
        <Field label="Username">
          <input
            autoComplete="username"
            autoFocus
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            aria-label="Username"
          />
        </Field>
        <Field label="Password">
          <input
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            aria-label="Password"
          />
        </Field>
        <label className="auth-remember">
          <input type="checkbox" checked={rememberMe} onChange={(event) => setRememberMe(event.target.checked)} />
          <span>Remember me</span>
        </label>
        <Button type="submit" variant="primary" icon={LogIn} busy={busy} disabled={!username.trim() || !password}>
          Sign In
        </Button>
      </form>
    </div>
  );
}
