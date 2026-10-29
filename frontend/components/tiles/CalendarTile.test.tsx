import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CalendarResult } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { CalendarTile } from "./CalendarTile";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const result: CalendarResult = {
  from: "2026-10-28",
  to: "2026-11-03",
  today: "2026-10-28",
  days: [
    {
      date: "2026-10-28",
      events: [
        { title: "Höstlov", start: "2026-10-26T00:00:00+01:00", end: "2026-10-31T00:00:00+01:00", allDay: true, calendarId: "c1", color: "#2e9d58" },
        { title: "Träning - Enköpings IBK F 12-14", start: "2026-10-28T17:00:00+01:00", end: "2026-10-28T18:30:00+01:00", allDay: false, calendarId: "c2", color: "#f5c242" },
      ],
    },
    {
      date: "2026-10-29",
      events: [
        { title: "Träning - Enköpings HF P16", start: "2026-10-29T17:15:00+01:00", end: "2026-10-29T19:30:00+01:00", allDay: false, calendarId: "c3", color: "#8a63d2" },
      ],
    },
  ],
};

function stubFetch(status: number, body: unknown) {
  const fetch = vi.fn(() => Promise.resolve(new Response(JSON.stringify(body), { status })));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

describe("CalendarTile", () => {
  it("lists events per day with times and colour dots", async () => {
    stubFetch(200, result);
    render(<CalendarTile />);
    const days = await screen.findAllByTestId("calendar-day");
    expect(days).toHaveLength(2);

    expect(within(days[0]).getByText("Wednesday")).toBeTruthy();
    expect(within(days[0]).getByText("28 October 2026")).toBeTruthy();
    expect(within(days[0]).getByText("All day")).toBeTruthy();
    expect(within(days[0]).getByText("17:00 – 18:30")).toBeTruthy();
    expect(within(days[0]).getByText("Träning - Enköpings IBK F 12-14")).toBeTruthy();

    const dot = days[1].querySelector(".cal-dot") as HTMLElement;
    expect(dot.style.background).toBe("rgb(138, 99, 210)");
  });

  it("uses the chosen language", async () => {
    stubFetch(200, result);
    render(
      <I18nProvider language="sv">
        <CalendarTile />
      </I18nProvider>,
    );
    const days = await screen.findAllByTestId("calendar-day");
    expect(within(days[0]).getByText("onsdag")).toBeTruthy();
    expect(within(days[0]).getByText("28 oktober 2026")).toBeTruthy();
    expect(within(days[0]).getByText("Heldag")).toBeTruthy();
  });

  it("says when there is nothing coming up", async () => {
    stubFetch(200, { ...result, days: [] });
    render(<CalendarTile />);
    expect(await screen.findByText("No events in this period.")).toBeTruthy();
  });

  it("links to Settings when no calendars are set up", async () => {
    stubFetch(404, { error: "no calendars are set up", code: "calendar_not_configured" });
    render(<CalendarTile />);
    expect(await screen.findByText(/Add calendars in/)).toBeTruthy();
    expect(screen.getByRole("link").getAttribute("href")).toBe("/settings/");
  });

  it("notes calendars that could not be loaded", async () => {
    stubFetch(200, { ...result, errors: [{ calendarId: "c9", error: "calendar server answered 404" }] });
    render(<CalendarTile />);
    expect(await screen.findByText(/Some calendars could not be loaded/)).toBeTruthy();
  });

  it("reloads when the calendar settings change (refreshKey)", async () => {
    const fetch = stubFetch(200, result);
    const { rerender } = render(<CalendarTile refreshKey={0} />);
    await screen.findAllByTestId("calendar-day");
    rerender(<CalendarTile refreshKey={1} />);
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  });
});

/** Answers like the server: a week from ?from= (default today = 2026-10-28). */
function stubWeeks() {
  const fetch = vi.fn((url: string) => {
    const from = new URL(url, "http://x").searchParams.get("from") ?? "2026-10-28";
    const [y, m, d] = from.split("-").map(Number);
    const to = new Date(Date.UTC(y, m - 1, d + 6)).toISOString().slice(0, 10);
    const body: CalendarResult = { from, to, today: "2026-10-28", days: from === "2026-10-28" ? result.days : [] };
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }));
  });
  vi.stubGlobal("fetch", fetch);
  return fetch;
}
/** Matches the period label whatever spaces/dots this Node's ICU puts in ("28 Oct – 3 Nov 2026"). */
const squash = (x: string) => x.replace(/[\s.]/g, "");
const period = (label: string) => (_: string, el: Element | null) =>
  !!el?.classList.contains("cal-range") && squash(el.textContent ?? "") === squash(label);
const lastUrl = (fetch: ReturnType<typeof vi.fn>) => String(fetch.mock.calls[fetch.mock.calls.length - 1][0]);

