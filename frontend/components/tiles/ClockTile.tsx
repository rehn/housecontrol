"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";

/** ISO 8601 week number (weeks start on Monday; week 1 contains the first Thursday). */
export function isoWeek(d: Date): number {
  const date = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()));
  const day = date.getUTCDay() || 7;
  date.setUTCDate(date.getUTCDate() + 4 - day); // Thursday of this week
  const yearStart = Date.UTC(date.getUTCFullYear(), 0, 1);
  return Math.ceil(((date.getTime() - yearStart) / 86_400_000 + 1) / 7);
}

/** Large clock + date in the app language (e.g. "onsdag 28 oktober 2026") and week number. */
export function ClockTile() {
  const { t, locale } = useI18n();
  // Render only after mount: the page is pre-rendered at build time, and the
  // build machine's clock must not end up in the HTML.
  const [now, setNow] = useState<Date | null>(null);

  useEffect(() => {
    setNow(new Date());
    const t = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(t);
  }, []);

  const time = now
    ? new Intl.DateTimeFormat(locale, { hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(now)
    : "--:--";
  const seconds = now ? String(now.getSeconds()).padStart(2, "0") : "";
  const date = now
    ? new Intl.DateTimeFormat(locale, { weekday: "long", day: "numeric", month: "long", year: "numeric" }).format(now)
    : "";

  return (
    <div className="tile clock-tile">
      <div className="clock-time">
        <span data-testid="clock-time">{time}</span>
        <sup className="clock-seconds">{seconds}</sup>
      </div>
      <div className="clock-date">{date}</div>
      <div className="clock-date" data-testid="clock-week">
        {now ? t("tile.week", { week: isoWeek(now) }) : ""}
      </div>
    </div>
  );
}
