"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";

/** Large clock + date in the app language (e.g. "onsdag 28 oktober 2026"). */
export function ClockTile() {
  const { locale } = useI18n();
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
    </div>
  );
}