describe("CalendarTile period", () => {
  it("shows which days are shown", async () => {
    stubWeeks();
    render(<CalendarTile />);
    expect(await screen.findByText(period("28 Oct – 3 Nov 2026"))).toBeTruthy();
    expect((screen.getByRole("button", { name: "Today" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("steps a period forward and back, and jumps back to today", async () => {
    const fetch = stubWeeks();
    render(<CalendarTile />);
    await screen.findByText(period("28 Oct – 3 Nov 2026"));

    fireEvent.click(screen.getByRole("button", { name: "Next period" }));
    expect(await screen.findByText(period("4 – 10 Nov 2026"))).toBeTruthy();
    expect(lastUrl(fetch)).toContain("/api/calendar?from=2026-11-04");
    expect(screen.getByText("No events in this period.")).toBeTruthy();
    const today = screen.getByRole("button", { name: "Today" }) as HTMLButtonElement;
    expect(today.disabled).toBe(false);

    fireEvent.click(screen.getByRole("button", { name: "Previous period" }));
    fireEvent.click(screen.getByRole("button", { name: "Previous period" }));
    expect(await screen.findByText(period("21 – 27 Oct 2026"))).toBeTruthy();
    expect(lastUrl(fetch)).toContain("from=2026-10-21");

    fireEvent.click(today);
    expect(await screen.findByText(period("28 Oct – 3 Nov 2026"))).toBeTruthy();
    expect(lastUrl(fetch)).not.toContain("from=");
    expect(await screen.findAllByTestId("calendar-day")).toHaveLength(2);
  });

  it("marks today in the list", async () => {
    stubWeeks();
    render(<CalendarTile />);
    const days = await screen.findAllByTestId("calendar-day");
    expect(days[0].classList.contains("today")).toBe(true);
    expect(days[1].classList.contains("today")).toBe(false);
  });

  it("has Swedish buttons", async () => {
    stubWeeks();
    render(
      <I18nProvider language="sv">
        <CalendarTile />
      </I18nProvider>,
    );
    expect(await screen.findByText(period("28 okt.–3 nov. 2026"))).toBeTruthy();
    expect(screen.getByRole("button", { name: "Idag" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Nästa period" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Föregående period" })).toBeTruthy();
  });
});

describe("CalendarTile keeps up with the day", () => {
  const localDay = (d: Date) =>
    `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

  /** Like the server: today comes from the (fake) clock. */
  function stubClock() {
    const fetch = vi.fn((url: string) => {
      const today = localDay(new Date());
      const from = new URL(url, "http://x").searchParams.get("from") ?? today;
      const [y, m, d] = from.split("-").map(Number);
      const to = new Date(Date.UTC(y, m - 1, d + 6)).toISOString().slice(0, 10);
      const body: CalendarResult = { from, to, today, days: [{ date: from, events: [] }] };
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) } as Response);
    });
    vi.stubGlobal("fetch", fetch);
    return fetch;
  }
  const flush = () => act(() => vi.advanceTimersByTimeAsync(0));
  const wait = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
  const shown = () => document.querySelector(".cal-range")?.textContent?.replace(/[\s.]/g, "");

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval", "Date"] });
    vi.setSystemTime(new Date(2026, 9, 28, 23, 58)); // 28 Oct, two minutes to midnight
  });
  afterEach(() => vi.useRealTimers());

  it("moves on to the new day just after midnight", async () => {
    stubClock();
    render(<CalendarTile />);
    await flush();
    expect(shown()).toBe("28Oct–3Nov2026");
    await wait(3 * 60 * 1000); // well before the 10-minute reload
    expect(shown()).toBe("29Oct–4Nov2026");
    expect(screen.getByTestId("calendar-day").classList.contains("today")).toBe(true);
  });

  it("reloads when the tablet wakes up", async () => {
    const fetch = stubClock();
    render(<CalendarTile />);
    await flush();
    const before = fetch.mock.calls.length;

    Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
    await flush();
    expect(fetch.mock.calls.length).toBe(before); // nothing while the screen is off

    Object.defineProperty(document, "visibilityState", { value: "visible", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
    await flush();
    expect(fetch.mock.calls.length).toBe(before + 1);
  });

  it("goes back to today a while after someone browsed to another week", async () => {
    vi.setSystemTime(new Date(2026, 9, 28, 12, 0));
    const fetch = stubClock();
    render(<CalendarTile />);
    await flush();

    fireEvent.click(screen.getByRole("button", { name: "Next period" }));
    await flush();
    expect(shown()).toBe("4–10Nov2026");
    await wait(4 * 60 * 1000);
    fireEvent.click(screen.getByRole("button", { name: "Next period" })); // still browsing: the wait starts over
    await flush();
    await wait(4 * 60 * 1000);
    expect(shown()).toBe("11–17Nov2026");

    await wait(60 * 1000 + 100);
    expect(shown()).toBe("28Oct–3Nov2026");
    expect(String(fetch.mock.calls[fetch.mock.calls.length - 1][0])).not.toContain("from=");
  });
});
