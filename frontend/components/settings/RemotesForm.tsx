"use client";

import { useEffect, useRef, useState } from "react";
import type { Device, RemoteAction, RemoteBinding, RemoteEvent, RemoteInfo, RemotesView } from "@/lib/api";
import { errorText, useI18n } from "@/lib/i18n";

/** The latest press reported by the server (live). */
export type LastPress = { remote: string; button: string; event: RemoteEvent; at: number };

export type GroupRef = { id: string; title: string };

type Props = {
  initial: RemotesView;
  /** all devices: lights and outlets can be controlled */
  devices: Device[];
  /** group cards on the home page */
  groups: GroupRef[];
  lastPress: LastPress | null;
  onSave: (bindings: RemoteBinding[]) => Promise<RemotesView>;
};

type T = ReturnType<typeof useI18n>["t"];

const PRESS_ACTIONS: RemoteAction[] = ["on", "off", "toggle", "brighter", "darker", "set"];
const HOLD_ACTIONS: RemoteAction[] = ["on", "off", "toggle", "dim_up", "dim_down", "set"];
// colour temperatures offered for "set", in mired (kelvin in the label)
const COLOR_TEMPS = [454, 370, 250, 153];
const HIGHLIGHT_MS = 4000;

const KNOWN_BUTTONS = ["on", "off", "toggle", "up", "down", "brightness_up", "brightness_down", "arrow_left", "arrow_right", "button"];

export function buttonLabel(t: T, id: string): string {
  if (KNOWN_BUTTONS.includes(id)) return t(`remotes.buttons.${id}`);
  const n = /^(?:button_)?(\d+)$/.exec(id);
  if (n) return t("remotes.buttons.number", { n: n[1] });
  return id;
}

const sameKey = (a: { remote: string; button: string; event: string }, b: { remote: string; button: string; event: string }) =>
  a.remote === b.remote && a.button === b.button && a.event === b.event;

const complete = (b: RemoteBinding) => b.targets.length > 0 && (b.action !== "set" || !!b.brightness || !!b.colorTemp);

/** Settings → Remotes: what each button on each remote does. */
export function RemotesForm(props: Props) {
  const { t } = useI18n();
  const [bindings, setBindings] = useState<RemoteBinding[]>(props.initial.bindings);
  const [picking, setPicking] = useState<RemoteBinding | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [, setTick] = useState(0);

  // let a "just pressed" highlight fade
  useEffect(() => {
    if (!props.lastPress) return;
    const left = props.lastPress.at + HIGHLIGHT_MS - Date.now();
    if (left <= 0) return;
    const id = setTimeout(() => setTick((n) => n + 1), left);
    return () => clearTimeout(id);
  }, [props.lastPress]);

  const save = async (next: RemoteBinding[]) => {
    setBindings(next);
    setError(null);
    try {
      await props.onSave(next.filter(complete));
    } catch (e) {
      setError(t("remotes.saveFailed", { error: errorText(t, e) }));
    }
  };

  const update = (b: RemoteBinding) => {
    const i = bindings.findIndex((x) => sameKey(x, b));
    const next = i >= 0 ? bindings.map((x, j) => (j === i ? b : x)) : [...bindings, b];
    if (!complete(b)) {
      setBindings(next); // keep it here until it has something to control
      return;
    }
    save(next);
  };

  const remove = (key: { remote: string; button: string; event: string }) => save(bindings.filter((x) => !sameKey(x, key)));

  const chooseAction = (remote: string, button: string, event: RemoteEvent, action: RemoteAction | "") => {
    const key = { remote, button, event };
    if (action === "") {
      remove(key);
      return;
    }
    const old = bindings.find((x) => sameKey(x, key));
    const b: RemoteBinding = { ...key, action, targets: old?.targets ?? [] };
    if (action === "set") {
      b.brightness = old?.brightness ?? 100;
      if (old?.colorTemp) b.colorTemp = old.colorTemp;
    }
    update(b);
    if (b.targets.length === 0) setPicking(b);
  };

  if (props.initial.remotes.length === 0) {
    return <p className="meta">{t("remotes.none")}</p>;
  }

  return (
    <div className="remotes-form">
      <p className="meta">{t("remotes.hint")}</p>
      {error && <div className="banner bad">{error}</div>}
      {props.initial.remotes.map((r) => (
        <RemoteBox
          key={r.id}
          remote={r}
          bindings={bindings}
          lastPress={props.lastPress?.remote === r.id ? props.lastPress : null}
          devices={props.devices}
          groups={props.groups}
          onAction={chooseAction}
          onPick={setPicking}
          onUpdate={update}
        />
      ))}
      {picking && (
        <TargetDialog
          binding={picking}
          devices={props.devices}
          groups={props.groups}
          onClose={(targets) => {
            setPicking(null);
            const b = { ...picking, targets };
            if (targets.length === 0 && !bindings.find((x) => sameKey(x, b) && complete(x))) {
              // nothing chosen for a new action: forget it
              setBindings((l) => l.filter((x) => !sameKey(x, b)));
              return;
            }
            if (targets.length > 0) update(b);
          }}
        />
      )}
    </div>
  );
}

