"use client";

import { useEffect, useState } from "react";
import type { Device, RemoteAction, RemoteButtonAction, RemoteConfig, RemoteEvent, RemoteInfo, RemotesView } from "@/lib/api";
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
  onSave: (configs: RemoteConfig[]) => Promise<RemotesView>;
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

/** Settings → Remotes: what each remote controls, and what its buttons do. */
export function RemotesForm(props: Props) {
  const { t } = useI18n();
  // saved configs (also for remotes not connected now)
  const [configs, setConfigs] = useState<RemoteConfig[]>(props.initial.configs);
  // button actions of remotes that don't control anything yet
  const [drafts, setDrafts] = useState<Record<string, RemoteButtonAction[]>>({});
  const [picking, setPicking] = useState<RemoteInfo | null>(null);
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

  const save = async (next: RemoteConfig[]) => {
    setConfigs(next);
    setError(null);
    try {
      await props.onSave(next);
    } catch (e) {
      setError(t("remotes.saveFailed", { error: errorText(t, e) }));
    }
  };

  const configOf = (id: string) => configs.find((c) => c.remote === id);
  const actionsOf = (r: RemoteInfo) => configOf(r.id)?.actions ?? drafts[r.id] ?? r.defaults;

  const setActions = (r: RemoteInfo, actions: RemoteButtonAction[]) => {
    const c = configOf(r.id);
    if (!c) {
      setDrafts((d) => ({ ...d, [r.id]: actions }));
      return;
    }
    save(configs.map((x) => (x.remote === r.id ? { ...x, actions } : x)));
  };

  const setTarget = (r: RemoteInfo, target: string | null) => {
    setPicking(null);
    const c = configOf(r.id);
    if (target === null) {
      if (c) {
        setDrafts((d) => ({ ...d, [r.id]: c.actions }));
        save(configs.filter((x) => x.remote !== r.id));
      }
      return;
    }
    if (c) save(configs.map((x) => (x.remote === r.id ? { ...x, target } : x)));
    else save([...configs, { remote: r.id, target, actions: actionsOf(r) }]);
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
          target={configOf(r.id)?.target ?? null}
          actions={actionsOf(r)}
          lastPress={props.lastPress?.remote === r.id ? props.lastPress : null}
          targetName={(tg) => targetName(t, tg, props.devices, props.groups)}
          onPickTarget={() => setPicking(r)}
          onActions={(a) => setActions(r, a)}
        />
      ))}
      {picking && (
        <TargetDialog
          remote={picking}
          current={configOf(picking.id)?.target ?? null}
          devices={props.devices}
          groups={props.groups}
          onChoose={(tg) => setTarget(picking, tg)}
          onClose={() => setPicking(null)}
        />
      )}
    </div>
  );
}

