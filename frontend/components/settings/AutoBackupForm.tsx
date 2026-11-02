"use client";

import { useEffect, useState } from "react";
import type { AutoBackupSettings, AutoBackupView } from "@/lib/api";
import { errorText, useI18n } from "@/lib/i18n";
import { saveFile as browserSave } from "./BackupForm";

type Props = {
  load: () => Promise<AutoBackupView>;
  save: (s: AutoBackupSettings) => Promise<AutoBackupView>;
  run: () => Promise<AutoBackupView>;
  download: (name: string) => Promise<{ blob: Blob; filename: string }>;
  restore: (name: string) => Promise<{ exported?: string }>;
  /** hands a file to the browser to save (default: a download link) */
  saveFile?: (blob: Blob, filename: string) => void;
  /** after a restore: reload so every setting is shown fresh */
  onRestored?: () => void;
  reloadAfterMs?: number;
};

const MIN_KEEP = 1;
const MAX_KEEP = 365;

type Msg = { kind: "ok" | "bad"; text: string } | null;

/** Settings → Backup: backups made every day on the server, and the stored ones. */
export function AutoBackupForm(props: Props) {
  const { t, locale } = useI18n();
  const [v, setV] = useState<AutoBackupView | null>(null);
  const [keepText, setKeepText] = useState("");
  const [keepBad, setKeepBad] = useState(false);
  const [running, setRunning] = useState(false);
  const [confirm, setConfirm] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<Msg>(null);

  const show = (next: AutoBackupView) => {
    setV(next);
    setKeepText(String(next.keep));
    setKeepBad(false);
  };

  useEffect(() => {
    props
      .load()
      .then(show)
      .catch((e) => setMsg({ kind: "bad", text: t("settings.backupFailed", { error: errorText(t, e) }) }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fmt = (iso: string, withWeekday = false) =>
    new Intl.DateTimeFormat(
      locale,
      withWeekday
        ? { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }
        : { dateStyle: "medium", timeStyle: "short" },
    ).format(new Date(iso));
  const size = (bytes: number) =>
    new Intl.NumberFormat(locale, { style: "unit", unit: "kilobyte", maximumFractionDigits: 1 }).format(bytes / 1024);

  const store = async (s: AutoBackupSettings) => {
    if (!v) return;
    setV({ ...v, ...s }); // show the change at once
    setMsg(null);
    try {
      show(await props.save(s));
    } catch (e) {
      setMsg({ kind: "bad", text: t("settings.backupFailed", { error: errorText(t, e) }) });
    }
  };

  const commitKeep = () => {
    if (!v) return;
    const n = Number(keepText);
    if (!Number.isInteger(n) || n < MIN_KEEP || n > MAX_KEEP) {
      setKeepBad(true);
      return;
    }
    setKeepBad(false);
    if (n !== v.keep) store({ enabled: v.enabled, time: v.time, keep: n });
  };

  const runNow = async () => {
    setRunning(true);
    setMsg(null);
    try {
      show(await props.run());
    } catch (e) {
      setMsg({ kind: "bad", text: t("settings.backupFailed", { error: errorText(t, e) }) });
    } finally {
      setRunning(false);
    }
  };

  const download = async (name: string) => {
    setMsg(null);
    try {
      const { blob, filename } = await props.download(name);
      (props.saveFile ?? browserSave)(blob, filename);
    } catch (e) {
      setMsg({ kind: "bad", text: t("settings.backupFailed", { error: errorText(t, e) }) });
    }
  };

  const restore = async (name: string) => {
    if (confirm !== name) {
      setConfirm(name);
      return;
    }
    setBusy(name);
    setMsg(null);
    try {
      const res = await props.restore(name);
      const date = res.exported
        ? new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(res.exported))
        : "";
      setMsg({ kind: "ok", text: t("settings.backupRestored", { date }) });
      setTimeout(() => props.onRestored?.(), props.reloadAfterMs ?? 1500);
    } catch (e) {
      setMsg({ kind: "bad", text: t("settings.backupFailed", { error: errorText(t, e) }) });
    } finally {
      setConfirm(null);
      setBusy(null);
    }
  };

  return (
    <section className="backup-box" role="group" aria-labelledby="autobackup-h">
      <h3 id="autobackup-h">{t("autoBackup.title")}</h3>
      {!v ? (
        msg && <div className={`banner ${msg.kind}`}>{msg.text}</div>
      ) : !v.available ? (
        <p className="meta">{t("autoBackup.noFolder")}</p>
      ) : (
        <>
          <p className="meta">
            {t("autoBackup.folder", { dir: v.dir ?? "" })} {v.encrypted && t("autoBackup.encrypted")}
          </p>
          {!v.encrypted && <div className="banner warn">{t("autoBackup.notEncrypted")}</div>}

          <label className="check-label">
            <input
              type="checkbox"
              checked={v.enabled}
              onChange={(e) => store({ enabled: e.target.checked, time: v.time, keep: v.keep })}
            />
            {t("autoBackup.daily")}
          </label>
          <div className="row autobackup-row">
            <label className="field">
              <span className="field-label">{t("autoBackup.time")}</span>
              <input
                type="time"
                value={v.time}
                disabled={!v.enabled}
                onChange={(e) => {
                  if (/^\d{2}:\d{2}$/.test(e.target.value)) store({ enabled: v.enabled, time: e.target.value, keep: v.keep });
                }}
              />
            </label>
            <div className="field">
              <label className="field-label" htmlFor="autobackup-keep">
                {t("autoBackup.keep")}
              </label>
              <span className="input-with-unit">
                <input
                  id="autobackup-keep"
                  type="number"
                  inputMode="numeric"
                  min={MIN_KEEP}
                  max={MAX_KEEP}
                  value={keepText}
                  aria-invalid={keepBad || undefined}
                  onChange={(e) => setKeepText(e.target.value)}
                  onBlur={commitKeep}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") commitKeep();
                  }}
                />
                <span className="meta">{t("autoBackup.keepUnit")}</span>
              </span>
            </div>
          </div>
          {keepBad ? (
            <p className="field-error">{t("autoBackup.keepRange")}</p>
          ) : (
            <p className="meta">{t("autoBackup.keepHint")}</p>
          )}
          {v.enabled && v.next && <p className="meta">{t("autoBackup.next", { date: fmt(v.next, true) })}</p>}
          {v.lastError && <div className="banner bad">{t("autoBackup.lastError", { error: v.lastError })}</div>}
          {msg && <div className={`banner ${msg.kind}`}>{msg.text}</div>}
          <div className="actions">
            <button type="button" className="btn primary" disabled={running} onClick={runNow}>
              {running ? t("autoBackup.running") : t("autoBackup.runNow")}
            </button>
          </div>

          <h4>{t("autoBackup.stored")}</h4>
          {v.backups.length === 0 ? (
            <p className="meta">{t("autoBackup.none")}</p>
          ) : (
            <ul className="stored-backups">
              {v.backups.map((b) => (
                <li key={b.name}>
                  <span className="stored-backup-info">
                    <span>{fmt(b.created)}</span>
                    <span className="meta">{size(b.size)}</span>
                  </span>
                  <span className="stored-backup-actions">
                    <button type="button" className="btn" onClick={() => download(b.name)}>
                      {t("autoBackup.download")}
                    </button>
                    <button
                      type="button"
                      className={`btn ${confirm === b.name ? "danger" : ""}`}
                      disabled={busy !== null}
                      onClick={() => restore(b.name)}
                    >
                      {busy === b.name
                        ? t("settings.backupRestoring")
                        : confirm === b.name
                          ? t("settings.backupRestoreConfirm")
                          : t("settings.backupRestore")}
                    </button>
                  </span>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}