function RemoteBox(props: {
  remote: RemoteInfo;
  bindings: RemoteBinding[];
  lastPress: LastPress | null;
  devices: Device[];
  groups: GroupRef[];
  onAction: (remote: string, button: string, event: RemoteEvent, action: RemoteAction | "") => void;
  onPick: (b: RemoteBinding) => void;
  onUpdate: (b: RemoteBinding) => void;
}) {
  const { t } = useI18n();
  const r = props.remote;
  const fresh = props.lastPress && Date.now() - props.lastPress.at < HIGHLIGHT_MS ? props.lastPress : null;
  const titleId = `remote-${r.id}`;
  return (
    <section className="remote-box" role="group" aria-labelledby={titleId}>
      <div className="remote-head">
        <h3 id={titleId}>{r.name}</h3>
        <span className="meta">
          {[r.vendor, r.model].filter(Boolean).join(" ")}
          {r.availability === "offline" && ` · ${t("remotes.offline")}`}
        </span>
      </div>
      <p className={`meta remote-last ${fresh ? "fresh" : ""}`} aria-live="polite">
        {props.lastPress
          ? t("remotes.lastPress", {
              button: buttonLabel(t, props.lastPress.button),
              event: t(`remotes.events.${props.lastPress.event}`),
            })
          : t("remotes.pressHint")}
      </p>
      {r.buttons.map((btn) => (
        <div key={btn.id} className={`remote-button ${fresh?.button === btn.id ? "pressed" : ""}`}>
          <div className="remote-button-name">{buttonLabel(t, btn.id)}</div>
          {btn.events.map((ev) => {
            const b = props.bindings.find((x) => x.remote === r.id && x.button === btn.id && x.event === ev);
            const label = `${buttonLabel(t, btn.id)} – ${t(`remotes.events.${ev}`)}`;
            return (
              <div key={ev} className="remote-row">
                <span className="remote-event">{t(`remotes.events.${ev}`)}</span>
                <select
                  aria-label={label}
                  value={b?.action ?? ""}
                  onChange={(e) => props.onAction(r.id, btn.id, ev, e.target.value as RemoteAction | "")}
                >
                  <option value="">{t("remotes.actions.none")}</option>
                  {(ev === "hold" ? HOLD_ACTIONS : PRESS_ACTIONS).map((a) => (
                    <option key={a} value={a}>
                      {t(`remotes.actions.${a}`)}
                    </option>
                  ))}
                </select>
                {b && (
                  <button
                    type="button"
                    className={`btn target-btn ${b.targets.length === 0 ? "warn" : ""}`}
                    onClick={() => props.onPick(b)}
                  >
                    {targetSummary(t, b.targets, props.devices, props.groups)}
                  </button>
                )}
                {b?.action === "set" && <SetFields binding={b} onChange={props.onUpdate} />}
              </div>
            );
          })}
        </div>
      ))}
    </section>
  );
}

