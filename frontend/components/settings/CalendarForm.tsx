"use client";

import { useRef, useState } from "react";
import type { CalendarInput, CalendarSetting } from "@/lib/api";
import { errorText, useI18n } from "@/lib/i18n";
import { CloseIcon } from "@/components/icons";

/** Colours to choose from for calendar dots. */
export const PALETTE = ["#f5c242", "#8a63d2", "#4f6bd6", "#4fa8e8", "#2e9d58", "#e0703a", "#d94c63", "#8a8a85"];

type Row = { id?: string; name: string; color: string; url?: string; urlHint?: string };
type TestResult = { ok: boolean; events?: number; error?: string };

function ColorPicker(props: { value: string; onChange: (c: string) => void; label: string }) {
  return (
    <div className="color-picker" role="radiogroup" aria-label={props.label}>
      {PALETTE.map((c) => (
        <button
          key={c}
          type="button"
          role="radio"
          aria-checked={props.value === c}
          aria-label={c}
          className={`color-swatch ${props.value === c ? "selected" : ""}`}
          style={{ background: c }}
          onClick={() => props.onChange(c)}
        />
      ))}
    </div>
  );
}

const toInput = (list: Row[]): CalendarInput[] =>
  list.map((r) =>
    r.id ? { id: r.id, name: r.name.trim(), color: r.color } : { name: r.name.trim(), color: r.color, url: r.url },
  );

