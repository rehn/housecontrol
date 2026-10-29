"use client";

import { useState, type FormEvent } from "react";
import { useAuth } from "@/lib/auth";
import { errorText, useI18n } from "@/lib/i18n";
import { LoginIcon, LogoutIcon } from "@/components/icons";

/** User name + password. Calls onDone after a successful login. */
export function LoginForm({ onDone, onCancel }: { onDone?: () => void; onCancel?: () => void }) {
  const { login } = useAuth();
  const { t } = useI18n();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await login(username, password);
      onDone?.();
    } catch (err) {
      setError(errorText(t, err));
      setPassword("");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="login-form" onSubmit={submit}>
      <p className="meta">{t("auth.hint")}</p>
      <label className="field">
        <span className="field-label">{t("auth.username")}</span>
        <input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
        />
      </label>
      <label className="field">
        <span className="field-label">{t("auth.password")}</span>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
        />
      </label>
      {error && <div className="banner bad">{error}</div>}
      <div className="actions">
        {onCancel && (
          <button type="button" className="btn" onClick={onCancel}>
            {t("common.cancel")}
          </button>
        )}
        <button type="submit" className="btn primary" disabled={busy || !username.trim() || !password}>
          {busy ? t("auth.loggingIn") : t("auth.login")}
        </button>
      </div>
    </form>
  );
}

/** The login form in a dialog, e.g. from the home page. */
export function LoginDialog({ onClose }: { onClose: () => void }) {
  const { t } = useI18n();
  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="login-title"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="login-title">{t("auth.title")}</h2>
        <LoginForm onDone={onClose} onCancel={onClose} />
      </div>
    </div>
  );
}

/** Log in / Log out icon for the home page menu. Nothing when login is switched off. */
export function AuthButton() {
  const { ready, required, loggedIn, logout } = useAuth();
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  if (!ready || !required) return null;
  return (
    <>
      {loggedIn ? (
        <button
          type="button"
          className="navlink nav-icon"
          aria-label={t("auth.logout")}
          title={t("auth.logout")}
          onClick={() => logout().catch(() => {})}
        >
          <LogoutIcon />
        </button>
      ) : (
        <button
          type="button"
          className="navlink nav-icon"
          aria-label={t("auth.login")}
          title={t("auth.login")}
          onClick={() => setOpen(true)}
        >
          <LoginIcon />
        </button>
      )}
      {open && <LoginDialog onClose={() => setOpen(false)} />}
    </>
  );
}