function SetFields({ binding, onChange }: { binding: RemoteBinding; onChange: (b: RemoteBinding) => void }) {
  const { t } = useI18n();
  const [pct, setPct] = useState(String(binding.brightness ?? ""));
  useEffect(() => setPct(String(binding.brightness ?? "")), [binding.brightness]);
  const commit = () => {
    const n = pct.trim() === "" ? 0 : Number(pct);
    if (!Number.isInteger(n) || n < 0 || n > 100) {
      setPct(String(binding.brightness ?? ""));
      return;
    }
    if (n === (binding.brightness ?? 0)) return;
    const b = { ...binding };
    if (n > 0) b.brightness = n;
    else delete b.brightness;
    onChange(b);
  };
  return (
    <span className="remote-set">
      <label className="field">
        <span className="field-label">{t("remotes.brightness")}</span>
        <input
          type="number"
          inputMode="numeric"
          min={1}
          max={100}
          value={pct}
          onChange={(e) => setPct(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => e.key === "Enter" && commit()}
        />
      </label>
      <label className="field">
        <span className="field-label">{t("remotes.colorTemp")}</span>
        <select
          value={binding.colorTemp ?? 0}
          onChange={(e) => {
            const b = { ...binding };
            const v = Number(e.target.value);
            if (v > 0) b.colorTemp = v;
            else delete b.colorTemp;
            onChange(b);
          }}
        >
          <option value={0}>{t("remotes.colorTempKeep")}</option>
          {COLOR_TEMPS.map((m) => (
            <option key={m} value={m}>
              {t("remotes.kelvin", { k: Math.round(1_000_000 / m / 100) * 100 })}
            </option>
          ))}
        </select>
      </label>
    </span>
  );
}

function targetSummary(t: T, targets: string[], devices: Device[], groups: GroupRef[]): string {
  if (targets.length === 0) return t("remotes.chooseTargets");
  const names = targets.map((tg) => {
    const [kind, id] = [tg.slice(0, tg.indexOf(":")), tg.slice(tg.indexOf(":") + 1)];
    if (kind === "group") return groups.find((g) => g.id === id)?.title ?? t("remotes.missingGroup");
    return devices.find((d) => d.id === id)?.name ?? t("remotes.missingDevice");
  });
  return names.length <= 3 ? names.join(", ") : t("remotes.manyTargets", { first: names.slice(0, 2).join(", "), count: names.length - 2 });
}

function TargetDialog(props: {
  binding: RemoteBinding;
  devices: Device[];
  groups: GroupRef[];
  onClose: (targets: string[]) => void;
}) {
  const { t } = useI18n();
  const [targets, setTargets] = useState<string[]>(props.binding.targets);
  const latest = useRef(targets);
  latest.current = targets;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && props.onClose(latest.current);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [props]);

  const controllable = props.devices
    .filter((d) => d.kind === "light" || d.kind === "switch")
    .sort((a, b) => a.name.localeCompare(b.name));
  const flip = (v: string) => setTargets((l) => (l.includes(v) ? l.filter((x) => x !== v) : [...l, v]));
  const row = (value: string, label: string) => (
    <label key={value} className={`pick-row ${targets.includes(value) ? "picked" : ""}`}>
      <input type="checkbox" checked={targets.includes(value)} onChange={() => flip(value)} />
      <span>{label}</span>
    </label>
  );

  return (
    <div className="dialog-backdrop" onClick={() => props.onClose(targets)}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="remote-targets-title"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="remote-targets-title">{t("remotes.targetsTitle")}</h2>
        <div className="pick-list">
          <div className="field-label">{t("remotes.groups")}</div>
          {props.groups.length === 0 ? (
            <p className="meta">{t("remotes.noGroups")}</p>
          ) : (
            props.groups.map((g) => row(`group:${g.id}`, g.title))
          )}
          <div className="field-label">{t("remotes.devices")}</div>
          {controllable.map((d) => row(`device:${d.id}`, d.name))}
        </div>
        <div className="actions">
          <button type="button" className="btn primary" onClick={() => props.onClose(targets)}>
            {t("remotes.done")}
          </button>
        </div>
      </div>
    </div>
  );
}

/** The group cards on the home page, on every tab (for choosing targets). */
export function groupCards(dashboard: { tabs: { columns?: { cards: { id: string; type: string; title?: string }[] }[] }[] } | null | undefined): GroupRef[] {
  const out: GroupRef[] = [];
  for (const tab of dashboard?.tabs ?? []) {
    for (const col of tab.columns ?? []) {
      for (const c of col.cards) {
        if (c.type === "group") out.push({ id: c.id, title: c.title || c.id });
      }
    }
  }
  return out;
}
