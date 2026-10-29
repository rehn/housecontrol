// Schedules: turn lights on/off at a time or relative to sunrise/sunset.
// Mirrors backend/internal/schedule. Plus pure helpers that describe them.
import type { T } from "./i18n";

export type TriggerKind = "time" | "sunrise" | "sunset";

export interface Trigger {
  kind: TriggerKind;
  time?: string; // HH:MM (kind time)
  offset?: number; // minutes, negative = before (sun kinds)
  notBefore?: string; // HH:MM (sun kinds)
  notAfter?: string; // HH:MM (sun kinds)
}

export interface Schedule {
  id: string;
  name: string;
  enabled: boolean;
  deviceIds: string[];
  days?: number[]; // 0 = Sunday … 6 = Saturday; empty = every day
  on?: Trigger;
  off?: Trigger;
  brightness?: number; // % when turning on; 0/absent = unchanged
}

export interface NextRun {
  action: "on" | "off";
  at: string; // RFC 3339 in the home's time zone
}

export interface SchedulePlace {
  known: boolean;
  name?: string;
  sunrise?: string;
  sunset?: string;
  polar?: "day" | "night";
}

export interface SchedulesView {
  schedules: Schedule[];
  place: SchedulePlace;
  next: Record<string, NextRun>;
}

/** Monday first, as shown in the day picker. */
export const WEEK = [1, 2, 3, 4, 5, 6, 0];

/** Short weekday name, e.g. "mån" / "Mon" (0 = Sunday). */
export function weekdayName(day: number, locale: string): string {
  // 4 Jan 2026 is a Sunday
  return new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" }).format(new Date(Date.UTC(2026, 0, 4 + day)));
}

export function describeTrigger(tr: Trigger, t: T): string {
  if (tr.kind === "time") return tr.time ?? "";
  const sun = t(tr.kind === "sunrise" ? "schedules.sunriseName" : "schedules.sunsetName");
  const offset = tr.offset ?? 0;
  let text =
    offset === 0
      ? t("schedules.atSun", { sun })
      : t(offset < 0 ? "schedules.beforeSun" : "schedules.afterSun", { min: Math.abs(offset), sun });
  if (tr.notBefore) text += `, ${t("schedules.notBeforeShort", { time: tr.notBefore })}`;
  if (tr.notAfter) text += `, ${t("schedules.notAfterShort", { time: tr.notAfter })}`;
  return text;
}

export function describeDays(days: number[] | undefined, t: T, locale: string): string {
  const set = [...new Set(days ?? [])].sort();
  if (set.length === 0 || set.length === 7) return t("schedules.everyDay");
  if (set.join() === "1,2,3,4,5") return t("schedules.weekdays");
  if (set.join() === "0,6") return t("schedules.weekends");
  return WEEK.filter((d) => set.includes(d))
    .map((d) => weekdayName(d, locale))
    .join(", ");
}

const localDate = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

/** "turns on today 16:12" / "tänds imorgon 07:00" / "tänds mån 06:30". */
export function describeNext(next: NextRun, now: Date, t: T, locale: string): string {
  const date = next.at.slice(0, 10);
  const time = next.at.slice(11, 16); // the home's clock, as sent by the server
  const tomorrow = new Date(now);
  tomorrow.setDate(now.getDate() + 1);
  const [y, m, d] = date.split("-").map(Number);
  const when =
    date === localDate(now)
      ? t("schedules.today")
      : date === localDate(tomorrow)
        ? t("schedules.tomorrow")
        : weekdayName(new Date(Date.UTC(y, m - 1, d)).getUTCDay(), locale);
  return t(next.action === "on" ? "schedules.nextOn" : "schedules.nextOff", { when, time });
}
