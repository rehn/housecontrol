"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ApiError, getWeather, type WeatherForecast } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { WeatherIcon } from "./WeatherIcon";
import { SunriseIcon, SunsetIcon } from "@/components/icons";

/** Display names of the providers (proper nouns, not translated). */
export const PROVIDER_NAMES: Record<string, string> = {
  smhi: "SMHI",
  metno: "MET Norway (Yr)",
  openmeteo: "Open-Meteo",
};

const RELOAD_MS = 10 * 60 * 1000; // backend caches 15 min anyway

/** "HH:MM" as the home's clock shows it (the server sends its own time zone). */
const clock = (iso: string) => iso.slice(11, 16);

type State =
  | { kind: "loading" }
  | { kind: "ok"; f: WeatherForecast }
  | { kind: "notConfigured" }
  | { kind: "error" };

/** Current weather + 5 days, from the provider chosen under Settings. */
export function WeatherTile({ refreshKey = 0 }: { refreshKey?: number }) {
  const { t, locale } = useI18n();
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    let alive = true;
    const load = () =>
      getWeather()
        .then((f) => alive && setState({ kind: "ok", f }))
        .catch((e) => {
          if (!alive) return;
          const notConfigured = e instanceof ApiError && e.code === "weather_not_configured";
          setState((prev) =>
            notConfigured ? { kind: "notConfigured" } : prev.kind === "ok" ? prev : { kind: "error" },
          );
        });
    load();
    const timer = setInterval(load, RELOAD_MS);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [refreshKey]);

  if (state.kind !== "ok") {
    return (
      <div className="tile weather-tile weather-empty">
        {state.kind === "loading" && <span className="meta">{t("weather.loading")}</span>}
        {state.kind === "error" && <span className="meta">{t("weather.unavailable")}</span>}
        {state.kind === "notConfigured" && (
          <span className="meta">
            {t("weather.notConfigured")}{" "}
            <Link href="/settings/" className="inline-link">
              {t("common.settings")}
            </Link>
          </span>
        )}
      </div>
    );
  }

  const { f } = state;
  const num = new Intl.NumberFormat(locale, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  const weekday = new Intl.DateTimeFormat(locale, { weekday: "short" });
  const today = f.daily[0];
  const source = [f.area || f.location, PROVIDER_NAMES[f.provider] ?? f.provider].filter(Boolean).join(" · ");

  return (
    <div className="tile weather-tile">
      <div className="weather-now">
        <WeatherIcon condition={f.current.condition} size={64} />
        <div className="weather-now-text">
          <span className="weather-condition">{t(`weather.conditions.${f.current.condition}`)}</span>
          <span className="meta">{source}</span>
        </div>
        <div className="weather-now-temp">
          <span className="weather-temp">{num.format(f.current.temperature)} °C</span>
          {today && (
            <span className="meta">
              {num.format(today.max)} °C / {num.format(today.min)} °C
            </span>
          )}
        </div>
      </div>

      {f.sun && (f.sun.polar || (f.sun.sunrise && f.sun.sunset)) && (
        <div className="weather-sun">
          {f.sun.polar ? (
            <span className="meta">{t(f.sun.polar === "day" ? "weather.polarDay" : "weather.polarNight")}</span>
          ) : (
            <>
              <span className="sun-time" aria-label={`${t("weather.sunrise")} ${clock(f.sun.sunrise!)}`}>
                <SunriseIcon />
                <span aria-hidden="true">{clock(f.sun.sunrise!)}</span>
              </span>
              <span className="sun-time" aria-label={`${t("weather.sunset")} ${clock(f.sun.sunset!)}`}>
                <SunsetIcon />
                <span aria-hidden="true">{clock(f.sun.sunset!)}</span>
              </span>
            </>
          )}
        </div>
      )}

      <div className="weather-days">
        {f.daily.map((d) => {
          const [y, m, day] = d.date.split("-").map(Number);
          return (
            <div key={d.date} className="weather-day" data-testid="weather-day">
              <span className="meta">{weekday.format(new Date(y, m - 1, day))}</span>
              <WeatherIcon condition={d.condition} size={36} />
              <span className="weather-max">{num.format(d.max)}°</span>
              <span className="meta">{num.format(d.min)}°</span>
            </div>
          );
        })}
      </div>
      {f.stale && <span className="meta weather-stale">{t("weather.stale")}</span>}
    </div>
  );
}
