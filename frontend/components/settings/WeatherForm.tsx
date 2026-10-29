"use client";

import { FormEvent, useState } from "react";
import type { Place, WeatherSettings } from "@/lib/api";
import { errorText, useI18n } from "@/lib/i18n";
import { PROVIDER_NAMES } from "@/components/tiles/WeatherTile";
import type { FoundPosition } from "@/lib/position";

interface Chosen {
  name: string;
  latitude: number;
  longitude: number;
  area?: string;
  approximate?: boolean;
}

type Saved = Pick<WeatherSettings, "provider" | "latitude" | "longitude" | "location">;

const placeLabel = (p: Place) => [p.name, p.region, p.country].filter(Boolean).join(", ");

/** Choose a free weather provider and search for the place to show. */
export function WeatherForm(props: {
  initial: WeatherSettings;
  /** may return the saved settings (with the looked-up area) */
  onSave: (w: Saved) => Promise<WeatherSettings | void>;
  search: (q: string) => Promise<Place[]>;
  /** "Use my position" */
  locate?: () => Promise<FoundPosition>;
}) {
  const { t } = useI18n();
  const providers = props.initial.providers.length ? props.initial.providers : ["smhi", "metno", "openmeteo"];
  const [provider, setProvider] = useState(props.initial.provider || providers[0]);
  const [place, setPlace] = useState<Chosen | null>(
    props.initial.configured
      ? {
          name: props.initial.location,
          latitude: props.initial.latitude,
          longitude: props.initial.longitude,
          area: props.initial.area || undefined,
        }
      : null,
  );
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Place[] | null>(null);
  const [busy, setBusy] = useState<"search" | "save" | "locate" | null>(null);
  const [msg, setMsg] = useState<{ kind: "ok" | "bad"; text: string } | null>(null);

  const doSearch = async (e?: FormEvent) => {
    e?.preventDefault();
    if (query.trim().length < 2) return;
    setBusy("search");
    setMsg(null);
    try {
      setResults(await props.search(query.trim()));
    } catch (err) {
      setResults(null);
      setMsg({ kind: "bad", text: t("settings.weatherSearchFailed", { error: errorText(t, err) }) });
    } finally {
      setBusy(null);
    }
  };

  const useMyPosition = async () => {
    if (!props.locate) return;
    setBusy("locate");
    setMsg(null);
    setResults(null);
    try {
      const p = await props.locate();
      setPlace({ name: p.name, latitude: p.latitude, longitude: p.longitude, area: p.area, approximate: p.approximate });
    } catch (err) {
      setMsg({ kind: "bad", text: t("settings.weatherLocateFailed", { error: errorText(t, err) }) });
    } finally {
      setBusy(null);
    }
  };

  const save = async () => {
    if (!place) return;
    setBusy("save");
    setMsg(null);
    try {
      const saved = await props.onSave({ provider, latitude: place.latitude, longitude: place.longitude, location: place.name });
      if (saved && saved.area) setPlace((p) => (p ? { ...p, area: saved.area } : p));
      setMsg({ kind: "ok", text: t("settings.weatherSaved") });
    } catch (err) {
      setMsg({ kind: "bad", text: t("settings.weatherSaveFailed", { error: errorText(t, err) }) });
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="weather-form">
      <div className="field-label">{t("settings.weatherProvider")}</div>
      <div className="provider-list" role="radiogroup" aria-label={t("settings.weatherProvider")}>
        {providers.map((id) => (
          <button
            key={id}
            type="button"
            role="radio"
            aria-checked={provider === id}
            className={`provider-option ${provider === id ? "selected" : ""}`}
            onClick={() => {
              setProvider(id);
              setMsg(null);
            }}
          >
            <span className="provider-name">{PROVIDER_NAMES[id] ?? id}</span>
            <span className="meta">{t(`settings.weatherProviderHint.${id}`)}</span>
          </button>
        ))}
      </div>

      <form className="place-search" onSubmit={doSearch}>
        <label className="field">
          <span className="field-label">{t("settings.weatherPlace")}</span>
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("settings.weatherPlacePlaceholder")}
            autoComplete="off"
          />
        </label>
        <button type="submit" className="btn" disabled={busy !== null || query.trim().length < 2}>
          {t("settings.weatherSearch")}
        </button>
      </form>
      {props.locate && (
        <div>
          <button type="button" className="btn" disabled={busy !== null} onClick={useMyPosition}>
            {busy === "locate" ? t("settings.weatherLocating") : t("settings.weatherUseMyPosition")}
          </button>
        </div>
      )}

      {results && results.length === 0 && <p className="meta">{t("settings.weatherNoPlaces")}</p>}
      {results && results.length > 0 && (
        <div className="pick-list">
          {results.map((p) => (
            <button
              key={`${p.latitude},${p.longitude}`}
              type="button"
              className="pick-row place-row"
              onClick={() => {
                setPlace({ name: p.name, latitude: p.latitude, longitude: p.longitude }); // area: looked up on save
                setResults(null);
                setMsg(null);
              }}
            >
              {placeLabel(p)}
            </button>
          ))}
        </div>
      )}

      <p className={place ? "" : "meta"}>
        {place
          ? t("settings.weatherSelected", {
              name: place.name,
              coords: `${place.latitude.toFixed(4)}, ${place.longitude.toFixed(4)}`,
            })
          : t("settings.weatherChoosePlace")}
      </p>
      {place?.area && <p className="meta">{t("settings.weatherArea", { area: place.area })}</p>}
      {place?.approximate && <div className="banner warn">{t("settings.weatherApproximate")}</div>}

      {msg && <div className={`banner ${msg.kind}`}>{msg.text}</div>}
      <div className="actions">
        <button type="button" className="btn primary" disabled={!place || busy !== null} onClick={save}>
          {busy === "save" ? t("common.saving") : t("common.save")}
        </button>
      </div>
    </div>
  );
}
