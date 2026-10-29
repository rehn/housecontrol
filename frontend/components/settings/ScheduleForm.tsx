"use client";

import { useEffect, useState } from "react";
import type { Device } from "@/lib/api";
import { newId } from "@/lib/dashboard";
import { errorText, useI18n } from "@/lib/i18n";
import {
  describeDays,
  describeNext,
  describeTrigger,
  WEEK,
  weekdayName,
  type Schedule,
  type SchedulesView,
  type Trigger,
  type TriggerKind,
} from "@/lib/schedules";
import { PencilIcon } from "@/components/icons";

type Props = {
  initial: SchedulesView;
  devices: Device[];
  /** saves the whole list; resolves with the server's view (next run times) */
  onSave: (list: Schedule[]) => Promise<SchedulesView>;
  now?: Date;
};

/** Settings → Schedules: a list of schedules; every change is saved at once. */
export function ScheduleForm(props: Props) {
  const { t, locale } = useI18n();
  const [view, setView] = useState(props.initial);
  const [dialog, setDialog] = useState<{ edit?: Schedule } | null>(null);
  const [msg, setMsg] = useState<{ kind: "ok" | "bad"; text: string } | null>(null);
  const now = props.now ?? new Date();
  const byId = Object.fromEntries(props.devices.map((d) => [d.id, d]));

  const save = async (list: Schedule[]) => {
    setMsg(null);
    try {
      const res = await props.onSave(list);
      setView(res);
      setDialog(null);
      setMsg({ kind: "ok", text: t("schedules.saved") });
    } catch (e) {
      setMsg({ kind: "bad", text: t("schedules.saveFailed", { error: errorText(t, e) }) });
    }
  };

  const isSun = (tr?: Trigger) => !!tr && tr.kind !== "time";
  const usesSun = view.schedules.some((s) => isSun(s.on) || isSun(s.off));
  const p = view.place;

  return (
    <div className="schedule-form">
      <p className="meta">{t("schedules.hint")}</p>
      {p.known && p.sunrise && p.sunset && (
        <p className="meta">
          {t("schedules.sunToday", { place: p.name ?? "", rise: p.sunrise.slice(11, 16), set: p.sunset.slice(11, 16) })}
        </p>
      )}
      {p.known && p.polar && <p className="meta">{t(p.polar === "day" ? "schedules.polarDay" : "schedules.polarNight", { place: p.name ?? "" })}</p>}
      {!p.known && usesSun && <div className="banner warn">{t("schedules.noPlace")}</div>}

      {view.schedules.length === 0 && <p className="meta">{t("schedules.none")}</p>}
      <ul className="schedule-list">
        {view.schedules.map((s) => {
          const next = view.next[s.id];
          const names = s.deviceIds.map((id) => byId[id]?.name ?? id).join(", ");
          return (
            <li key={s.id} className={`schedule-row ${s.enabled ? "" : "disabled"}`}>
              <button
                type="button"
                role="switch"
                aria-checked={s.enabled}
                aria-label={t("schedules.enabled", { name: s.name })}
                className={`switch ${s.enabled ? "on" : ""}`}
                onClick={() => save(view.schedules.map((x) => (x.id === s.id ? { ...x, enabled: !x.enabled } : x)))}
              >
                <span />
              </button>
              <div className="schedule-main">
                <div className="schedule-name">{s.name}</div>
                <div className="schedule-what">
                  {s.on && <span>{t("schedules.onAt", { when: describeTrigger(s.on, t) })}</span>}
                  {s.on && s.brightness ? <span> ({t("schedules.brightnessShort", { pct: s.brightness })})</span> : null}
                  {s.on && s.off && <span> · </span>}
                  {s.off && <span>{t("schedules.offAt", { when: describeTrigger(s.off, t) })}</span>}
                </div>
                <div className="meta">
                  {describeDays(s.days, t, locale)} · {names}
                </div>
                {s.enabled && next && <div className="meta schedule-next">{describeNext(next, now, t, locale)}</div>}
              </div>
              <button
                type="button"
                className="icon-btn"
                aria-label={t("schedules.edit", { name: s.name })}
                onClick={() => setDialog({ edit: s })}
              >
                <PencilIcon />
              </button>
            </li>
          );
        })}
      </ul>

      {msg && <div className={`banner ${msg.kind}`}>{msg.text}</div>}
      <div className="actions">
        <button type="button" className="btn primary" onClick={() => setDialog({})}>
          <span aria-hidden="true">+</span> {t("schedules.add")}
        </button>
      </div>

      {dialog && (
        <ScheduleDialog
          initial={dialog.edit}
          devices={props.devices}
          onClose={() => setDialog(null)}
          onSubmit={(s) =>
            save(dialog.edit ? view.schedules.map((x) => (x.id === s.id ? s : x)) : [...view.schedules, s])
          }
          onRemove={dialog.edit ? () => save(view.schedules.filter((x) => x.id !== dialog.edit!.id)) : undefined}
        />
      )}
    </div>
  );
}

