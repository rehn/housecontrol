"use client";

import { useState } from "react";
import { errorText, useI18n } from "@/lib/i18n";

type Props = {
  /** returns the backup file: encrypted with the password, or readable JSON when null */
  download: (password: string | null) => Promise<{ blob: Blob; filename: string }>;
  /** sends a backup file; resolves with when it was made */
  restore: (file: File, password: string) => Promise<{ exported?: string }>;
  /** hands the file to the browser to save (default: a download link) */
  save?: (blob: Blob, filename: string) => void;
  /** after a restore: reload so every setting is shown fresh */
  onRestored?: () => void;
  /** how long the "restored" message shows before onRestored (ms) */
  reloadAfterMs?: number;
};

const MIN = 8;

/** Browser download of a Blob. */
export function saveFile(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

/** Settings → Backup: save all settings to a password-protected file, or restore one. */
export function BackupForm(props: Props) {
  const { t, locale } = useI18n();
  const save = props.save ?? saveFile;

  const [protect, setProtect] = useState(true);
  const [pw, setPw] = useState("");
  const [pw2, setPw2] = useState("");
  const [exporting, setExporting] = useState(false);
  const [exportMsg, setExportMsg] = useState<{ kind: "ok" | "bad"; text: string } | null>(null);

  const [file, setFile] = useState<File | null>(null);
  const [rpw, setRpw] = useState("");
  const [confirm, setConfirm] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [restoreMsg, setRestoreMsg] = useState<{ kind: "ok" | "bad"; text: string } | null>(null);

  const tooShort = pw.length > 0 && [...pw].length < MIN;
  const mismatch = pw2.length > 0 && pw !== pw2;
  const canExport = !exporting && (!protect || ([...pw].length >= MIN && pw === pw2));

  const doExport = async () => {
    setExporting(true);
    setExportMsg(null);
    try {
      const { blob, filename } = await props.download(protect ? pw : null);
      save(blob, filename);
      setPw("");
      setPw2("");
      setExportMsg({
        kind: "ok",
        text: t(protect ? "settings.backupDownloaded" : "settings.backupDownloadedPlain", { file: filename }),
      });
    } catch (e) {
      setExportMsg({ kind: "bad", text: t("settings.backupFailed", { error: errorText(t, e) }) });
    } finally {
      setExporting(false);
    }
  };

  const doRestore = async () => {
    if (!file) return;
    if (!confirm) {
      setConfirm(true);
      return;
    }
    setRestoring(true);
    setRestoreMsg(null);
    try {
      const res = await props.restore(file, rpw);
      const date = res.exported
        ? new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(res.exported))
        : "";
      setRestoreMsg({ kind: "ok", text: t("settings.backupRestored", { date }) });
      setTimeout(() => props.onRestored?.(), props.reloadAfterMs ?? 1500);
    } catch (e) {
      setRestoreMsg({ kind: "bad", text: t("settings.backupFailed", { error: errorText(t, e) }) });
      setConfirm(false);
    } finally {
      setRestoring(false);
    }
  };

  return (
    <div className="backup-form">
      <section className="backup-box" role="group" aria-labelledby="backup-export-h">
        <h3 id="backup-export-h">{t("settings.backupExport")}</h3>
        <p className="meta">{t("settings.backupExportHint")}</p>
        <label className="check-label">
          <input
            type="checkbox"
            checked={protect}
            onChange={(e) => {
              setProtect(e.target.checked);
              setExportMsg(null);
            }}
          />
          {t("settings.backupProtect")}
        </label>
        {protect ? (
          <>
            <div className="row">
              <label className="field">
                <span className="field-label">{t("settings.backupPassword")}</span>
                <input type="password" autoComplete="new-password" value={pw} onChange={(e) => setPw(e.target.value)} />
              </label>
              <label className="field">
                <span className="field-label">{t("settings.backupPasswordRepeat")}</span>
                <input type="password" autoComplete="new-password" value={pw2} onChange={(e) => setPw2(e.target.value)} />
              </label>
            </div>
            {tooShort && <p className="field-error">{t("settings.backupPasswordShort", { min: MIN })}</p>}
            {!tooShort && mismatch && <p className="field-error">{t("settings.backupPasswordMismatch")}</p>}
          </>
        ) : (
          <div className="banner warn">{t("settings.backupPlainWarning")}</div>
        )}
        {exportMsg && <div className={`banner ${exportMsg.kind}`}>{exportMsg.text}</div>}
        <div className="actions">
          <button type="button" className="btn primary" disabled={!canExport} onClick={doExport}>
            {exporting ? t("settings.backupEncrypting") : t("settings.backupDownload")}
          </button>
        </div>
      </section>

      <section className="backup-box" role="group" aria-labelledby="backup-import-h">
        <h3 id="backup-import-h">{t("settings.backupImport")}</h3>
        <p className="meta">{t("settings.backupImportHint")}</p>
        <label className="field">
          <span className="field-label">{t("settings.backupFile")}</span>
          <input
            type="file"
            accept=".hcbackup,.json,application/json,application/octet-stream"
            onChange={(e) => {
              setFile(e.target.files?.[0] ?? null);
              setConfirm(false);
              setRestoreMsg(null);
            }}
          />
        </label>
        <label className="field">
          <span className="field-label">
            {t("settings.backupPassword")} <span className="meta">{t("settings.backupPasswordIfProtected")}</span>
          </span>
          <input
            type="password"
            autoComplete="off"
            value={rpw}
            onChange={(e) => {
              setRpw(e.target.value);
              setConfirm(false);
            }}
          />
        </label>
        {restoreMsg && <div className={`banner ${restoreMsg.kind}`}>{restoreMsg.text}</div>}
        <div className="actions">
          <button
            type="button"
            className={`btn ${confirm ? "danger" : "primary"}`}
            disabled={!file || restoring}
            onClick={doRestore}
          >
            {restoring ? t("settings.backupRestoring") : confirm ? t("settings.backupRestoreConfirm") : t("settings.backupRestore")}
          </button>
        </div>
      </section>
    </div>
  );
}
