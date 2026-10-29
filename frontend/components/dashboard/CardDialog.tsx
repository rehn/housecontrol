"use client";

import { useEffect, useState } from "react";
import type { Device } from "@/lib/api";
import { Card, CardType, newId } from "@/lib/dashboard";
import { CalendarIcon, ClockIcon, GroupIcon, BulbIcon } from "@/components/icons";
import { WeatherIcon } from "@/components/tiles/WeatherIcon";
import { useI18n } from "@/lib/i18n";

// label/hint are keys in locales/*.json (cardDialog.*)
const TYPES: { type: CardType; label: string; hint: string; icon: React.ReactNode }[] = [
  { type: "device", label: "typeDevice", hint: "typeDeviceHint", icon: <BulbIcon size={28} /> },
  { type: "group", label: "typeGroup", hint: "typeGroupHint", icon: <GroupIcon size={28} /> },
  { type: "clock", label: "typeClock", hint: "typeClockHint", icon: <ClockIcon size={28} /> },
  { type: "weather", label: "typeWeather", hint: "typeWeatherHint", icon: <WeatherIcon condition="partly_cloudy" size={28} /> },
  { type: "calendar", label: "typeCalendar", hint: "typeCalendarHint", icon: <CalendarIcon size={28} /> },
];

// card types without settings are added right away
const instant = (type: CardType) => type === "clock" || type === "weather" || type === "calendar";

/** Add a new card (pick type, then details) or edit an existing one. */
export function CardDialog(props: {
  devices: Device[];
  usedDeviceIds: Set<string>;
  initial?: Card;
  onSubmit: (card: Card) => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const editing = !!props.initial;
  const [type, setType] = useState<CardType | null>(props.initial?.type ?? null);
  const [title, setTitle] = useState(props.initial?.title ?? "");
  const [deviceId, setDeviceId] = useState(props.initial?.deviceId ?? "");
  const [deviceIds, setDeviceIds] = useState<string[]>(props.initial?.deviceIds ?? []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && props.onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [props]);

  const controllable = props.devices
    .filter((d) => d.kind === "light" || d.kind === "switch")
    .sort((a, b) => a.name.localeCompare(b.name));

  const submit = (t: CardType) => {
    const id = props.initial?.id ?? newId();
    const trimmed = title.trim();
    if (instant(t)) props.onSubmit({ id, type: t });
    if (t === "device") props.onSubmit({ id, type: "device", deviceId, ...(trimmed ? { title: trimmed } : {}) });
    if (t === "group") props.onSubmit({ id, type: "group", title: trimmed, deviceIds });
  };

  const valid =
    type === "device" ? deviceId !== "" : type === "group" ? title.trim() !== "" && deviceIds.length > 0 : true;

  const toggleMember = (id: string) =>
    setDeviceIds((ids) => (ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]));

  return (
    <div className="dialog-backdrop" onClick={props.onClose}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="card-dialog-title"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="card-dialog-title">
          {editing
            ? t("cardDialog.titleEdit")
            : type === "device"
              ? t("cardDialog.titleAddDevice")
              : type === "group"
                ? t("cardDialog.titleAddGroup")
                : t("cardDialog.titleAdd")}
        </h2>

        {!type && (
          <div className="type-picker">
            {TYPES.map((o) => (
              <button
                key={o.type}
                type="button"
                className="type-option"
                onClick={() => (instant(o.type) ? submit(o.type) : setType(o.type))}
              >
                <span className="type-icon">{o.icon}</span>
                <span className="type-label">{t(`cardDialog.${o.label}`)}</span>
                <span className="type-hint">{t(`cardDialog.${o.hint}`)}</span>
              </button>
            ))}
          </div>
        )}

        {type === "group" && (
          <>
            <label className="field">
              <span className="field-label">{t("cardDialog.name")}</span>
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t("cardDialog.groupNamePlaceholder")}
                maxLength={60}
              />
            </label>
            <div className="field-label">{t("cardDialog.devicesSelected", { count: deviceIds.length })}</div>
            <div className="pick-list">
              {controllable.map((d) => (
                <label key={d.id} className={`pick-row ${deviceIds.includes(d.id) ? "picked" : ""}`}>
                  <input type="checkbox" checked={deviceIds.includes(d.id)} onChange={() => toggleMember(d.id)} />
                  <span>{d.name}</span>
                </label>
              ))}
              {controllable.length === 0 && <p className="meta">{t("cardDialog.noControllable")}</p>}
            </div>
          </>
        )}

        {type === "device" && (
          <>
            <div className="field-label">{t("cardDialog.device")}</div>
            <div className="pick-list">
              {controllable.map((d) => (
                <label key={d.id} className={`pick-row ${deviceId === d.id ? "picked" : ""}`}>
                  <input
                    type="radio"
                    name="device"
                    checked={deviceId === d.id}
                    onChange={() => setDeviceId(d.id)}
                  />
                  <span>{d.name}</span>
                  {props.usedDeviceIds.has(d.id) && d.id !== props.initial?.deviceId && (
                    <span className="pick-badge" aria-hidden="true">
                      {t("cardDialog.onDashboard")}
                    </span>
                  )}
                </label>
              ))}
              {controllable.length === 0 && <p className="meta">{t("cardDialog.noControllable")}</p>}
            </div>
            <label className="field">
              <span className="field-label">
                {t("cardDialog.name")} <span className="meta">{t("cardDialog.nameOptional")}</span>
              </span>
              <input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={60} />
            </label>
          </>
        )}

        <div className="actions">
          <button type="button" className="btn" onClick={props.onClose}>
            {t("common.cancel")}
          </button>
          {type && !instant(type) && (
            <button type="button" className="btn primary" disabled={!valid} onClick={() => submit(type)}>
              {editing ? t("common.done") : t("common.add")}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
