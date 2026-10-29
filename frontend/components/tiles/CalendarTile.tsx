"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ApiError, getCalendar, type CalendarEvent, type CalendarResult } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { ChevronLeftIcon, ChevronRightIcon } from "@/components/icons";

const RELOAD_MS = 10 * 60 * 1000; // backend caches 15 min anyway
const PERIOD_DAYS = 7; // matches the backend
const BACK_TO_TODAY_MS = 5 * 60 * 1000; // after browsing to another week, return to today when left alone

/** Milliseconds until just after the coming local midnight. */
function untilMidnight(): number {
  const now = new Date();
  const next = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 0, 0, 5);
  return next.getTime() - now.getTime();
}

type State =
  | { kind: "loading" }
  | { kind: "ok"; r: CalendarResult }
  | { kind: "notConfigured" }
  | { kind: "error" };

/** "HH:MM" from an ISO time, as the home's clock shows it (no browser time zone math). */
const clock = (iso: string) => iso.slice(11, 16);

function timeText(e: CalendarEvent, date: string, allDay: string): string {
  if (e.allDay) return allDay;
  const startsToday = e.start.slice(0, 10) === date;
  const endsToday = e.end.slice(0, 10) === date || e.end.slice(0, 16) === `${nextDay(date)}T00:00`;
  return `${startsToday ? clock(e.start) : "…"} – ${endsToday ? clock(e.end) : "…"}`;
}

const utcDate = (date: string) => {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d));
};

function addDays(date: string, n: number): string {
  const d = utcDate(date);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

const nextDay = (date: string) => addDays(date, 1);

/** A week of events from the calendars set up under Settings, with arrows to step weeks and a Today button. */
export function CalendarTile({ refreshKey = 0 }: { refreshKey?: number }) {
  const { t, locale } = useI18n();
  const [state, setState] = useState<State>({ kind: "loading" });
  const [from, setFrom] = useState<string | null>(null); // null = starting today
  const [busy, setBusy] = useState(false);
  const [wake, setWake] = useState(0); // bumped at midnight and when the screen comes back on

  // a new day: reload just after midnight
  useEffect(() => {
    const id = setTimeout(() => setWake((w) => w + 1), untilMidnight());
    return () => clearTimeout(id);
  }, [wake]);

  // the tablet woke up / the page was shown again: timers may have been paused, so reload
  useEffect(() => {
    const onShow = () => document.visibilityState === "visible" && setWake((w) => w + 1);
    document.addEventListener("visibilitychange", onShow);
    window.addEventListener("pageshow", onShow);
    return () => {
      document.removeEventListener("visibilitychange", onShow);
      window.removeEventListener("pageshow", onShow);
    };
  }, []);

  // browsed to another week and left it there: go back to today after a while
  useEffect(() => {
    if (from === null) return;
    const id = setTimeout(() => setFrom(null), BACK_TO_TODAY_MS);
    return () => clearTimeout(id);
  }, [from]);

  useEffect(() => {
    let alive = true;
    setBusy(true);
    const load = () =>
      getCalendar(from ?? undefined)
        .then((r) => alive && setState({ kind: "ok", r }))
        .catch((e) => {
          if (!alive) return;
          const notConfigured = e instanceof ApiError && e.code === "calendar_not_configured";
          setState((prev) => {
            if (notConfigured) return { kind: "notConfigured" };
            // a failed background reload keeps the week on screen; a failed jump to another week doesn't
            const sameWeek = prev.kind === "ok" && prev.r.from === (from ?? prev.r.today);
            return sameWeek ? prev : { kind: "error" };
          });
        })
        .finally(() => alive && setBusy(false));
    load();
    const timer = setInterval(load, RELOAD_MS);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [refreshKey, from, wake]);

  const shown = state.kind === "ok" ? state.r : null;
  const start = from ?? shown?.from ?? null;
  const step = (n: number) => start && setFrom(addDays(start, n * PERIOD_DAYS));

  const range = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
  const nav =
    state.kind === "notConfigured" ? null : (
      <div className="cal-nav">
        <button
          type="button"
          className="icon-btn"
          aria-label={t("calendar.previous")}
          disabled={!start}
          onClick={() => step(-1)}
        >
          <ChevronLeftIcon />
        </button>
        <span className="cal-range" aria-live="polite">
          {shown ? range.formatRange(utcDate(shown.from), utcDate(shown.to)) : ""}
        </span>
        <button type="button" className="icon-btn" aria-label={t("calendar.next")} disabled={!start} onClick={() => step(1)}>
          <ChevronRightIcon />
        </button>
        <button type="button" className="btn cal-today" disabled={from === null} onClick={() => setFrom(null)}>
          {t("calendar.today")}
        </button>
      </div>
    );

  if (state.kind !== "ok") {
    return (
      <div className="tile calendar-tile">
        {nav}
        <div className="calendar-empty">
          {state.kind === "loading" && <span className="meta">{t("calendar.loading")}</span>}
          {state.kind === "error" && <span className="meta">{t("calendar.unavailable")}</span>}
          {state.kind === "notConfigured" && (
            <span className="meta">
              {t("calendar.notConfigured")}{" "}
              <Link href="/settings/" className="inline-link">
                {t("common.settings")}
              </Link>
            </span>
          )}
        </div>
      </div>
    );
  }

  const weekday = new Intl.DateTimeFormat(locale, { weekday: "long", timeZone: "UTC" });
  const longDate = new Intl.DateTimeFormat(locale, { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });
  const { days, errors, today } = state.r;

  return (
    <div className={`tile calendar-tile ${busy ? "busy" : ""}`}>
      {nav}
      {days.length === 0 && <p className="meta calendar-none">{t("calendar.empty")}</p>}
      {days.map((d) => {
        const date = utcDate(d.date);
        return (
          <section key={d.date} className={`cal-day ${d.date === today ? "today" : ""}`} data-testid="calendar-day">
            <header className="cal-day-head">
              <span>{weekday.format(date)}</span>
              <span>{longDate.format(date)}</span>
            </header>
            <ul className="cal-events">
              {d.events.map((e, i) => (
                <li key={`${e.calendarId}-${e.start}-${i}`} className="cal-event">
                  <span className="cal-time">{timeText(e, d.date, t("calendar.allDay"))}</span>
                  <span className="cal-dot" style={{ background: e.color }} aria-hidden="true" />
                  <span className="cal-title">{e.title}</span>
                </li>
              ))}
            </ul>
          </section>
        );
      })}
      {errors && errors.length > 0 && <p className="meta calendar-errors">{t("calendar.someFailed")}</p>}
    </div>
  );
}