function RemoteBox(props: {
  remote: RemoteInfo;
  target: string | null;
  actions: RemoteButtonAction[];
  lastPress: LastPress | null;
  targetName: (target: string) => string;
  onPickTarget: () => void;
  onActions: (a: RemoteButtonAction[]) => void;
}) {
  const { t } = useI18n();
  const r = props.remote;
  const fresh = props.lastPress && Date.now() - props.lastPress.at < HIGHLIGHT_MS ? props.lastPress : null;
  const titleId = `remote-${r.id}`;
  const find = (button: string, event: RemoteEvent) => props.actions.find((a) => a.button === button && a.event === event);

  const choose = (button: string, event: RemoteEvent, action: RemoteAction | "") => {
    const old = find(button, event);
    if (action === "") {
      props.onActions(props.actions.filter((a) => a !== old));
      return;
    }
    const next: RemoteButtonAction = { button, event, action };
    if (action === "set") {
      next.brightness = old?.brightness ?? 100;
      if (old?.colorTemp) next.colorTemp = old.colorTemp;
    }
    props.onActions(old ? props.actions.map((a) => (a === old ? next : a)) : [...props.actions, next]);
  };
  const replace = (old: RemoteButtonAction, next: RemoteButtonAction) =>
    props.onActions(props.actions.map((a) => (a === old ? next : a)));

  // folded up by default; a press on this remote opens it
  const [open, setOpen] = useState(false);
  const pressedAt = props.lastPress?.at;
  useEffect(() => {
    if (pressedAt && Date.now() - pressedAt < HIGHLIGHT_MS) setOpen(true);
  }, [pressedAt]);
  const bodyId = `remote-body-${r.id}`;

  const isDefault =
    props.actions.length === r.defaults.length &&
    r.defaults.every((d) => props.actions.some((a) => a.button === d.button && a.event === d.event && a.action === d.action));

  return (
    <section className={`remote-box ${props.target ? "" : "unset"} ${open ? "open" : ""}`} role="group" aria-labelledby={titleId}>
      <div className="remote-head">
        <h3 id={titleId}>
          <button type="button" className="remote-toggle" aria-expanded={open} aria-controls={bodyId} onClick={() => setOpen((o) => !o)}>
            <span className="chevron" aria-hidden="true">
              ›
            </span>
            {r.name}
          </button>
        </h3>
        <span className="meta">
          {[r.vendor, r.model].filter(Boolean).join(" ")}
          {r.availability === "offline" && ` · ${t("remotes.offline")}`}
        </span>
        <span className={`remote-status ${props.target ? "" : "unset"}`}>
          {props.target ? `${t("remotes.controls")}: ${props.targetName(props.target)}` : t("remotes.notSetUp")}
        </span>
        {fresh && !open && <span className="remote-pressed-dot" aria-hidden="true" />}
      </div>

      {open && (
        <div className="remote-body" id={bodyId}>
          <div className="remote-target">
            <button type="button" className={`btn ${props.target ? "" : "primary"}`} onClick={props.onPickTarget}>
              {props.target ? `${t("remotes.controls")}: ${props.targetName(props.target)}` : t("remotes.chooseTarget")}
            </button>
            {!props.target && <span className="meta">{t("remotes.notActive")}</span>}
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
                const a = find(btn.id, ev);
                return (
                  <div key={ev} className="remote-row">
                    <span className="remote-event">{t(`remotes.events.${ev}`)}</span>
                    <select
                      aria-label={`${buttonLabel(t, btn.id)} – ${t(`remotes.events.${ev}`)}`}
                      value={a?.action ?? ""}
                      onChange={(e) => choose(btn.id, ev, e.target.value as RemoteAction | "")}
                    >
                      <option value="">{t("remotes.actions.none")}</option>
                      {(ev === "hold" ? HOLD_ACTIONS : PRESS_ACTIONS).map((x) => (
                        <option key={x} value={x}>
                          {t(`remotes.actions.${x}`)}
                        </option>
                      ))}
                    </select>
                    {a?.action === "set" && <SetFields action={a} onChange={(n) => replace(a, n)} />}
                  </div>
                );
              })}
            </div>
          ))}

          {!isDefault && r.defaults.length > 0 && (
            <div className="actions">
              <button type="button" className="btn" onClick={() => props.onActions(r.defaults)}>
                {t("remotes.resetDefaults")}
              </button>
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function SetFields({ action, onChange }: { action: RemoteButtonAction; onChange: (a: RemoteButtonAction) => void }) {
  const { t } = useI18n();
  const [pct, setPct] = useState(String(action.brightness ?? ""));
  useEffect(() => setPct(String(action.brightness ?? "")), [action.brightness]);
  const commit = () => {
    const n = pct.trim() === "" ? 0 : Number(pct);
    if (!Number.isInteger(n) || n < 0 || n > 100 || (n === 0 && !action.colorTemp)) {
      setPct(String(action.brightness ?? ""));
      return;
    }
    if (n === (action.brightness ?? 0)) return;
    const a = { ...action };
    if (n > 0) a.brightness = n;
    else delete a.brightness;
    onChange(a);
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
          value={action.colorTemp ?? 0}
          onChange={(e) => {
            const a = { ...action };
            const v = Number(e.target.value);
            if (v > 0) a.colorTemp = v;
            else delete a.colorTemp;
            if (!a.colorTemp && !a.brightness) a.brightness = 100;
            onChange(a);
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

function targetName(t: T, target: string, devices: Device[], groups: GroupRef[]): string {
  const i = target.indexOf(":");
  const kind = target.slice(0, i);
  const id = target.slice(i + 1);
  if (kind === "group") return groups.find((g) => g.id === id)?.title ?? t("remotes.missingGroup");
  return devices.find((d) => d.id === id)?.name ?? t("remotes.missingDevice");
}

function TargetDialog(props: {
  remote: RemoteInfo;
  current: string | null;
  devices: Device[];
  groups: GroupRef[];
  onChoose: (target: string | null) => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && props.onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [props]);

  const controllable = props.devices
    .filter((d) => d.kind === "light" || d.kind === "switch")
    .sort((a, b) => a.name.localeCompare(b.name));
  const name = `remote-target-${props.remote.id}`;
  const row = (value: string | null, label: string) => (
    <label key={value ?? "none"} className={`pick-row ${props.current === value ? "picked" : ""}`}>
      <input type="radio" name={name} checked={props.current === value} onChange={() => props.onChoose(value)} />
      <span>{label}</span>
    </label>
  );

  return (
    <div className="dialog-backdrop" onClick={props.onClose}>
      <div className="dialog" role="dialog" aria-modal="true" aria-labelledby="remote-target-title" onClick={(e) => e.stopPropagation()}>
        <h2 id="remote-target-title">{t("remotes.targetTitle", { name: props.remote.name })}</h2>
        <div className="pick-list">
          <div className="field-label">{t("remotes.groups")}</div>
          {props.groups.length === 0 ? (
            <p className="meta">{t("remotes.noGroups")}</p>
          ) : (
            props.groups.map((g) => row(`group:${g.id}`, g.title))
          )}
          <div className="field-label">{t("remotes.devices")}</div>
          {controllable.map((d) => row(`device:${d.id}`, d.name))}
          {props.current && (
            <>
              <div className="field-label" />
              {row(null, t("remotes.targetNone"))}
            </>
          )}
        </div>
        <div className="actions">
          <button type="button" className="btn" onClick={props.onClose}>
            {t("common.cancel")}
          </button>
        </div>
      </div>
    </div>
  );
}

/** The group cards on the home page, on every tab (for choosing targets). */
export function groupCards(
  dashboard: { tabs: { columns?: { cards: { id: string; type: string; title?: string }[] }[] }[] } | null | undefined,
): GroupRef[] {
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