/** One trigger: on/off checkbox, time or sun ± minutes, optional limits. */
function TriggerFields(props: {
  label: string;
  value?: Trigger;
  onChange: (t?: Trigger) => void;
}) {
  const { t } = useI18n();
  const tr = props.value;
  const set = (patch: Partial<Trigger>) => props.onChange({ ...(tr as Trigger), ...patch });
  const minutes = Math.abs(tr?.offset ?? 0);
  const before = (tr?.offset ?? 0) < 0;
  const kinds: { kind: TriggerKind; label: string }[] = [
    { kind: "time", label: t("schedules.kindTime") },
    { kind: "sunrise", label: t("schedules.kindSunrise") },
    { kind: "sunset", label: t("schedules.kindSunset") },
  ];

  return (
    <fieldset className="trigger">
      <legend>
        <label className="check-label">
          <input
            type="checkbox"
            checked={!!tr}
            onChange={(e) => props.onChange(e.target.checked ? { kind: "time", time: "" } : undefined)}
          />
          {props.label}
        </label>
      </legend>
      {tr && (
        <>
          <div className="choice-row" role="radiogroup">
            {kinds.map((k) => (
              <label key={k.kind} className={`choice small ${tr.kind === k.kind ? "selected" : ""}`}>
                <input
                  type="radio"
                  className="visually-hidden"
                  checked={tr.kind === k.kind}
                  onChange={() =>
                    props.onChange(k.kind === "time" ? { kind: "time", time: tr.time ?? "" } : { kind: k.kind, offset: tr.offset, notBefore: tr.notBefore, notAfter: tr.notAfter })
                  }
                />
                {k.label}
              </label>
            ))}
          </div>
          {tr.kind === "time" ? (
            <label className="field inline">
              <span className="field-label">{t("schedules.time")}</span>
              <input type="time" value={tr.time ?? ""} onChange={(e) => set({ time: e.target.value })} />
            </label>
          ) : (
            <>
              <div className="trigger-row">
                <label className="field inline">
                  <span className="field-label">{t("schedules.offset")}</span>
                  <input
                    type="number"
                    inputMode="numeric"
                    min={0}
                    max={240}
                    step={5}
                    value={minutes}
                    onChange={(e) => {
                      const m = Math.min(240, Math.max(0, Math.round(Number(e.target.value) || 0)));
                      set({ offset: before ? -m : m });
                    }}
                  />
                </label>
                <select
                  aria-label={t("schedules.direction")}
                  value={before ? "before" : "after"}
                  onChange={(e) => set({ offset: e.target.value === "before" ? -minutes : minutes })}
                >
                  <option value="before">{t("schedules.before")}</option>
                  <option value="after">{t("schedules.after")}</option>
                </select>
                <span className="meta">{tr.kind === "sunrise" ? t("schedules.sunriseName") : t("schedules.sunsetName")}</span>
              </div>
              <div className="trigger-row">
                <label className="field inline">
                  <span className="field-label">
                    {t("schedules.notBefore")} <span className="meta">{t("schedules.optional")}</span>
                  </span>
                  <input type="time" value={tr.notBefore ?? ""} onChange={(e) => set({ notBefore: e.target.value || undefined })} />
                </label>
                <label className="field inline">
                  <span className="field-label">
                    {t("schedules.notAfter")} <span className="meta">{t("schedules.optional")}</span>
                  </span>
                  <input type="time" value={tr.notAfter ?? ""} onChange={(e) => set({ notAfter: e.target.value || undefined })} />
                </label>
              </div>
            </>
          )}
        </>
      )}
    </fieldset>
  );
}

const triggerOk = (tr?: Trigger) =>
  !tr || (tr.kind === "time" ? /^\d\d:\d\d$/.test(tr.time ?? "") : !(tr.notBefore && tr.notAfter && tr.notBefore > tr.notAfter));

/** Clean a trigger for saving: only the fields its kind uses, no zero offset. */
function cleanTrigger(tr?: Trigger): Trigger | undefined {
  if (!tr) return undefined;
  if (tr.kind === "time") return { kind: "time", time: tr.time };
  const out: Trigger = { kind: tr.kind };
  if (tr.offset) out.offset = tr.offset;
  if (tr.notBefore) out.notBefore = tr.notBefore;
  if (tr.notAfter) out.notAfter = tr.notAfter;
  return out;
}

