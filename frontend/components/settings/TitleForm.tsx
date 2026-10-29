"use client";

import { FormEvent, useEffect, useState } from "react";
import { errorText, useI18n } from "@/lib/i18n";
import { DEFAULT_TITLE, MAX_TITLE_LEN } from "@/lib/title";

/** Edit the app title shown in the header and browser tab. */
export function TitleForm(props: { value: string; onSave: (title: string) => Promise<void> }) {
  const { t } = useI18n();
  const [title, setTitle] = useState(props.value);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ kind: "ok" | "bad"; text: string } | null>(null);

  // follow changes made elsewhere (another tablet) while not editing
  useEffect(() => setTitle(props.value), [props.value]);

  const changed = title.trim() !== props.value.trim();

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      await props.onSave(title.trim());
      setMsg({ kind: "ok", text: t("settings.titleSaved") });
    } catch (err) {
      setMsg({ kind: "bad", text: t("settings.titleSaveFailed", { error: errorText(t, err) }) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={save}>
      <label className="field">
        <span className="field-label">{t("settings.appTitle")}</span>
        <input
          value={title}
          onChange={(e) => {
            setTitle(e.target.value);
            setMsg(null);
          }}
          placeholder={DEFAULT_TITLE}
          maxLength={MAX_TITLE_LEN}
        />
      </label>
      <p className="meta">{t("settings.appTitleHint", { default: DEFAULT_TITLE })}</p>
      {msg && <div className={`banner ${msg.kind}`}>{msg.text}</div>}
      <div className="actions">
        <button type="submit" className="btn primary" disabled={busy || !changed}>
          {busy ? t("common.saving") : t("common.save")}
        </button>
      </div>
    </form>
  );
}