/** Manage iCal calendars. Every change (add, rename, recolour, remove) is saved on its own – no Save button. */
export function CalendarForm(props: {
  initial: CalendarSetting[];
  onSave: (c: CalendarInput[]) => Promise<CalendarSetting[]>;
  test: (url: string) => Promise<TestResult>;
}) {
  const { t } = useI18n();
  const [rows, setRows] = useState<Row[]>(props.initial.map((c) => ({ ...c })));
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [color, setColor] = useState(PALETTE[(props.initial.length + 1) % PALETTE.length]);
  const [testMsg, setTestMsg] = useState<{ kind: "ok" | "bad" | "info"; text: string } | null>(null);
  const [busy, setBusy] = useState<"test" | "add" | null>(null);
  const [msg, setMsg] = useState<{ kind: "ok" | "bad"; text: string } | null>(null);

  // what the server has; used to skip no-op saves and to undo a failed one
  const saved = useRef<Row[]>(props.initial.map((c) => ({ ...c })));
  // saves run one after another so a quick rename + recolour can't overtake each other
  const queue = useRef<Promise<unknown>>(Promise.resolve());
  // the latest list sent to the server (saved or on its way), so Enter + blur don't save twice
  const sent = useRef<Row[]>(saved.current);

  const persist = (next: Row[]) => {
    setRows(next);
    sent.current = next;
    setMsg(null);
    const run = queue.current.then(async () => {
      try {
        const res = await props.onSave(toInput(next));
        saved.current = res.map((c) => ({ ...c }));
        setMsg({ kind: "ok", text: t("settings.calendarSaved") });
      } catch (e) {
        sent.current = saved.current;
        setRows(saved.current.map((c) => ({ ...c })));
        setMsg({ kind: "bad", text: t("settings.calendarSaveFailed", { error: errorText(t, e) }) });
      }
    });
    queue.current = run;
    return run;
  };

  const editName = (i: number, value: string) => {
    setRows((r) => r.map((row, j) => (j === i ? { ...row, name: value } : row)));
    setMsg(null);
  };

  const commitName = (i: number) => {
    const row = rows[i];
    const before = sent.current.find((c) => c.id === row.id);
    if (!row.name.trim()) {
      if (before) editName(i, before.name); // empty name: put the old one back
      return;
    }
    if (before && before.name === row.name.trim()) return;
    persist(rows.map((r, j) => (j === i ? { ...r, name: r.name.trim() } : r)));
  };

  const runTest = async () => {
    setBusy("test");
    setTestMsg({ kind: "info", text: t("settings.calendarTesting") });
    try {
      const r = await props.test(url.trim());
      setTestMsg(
        r.ok
          ? { kind: "ok", text: t("settings.calendarTestOk", { count: r.events ?? 0 }) }
          : { kind: "bad", text: t("settings.calendarTestFailed", { error: r.error ?? "" }) },
      );
    } catch (e) {
      setTestMsg({ kind: "bad", text: t("settings.calendarTestFailed", { error: errorText(t, e) }) });
    } finally {
      setBusy(null);
    }
  };

  /** Adding saves right away, so a new calendar never sits unsaved in the list. */
  const add = async () => {
    if (!name.trim() || !url.trim()) {
      setTestMsg({ kind: "bad", text: t("settings.calendarMissing") });
      return;
    }
    setBusy("add");
    setMsg(null);
    const run = queue.current.then(async () => {
      try {
        const res = await props.onSave(toInput([...sent.current, { name: name.trim(), color, url: url.trim() }]));
        saved.current = res.map((c) => ({ ...c }));
        sent.current = saved.current;
        setRows(res.map((c) => ({ ...c })));
        setName("");
        setUrl("");
        setColor(PALETTE[(res.length + 1) % PALETTE.length]);
        setTestMsg(null);
        setMsg({ kind: "ok", text: t("settings.calendarAdded") });
      } catch (e) {
        setTestMsg({ kind: "bad", text: t("settings.calendarSaveFailed", { error: errorText(t, e) }) });
      } finally {
        setBusy(null);
      }
    });
    queue.current = run;
    await run;
  };

  return (
    <div className="calendar-form">
      <p className="meta">{t("settings.calendarsHint")}</p>

      {rows.length === 0 && <p className="meta">{t("settings.calendarNone")}</p>}
      <ul className="calendar-list">
        {rows.map((r, i) => (
          <li key={r.id ?? `new-${i}`} className="calendar-row">
            <span className="cal-dot big" style={{ background: r.color }} aria-hidden="true" />
            <div className="calendar-row-main">
              <input
                value={r.name}
                aria-label={t("settings.calendarName")}
                maxLength={40}
                onChange={(e) => editName(i, e.target.value)}
                onBlur={() => commitName(i)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") commitName(i);
                }}
              />
              <span className="meta">{r.urlHint ?? r.url}</span>
              <ColorPicker value={r.color} onChange={(c) => c !== r.color && persist(rows.map((x, j) => (j === i ? { ...x, color: c } : x)))} label={t("settings.calendarColor")} />
            </div>
            <button
              type="button"
              className="icon-btn danger"
              aria-label={t("settings.calendarRemove", { name: r.name })}
              onClick={() => persist(rows.filter((_, j) => j !== i))}
            >
              <CloseIcon />
            </button>
          </li>
        ))}
      </ul>

      {msg && <div className={`banner ${msg.kind}`}>{msg.text}</div>}
      <div className="calendar-add" role="group" aria-label={t("settings.calendarAdd")}>
        <div className="field-label">{t("settings.calendarAdd")}</div>
        <label className="field">
          <span className="field-label">{t("settings.calendarName")}</span>
          <input value={name} maxLength={40} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="field">
          <span className="field-label">{t("settings.calendarUrl")}</span>
          <input
            value={url}
            onChange={(e) => {
              setUrl(e.target.value);
              setTestMsg(null);
            }}
            placeholder="https://… / webcal://…"
            autoComplete="off"
            spellCheck={false}
          />
        </label>
        <ColorPicker value={color} onChange={setColor} label={t("settings.calendarColor")} />
        {testMsg && <div className={`banner ${testMsg.kind}`}>{testMsg.text}</div>}
        <div className="actions">
          <button type="button" className="btn" disabled={busy !== null || !url.trim()} onClick={runTest}>
            {busy === "test" ? t("settings.calendarTesting") : t("settings.calendarTest")}
          </button>
          <button type="button" className="btn primary" disabled={busy !== null} onClick={add}>
            {busy === "add" ? t("common.saving") : t("settings.calendarAdd")}
          </button>
        </div>
      </div>
    </div>
  );
}