function ScheduleDialog(props: {
  initial?: Schedule;
  devices: Device[];
  onSubmit: (s: Schedule) => void;
  onRemove?: () => void;
  onClose: () => void;
}) {
  const { t, locale } = useI18n();
  const init = props.initial;
  const [name, setName] = useState(init?.name ?? "");
  const [deviceIds, setDeviceIds] = useState<string[]>(init?.deviceIds ?? []);
  const [days, setDays] = useState<number[]>(init?.days ?? []);
  const [on, setOn] = useState<Trigger | undefined>(init?.on);
  const [off, setOff] = useState<Trigger | undefined>(init?.off);
  const [useBrightness, setUseBrightness] = useState(!!init?.brightness);
  const [brightness, setBrightness] = useState(init?.brightness || 50);
  const [confirmRemove, setConfirmRemove] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && props.onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [props]);

  const controllable = props.devices
    .filter((d) => d.kind === "light" || d.kind === "switch")
    .sort((a, b) => a.name.localeCompare(b.name));
  const toggle = <V,>(list: V[], v: V) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);

  const nothingToDo = !on && !off;
  const valid = name.trim() !== "" && deviceIds.length > 0 && !nothingToDo && triggerOk(on) && triggerOk(off);

  const submit = () => {
    const s: Schedule = {
      id: init?.id ?? newId("sched"),
      name: name.trim(),
      enabled: init?.enabled ?? true,
      deviceIds,
      days: [...days].sort((a, b) => a - b),
    };
    const cOn = cleanTrigger(on);
    const cOff = cleanTrigger(off);
    if (cOn) s.on = cOn;
    if (cOff) s.off = cOff;
    if (cOn && useBrightness) s.brightness = brightness;
    props.onSubmit(s);
  };

  return (
    <div className="dialog-backdrop" onClick={props.onClose}>
      <div className="dialog schedule-dialog" role="dialog" aria-modal="true" aria-labelledby="schedule-dialog-title" onClick={(e) => e.stopPropagation()}>
        <h2 id="schedule-dialog-title">{init ? t("schedules.dialogEdit") : t("schedules.dialogAdd")}</h2>

        <label className="field">
          <span className="field-label">{t("schedules.name")}</span>
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={40} />
        </label>

        <div className="field-label">{t("schedules.devices", { count: deviceIds.length })}</div>
        <div className="pick-list">
          {controllable.map((d) => (
            <label key={d.id} className={`pick-row ${deviceIds.includes(d.id) ? "picked" : ""}`}>
              <input type="checkbox" checked={deviceIds.includes(d.id)} onChange={() => setDeviceIds((l) => toggle(l, d.id))} />
              <span>{d.name}</span>
            </label>
          ))}
        </div>

        <div className="field-label">{t("schedules.days")}</div>
        <div className="day-picker">
          {WEEK.map((d) => (
            <label key={d} className={`day ${days.includes(d) ? "picked" : ""}`}>
              <input type="checkbox" className="visually-hidden" checked={days.includes(d)} onChange={() => setDays((l) => toggle(l, d))} />
              {weekdayName(d, locale)}
            </label>
          ))}
        </div>
        <p className="meta field-hint">{t("schedules.daysHint")}</p>

        <TriggerFields label={t("schedules.turnOn")} value={on} onChange={setOn} />
        {on && (
          <div className="trigger-row">
            <label className="check-label">
              <input type="checkbox" checked={useBrightness} onChange={(e) => setUseBrightness(e.target.checked)} />
              {t("schedules.setBrightness")}
            </label>
            {useBrightness && (
              <>
                <label className="field inline grow">
                  <span className="field-label">{t("schedules.brightness")}</span>
                  <input type="range" min={1} max={100} value={brightness} onChange={(e) => setBrightness(Number(e.target.value))} />
                </label>
                <span className="schedule-pct">{brightness} %</span>
              </>
            )}
          </div>
        )}
        <TriggerFields label={t("schedules.turnOff")} value={off} onChange={setOff} />
        {nothingToDo && <p className="field-error">{t("schedules.needsOnOrOff")}</p>}

        {props.onRemove && (
          <div className="actions tab-dialog-tools">
            <button type="button" className="btn danger" onClick={() => (confirmRemove ? props.onRemove?.() : setConfirmRemove(true))}>
              {confirmRemove ? t("schedules.removeConfirm") : t("schedules.remove")}
            </button>
          </div>
        )}
        <div className="actions">
          <button type="button" className="btn" onClick={props.onClose}>
            {t("common.cancel")}
          </button>
          <button type="button" className="btn primary" disabled={!valid} onClick={submit}>
            {init ? t("common.done") : t("common.add")}
          </button>
        </div>
      </div>
    </div>
  );
}
